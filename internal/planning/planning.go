package planning

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/setupproof/setupproof/internal/app"
	"github.com/setupproof/setupproof/internal/config"
	"github.com/setupproof/setupproof/internal/duration"
	"github.com/setupproof/setupproof/internal/markdown"
	"github.com/setupproof/setupproof/internal/project"
)

const PlanSchemaVersion = "1.0.0"
const DefaultDockerImage = "ubuntu:24.04"

var ErrNoTarget = errors.New("no target files provided and README.md was not found; pass a Markdown file or add files to setupproof.yml")

type Request struct {
	Args []string
	CWD  string

	Positional []string
	ConfigPath string

	Runner    string
	HasRunner bool

	Timeout    string
	HasTimeout bool

	Network    bool
	HasNetwork bool

	RequireBlocks    bool
	HasRequireBlocks bool

	IncludeUntracked bool
}

type Result struct {
	Plan     Plan
	ExitCode int
}

type Plan struct {
	Kind              string     `json:"kind"`
	SchemaVersion     string     `json:"schemaVersion"`
	SetupproofVersion string     `json:"setupproofVersion"`
	Invocation        Invocation `json:"invocation"`
	Files             []string   `json:"files"`
	Defaults          Options    `json:"defaults"`
	Workspace         Workspace  `json:"workspace"`
	Runner            Runner     `json:"runner"`
	Warnings          []string   `json:"warnings"`
	ValidationErrors  []string   `json:"validationErrors,omitempty"`
	Blocks            []Block    `json:"blocks"`
	Env               Env        `json:"env"`
}

type Invocation struct {
	Args          []string `json:"args"`
	ConfigPath    string   `json:"configPath,omitempty"`
	DryRun        bool     `json:"dryRun"`
	RequireBlocks bool     `json:"requireBlocks"`
}

type Workspace struct {
	Mode              string `json:"mode"`
	Source            string `json:"source"`
	IncludedUntracked bool   `json:"includedUntracked"`
}

type Runner struct {
	Kind            string       `json:"kind"`
	Workspace       string       `json:"workspace"`
	NetworkPolicy   string       `json:"networkPolicy"`
	NetworkEnforced bool         `json:"networkEnforced"`
	Error           *RunnerError `json:"error,omitempty"`
}

type RunnerError struct {
	Reason string `json:"reason"`
}

type Options struct {
	Runner          string `json:"runner"`
	DockerImage     string `json:"dockerImage,omitempty"`
	Timeout         string `json:"timeout"`
	TimeoutMs       int64  `json:"timeoutMs"`
	Strict          bool   `json:"strict"`
	Isolated        bool   `json:"isolated"`
	StateMode       string `json:"stateMode"`
	Stdin           string `json:"stdin"`
	TTY             bool   `json:"tty"`
	NetworkPolicy   string `json:"networkPolicy"`
	NetworkEnforced bool   `json:"networkEnforced"`
}

type Block struct {
	ID          string            `json:"id"`
	ExplicitID  bool              `json:"explicitId"`
	QualifiedID string            `json:"qualifiedId"`
	File        string            `json:"file"`
	Line        int               `json:"line"`
	MarkerLine  int               `json:"markerLine"`
	Language    string            `json:"language"`
	Shell       string            `json:"shell"`
	MarkerForm  string            `json:"markerForm"`
	Metadata    map[string]string `json:"metadata"`
	Source      string            `json:"source"`
	Options     Options           `json:"options"`
}

type Env struct {
	Allow []string  `json:"allow"`
	Pass  []EnvPass `json:"pass"`
}

type EnvPass struct {
	Name     string `json:"name"`
	Secret   bool   `json:"secret"`
	Required bool   `json:"required"`
	Present  bool   `json:"present"`
}

type TargetFile struct {
	Rel string
	Abs string
}

type optionState struct {
	Runner      string
	DockerImage string
	Timeout     string
	Strict      bool
	Isolated    bool
	Network     *bool
}

type blockConfigEntry struct {
	Key  string
	File string
	ID   string
}

