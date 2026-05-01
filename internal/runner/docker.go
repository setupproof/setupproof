package runner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/setupproof/setupproof/internal/planning"
	"github.com/setupproof/setupproof/internal/report"
)

const (
	dockerBinary            = "docker"
	containerWorkspaceRoot  = "/workspace"
	containerStateDir       = "/workspace/.setupproof"
	containerHome           = "/workspace/.setupproof/home"
	containerTmp            = "/workspace/.setupproof/tmp"
	containerCache          = "/workspace/.setupproof/cache"
	containerLifecycleShell = "while :; do sleep 3600; done"
)

type dockerContainer struct {
	name  string
	image string
	ws    *workspace
}

type dockerCommandResult struct {
	exitCode         int
	timedOut         bool
	cleanupCompleted bool
	stdout           string
	stderr           string
	stdoutTruncated  bool
	stderrTruncated  bool
	err              error
}

type dockerImageRecorder struct {
	mu   sync.Mutex
	seen map[string]bool
}

func runPreparedDocker(req planning.Request, plan planning.Plan, opts Options, stderr io.Writer) executionRun {
	if err := validateDockerPlan(plan); err != nil {
		fmt.Fprintln(stderr, err)
		return executionRun{code: 2}
	}
	source, warnings, code, runnerError, ok := prepareWorkspaceSource(req, plan, stderr)
	if !ok {
		return executionRun{code: code, warnings: warnings, runnerError: runnerError}
	}

	ctx, stop, signalReason := interruptContext()
	defer stop()

	exitCode := 0
	var blockReports []report.Block
	blocksByFile := groupBlocksByFile(plan.Blocks)
	imageRecorder := newDockerImageRecorder()
	for _, file := range plan.Files {
		blocks := blocksByFile[file]
		if len(blocks) == 0 {
			continue
		}
		fileCode, fileReports, fileRunnerError := runDockerFile(ctx, file, blocks, plan.Env, source, opts, stderr, imageRecorder, signalReason)
		blockReports = append(blockReports, fileReports...)
		if fileCode == 2 || fileCode == 3 {
			return executionRun{code: fileCode, blocks: blockReports, warnings: warnings, runnerError: fileRunnerError}
		}
		if fileCode == 1 {
			exitCode = 1
		}
		if ctx.Err() != nil {
			return executionRun{code: 3, blocks: blockReports, warnings: warnings, runnerError: signalReason()}
		}
	}
	return executionRun{code: exitCode, blocks: blockReports, warnings: warnings}
}

func validateDockerPlan(plan planning.Plan) error {
	sharedImages := make(map[string]string)
	for _, block := range plan.Blocks {
		if block.Options.Runner != "docker" {
			return fmt.Errorf("runner %q cannot be mixed with runner \"docker\" in one execution", block.Options.Runner)
		}
		if block.Options.DockerImage == "" {
			return fmt.Errorf("%s: docker image is required", block.QualifiedID)
		}
		if !block.Options.Isolated {
			if existing := sharedImages[block.File]; existing != "" && existing != block.Options.DockerImage {
				return fmt.Errorf("%s: mixed docker images in one shared target file are not implemented", block.QualifiedID)
			}
			sharedImages[block.File] = block.Options.DockerImage
		}
	}
	return nil
}

