package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/setupproof/setupproof/internal/shellquote"
)

type reportJSON struct {
	Kind     string   `json:"kind"`
	Result   string   `json:"result"`
	ExitCode int      `json:"exitCode"`
	Warnings []string `json:"warnings"`
	Runner   struct {
		Kind  string `json:"kind"`
		Error *struct {
			Reason string `json:"reason"`
		} `json:"error"`
	} `json:"runner"`
	Blocks []struct {
		ID               string `json:"id"`
		QualifiedID      string `json:"qualifiedId"`
		Runner           string `json:"runner"`
		Timeout          string `json:"timeout"`
		TimeoutMs        int64  `json:"timeoutMs"`
		Result           string `json:"result"`
		ExitCode         int    `json:"exitCode"`
		StdoutTail       string `json:"stdoutTail"`
		StderrTail       string `json:"stderrTail"`
		CleanupCompleted *bool  `json:"cleanupCompleted"`
		DurationMs       int64  `json:"durationMs"`
		Interactive      string `json:"interactiveCommand"`
		Truncated        struct {
			Stdout bool `json:"stdout"`
			Stderr bool `json:"stderr"`
		} `json:"truncated"`
	} `json:"blocks"`
}

func TestExecutionJSONReportKeepsStdoutCleanAndPreservesPlanBoundary(t *testing.T) {
	dir := gitRepoForCLI(t)
	writeCLIFile(t, dir, "README.md", "```sh setupproof id=quickstart\nprintf 'command-log\\n'\n```\n")
	runGit(t, dir, "add", "--", "README.md")
	chdir(t, dir)

	var dryStdout bytes.Buffer
	var dryStderr bytes.Buffer
	if code := Run([]string{"--dry-run", "--json", "README.md"}, &dryStdout, &dryStderr); code != 0 {
		t.Fatalf("dry-run exit code = %d, stderr = %q", code, dryStderr.String())
	}
	var plan map[string]any
	if err := json.Unmarshal(dryStdout.Bytes(), &plan); err != nil {
		t.Fatalf("dry-run stdout is not JSON: %v\n%s", err, dryStdout.String())
	}
	if plan["kind"] != "plan" {
		t.Fatalf("dry-run kind = %#v", plan["kind"])
	}
	block := plan["blocks"].([]any)[0].(map[string]any)
	if _, ok := block["result"]; ok {
		t.Fatalf("plan block included execution result: %s", dryStdout.String())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--json", "README.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "command-log") {
		t.Fatalf("command output was not routed to stderr: %q", stderr.String())
	}
	var report reportJSON
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not clean report JSON: %v\n%s", err, stdout.String())
	}
	if report.Kind != "report" || report.Result != "passed" || report.ExitCode != 0 {
		t.Fatalf("unexpected report summary: %#v", report)
	}
	if len(report.Blocks) != 1 || report.Blocks[0].Result != "passed" || report.Blocks[0].StdoutTail != "command-log\n" {
		t.Fatalf("unexpected block report: %#v", report.Blocks)
	}
	if report.Blocks[0].Runner != "local" || report.Blocks[0].Timeout != "120s" || report.Blocks[0].TimeoutMs != 120000 {
		t.Fatalf("stable execution fields missing from block report: %#v", report.Blocks[0])
	}
}

func TestExecutionReportsFailureAndTimeout(t *testing.T) {
	dir := gitRepoForCLI(t)
	writeCLIFile(t, dir, "README.md", "```sh setupproof id=fail\nexit 7\n```\n")
	runGit(t, dir, "add", "--", "README.md")
	chdir(t, dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--json", "README.md"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("failure exit code = %d, stderr = %q", code, stderr.String())
	}
	report := decodeReport(t, stdout.Bytes())
	if report.Result != "failed" || report.Blocks[0].Result != "failed" || report.Blocks[0].ExitCode != 7 {
		t.Fatalf("failure report = %#v", report)
	}

	writeCLIFile(t, dir, "README.md", "```sh setupproof id=timeout timeout=1s\nsleep 10\n```\n")
	runGit(t, dir, "add", "--", "README.md")
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"--json", "README.md"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("timeout exit code = %d, stderr = %q", code, stderr.String())
	}
	report = decodeReport(t, stdout.Bytes())
	if report.Result != "failed" || report.Blocks[0].Result != "timeout" || report.Blocks[0].ExitCode != 1 {
		t.Fatalf("timeout report = %#v", report)
	}
	if report.Blocks[0].CleanupCompleted == nil || !*report.Blocks[0].CleanupCompleted {
		t.Fatalf("timeout cleanup completion missing from report: %#v", report.Blocks[0])
	}
}