func Build(req Request) (Result, error) {
	resolver, err := project.NewResolver(req.CWD)
	if err != nil {
		return Result{}, err
	}

	cfg, configRel, err := loadConfig(req, resolver)
	if err != nil {
		return Result{}, err
	}

	if cfg != nil {
		if err := validateConfigReferences(*cfg, resolver); err != nil {
			return Result{}, err
		}
	}

	targets, err := selectTargets(req, resolver, cfg)
	if err != nil {
		return Result{}, err
	}

	blockConfigs, blockConfigEntries, err := indexedBlockConfigs(cfg, resolver)
	if err != nil {
		return Result{}, err
	}

	defaultState, requireBlocks, err := defaultOptions(req, cfg)
	if err != nil {
		return Result{}, err
	}

	defaultPlanOptions, err := materializeOptions(defaultState)
	if err != nil {
		return Result{}, err
	}

	plan := Plan{
		Kind:              "plan",
		SchemaVersion:     PlanSchemaVersion,
		SetupproofVersion: app.Version,
		Invocation: Invocation{
			Args:          append([]string(nil), req.Args...),
			ConfigPath:    configRel,
			DryRun:        true,
			RequireBlocks: requireBlocks,
		},
		Files:    make([]string, 0, len(targets)),
		Defaults: defaultPlanOptions,
		Workspace: Workspace{
			Mode:              "temporary",
			Source:            "tracked-plus-modified",
			IncludedUntracked: req.IncludeUntracked,
		},
		Runner:   runnerSummary(defaultPlanOptions),
		Warnings: []string{},
		Blocks:   []Block{},
		Env:      planEnv(cfg),
	}

	targetFiles := targetFileSet(targets)
	usedBlockConfigs := make(map[string]bool)
	explicitIDs := make(map[string]map[string]bool)
	for _, target := range targets {
		plan.Files = append(plan.Files, target.Rel)
		contents, err := os.ReadFile(target.Abs)
		if err != nil {
			return Result{}, err
		}
		discovered := markdown.Discover(target.Rel, contents)
		for _, discoveredBlock := range discovered {
			for _, warning := range discoveredBlock.Warnings {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("%s:%d: %s", target.Rel, discoveredBlock.MarkerLine, warning))
			}
			explicitID := discoveredBlock.Metadata["id"]
			if explicitID != "" {
				if explicitIDs[target.Rel] == nil {
					explicitIDs[target.Rel] = make(map[string]bool)
				}
				if explicitIDs[target.Rel][explicitID] {
					return Result{}, fmt.Errorf("duplicate explicit block id %q in %s", explicitID, target.Rel)
				}
				explicitIDs[target.Rel][explicitID] = true
			}

			blockID := explicitID
			explicit := true
			if blockID == "" {
				blockID = implicitID(discoveredBlock.Line)
				explicit = false
			}

			state := defaultState
			if explicit {
				configKey := target.Rel + "#" + blockID
				if blockConfig, ok := blockConfigs[configKey]; ok {
					state = applyBlockConfig(state, blockConfig)
					usedBlockConfigs[configKey] = true
				}
			}
			state, optionErrors, optionWarnings := applyInlineMetadata(state, discoveredBlock.Metadata, target.Rel, blockID)
			plan.ValidationErrors = append(plan.ValidationErrors, optionErrors...)
			plan.Warnings = append(plan.Warnings, optionWarnings...)

			options, err := materializeOptions(state)
			if err != nil {
				plan.ValidationErrors = append(plan.ValidationErrors, fmt.Sprintf("%s#%s: %v", target.Rel, blockID, err))
				options = fallbackOptions(state)
			}

			plan.Blocks = append(plan.Blocks, Block{
				ID:          blockID,
				ExplicitID:  explicit,
				QualifiedID: target.Rel + "#" + blockID,
				File:        target.Rel,
				Line:        discoveredBlock.Line,
				MarkerLine:  discoveredBlock.MarkerLine,
				Language:    discoveredBlock.Language,
				Shell:       discoveredBlock.Shell,
				MarkerForm:  discoveredBlock.MarkerForm,
				Metadata:    copyMetadata(discoveredBlock.Metadata),
				Source:      trimOneTrailingNewline(discoveredBlock.Text),
				Options:     options,
			})
		}
	}
	plan.ValidationErrors = append(plan.ValidationErrors, unusedBlockConfigErrors(blockConfigEntries, targetFiles, usedBlockConfigs)...)

	if len(plan.Blocks) == 0 {
		message := "no marked blocks found"
		if requireBlocks {
			plan.ValidationErrors = append(plan.ValidationErrors, message)
		} else {
			plan.Warnings = append(plan.Warnings, message)
		}
	}
	if len(plan.ValidationErrors) == 0 {
		plan.Runner = summarizeRunner(defaultPlanOptions, plan.Blocks)
	}

	exitCode := 0
	if len(plan.ValidationErrors) > 0 {
		exitCode = 2
	}
	if requireBlocks && len(plan.Blocks) == 0 {
		exitCode = 4
	}

	return Result{Plan: plan, ExitCode: exitCode}, nil
}

