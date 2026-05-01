package planning

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildUsesConfigFilesAndAppliesPrecedence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "<!-- setupproof id=quickstart timeout=90s strict=false image=alpine@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa -->\n```sh\nnpm test\n```\n")
	writeFile(t, dir, "setupproof.yml", `version: 1
defaults:
  runner: local
  image: ubuntu:24.04
  timeout: 120s
  strict: true
  isolated: false
files:
  - README.md
blocks:
  - file: README.md
    id: quickstart
    timeout: 180s
    isolated: true
`)

	result, err := Build(Request{
		CWD:              dir,
		Args:             []string{"--dry-run", "--json"},
		HasRunner:        true,
		Runner:           "docker",
		HasRequireBlocks: true,
		RequireBlocks:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, errors = %#v", result.ExitCode, result.Plan.ValidationErrors)
	}
	if len(result.Plan.Blocks) != 1 {
		t.Fatalf("blocks = %#v", result.Plan.Blocks)
	}

	block := result.Plan.Blocks[0]
	if block.ID != "quickstart" || !block.ExplicitID {
		t.Fatalf("block identity = %#v", block)
	}
	if block.Options.Runner != "docker" {
		t.Fatalf("runner = %q", block.Options.Runner)
	}
	if block.Options.DockerImage != "alpine@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("docker image = %q", block.Options.DockerImage)
	}
	if block.Options.Timeout != "90s" || block.Options.TimeoutMs != 90000 {
		t.Fatalf("timeout = %q/%d", block.Options.Timeout, block.Options.TimeoutMs)
	}
	if block.Options.Strict {
		t.Fatalf("strict should come from inline metadata")
	}
	if !block.Options.Isolated {
		t.Fatalf("isolated should come from block config")
	}
}

