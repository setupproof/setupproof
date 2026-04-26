package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestListPrintsMarkedBlocks(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	path := filepath.Join(dir, "README.md")
	contents := []byte("<!-- setupproof id=quickstart -->\n```sh\nnpm test\n```\n\n```sh\nignored\n```\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--list", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "README.md:2 id=quickstart language=sh marker=html-comment") {
		t.Fatalf("unexpected output: %q", output)
	}
	if strings.Contains(output, "ignored") {
		t.Fatalf("unmarked block leaked into list output: %q", output)
	}
}

func TestListDoesNotExecuteCommands(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	marker := filepath.Join(dir, "ran")
	path := filepath.Join(dir, "README.md")
	contents := []byte("```sh setupproof id=quickstart\ntouch " + marker + "\n```\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--list", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("--list executed command or created marker file: %v", err)
	}
}

func TestListReportsNoMarkedBlocks(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	path := filepath.Join(dir, "README.md")
	if err := os.WriteFile(path, []byte("```sh\nnpm test\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--list", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.String() != "No marked blocks found.\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "warning: no marked blocks found") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestListUsesNoArgumentReadmeResolution(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := os.WriteFile("README.md", []byte("```sh setupproof id=quickstart\nnpm test\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout.String(), "README.md:1 id=quickstart language=sh marker=info-string") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "setupproof ") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestHelpDiscoveryAliasesAndFlags(t *testing.T) {
	for _, args := range [][]string{
		{"help"},
		{"README.md", "--help"},
		{"--help", "README.md"},
	} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run(args, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("Run(%v) exit code = %d, stderr = %q", args, code, stderr.String())
		}
		output := stdout.String()
		for _, want := range []string{
			"Usage:",
			"--config <path>",
			"--report-json <path>",
			"--report-file <path>",
			"--network <true|false>",
			"--include-untracked",
			"--keep-workspace",
			"--no-color",
			"--no-glyphs",
			"local, action-local, or docker",
		} {
			if !strings.Contains(output, want) {
				t.Fatalf("help output missing %q:\n%s", want, output)
			}
		}
	}
}

func TestReportGitHubStepSummaryCommand(t *testing.T) {
	input := `{"kind":"report","schemaVersion":"1.0.0","setupproofVersion":"0.1.0","startedAt":"2026-04-24T00:00:00Z","durationMs":7,"result":"failed","exitCode":1,"invocation":{"args":[]},"workspace":{"mode":"temporary","source":"tracked","includedUntracked":false},"runner":{"kind":"local","workspace":"temporary","networkPolicy":"host","networkEnforced":false},"files":["README.md"],"warnings":[],"blocks":[{"id":"fail","qualifiedId":"README.md#fail","file":"README.md","line":1,"language":"sh","shell":"sh","source":"false","strict":true,"stdin":"closed","tty":false,"stateMode":"shared","isolated":false,"runner":"local","timeout":"120s","timeoutMs":120000,"result":"failed","exitCode":1,"reason":"exit-code","durationMs":1,"stdoutTail":"","stderrTail":"","truncated":{"stdout":false,"stderr":false}}]}`
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = reader.Close()
	})
	if _, err := writer.WriteString(input); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"report", "--format=github-step-summary", "--mode", "run", "--status", "1", "--report-json", "report.json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	for _, want := range []string{"result: failed", "Failing Blocks", "README.md#fail", "exit-code"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("summary missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestDryRunJSONUsesConfigAndKeepsStdoutClean(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := os.WriteFile("README.md", []byte("```sh setupproof id=quickstart timeout=30s\nnpm test\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("setupproof.yml", []byte("version: 1\nfiles:\n  - README.md\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q", stderr.String())
	}

	var plan struct {
		Kind   string   `json:"kind"`
		Files  []string `json:"files"`
		Blocks []struct {
			ID      string `json:"id"`
			Options struct {
				TimeoutMs int64 `json:"timeoutMs"`
			} `json:"options"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if plan.Kind != "plan" {
		t.Fatalf("kind = %q", plan.Kind)
	}
	if len(plan.Files) != 1 || plan.Files[0] != "README.md" {
		t.Fatalf("files = %#v", plan.Files)
	}
	if len(plan.Blocks) != 1 || plan.Blocks[0].ID != "quickstart" || plan.Blocks[0].Options.TimeoutMs != 30000 {
		t.Fatalf("blocks = %#v", plan.Blocks)
	}
}

func TestDryRunDoesNotExecuteCommands(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	marker := filepath.Join(dir, "ran")
	if err := os.WriteFile("README.md", []byte("```sh setupproof id=quickstart\n"+
		"touch "+marker+"\n"+
		"```\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--dry-run", "--json", "README.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("--dry-run executed command or created marker file: %v", err)
	}
}

func TestDryRunIncludesUntrackedFlagInPlan(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := os.WriteFile("README.md", []byte("```sh setupproof id=quickstart\ntrue\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--dry-run", "--json", "--include-untracked", "README.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	var plan struct {
		Workspace struct {
			IncludedUntracked bool `json:"includedUntracked"`
		} `json:"workspace"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if !plan.Workspace.IncludedUntracked {
		t.Fatalf("includedUntracked = false in plan: %s", stdout.String())
	}
}

func TestDryRunRequireBlocksReturnsExitCodeFour(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := os.WriteFile("README.md", []byte("```sh\nnpm test\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--dry-run", "--json", "--require-blocks"}, &stdout, &stderr)
	if code != 4 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "error: no marked blocks found") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	var plan map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
}

func TestDryRunRejectsInvalidTimeout(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := os.WriteFile("README.md", []byte("```sh setupproof id=quickstart\nnpm test\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--dry-run", "--json", "--timeout=0", "README.md"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "timeout") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestDryRunRejectsInvalidNetworkValue(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--dry-run", "--json", "--network=maybe", "README.md"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--network must be true or false") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestDryRunRejectsConfigPathEscape(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	outside := filepath.Join(t.TempDir(), "setupproof.yml")
	if err := os.WriteFile(outside, []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--dry-run", "--json", "--config", outside, "README.md"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "outside the repository root") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestDryRunRejectsConfigSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	outside := filepath.Join(t.TempDir(), "setupproof.yml")
	if err := os.WriteFile(outside, []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, "linked-setupproof.yml"); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--dry-run", "--json", "--config", "linked-setupproof.yml", "README.md"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "outside the repository root") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestDryRunRejectsReportSinks(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--dry-run", "--json", "--report-json", "plan.json", "README.md"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "use --dry-run --json > plan.json") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestListJSONErrorSuggestsDryRunJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--list", "--json", "README.md"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "use --dry-run --json") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestDefaultCommandRunsLocalRunnerInGitWorktree(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	runGit(t, dir, "init")
	if err := os.WriteFile("README.md", []byte("```sh setupproof id=quickstart\ntrue\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "--", "README.md")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"README.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "README.md#quickstart") || !strings.Contains(stdout.String(), "result=passed") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestDefaultCommandRunsActionLocalRunnerInGitWorktree(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	runGit(t, dir, "init")
	if err := os.WriteFile("README.md", []byte("```sh setupproof id=quickstart\ntrue\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "--", "README.md")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--runner=action-local", "README.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "README.md#quickstart") ||
		!strings.Contains(stdout.String(), "runner=action-local") ||
		!strings.Contains(stdout.String(), "result=passed") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestSuggestPrintsCandidatesAndDoesNotExecute(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	marker := "ran"
	if err := os.WriteFile("README.md", []byte("```sh\ntouch "+marker+"\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"suggest", "README.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "suggested canonical marker: <!-- setupproof id=line-1 -->") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("suggest executed command or wrote marker file: %v", err)
	}
}

func TestReviewShowsExecutionSemanticsWithoutExecuting(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	marker := filepath.Join(dir, "ran")
	if err := os.WriteFile("README.md", []byte("```sh setupproof id=quickstart\ntouch "+marker+"\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("setupproof.yml", []byte("version: 1\nenv:\n  allow:\n    - NODE_ENV\n  pass:\n    - name: SDK_API_KEY\n      secret: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"review", "README.md", "--report-file", "report.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"README.md#quickstart",
		"workspace copy mode: tracked-plus-modified",
		"stdin mode: closed",
		"tty mode: false",
		"state mode: shared",
		"environment variables: NODE_ENV, SDK_API_KEY",
		"secret environment variables: SDK_API_KEY",
		"report sinks: markdown:report.md",
		"touch " + marker,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("review output missing %q:\n%s", want, output)
		}
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("review executed command or wrote marker file: %v", err)
	}
}

func TestReviewShowsDockerImageWithoutExecuting(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	marker := filepath.Join(dir, "ran")
	if err := os.WriteFile("README.md", []byte("```sh setupproof id=quickstart runner=docker image=ubuntu@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n"+
		"touch "+marker+"\n"+
		"```\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"review", "README.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "docker image: ubuntu@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("review executed command or wrote marker file: %v", err)
	}
}

func TestDoctorRunsNonExecutingChecks(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := os.WriteFile("README.md", []byte("```sh setupproof id=quickstart\nnpm test\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"doctor", "README.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	for _, want := range []string{"root:", "temporary workspace: writable", "targets: 1", "marked blocks: 1", "platform:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("doctor output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestDoctorChecksDockerWhenConfiguredWithoutRunningBlocks(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	marker := filepath.Join(dir, "ran")
	if err := os.WriteFile("README.md", []byte("```sh setupproof id=quickstart runner=docker image=ubuntu:24.04\n"+
		"touch "+marker+"\n"+
		"```\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dockerLog := filepath.Join(dir, "docker.log")
	installFakeDockerCLI(t, `printf '%s\n' "$*" >> "$FAKE_DOCKER_LOG"
case "$1" in
info)
  exit 0
  ;;
*)
  exit 1
  ;;
esac
`)
	t.Setenv("FAKE_DOCKER_LOG", dockerLog)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"doctor", "README.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "docker: configured; daemon reachable") {
		t.Fatalf("doctor output missing docker check:\n%s", stdout.String())
	}
	log, err := os.ReadFile(dockerLog)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(log)) != "info" {
		t.Fatalf("doctor should only check docker info, log = %q", string(log))
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("doctor executed marked command: %v", err)
	}
}

func TestDoctorReportsExplicitConfigPath(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := os.WriteFile("README.md", []byte("```sh setupproof id=quickstart\ntrue\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("custom.yml", []byte("version: 1\nfiles:\n  - README.md\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("setupproof.yml", []byte("version: 1\nfiles:\n  - missing.md\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"doctor", "--config", "custom.yml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "config: custom.yml") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestInitCheckDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := os.WriteFile("README.md", []byte("```sh setupproof id=quickstart\nnpm test\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"init", "--check"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "workflow write: not attempted") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if _, err := os.Stat("setupproof.yml"); !os.IsNotExist(err) {
		t.Fatalf("init --check wrote config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(".github", "workflows")); !os.IsNotExist(err) {
		t.Fatalf("init --check wrote workflow path: %v", err)
	}
}

func TestInitWritesConservativeConfigOnly(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"init"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	config := readCLIFile(t, filepath.Join(dir, "setupproof.yml"))
	for _, want := range []string{
		"version: 1\n",
		"defaults:\n",
		"  runner: local\n",
		"  timeout: 120s\n",
		"  requireBlocks: true\n",
		"  strict: true\n",
		"  isolated: false\n",
		"files:\n",
		"  - README.md\n",
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("config missing %q:\n%s", want, config)
		}
	}
	if strings.Contains(config, "env:") || strings.Contains(config, "pass:") {
		t.Fatalf("config should not include env passthrough:\n%s", config)
	}
	if !strings.Contains(stdout.String(), "wrote setupproof.yml") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "no marked blocks detected; next command: setupproof suggest README.md") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(".github", "workflows")); !os.IsNotExist(err) {
		t.Fatalf("init without --workflow wrote workflow path: %v", err)
	}
}

func TestInitNextCommandReviewsMarkedReadme(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeCLIFile(t, dir, "README.md", "```sh setupproof id=quickstart\ntrue\n```\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"init"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "next command: setupproof review README.md") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestInitRefusesOverwriteUnlessForced(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := os.WriteFile("setupproof.yml", []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"init"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "already exists; pass --force") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if got := readCLIFile(t, filepath.Join(dir, "setupproof.yml")); got != "version: 1\n" {
		t.Fatalf("config was overwritten without --force:\n%s", got)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"init", "--force", "docs/setup.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("forced init exit code = %d, stderr = %q", code, stderr.String())
	}
	if got := readCLIFile(t, filepath.Join(dir, "setupproof.yml")); !strings.Contains(got, "  - docs/setup.md\n") {
		t.Fatalf("forced config did not use explicit file:\n%s", got)
	}
}

func TestInitWorkflowPrintsConservativeWorkflowOnly(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"init", "--workflow", "--print"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"pull_request:",
		"permissions:\n  contents: read",
		"runs-on: ubuntu-24.04",
		"docs/adr/0009-github-actions-checkout-strategy.md",
		"go build -o \"$RUNNER_TEMP/setupproof\" ./cmd/setupproof",
		"uses: ./",
		"mode: review",
		"require-blocks: \"true\"",
		"cli-path: ${{ runner.temp }}/setupproof",
		"files: |\n            README.md",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("workflow output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "@v1") {
		t.Fatalf("workflow should not advertise moving action tags:\n%s", output)
	}
	if strings.Contains(output, "github.token") {
		t.Fatalf("workflow should not depend on the default token:\n%s", output)
	}
	if _, err := os.Stat("setupproof.yml"); !os.IsNotExist(err) {
		t.Fatalf("init --workflow --print wrote config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(".github", "workflows")); !os.IsNotExist(err) {
		t.Fatalf("init --workflow --print wrote workflow path: %v", err)
	}
}

func TestInitWorkflowRejectsDownstreamRoot(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"init", "--workflow"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "init --workflow is source-tree-only") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if _, err := os.Stat("setupproof.yml"); !os.IsNotExist(err) {
		t.Fatalf("init wrote config after workflow root check failed: %v", err)
	}
}

func TestInitWorkflowWritesOnlyWhenRequestedAndPreflightsOverwrites(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeSourceTreeWorkflowFixture(t, dir)
	if err := os.MkdirAll(filepath.Join(".github", "workflows"), 0o700); err != nil {
		t.Fatal(err)
	}
	workflowPath := filepath.Join(".github", "workflows", "setupproof.yml")
	if err := os.WriteFile(workflowPath, []byte("existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"init", "--workflow"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "already exists; pass --force") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if _, err := os.Stat("setupproof.yml"); !os.IsNotExist(err) {
		t.Fatalf("init wrote config after workflow preflight failed: %v", err)
	}
	if got := readCLIFile(t, workflowPath); got != "existing\n" {
		t.Fatalf("workflow overwritten without --force:\n%s", got)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"init", "--workflow", "--force"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("forced workflow init exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(readCLIFile(t, workflowPath), "require-blocks: \"true\"") {
		t.Fatalf("workflow missing explicit require-blocks:\n%s", readCLIFile(t, workflowPath))
	}
	if !strings.Contains(stderr.String(), "generated workflow is source-tree-only") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "wrote setupproof.yml") || !strings.Contains(stdout.String(), "wrote .github/workflows/setupproof.yml") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func writeSourceTreeWorkflowFixture(t *testing.T, root string) {
	t.Helper()
	writeCLIFile(t, root, "action.yml", "name: SetupProof\n")
	writeCLIFile(t, root, filepath.Join("cmd", "setupproof", "main.go"), "package main\n")
	writeCLIFile(t, root, filepath.Join("scripts", "github-action.sh"), "#!/usr/bin/env bash\n")
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	// These tests intentionally avoid t.Parallel because process cwd is global.
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatal(err)
		}
	})
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}