func loadConfig(req Request, resolver project.Resolver) (*config.Config, string, error) {
	if req.ConfigPath != "" {
		resolved, err := resolver.ResolveConfig(req.ConfigPath)
		if err != nil {
			return nil, "", err
		}
		cfg, err := config.Load(resolved.Abs)
		if err != nil {
			return nil, "", err
		}
		return cfg, resolved.Rel, nil
	}

	resolved, ok, err := resolver.DiscoverConfig()
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "", nil
	}
	cfg, err := config.Load(resolved.Abs)
	if err != nil {
		return nil, "", err
	}
	return cfg, resolved.Rel, nil
}

func validateConfigReferences(cfg config.Config, resolver project.Resolver) error {
	if cfg.Defaults.Runner != nil {
		if err := validateRunner(*cfg.Defaults.Runner); err != nil {
			return err
		}
	}
	if cfg.Defaults.Image != nil {
		if err := validateDockerImage(*cfg.Defaults.Image); err != nil {
			return err
		}
	}
	if cfg.Defaults.Timeout != nil {
		if _, err := duration.ParseMillis(*cfg.Defaults.Timeout); err != nil {
			return err
		}
	}
	for _, file := range cfg.Files {
		if _, err := resolver.ResolveConfigTarget(file); err != nil {
			return err
		}
	}
	for _, block := range cfg.Blocks {
		if block.File == "" || block.ID == "" {
			return fmt.Errorf("block config entries must include both file and id")
		}
		if _, err := resolver.RelForConfigPath(block.File); err != nil {
			return err
		}
		if _, err := resolver.ResolveConfigTarget(block.File); err != nil {
			return err
		}
		if block.Runner != nil {
			if err := validateRunner(*block.Runner); err != nil {
				return err
			}
		}
		if block.Image != nil {
			if err := validateDockerImage(*block.Image); err != nil {
				return err
			}
		}
		if block.Timeout != nil {
			if _, err := duration.ParseMillis(*block.Timeout); err != nil {
				return err
			}
		}
	}
	for _, pass := range cfg.Env.Pass {
		if pass.Name == "" {
			return fmt.Errorf("env.pass entries must include name")
		}
	}
	return nil
}

func ResolveTargets(req Request) ([]TargetFile, error) {
	resolver, err := project.NewResolver(req.CWD)
	if err != nil {
		return nil, err
	}
	cfg, _, err := loadConfig(req, resolver)
	if err != nil {
		return nil, err
	}
	return selectTargets(req, resolver, cfg)
}

func selectTargets(req Request, resolver project.Resolver, cfg *config.Config) ([]TargetFile, error) {
	var resolved []TargetFile
	if len(req.Positional) > 0 {
		for _, input := range req.Positional {
			file, err := resolver.ResolvePositionalTarget(input)
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, TargetFile{Rel: file.Rel, Abs: file.Abs})
		}
		return resolved, nil
	}

	if cfg != nil && len(cfg.Files) > 0 {
		for _, input := range cfg.Files {
			file, err := resolver.ResolveConfigTarget(input)
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, TargetFile{Rel: file.Rel, Abs: file.Abs})
		}
		return resolved, nil
	}

	file, err := resolver.ResolveConfigTarget("README.md")
	if err == nil {
		return []TargetFile{{Rel: file.Rel, Abs: file.Abs}}, nil
	}
	if os.IsNotExist(err) {
		return nil, ErrNoTarget
	}
	return nil, err
}