func TestBuildDefaultsDockerImageForDockerRunner(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```sh setupproof runner=docker\nnpm test\n```\n")

	result, err := Build(Request{CWD: dir, Positional: []string{"README.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, errors = %#v", result.ExitCode, result.Plan.ValidationErrors)
	}
	if got := result.Plan.Blocks[0].Options.DockerImage; got != DefaultDockerImage {
		t.Fatalf("docker image = %q", got)
	}
	if result.Plan.Runner.Kind != "docker" || result.Plan.Runner.Workspace != "container" || result.Plan.Runner.NetworkPolicy != "container-default" {
		t.Fatalf("runner summary = %#v", result.Plan.Runner)
	}
}

func TestBuildRejectsInvalidDockerImageDigest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```sh setupproof runner=docker image=ubuntu@sha256:nothex\nnpm test\n```\n")

	result, err := Build(Request{CWD: dir, Positional: []string{"README.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 2 {
		t.Fatalf("exit code = %d", result.ExitCode)
	}
	if len(result.Plan.ValidationErrors) == 0 {
		t.Fatalf("expected validation errors")
	}
}

func TestBuildRejectsDockerImageDigestWithoutAlgorithm(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```sh setupproof runner=docker image=ubuntu@deadbeef\nnpm test\n```\n")

	result, err := Build(Request{CWD: dir, Positional: []string{"README.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 2 {
		t.Fatalf("exit code = %d", result.ExitCode)
	}
	if len(result.Plan.ValidationErrors) == 0 {
		t.Fatalf("expected validation errors")
	}
}

func TestBuildMixedRunnerReportsMixedWorkspace(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```sh setupproof id=local\ntrue\n```\n\n```sh setupproof id=docker runner=docker\ntrue\n```\n")

	result, err := Build(Request{CWD: dir, Positional: []string{"README.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 2 {
		t.Fatalf("exit code = %d, errors = %#v", result.ExitCode, result.Plan.ValidationErrors)
	}
	if result.Plan.Runner.Kind != "mixed" || result.Plan.Runner.Workspace != "mixed" || result.Plan.Runner.NetworkPolicy != "mixed" {
		t.Fatalf("runner summary = %#v", result.Plan.Runner)
	}
	if !contains(result.Plan.ValidationErrors, "mixed runners in one execution are not implemented; pass --runner=local or --runner=docker for the whole run") {
		t.Fatalf("validation errors = %#v", result.Plan.ValidationErrors)
	}
}

func TestBuildUsesRootReadmeWithoutArguments(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```sh setupproof\nnpm test\n```\n")

	result, err := Build(Request{CWD: dir, Args: []string{"--dry-run", "--json"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d", result.ExitCode)
	}
	if len(result.Plan.Files) != 1 || result.Plan.Files[0] != "README.md" {
		t.Fatalf("files = %#v", result.Plan.Files)
	}
	if result.Plan.Blocks[0].ID != "line-1" || result.Plan.Blocks[0].ExplicitID {
		t.Fatalf("implicit id block = %#v", result.Plan.Blocks[0])
	}
}

func TestBuildRejectsDuplicateExplicitIDs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```sh setupproof id=dup\na\n```\n\n```sh setupproof id=dup\nb\n```\n")

	_, err := Build(Request{CWD: dir, Positional: []string{"README.md"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildRejectsInvalidRunnerInConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```sh setupproof\nnpm test\n```\n")
	writeFile(t, dir, "setupproof.yml", "version: 1\ndefaults:\n  runner: invalid\nfiles:\n  - README.md\n")

	_, err := Build(Request{CWD: dir})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildRejectsMissingConfigFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "setupproof.yml", "version: 1\nfiles:\n  - missing.md\n")

	_, err := Build(Request{CWD: dir})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildRejectsEscapingConfigFileEntry(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```sh setupproof\nnpm test\n```\n")
	writeFile(t, dir, "setupproof.yml", "version: 1\nfiles:\n  - ../README.md\n")

	_, err := Build(Request{CWD: dir})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildRequireBlocksReturnsExitCodeFour(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```sh\nnpm test\n```\n")

	result, err := Build(Request{
		CWD:              dir,
		Positional:       []string{"README.md"},
		HasRequireBlocks: true,
		RequireBlocks:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 4 {
		t.Fatalf("exit code = %d", result.ExitCode)
	}
	if len(result.Plan.ValidationErrors) != 1 {
		t.Fatalf("validation errors = %#v", result.Plan.ValidationErrors)
	}
}

func TestBuildWarnsWhenBlocksAreMissingButNotRequired(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```sh\nnpm test\n```\n")

	result, err := Build(Request{CWD: dir, Positional: []string{"README.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d", result.ExitCode)
	}
	if len(result.Plan.Warnings) != 1 {
		t.Fatalf("warnings = %#v", result.Plan.Warnings)
	}
}

func TestBuildRejectsUnsupportedMarkedLanguage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```zsh setupproof id=install\nnpm test\n```\n")
	writeFile(t, dir, "setupproof.yml", `version: 1
blocks:
  - file: README.md
    id: install
    timeout: 90s
`)

	result, err := Build(Request{
		CWD:              dir,
		Positional:       []string{"README.md"},
		HasRequireBlocks: true,
		RequireBlocks:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 2 {
		t.Fatalf("exit code = %d, errors = %#v", result.ExitCode, result.Plan.ValidationErrors)
	}
	if len(result.Plan.Blocks) != 0 {
		t.Fatalf("blocks = %#v", result.Plan.Blocks)
	}
	if !contains(result.Plan.ValidationErrors, `README.md:1: marked block language "zsh" is not supported; use sh, bash, or shell`) {
		t.Fatalf("validation errors = %#v", result.Plan.ValidationErrors)
	}
	if contains(result.Plan.ValidationErrors, "README.md#install: block config does not match any explicit marker id in selected file") {
		t.Fatalf("unsupported marked block config was treated as unused: %#v", result.Plan.ValidationErrors)
	}
}

func TestBuildRejectsNetworkFalseForLocalRunner(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```sh setupproof network=false\nnpm test\n```\n")

	result, err := Build(Request{CWD: dir, Positional: []string{"README.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 2 {
		t.Fatalf("exit code = %d", result.ExitCode)
	}
	if len(result.Plan.ValidationErrors) == 0 {
		t.Fatalf("expected validation errors")
	}
}

func TestBuildWarnsForUnknownMarkerMetadata(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "<!-- setupproof id=quickstart idd=typo bare -->\n```sh\nnpm test\n```\n")

	result, err := Build(Request{CWD: dir, Positional: []string{"README.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, errors = %#v", result.ExitCode, result.Plan.ValidationErrors)
	}
	if len(result.Plan.Warnings) != 2 {
		t.Fatalf("warnings = %#v", result.Plan.Warnings)
	}
	if !contains(result.Plan.Warnings, `README.md:1: marker metadata token bare must use key=value`) {
		t.Fatalf("warnings = %#v", result.Plan.Warnings)
	}
	if !contains(result.Plan.Warnings, `README.md#quickstart: unknown marker metadata key "idd" (did you mean "id"?)`) {
		t.Fatalf("warnings = %#v", result.Plan.Warnings)
	}
}

func TestBuildWarnsWhenInlineImageIsIgnoredByLocalRunner(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```sh setupproof id=quickstart image=alpine:3.20\nnpm test\n```\n")

	result, err := Build(Request{CWD: dir, Positional: []string{"README.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, errors = %#v", result.ExitCode, result.Plan.ValidationErrors)
	}
	if got := result.Plan.Blocks[0].Options.DockerImage; got != "" {
		t.Fatalf("docker image = %q", got)
	}
	if !contains(result.Plan.Warnings, "README.md#quickstart: image metadata is ignored unless runner=docker") {
		t.Fatalf("warnings = %#v", result.Plan.Warnings)
	}
}

func TestBuildPlanEnvPassReportsPresenceWithoutValues(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SETUPPROOF_PRESENT_SECRET", "super-secret-value")
	writeFile(t, dir, "README.md", "```sh setupproof id=quickstart\ntrue\n```\n")
	writeFile(t, dir, "setupproof.yml", `version: 1
files:
  - README.md
env:
  pass:
    - name: SETUPPROOF_PRESENT_SECRET
      secret: true
      required: true
    - name: SETUPPROOF_MISSING_OPTIONAL
      secret: true
      required: false
`)

	result, err := Build(Request{CWD: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Plan.Env.Pass) != 2 {
		t.Fatalf("env pass = %#v", result.Plan.Env.Pass)
	}
	if result.Plan.Env.Pass[0].Name != "SETUPPROOF_MISSING_OPTIONAL" || result.Plan.Env.Pass[0].Present {
		t.Fatalf("missing env presence = %#v", result.Plan.Env.Pass[0])
	}
	if result.Plan.Env.Pass[1].Name != "SETUPPROOF_PRESENT_SECRET" || !result.Plan.Env.Pass[1].Present {
		t.Fatalf("present env presence = %#v", result.Plan.Env.Pass[1])
	}
}

func TestBuildRejectsInvalidAndDuplicateEnvironmentNames(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```sh setupproof id=quickstart\ntrue\n```\n")
	writeFile(t, dir, "setupproof.yml", `version: 1
files:
  - README.md
env:
  allow:
    - 1BAD
`)

	_, err := Build(Request{CWD: dir})
	if err == nil {
		t.Fatal("expected invalid env name error")
	}

	writeFile(t, dir, "setupproof.yml", `version: 1
files:
  - README.md
env:
  allow:
    - SETUPPROOF_DUP
  pass:
    - name: SETUPPROOF_DUP
`)

	_, err = Build(Request{CWD: dir})
	if err == nil {
		t.Fatal("expected duplicate env name error")
	}
}

func TestBuildRejectsMissingNoArgumentTarget(t *testing.T) {
	dir := t.TempDir()
	_, err := Build(Request{CWD: dir})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildRejectsBlockConfigWithoutFileAndID(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```sh setupproof\nnpm test\n```\n")
	writeFile(t, dir, "setupproof.yml", "version: 1\nblocks:\n  - file: README.md\n")

	_, err := Build(Request{CWD: dir, Positional: []string{"README.md"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildRejectsUnusedBlockConfigForSelectedFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```sh setupproof id=quickstart\nnpm test\n```\n")
	writeFile(t, dir, "setupproof.yml", `version: 1
files:
  - README.md
blocks:
  - file: README.md
    id: quick-start
    timeout: 90s
`)

	result, err := Build(Request{CWD: dir})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 2 {
		t.Fatalf("exit code = %d", result.ExitCode)
	}
	if !contains(result.Plan.ValidationErrors, "README.md#quick-start: block config does not match any explicit marker id in selected file") {
		t.Fatalf("validation errors = %#v", result.Plan.ValidationErrors)
	}
}

func TestBuildIgnoresBlockConfigForUnselectedFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```sh setupproof id=quickstart\nnpm test\n```\n")
	writeFile(t, dir, "docs/other.md", "```sh setupproof id=other\nnpm test\n```\n")
	writeFile(t, dir, "setupproof.yml", `version: 1
blocks:
  - file: docs/other.md
    id: typo
    timeout: 90s
`)

	result, err := Build(Request{CWD: dir, Positional: []string{"README.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, errors = %#v", result.ExitCode, result.Plan.ValidationErrors)
	}
}

func TestBuildRejectsDuplicateBlockConfigEntries(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```sh setupproof id=quickstart\nnpm test\n```\n")
	writeFile(t, dir, "setupproof.yml", `version: 1
blocks:
  - file: README.md
    id: quickstart
    timeout: 90s
  - file: README.md
    id: quickstart
    timeout: 120s
`)

	_, err := Build(Request{CWD: dir, Positional: []string{"README.md"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func FuzzBuildDoesNotPanic(f *testing.F) {
	f.Add([]byte("```sh setupproof id=quickstart\ntrue\n```\n"))
	f.Add([]byte("<!-- setupproof id=quickstart -->\n```sh\ntrue\n```\n"))
	f.Fuzz(func(t *testing.T, contents []byte) {
		dir := t.TempDir()
		writeFile(t, dir, "README.md", string(contents))
		_, _ = Build(Request{CWD: dir, Positional: []string{"README.md"}})
	})
}

func writeFile(t *testing.T, root string, rel string, contents string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