func TestTerminalFailureGuidance(t *testing.T) {
	dir := gitRepoForCLI(t)
	writeCLIFile(t, dir, "README.md", "```sh setupproof id=fail\nexit 7\n```\n")
	runGit(t, dir, "add", "--", "README.md")
	chdir(t, dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--no-color", "--no-glyphs", "README.md"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("failure exit code = %d, stderr = %q", code, stderr.String())
	}
	for _, want := range []string{
		"README.md#fail file=README.md:1 runner=local timeout=120s result=failed exit=7 reason=exit-code",
		"next command: setupproof review README.md",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("failure terminal output missing %q:\n%s", want, stdout.String())
		}
	}

	writeCLIFile(t, dir, "README.md", "```sh setupproof id=timeout timeout=1s\nsleep 10\n```\n")
	runGit(t, dir, "add", "--", "README.md")
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"--no-color", "--no-glyphs", "README.md"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("timeout exit code = %d, stderr = %q", code, stderr.String())
	}
	for _, want := range []string{
		"README.md#timeout file=README.md:1 runner=local timeout=1s result=timeout exit=1 reason=timeout",
		"next command: setupproof review README.md",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("timeout terminal output missing %q:\n%s", want, stdout.String())
		}
	}

	writeCLIFile(t, dir, "README.md", "```sh setupproof id=interactive\nread answer\n```\n")
	runGit(t, dir, "add", "--", "README.md")
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"--no-color", "--no-glyphs", "README.md"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("interactive exit code = %d, stderr = %q", code, stderr.String())
	}
	for _, want := range []string{
		"README.md#interactive file=README.md:1 runner=local timeout=120s result=failed exit=1 reason=interactive-command command=read",
		"next command: setupproof review README.md",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("interactive terminal output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestTerminalInfrastructureGuidance(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	dir := gitRepoForCLI(t)
	writeCLIFile(t, dir, "README.md", "```bash setupproof id=missing-shell\ntrue\n```\n")
	runGit(t, dir, "add", "--", "README.md")
	pathDir := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(pathDir, "git")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	t.Setenv("PATH", pathDir)
	chdir(t, dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--no-color", "--no-glyphs", "README.md"}, &stdout, &stderr)
	if code != 3 {
		t.Fatalf("infrastructure exit code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"README.md#missing-shell file=README.md:1 runner=local timeout=120s result=error reason=shell_unavailable",
		"next command: setupproof review README.md",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("infrastructure terminal output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestTerminalDockerStartupGuidanceIncludesBlockContext(t *testing.T) {
	dir := gitRepoForCLI(t)
	writeCLIFile(t, dir, "README.md", "```sh setupproof runner=docker image=missing:latest id=docker\ntrue\n```\n")
	runGit(t, dir, "add", "--", "README.md")
	installFakeDockerCLI(t, `case "$1" in
run)
  printf 'manifest unknown\n' >&2
  exit 125
  ;;
*)
  exit 0
  ;;
esac
`)
	chdir(t, dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--no-color", "--no-glyphs", "README.md"}, &stdout, &stderr)
	if code != 3 {
		t.Fatalf("docker infrastructure exit code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"README.md#docker file=README.md:1 runner=docker image=missing:latest timeout=120s result=error reason=image_pull_failed",
		"next command: setupproof review README.md",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("docker infrastructure terminal output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestExecutionInfrastructureReportIncludesRunnerError(t *testing.T) {
	dir := gitRepoForCLI(t)
	writeCLIFile(t, dir, "README.md", "```sh setupproof runner=docker image=missing:latest id=docker\ntrue\n```\n")
	runGit(t, dir, "add", "--", "README.md")
	installFakeDockerCLI(t, `case "$1" in
run)
  printf 'manifest unknown\n' >&2
  exit 125
  ;;
*)
  exit 0
  ;;
esac
`)
	chdir(t, dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--json", "README.md"}, &stdout, &stderr)
	if code != 3 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	report := decodeReport(t, stdout.Bytes())
	if report.Result != "error" || report.ExitCode != 3 {
		t.Fatalf("infrastructure report summary = %#v", report)
	}
	if report.Runner.Kind != "docker" || report.Runner.Error == nil || report.Runner.Error.Reason != "image_pull_failed" {
		t.Fatalf("runner error missing from report: %#v", report.Runner)
	}
	if len(report.Blocks) != 1 || report.Blocks[0].QualifiedID != "README.md#docker" || report.Blocks[0].Result != "error" {
		t.Fatalf("block context missing from infrastructure report: %#v", report.Blocks)
	}
	if report.Warnings == nil || report.Blocks == nil {
		t.Fatalf("report arrays must not be null: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), `"warnings":null`) || strings.Contains(stdout.String(), `"blocks":null`) {
		t.Fatalf("report contains null arrays: %s", stdout.String())
	}
}

func TestExecutionNoBlocksProducesNoopReport(t *testing.T) {
	dir := t.TempDir()
	writeCLIFile(t, dir, "README.md", "```sh\nprintf 'not marked\\n'\n```\n")
	chdir(t, dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--json", "README.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "warning: no marked blocks found") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	report := decodeReport(t, stdout.Bytes())
	if report.Result != "noop" || len(report.Blocks) != 0 {
		t.Fatalf("noop report = %#v", report)
	}
}

func TestExecutionReportsRedactSecretsAcrossSinks(t *testing.T) {
	dir := gitRepoForCLI(t)
	secret := "report-secret-value"
	t.Setenv("SETUPPROOF_REPORT_SECRET", secret)
	writeCLIFile(t, dir, "setupproof.yml", "version: 1\n"+
		"env:\n"+
		"  pass:\n"+
		"    - name: SETUPPROOF_REPORT_SECRET\n"+
		"      secret: true\n")
	writeCLIFile(t, dir, "README.md", "```sh setupproof id=secret\n"+
		"printf '%s\\n' \"$SETUPPROOF_REPORT_SECRET\"\n"+
		"printf '%s\\n' \"$SETUPPROOF_REPORT_SECRET\" >&2\n"+
		"```\n")
	runGit(t, dir, "add", "--", "README.md", "setupproof.yml")
	chdir(t, dir)

	jsonPath := filepath.Join(dir, "report.json")
	markdownPath := filepath.Join(dir, "report.md")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--json", "--report-json", jsonPath, "--report-file", markdownPath, "README.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	for name, contents := range map[string]string{
		"stdout":      stdout.String(),
		"stderr":      stderr.String(),
		"report.json": readCLIFile(t, jsonPath),
		"report.md":   readCLIFile(t, markdownPath),
	} {
		if strings.Contains(contents, secret) {
			t.Fatalf("%s leaked secret:\n%s", name, contents)
		}
		if !strings.Contains(contents, "[redacted]") {
			t.Fatalf("%s did not contain redacted value:\n%s", name, contents)
		}
	}
}

func TestExecutionReportTailsAreCapped(t *testing.T) {
	dir := gitRepoForCLI(t)
	writeCLIFile(t, dir, "README.md", "```sh setupproof id=big\nawk 'BEGIN { for (i = 0; i < 70000; i++) printf \"A\" }'\n```\n")
	runGit(t, dir, "add", "--", "README.md")
	chdir(t, dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--json", "README.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	report := decodeReport(t, stdout.Bytes())
	if got := len([]byte(report.Blocks[0].StdoutTail)); got != 64*1024 {
		t.Fatalf("stdout tail length = %d", got)
	}
	if !report.Blocks[0].Truncated.Stdout {
		t.Fatalf("stdout truncation flag was false: %#v", report.Blocks[0].Truncated)
	}
}

func TestMarkdownReportFileStripsANSIAndKeepsTerminalReport(t *testing.T) {
	dir := gitRepoForCLI(t)
	writeCLIFile(t, dir, "README.md", "```sh setupproof id=color\nprintf '\\033[31mred\\033[0m\\n'\n```\n")
	runGit(t, dir, "add", "--", "README.md")
	chdir(t, dir)

	reportPath := filepath.Join(dir, "report.md")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--report-file", reportPath, "README.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "README.md#color") || !strings.Contains(stdout.String(), "result=passed") {
		t.Fatalf("terminal report missing when --report-file is used: %q", stdout.String())
	}
	markdown := readCLIFile(t, reportPath)
	if strings.Contains(markdown, "\x1b[31m") || strings.Contains(markdown, "\x1b[0m") {
		t.Fatalf("markdown report kept ANSI escapes:\n%s", markdown)
	}
	if !strings.Contains(markdown, "red") || !strings.Contains(markdown, "result: passed") {
		t.Fatalf("markdown report missing expected content:\n%s", markdown)
	}
}

func TestReportPathValidationRejectsDirectoryAndSymlinkBeforeExecution(t *testing.T) {
	dir := gitRepoForCLI(t)
	marker := filepath.Join(dir, "ran")
	writeCLIFile(t, dir, "README.md", "```sh setupproof id=run\ntouch "+shellquote.Quote(marker)+"\n```\n")
	runGit(t, dir, "add", "--", "README.md")
	chdir(t, dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--report-file", dir, "README.md"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("directory report path exit code = %d", code)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("command executed despite invalid directory report path: %v", err)
	}

	target := filepath.Join(dir, "target.json")
	link := filepath.Join(dir, "link.json")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"--report-json", link, "README.md"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("symlink report path exit code = %d", code)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("command executed despite invalid symlink report path: %v", err)
	}
}

func TestReportPathValidationRejectsCollidingReportSinks(t *testing.T) {
	dir := gitRepoForCLI(t)
	marker := filepath.Join(dir, "ran")
	writeCLIFile(t, dir, "README.md", "```sh setupproof id=run\ntouch "+shellquote.Quote(marker)+"\n```\n")
	runGit(t, dir, "add", "--", "README.md")
	chdir(t, dir)

	reportPath := filepath.Join(dir, "report.out")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--report-json", reportPath, "--report-file", reportPath, "README.md"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("colliding report path exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "must use different paths") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("command executed despite colliding report paths: %v", err)
	}
}

func TestDryRunRejectsMarkdownReportSink(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--dry-run", "--json", "--report-file", "report.md", "README.md"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "cannot be combined") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func decodeReport(t *testing.T, data []byte) reportJSON {
	t.Helper()
	var report reportJSON
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("report JSON decode failed: %v\n%s", err, string(data))
	}
	return report
}

func gitRepoForCLI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	return dir
}

func writeCLIFile(t *testing.T, root string, rel string, contents string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readCLIFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

func installFakeDockerCLI(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "docker")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