func indexedBlockConfigs(cfg *config.Config, resolver project.Resolver) (map[string]config.Block, []blockConfigEntry, error) {
	index := make(map[string]config.Block)
	var entries []blockConfigEntry
	if cfg == nil {
		return index, entries, nil
	}
	for _, block := range cfg.Blocks {
		rel, err := resolver.RelForConfigPath(block.File)
		if err != nil {
			return nil, nil, err
		}
		key := rel + "#" + block.ID
		if _, exists := index[key]; exists {
			return nil, nil, fmt.Errorf("%s#%s: duplicate block config entry", rel, block.ID)
		}
		index[key] = block
		entries = append(entries, blockConfigEntry{Key: key, File: rel, ID: block.ID})
	}
	return index, entries, nil
}

func targetFileSet(targets []TargetFile) map[string]bool {
	files := make(map[string]bool, len(targets))
	for _, target := range targets {
		files[target.Rel] = true
	}
	return files
}

func unusedBlockConfigErrors(entries []blockConfigEntry, targetFiles map[string]bool, used map[string]bool) []string {
	var errors []string
	for _, entry := range entries {
		if !targetFiles[entry.File] || used[entry.Key] {
			continue
		}
		errors = append(errors, fmt.Sprintf("%s#%s: block config does not match any explicit marker id in selected file", entry.File, entry.ID))
	}
	return errors
}

func defaultOptions(req Request, cfg *config.Config) (optionState, bool, error) {
	state := optionState{
		Runner:   "local",
		Timeout:  "120s",
		Strict:   true,
		Isolated: false,
	}
	requireBlocks := false

	if cfg != nil {
		if cfg.Defaults.Runner != nil {
			state.Runner = *cfg.Defaults.Runner
		}
		if cfg.Defaults.Image != nil {
			state.DockerImage = *cfg.Defaults.Image
		}
		if cfg.Defaults.Timeout != nil {
			state.Timeout = *cfg.Defaults.Timeout
		}
		if cfg.Defaults.Strict != nil {
			state.Strict = *cfg.Defaults.Strict
		}
		if cfg.Defaults.Isolated != nil {
			state.Isolated = *cfg.Defaults.Isolated
		}
		if cfg.Defaults.Network != nil {
			value := *cfg.Defaults.Network
			state.Network = &value
		}
		if cfg.Defaults.RequireBlocks != nil {
			requireBlocks = *cfg.Defaults.RequireBlocks
		}
	}

	if req.HasRunner {
		state.Runner = req.Runner
	}
	if req.HasTimeout {
		state.Timeout = req.Timeout
	}
	if req.HasNetwork {
		value := req.Network
		state.Network = &value
	}
	if req.HasRequireBlocks {
		requireBlocks = req.RequireBlocks
	}

	if err := validateRunner(state.Runner); err != nil {
		return optionState{}, false, err
	}
	if _, err := duration.ParseMillis(state.Timeout); err != nil {
		return optionState{}, false, err
	}
	return state, requireBlocks, nil
}

func applyBlockConfig(state optionState, block config.Block) optionState {
	if block.Runner != nil {
		state.Runner = *block.Runner
	}
	if block.Image != nil {
		state.DockerImage = *block.Image
	}
	if block.Timeout != nil {
		state.Timeout = *block.Timeout
	}
	if block.Strict != nil {
		state.Strict = *block.Strict
	}
	if block.Isolated != nil {
		state.Isolated = *block.Isolated
	}
	if block.Network != nil {
		value := *block.Network
		state.Network = &value
	}
	return state
}