func runDockerFile(ctx context.Context, file string, blocks []planning.Block, envPlan planning.Env, source workspaceSource, opts Options, stderr io.Writer, imageRecorder *dockerImageRecorder, signalReason func() string) (code int, blockReports []report.Block, runnerError string) {
	sharedWorkspace, err := createDockerWorkspace(source)
	if err != nil {
		fmt.Fprintf(stderr, "%s: workspace setup failed: %v\n", file, err)
		return 3, blockReports, "workspace_setup_failed"
	}
	if opts.KeepWorkspace {
		fmt.Fprint(stderr, keepWorkspaceWarning(file, sharedWorkspace.repoRoot))
	}
	defer func() {
		if err := sharedWorkspace.cleanup(opts.KeepWorkspace); err != nil {
			fmt.Fprintf(stderr, "%s: cleanup failed: %v\n", file, err)
			code, runnerError = codeWithCleanupFailure(code, runnerError)
		}
	}()

	baseEnv, secretValues, err := baselineDockerEnv(envPlan, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2, blockReports, ""
	}
	state := fileState{cwd: containerWorkspaceRoot, env: baseEnv}
	exitCode := 0
	stopped := false
	var container *dockerContainer

	cleanupContainer := func() (int, string) {
		if container == nil {
			return 0, ""
		}
		if err := removeDockerContainer(context.Background(), container.name, stderr); err != nil {
			fmt.Fprintf(stderr, "%s: cleanup failed: %v\n", file, err)
			container = nil
			return 3, "cleanup_failed"
		}
		container = nil
		return 0, ""
	}
	defer func() {
		if cleanupCode, cleanupReason := cleanupContainer(); cleanupCode != 0 {
			if code == 0 {
				code = cleanupCode
				runnerError = cleanupReason
			}
		}
	}()

	startContainer := func(block planning.Block) (int, string) {
		if err := ensureDockerWorkspaceDirs(sharedWorkspace); err != nil {
			fmt.Fprintf(stderr, "%s: workspace setup failed: %v\n", file, err)
			return 3, "workspace_setup_failed"
		}
		started, reason, err := startDockerContainer(ctx, sharedWorkspace, block.Options.DockerImage, state.env, block.Options.NetworkPolicy, stderr, secretValues)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 3, reason
		}
		container = started
		imageRecorder.record(ctx, block.Options.DockerImage, stderr)
		return 0, ""
	}

	for _, block := range blocks {
		if stopped {
			blockReports = append(blockReports, reportBlock(block, blockOutcome{result: "skipped", reason: "fail-fast"}, report.Output{}, 0))
			continue
		}

		activeWorkspace := sharedWorkspace
		activeState := state
		activeContainer := container
		blockSecrets := secretValues
		isolated := block.Options.Isolated

		if isolated {
			activeWorkspace, err = createDockerWorkspace(source)
			if err != nil {
				fmt.Fprintf(stderr, "%s: workspace setup failed: %v\n", block.QualifiedID, err)
				return 3, blockReports, "workspace_setup_failed"
			}
			if opts.KeepWorkspace {
				fmt.Fprint(stderr, keepWorkspaceWarning(block.QualifiedID, activeWorkspace.repoRoot))
			}
			isolatedEnv, isolatedSecrets, err := baselineDockerEnv(envPlan, stderr)
			if err != nil {
				_ = activeWorkspace.cleanup(opts.KeepWorkspace)
				fmt.Fprintln(stderr, err)
				return 2, blockReports, ""
			}
			blockSecrets = append(append([]string(nil), secretValues...), isolatedSecrets...)
			activeState = fileState{cwd: containerWorkspaceRoot, env: isolatedEnv}
			if err := ensureDockerWorkspaceDirs(activeWorkspace); err != nil {
				_ = activeWorkspace.cleanup(opts.KeepWorkspace)
				fmt.Fprintf(stderr, "%s: workspace setup failed: %v\n", block.QualifiedID, err)
				return 3, blockReports, "workspace_setup_failed"
			}
			var startReason string
			activeContainer, startReason, err = startDockerContainer(ctx, activeWorkspace, block.Options.DockerImage, activeState.env, block.Options.NetworkPolicy, stderr, blockSecrets)
			if err != nil {
				blockReports = append(blockReports, reportBlock(block, blockOutcome{result: "error", reason: startReason}, report.Output{}, 0))
				_ = activeWorkspace.cleanup(opts.KeepWorkspace)
				fmt.Fprintln(stderr, err)
				return 3, blockReports, startReason
			}
			imageRecorder.record(ctx, block.Options.DockerImage, stderr)
		} else if activeContainer == nil {
			if code, reason := startContainer(block); code != 0 {
				blockReports = append(blockReports, reportBlock(block, blockOutcome{result: "error", reason: reason}, report.Output{}, 0))
				return code, blockReports, reason
			}
			activeContainer = container
		}

		outcome, nextState, containerAlive, output, durationMs := runDockerBlock(ctx, block, activeContainer, activeWorkspace, activeState, stderr, blockSecrets)
		blockReports = append(blockReports, reportBlock(block, outcome, output, durationMs))

		if isolated {
			if activeContainer != nil && containerAlive {
				if err := removeDockerContainer(context.Background(), activeContainer.name, stderr); err != nil {
					fmt.Fprintf(stderr, "%s: cleanup failed: %v\n", block.QualifiedID, err)
					_ = activeWorkspace.cleanup(opts.KeepWorkspace)
					return 3, blockReports, "cleanup_failed"
				}
			}
			if err := activeWorkspace.cleanup(opts.KeepWorkspace); err != nil {
				fmt.Fprintf(stderr, "%s: cleanup failed: %v\n", block.QualifiedID, err)
				return 3, blockReports, "cleanup_failed"
			}
		} else if !containerAlive {
			container = nil
		}

		switch outcome.result {
		case "passed":
			if !isolated {
				state = nextState
			}
		case "error":
			return 3, blockReports, outcome.reason
		default:
			exitCode = 1
			if opts.FailFast {
				stopped = true
			}
		}

		if ctx.Err() != nil {
			return 3, blockReports, signalReason()
		}
	}
	return exitCode, blockReports, ""
}

