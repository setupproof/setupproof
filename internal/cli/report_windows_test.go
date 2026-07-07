//go:build windows

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeWindowsUnsupportedExecutionReportStaysSchemaCompatible(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "")
	dir := gitRepoForCLI(t)
	writeCLIFile(t, dir, filepath.ToSlash(filepath.Join("docs", "README.md")), "```sh setupproof id=quickstart\ntrue\n```\n")
	runGit(t, dir, "add", "--", filepath.ToSlash(filepath.Join("docs", "README.md")))
	if err := os.Mkdir(filepath.Join(dir, "reports"), 0o700); err != nil {
		t.Fatal(err)
	}
	chdir(t, dir)

	reportPath := filepath.Join("reports", "setupproof-report.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--json", "--report-json", reportPath, "--no-color", "--no-glyphs", "docs/README.md"}, &stdout, &stderr)
	if code != 3 {
		t.Fatalf("exit code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "native Windows execution is unsupported in v0.1") {
		t.Fatalf("stderr missing native Windows boundary: %q", stderr.String())
	}
	validateJSONSchema(t, "schemas/setupproof-report.schema.json", stdout.Bytes())

	reportFile := filepath.Join(dir, reportPath)
	reportBytes, err := os.ReadFile(reportFile)
	if err != nil {
		t.Fatal(err)
	}
	validateJSONSchema(t, "schemas/setupproof-report.schema.json", reportBytes)
	if !bytes.Equal(stdout.Bytes(), reportBytes) {
		t.Fatalf("stdout report and file report differ\nstdout=%s\nfile=%s", stdout.String(), string(reportBytes))
	}

	var parsed struct {
		Result    string   `json:"result"`
		ExitCode  int      `json:"exitCode"`
		Files     []string `json:"files"`
		Workspace struct {
			Mode              string `json:"mode"`
			Source            string `json:"source"`
			IncludedUntracked bool   `json:"includedUntracked"`
		} `json:"workspace"`
		Runner struct {
			Kind  string `json:"kind"`
			Error struct {
				Reason string `json:"reason"`
			} `json:"error"`
		} `json:"runner"`
		Blocks []json.RawMessage `json:"blocks"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Result != "error" || parsed.ExitCode != 3 || parsed.Runner.Error.Reason != "unsupported_platform" {
		t.Fatalf("unsupported platform report = %#v", parsed)
	}
	if parsed.Runner.Kind != "local" || parsed.Workspace.Mode != "temporary" || parsed.Workspace.Source != "tracked-plus-modified" {
		t.Fatalf("runner/workspace report = %#v", parsed)
	}
	if len(parsed.Files) != 1 || parsed.Files[0] != "docs/README.md" || strings.Contains(parsed.Files[0], `\`) {
		t.Fatalf("report files must stay slash-normalized: %#v", parsed.Files)
	}
	if len(parsed.Blocks) != 0 {
		t.Fatalf("native Windows unsupported report should not include executed block reports: %#v", parsed.Blocks)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"--no-color", "--no-glyphs", "docs/README.md"}, &stdout, &stderr)
	if code != 3 {
		t.Fatalf("terminal exit code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"SetupProof runner error runner=local reason=unsupported_platform",
		"next command: setupproof doctor docs/README.md",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("terminal output missing %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), `\`) {
		t.Fatalf("terminal output should keep repository path slash-normalized:\n%s", stdout.String())
	}
}