func applyInlineMetadata(state optionState, metadata map[string]string, file string, id string) (optionState, []string, []string) {
	var errors []string
	var warnings []string
	for _, key := range sortedKeys(metadata) {
		value := metadata[key]
		switch key {
		case "id":
			continue
		case "runner":
			state.Runner = value
		case "image":
			state.DockerImage = value
		case "timeout":
			state.Timeout = value
		case "strict":
			parsed, err := parseMarkerBool(value)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s#%s: strict metadata %v", file, id, err))
			} else {
				state.Strict = parsed
			}
		case "isolated":
			parsed, err := parseMarkerBool(value)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s#%s: isolated metadata %v", file, id, err))
			} else {
				state.Isolated = parsed
			}
		case "network":
			parsed, err := parseMarkerBool(value)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s#%s: network metadata %v", file, id, err))
			} else {
				state.Network = &parsed
			}
		default:
			warning := fmt.Sprintf("%s#%s: unknown marker metadata key %q", file, id, key)
			if suggestion := closestMarkerMetadataKey(key); suggestion != "" {
				warning += fmt.Sprintf(" (did you mean %q?)", suggestion)
			}
			warnings = append(warnings, warning)
		}
	}
	if metadata["image"] != "" && state.Runner != "docker" {
		warnings = append(warnings, fmt.Sprintf("%s#%s: image metadata is ignored unless runner=docker", file, id))
	}
	return state, errors, warnings
}

func closestMarkerMetadataKey(key string) string {
	known := []string{"id", "runner", "image", "timeout", "strict", "isolated", "network"}
	best := ""
	bestDistance := 3
	for _, candidate := range known {
		distance := editDistance(key, candidate)
		if distance < bestDistance {
			best = candidate
			bestDistance = distance
		}
	}
	if bestDistance <= 2 {
		return best
	}
	return ""
}

func editDistance(a string, b string) int {
	if a == b {
		return 0
	}
	previous := make([]int, len(b)+1)
	current := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			current[j] = minInt(
				previous[j]+1,
				current[j-1]+1,
				previous[j-1]+cost,
			)
		}
		previous, current = current, previous
	}
	return previous[len(b)]
}

func minInt(first int, rest ...int) int {
	minimum := first
	for _, value := range rest {
		if value < minimum {
			minimum = value
		}
	}
	return minimum
}

func materializeOptions(state optionState) (Options, error) {
	if err := validateRunner(state.Runner); err != nil {
		return Options{}, err
	}
	if state.DockerImage != "" {
		if err := validateDockerImage(state.DockerImage); err != nil {
			return Options{}, err
		}
	}
	timeoutMs, err := duration.ParseMillis(state.Timeout)
	if err != nil {
		return Options{}, err
	}
	networkPolicy, networkEnforced, err := networkPolicy(state.Runner, state.Network)
	if err != nil {
		return Options{}, err
	}
	stateMode := "shared"
	if state.Isolated {
		stateMode = "isolated"
	}
	dockerImage := ""
	if state.Runner == "docker" {
		dockerImage = state.DockerImage
		if dockerImage == "" {
			dockerImage = DefaultDockerImage
		}
	}
	return Options{
		Runner:          state.Runner,
		DockerImage:     dockerImage,
		Timeout:         state.Timeout,
		TimeoutMs:       timeoutMs,
		Strict:          state.Strict,
		Isolated:        state.Isolated,
		StateMode:       stateMode,
		Stdin:           "closed",
		TTY:             false,
		NetworkPolicy:   networkPolicy,
		NetworkEnforced: networkEnforced,
	}, nil
}

func fallbackOptions(state optionState) Options {
	if err := validateRunner(state.Runner); err != nil {
		state.Runner = "local"
	}
	if _, err := duration.ParseMillis(state.Timeout); err != nil {
		state.Timeout = "120s"
	}
	if state.DockerImage != "" {
		if err := validateDockerImage(state.DockerImage); err != nil {
			state.DockerImage = ""
		}
	}
	if _, _, err := networkPolicy(state.Runner, state.Network); err != nil {
		state.Network = nil
	}
	options, err := materializeOptions(state)
	if err == nil {
		return options
	}
	options, _ = materializeOptions(optionState{
		Runner:   "local",
		Timeout:  "120s",
		Strict:   true,
		Isolated: false,
	})
	return options
}