func runDockerBlock(ctx context.Context, block planning.Block, container *dockerContainer, ws *workspace, state fileState, stderr io.Writer, secretValues []string) (blockOutcome, fileState, bool, report.Output, int64) {
	started := time.Now()
	if command, ok := classifyInteractive(block.Source); ok {
		fmt.Fprintf(stderr, "%s: non-interactive execution cannot run common interactive command %q\n", block.QualifiedID, command)
		return blockOutcome{result: "failed", exitCode: 1, reason: "interactive-command", interactive: command}, state, true, report.Output{}, elapsedMillis(started)
	}

	controlLocal, err := os.MkdirTemp(filepath.Join(ws.repoRoot, ".setupproof"), "control-")
	if err != nil {
		return blockOutcome{result: "error", reason: "workspace_setup_failed"}, state, true, report.Output{}, elapsedMillis(started)
	}
	stateCWDLocal := filepath.Join(controlLocal, "cwd")
	stateEnvLocal := filepath.Join(controlLocal, "env")
	startMarkerLocal := filepath.Join(controlLocal, "started")
	scriptLocal := filepath.Join(controlLocal, "block.sh")

	stateCWDContainer := containerPath(ws, stateCWDLocal)
	stateEnvContainer := containerPath(ws, stateEnvLocal)
	startMarkerContainer := containerPath(ws, startMarkerLocal)
	scriptContainer := containerPath(ws, scriptLocal)

	if err := os.WriteFile(scriptLocal, []byte(shellScriptWithStart(block, stateCWDContainer, stateEnvContainer, startMarkerContainer)), 0o700); err != nil {
		return blockOutcome{result: "error", reason: "workspace_setup_failed"}, state, true, report.Output{}, elapsedMillis(started)
	}

	timeout := time.Duration(block.Options.TimeoutMs) * time.Millisecond
	blockCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := dockerExec(blockCtx, container.name, state.cwd, state.env, block.Shell, scriptContainer, stderr, secretValues)
	output := report.Output{
		StdoutTail:      result.stdout,
		StderrTail:      result.stderr,
		StdoutTruncated: result.stdoutTruncated,
		StderrTruncated: result.stderrTruncated,
	}
	if result.timedOut {
		return blockOutcome{result: "timeout", exitCode: 1, reason: "timeout", cleanupCompleted: boolPointer(result.cleanupCompleted)}, state, false, output, elapsedMillis(started)
	}
	if result.err != nil {
		return blockOutcome{result: "error", reason: classifyDockerCommandError(result)}, state, true, output, elapsedMillis(started)
	}
	if result.exitCode != 0 {
		if (result.exitCode == 126 || result.exitCode == 127) && !fileExists(startMarkerLocal) {
			return blockOutcome{result: "error", reason: "process_start_failed"}, state, true, output, elapsedMillis(started)
		}
		return blockOutcome{result: "failed", exitCode: result.exitCode, reason: "exit-code"}, state, true, output, elapsedMillis(started)
	}

	nextState, err := readStateFiles(stateCWDLocal, stateEnvLocal, state)
	if err == nil {
		err = validateContainerStateCWD(nextState.cwd)
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s: warning: could not capture shell state: %v\n", block.QualifiedID, err)
		return blockOutcome{result: "passed", exitCode: 0}, state, true, output, elapsedMillis(started)
	}
	return blockOutcome{result: "passed", exitCode: 0}, nextState, true, output, elapsedMillis(started)
}

