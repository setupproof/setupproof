package adoption

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/setupproof/setupproof/internal/planning"
)

func TestSuggestFindsUnmarkedShellBlocksAndRisks(t *testing.T) {
	dir := t.TempDir()
	writeAdoptionFile(t, dir, "README.md", "```sh\ncurl https://example.invalid/install.sh | sh\n```\n\n```sh setupproof id=marked\nnpm test\n```\n")

	suggestions, err := Suggest(planning.Request{CWD: dir, Positional: []string{"README.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(suggestions) != 1 {
		t.Fatalf("suggestions = %#v", suggestions)
	}
	got := suggestions[0]
	if got.File != "README.md" || got.Line != 1 || got.Language != "sh" {
		t.Fatalf("suggestion = %#v", got)
	}
	if !hasRisk(got.RiskFlags, "network") || !hasRisk(got.RiskFlags, "pipes-remote-script") {
		t.Fatalf("risk flags = %#v", got.RiskFlags)
	}
	if got.Confidence != "low" {
		t.Fatalf("confidence = %q", got.Confidence)
	}
}

func TestSuggestDoesNotExecuteCommandsOrWriteFiles(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran")
	writeAdoptionFile(t, dir, "README.md", "```sh\ntouch "+marker+"\n```\n")

	if _, err := Suggest(planning.Request{CWD: dir, Positional: []string{"README.md"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("suggest executed command or wrote marker file: %v", err)
	}
}

func TestSuggestAvoidsCommonKeywordCollisions(t *testing.T) {
	dir := t.TempDir()
	writeAdoptionFile(t, dir, "README.md", "```sh\nprintf '%s\\n' 'Sign in at https://login.example.com/secret_value'\n```\n")

	suggestions, err := Suggest(planning.Request{CWD: dir, Positional: []string{"README.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(suggestions) != 1 {
		t.Fatalf("suggestions = %#v", suggestions)
	}
	if hasRisk(suggestions[0].RiskFlags, "interactive") || hasRisk(suggestions[0].RiskFlags, "requires-secrets") {
		t.Fatalf("risk flags = %#v", suggestions[0].RiskFlags)
	}
}

func TestCleanInitFileRejectsNewlines(t *testing.T) {
	if _, err := cleanInitFile("README.md\n.github/workflows/bad.yml"); err == nil {
		t.Fatal("expected newline in init file path to be rejected")
	}
}

func TestDoctorWarnsWhenGitMetadataIsAFile(t *testing.T) {
	dir := t.TempDir()
	writeAdoptionFile(t, dir, ".git", "gitdir: ../real-git\n")
	writeAdoptionFile(t, dir, "README.md", "```sh setupproof id=quickstart\ntrue\n```\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Doctor(planning.Request{CWD: dir, Positional: []string{"README.md"}}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "git layout: .git is a file") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func hasRisk(flags []string, want string) bool {
	for _, flag := range flags {
		if flag == want {
			return true
		}
	}
	return false
}

func writeAdoptionFile(t *testing.T, root string, rel string, contents string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