func networkPolicy(runner string, network *bool) (string, bool, error) {
	if network != nil && !*network {
		if runner != "docker" {
			return "", false, fmt.Errorf("network=false is not enforceable with runner %s; choose --runner=docker or remove network=false", runner)
		}
		return "disabled", true, nil
	}
	if runner == "docker" {
		return "container-default", false, nil
	}
	return "host", false, nil
}

func runnerSummary(options Options) Runner {
	workspace := "temporary"
	if options.Runner == "docker" {
		workspace = "container"
	}
	return Runner{
		Kind:            options.Runner,
		Workspace:       workspace,
		NetworkPolicy:   options.NetworkPolicy,
		NetworkEnforced: options.NetworkEnforced,
	}
}

func summarizeRunner(defaultOptions Options, blocks []Block) Runner {
	if len(blocks) == 0 {
		return runnerSummary(defaultOptions)
	}
	first := runnerSummary(blocks[0].Options)
	for _, block := range blocks[1:] {
		current := runnerSummary(block.Options)
		if current.Kind != first.Kind ||
			current.Workspace != first.Workspace ||
			current.NetworkPolicy != first.NetworkPolicy ||
			current.NetworkEnforced != first.NetworkEnforced {
			return Runner{
				Kind:            "mixed",
				Workspace:       "mixed",
				NetworkPolicy:   "mixed",
				NetworkEnforced: false,
			}
		}
	}
	return first
}

func planEnv(cfg *config.Config) Env {
	env := Env{Allow: []string{}, Pass: []EnvPass{}}
	if cfg == nil {
		return env
	}
	env.Allow = append(env.Allow, cfg.Env.Allow...)
	sort.Strings(env.Allow)
	for _, pass := range cfg.Env.Pass {
		_, present := os.LookupEnv(pass.Name)
		env.Pass = append(env.Pass, EnvPass{
			Name:     pass.Name,
			Secret:   boolValue(pass.Secret),
			Required: boolValue(pass.Required),
			Present:  present,
		})
	}
	sort.Slice(env.Pass, func(i, j int) bool {
		return env.Pass[i].Name < env.Pass[j].Name
	})
	return env
}

func validateRunner(value string) error {
	switch value {
	case "local", "action-local", "docker":
		return nil
	default:
		return fmt.Errorf("invalid runner %q", value)
	}
}

func validateDockerImage(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("docker image must not be empty")
	}
	if strings.ContainsAny(trimmed, " \t\r\n") {
		return fmt.Errorf("docker image %q must not contain whitespace", value)
	}
	digestValue := digestPart(trimmed)
	if digestValue != "" && !strings.HasPrefix(digestValue, "sha256:") {
		return fmt.Errorf("docker image %q has an unsupported digest algorithm; use sha256", value)
	}
	if digest, ok := strings.CutPrefix(digestValue, "sha256:"); ok {
		if len(digest) != 64 {
			return fmt.Errorf("docker image %q has an invalid sha256 digest length", value)
		}
		for _, r := range digest {
			if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
				return fmt.Errorf("docker image %q has an invalid sha256 digest", value)
			}
		}
	}
	return nil
}

func digestPart(image string) string {
	index := strings.LastIndex(image, "@")
	if index < 0 {
		return ""
	}
	return image[index+1:]
}

func parseMarkerBool(value string) (bool, error) {
	switch strings.TrimSpace(value) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("must be true or false")
	}
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func implicitID(line int) string {
	return "line-" + strconv.Itoa(line)
}

func trimOneTrailingNewline(text string) string {
	if strings.HasSuffix(text, "\n") {
		text = strings.TrimSuffix(text, "\n")
		return strings.TrimSuffix(text, "\r")
	}
	return text
}

func copyMetadata(metadata map[string]string) map[string]string {
	copied := make(map[string]string, len(metadata))
	for key, value := range metadata {
		copied[key] = value
	}
	return copied
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