func startDockerContainer(ctx context.Context, ws *workspace, image string, env []string, networkPolicy string, stderr io.Writer, secretValues []string) (*dockerContainer, string, error) {
	name := dockerContainerName()
	args := dockerRunArgs(dockerRunSpec{
		Name:          name,
		Image:         image,
		Workspace:     ws.repoRoot,
		User:          hostDockerUser(),
		Env:           env,
		NetworkPolicy: networkPolicy,
	})
	result := runDockerCommand(ctx, args, false, stderr, nil, secretValues)
	if result.err != nil || result.exitCode != 0 {
		reason := classifyDockerCommandError(result)
		return nil, reason, fmt.Errorf("docker runner error: %s%s", reason, dockerErrorDetail(result))
	}
	return &dockerContainer{name: name, image: image, ws: ws}, "", nil
}

func dockerExec(ctx context.Context, name string, cwd string, env []string, shell string, script string, stderr io.Writer, secretValues []string) dockerCommandResult {
	args := dockerExecArgs(dockerExecSpec{
		Name:   name,
		CWD:    cwd,
		Env:    env,
		Shell:  shell,
		Script: script,
	})
	return runDockerCommand(ctx, args, true, stderr, func() bool {
		return removeDockerContainer(context.Background(), name, io.Discard) == nil
	}, secretValues)
}

func removeDockerContainer(ctx context.Context, name string, stderr io.Writer) error {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}
	result := runDockerCommand(ctx, dockerRemoveArgs(name), false, stderr, nil, nil)
	if result.err != nil {
		return result.err
	}
	if result.exitCode != 0 {
		return fmt.Errorf("docker rm exited %d", result.exitCode)
	}
	return nil
}

type dockerRunSpec struct {
	Name          string
	Image         string
	Workspace     string
	User          string
	Env           []string
	NetworkPolicy string
}

func dockerRunArgs(spec dockerRunSpec) []string {
	args := []string{
		"run",
		"--detach",
		"--name", spec.Name,
		"--workdir", containerWorkspaceRoot,
		"--mount", "type=bind,src=" + spec.Workspace + ",dst=" + containerWorkspaceRoot,
		"--read-only",
		"--tmpfs", "/tmp",
		"--cap-drop=ALL",
		"--security-opt=no-new-privileges",
	}
	if spec.User != "" {
		args = append(args, "--user", spec.User)
	}
	if spec.NetworkPolicy == "disabled" {
		args = append(args, "--network", "none")
	}
	for _, env := range sortedEnv(spec.Env) {
		args = append(args, "--env", env)
	}
	args = append(args, spec.Image, "sh", "-c", containerLifecycleShell)
	return args
}

type dockerExecSpec struct {
	Name   string
	CWD    string
	Env    []string
	Shell  string
	Script string
}

func dockerExecArgs(spec dockerExecSpec) []string {
	args := []string{
		"exec",
		"--workdir", spec.CWD,
	}
	for _, env := range sortedEnv(spec.Env) {
		args = append(args, "--env", env)
	}
	args = append(args, spec.Name, spec.Shell, spec.Script)
	return args
}

func dockerRemoveArgs(name string) []string {
	return []string{"rm", "-f", name}
}

func runDockerCommand(ctx context.Context, args []string, emitOutput bool, stderr io.Writer, onTimeout func() bool, secretValues []string) dockerCommandResult {
	cmd := exec.Command(dockerBinary, args...)
	configureProcess(cmd)

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return dockerCommandResult{err: err}
	}
	defer devNull.Close()
	cmd.Stdin = devNull

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return dockerCommandResult{err: err}
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return dockerCommandResult{err: err}
	}
	if err := cmd.Start(); err != nil {
		return dockerCommandResult{err: err}
	}

	stdoutTail := report.NewTail(report.MaxTailBytes)
	stderrTail := report.NewTail(report.MaxTailBytes)
	redactor := report.NewRedactor(secretValues)
	stderrSink := synchronizedWriter(stderr)
	var copies sync.WaitGroup
	copies.Add(2)
	go func() {
		defer copies.Done()
		collector := &report.StreamCollector{Tail: stdoutTail, Redactor: redactor}
		if emitOutput {
			collector.Sink = stderrSink
		}
		_, _ = io.Copy(collector, stdoutPipe)
		_ = collector.Flush()
	}()
	go func() {
		defer copies.Done()
		collector := &report.StreamCollector{Tail: stderrTail, Redactor: redactor}
		if emitOutput {
			collector.Sink = stderrSink
		}
		_, _ = io.Copy(collector, stderrPipe)
		_ = collector.Flush()
	}()

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	var waitErr error
	select {
	case waitErr = <-waitCh:
	case <-ctx.Done():
		cleanupCompleted := true
		if onTimeout != nil {
			cleanupCompleted = onTimeout()
		}
		cleanupCompleted = terminateProcessTree(cmd) && cleanupCompleted
		<-waitCh
		copies.Wait()
		return dockerCommandResult{
			exitCode:         1,
			timedOut:         true,
			cleanupCompleted: cleanupCompleted,
			stdout:           stdoutTail.String(),
			stderr:           stderrTail.String(),
			stdoutTruncated:  stdoutTail.Truncated(),
			stderrTruncated:  stderrTail.Truncated(),
		}
	}
	copies.Wait()

	result := dockerCommandResult{
		stdout:          stdoutTail.String(),
		stderr:          stderrTail.String(),
		stdoutTruncated: stdoutTail.Truncated(),
		stderrTruncated: stderrTail.Truncated(),
	}
	if waitErr == nil {
		return result
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		result.exitCode = exitErr.ExitCode()
		return result
	}
	result.err = waitErr
	return result
}

func classifyDockerCommandError(result dockerCommandResult) string {
	if result.err != nil {
		if errors.Is(result.err, exec.ErrNotFound) {
			return "docker_unavailable"
		}
		return "other"
	}
	lower := strings.ToLower(result.stderr + "\n" + result.stdout)
	switch {
	case strings.Contains(lower, "cannot connect to the docker daemon"),
		strings.Contains(lower, "is the docker daemon running"),
		strings.Contains(lower, "permission denied"):
		return "docker_unavailable"
	case strings.Contains(lower, "pull access denied"),
		strings.Contains(lower, "manifest unknown"),
		strings.Contains(lower, "not found"),
		strings.Contains(lower, "no such image"):
		return "image_pull_failed"
	case result.exitCode == 125:
		return "process_start_failed"
	default:
		return "process_start_failed"
	}
}

func dockerErrorDetail(result dockerCommandResult) string {
	detail := strings.TrimSpace(result.stderr)
	if detail == "" {
		detail = strings.TrimSpace(result.stdout)
	}
	if detail == "" && result.err != nil {
		detail = result.err.Error()
	}
	if detail == "" {
		return ""
	}
	return ": " + detail
}

func baselineDockerEnv(envPlan planning.Env, stderr io.Writer) ([]string, []string, error) {
	values := map[string]string{
		"HOME":              containerHome,
		"TMPDIR":            containerTmp,
		"CI":                "true",
		"SETUPPROOF":        "1",
		"LANG":              "C.UTF-8",
		"XDG_CACHE_HOME":    containerCache,
		"npm_config_cache":  containerCache + "/npm",
		"YARN_CACHE_FOLDER": containerCache + "/yarn",
		"PIP_CACHE_DIR":     containerCache + "/pip",
		"CARGO_HOME":        containerCache + "/cargo",
		"GOCACHE":           containerCache + "/go-build",
		"GOMODCACHE":        containerCache + "/go/pkg/mod",
	}
	if lcAll := os.Getenv("LC_ALL"); safeLocale(lcAll) {
		values["LC_ALL"] = lcAll
	}

	var warnings []string
	for _, name := range envPlan.Allow {
		if value, ok := os.LookupEnv(name); ok {
			values[name] = value
		} else {
			warnings = append(warnings, name)
		}
	}

	var secretValues []string
	for _, pass := range envPlan.Pass {
		value, ok := os.LookupEnv(pass.Name)
		if !ok {
			if pass.Required {
				return nil, nil, fmt.Errorf("required environment variable %s is missing", pass.Name)
			}
			warnings = append(warnings, pass.Name)
			continue
		}
		values[pass.Name] = value
		if pass.Secret && value != "" {
			secretValues = append(secretValues, value)
		}
	}
	for _, name := range warnings {
		fmt.Fprintf(stderr, "warning: optional environment variable %s is not set\n", name)
	}

	env := make([]string, 0, len(values))
	for name, value := range values {
		env = append(env, name+"="+value)
	}
	sort.Strings(env)
	return env, secretValues, nil
}

func ensureDockerWorkspaceDirs(ws *workspace) error {
	for _, rel := range []string{
		".setupproof/home",
		".setupproof/tmp",
		".setupproof/cache",
		".setupproof/cache/npm",
		".setupproof/cache/yarn",
		".setupproof/cache/pip",
		".setupproof/cache/cargo",
		".setupproof/cache/go-build",
		".setupproof/cache/go/pkg/mod",
	} {
		if err := os.MkdirAll(filepath.Join(ws.repoRoot, rel), 0o700); err != nil {
			return err
		}
	}
	return nil
}

func newDockerImageRecorder() *dockerImageRecorder {
	return &dockerImageRecorder{seen: make(map[string]bool)}
}

func (r *dockerImageRecorder) record(ctx context.Context, image string, stderr io.Writer) {
	r.mu.Lock()
	if r.seen[image] {
		r.mu.Unlock()
		return
	}
	r.seen[image] = true
	r.mu.Unlock()
	recordDockerImage(ctx, image, stderr)
}

func recordDockerImage(ctx context.Context, image string, stderr io.Writer) {
	if digestPinnedImage(image) {
		fmt.Fprintf(stderr, "docker image digest: %s\n", image)
		return
	}
	fmt.Fprintf(stderr, "warning: docker image %s is not digest-pinned and is non-reproducible\n", image)
	if digest := resolvedDockerImageDigest(ctx, image); digest != "" {
		fmt.Fprintf(stderr, "docker image digest: %s\n", digest)
	}
}

func resolvedDockerImageDigest(ctx context.Context, image string) string {
	inspectCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	result := runDockerCommand(inspectCtx, []string{"image", "inspect", "--format", "{{json .RepoDigests}}", image}, false, io.Discard, nil, nil)
	stdout := strings.TrimSpace(result.stdout)
	if result.err != nil || result.exitCode != 0 || stdout == "" {
		return ""
	}
	var digests []string
	if err := json.Unmarshal([]byte(stdout), &digests); err != nil {
		return ""
	}
	if len(digests) == 0 {
		return ""
	}
	return digests[0]
}

func digestPinnedImage(image string) bool {
	index := strings.LastIndex(image, "@")
	if index < 0 {
		return false
	}
	digest := image[index+1:]
	return strings.HasPrefix(digest, "sha256:")
}

func containerPath(ws *workspace, localPath string) string {
	rel, err := filepath.Rel(ws.repoRoot, localPath)
	if err != nil {
		return containerWorkspaceRoot
	}
	return containerWorkspaceRoot + "/" + filepath.ToSlash(rel)
}

func validateContainerStateCWD(cwd string) error {
	cleaned := path.Clean(cwd)
	if !path.IsAbs(cleaned) {
		return fmt.Errorf("captured cwd %s is not an absolute container path", cwd)
	}
	if cleaned != containerWorkspaceRoot && !strings.HasPrefix(cleaned, containerWorkspaceRoot+"/") {
		return fmt.Errorf("captured cwd %s resolves outside the container workspace", cwd)
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func sortedEnv(env []string) []string {
	copied := append([]string(nil), env...)
	sort.Strings(copied)
	return copied
}

func dockerContainerName() string {
	var nonce [6]byte
	if _, err := rand.Read(nonce[:]); err == nil {
		return fmt.Sprintf("setupproof-%d-%d-%s", os.Getpid(), time.Now().UnixNano(), hex.EncodeToString(nonce[:]))
	}
	return fmt.Sprintf("setupproof-%d-%d", os.Getpid(), time.Now().UnixNano())
}

func hostDockerUser() string {
	uid := os.Getuid()
	gid := os.Getgid()
	if uid < 0 || gid < 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", uid, gid)
}
