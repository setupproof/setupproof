package report

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/setupproof/setupproof/internal/planning"
)

func TestStreamCollectorRedactsSecretsAcrossWrites(t *testing.T) {
	secret := "split-secret-value"
	var sink bytes.Buffer
	tail := NewTail(MaxTailBytes)
	collector := &StreamCollector{
		Sink:     &sink,
		Tail:     tail,
		Redactor: NewRedactor([]string{secret}),
	}

	if _, err := collector.Write([]byte("before split-")); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.Write([]byte("secret-value after")); err != nil {
		t.Fatal(err)
	}
	if err := collector.Flush(); err != nil {
		t.Fatal(err)
	}

	for name, contents := range map[string]string{
		"sink": sink.String(),
		"tail": tail.String(),
	} {
		if strings.Contains(contents, secret) {
			t.Fatalf("%s leaked split secret: %q", name, contents)
		}
		if !strings.Contains(contents, "[redacted]") {
			t.Fatalf("%s did not redact split secret: %q", name, contents)
		}
	}
}

func TestStreamCollectorRedactsSecretAtSafeLimitBoundary(t *testing.T) {
	secret := "abcdef"
	var sink bytes.Buffer
	tail := NewTail(MaxTailBytes)
	collector := &StreamCollector{
		Sink:     &sink,
		Tail:     tail,
		Redactor: NewRedactor([]string{secret}),
	}

	if _, err := collector.Write([]byte("xxabc")); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.Write([]byte("defyy")); err != nil {
		t.Fatal(err)
	}
	if err := collector.Flush(); err != nil {
		t.Fatal(err)
	}

	for name, contents := range map[string]string{
		"sink": sink.String(),
		"tail": tail.String(),
	} {
		if strings.Contains(contents, secret) {
			t.Fatalf("%s leaked boundary secret: %q", name, contents)
		}
		if contents != "xx[redacted]yy" {
			t.Fatalf("%s = %q", name, contents)
		}
	}
}

func TestStreamCollectorFlushesCompleteLinesBeforeLongSecretSuffix(t *testing.T) {
	secret := strings.Repeat("s", 64)
	var sink bytes.Buffer
	collector := &StreamCollector{
		Sink:     &sink,
		Tail:     NewTail(MaxTailBytes),
		Redactor: NewRedactor([]string{secret}),
	}

	if _, err := collector.Write([]byte("short line\n")); err != nil {
		t.Fatal(err)
	}
	if got := sink.String(); got != "short line\n" {
		t.Fatalf("sink after first write = %q", got)
	}

	if _, err := collector.Write([]byte(strings.Repeat("s", 32))); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.Write([]byte(strings.Repeat("s", 32))); err != nil {
		t.Fatal(err)
	}
	if err := collector.Flush(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sink.String(), secret) {
		t.Fatalf("sink leaked secret: %q", sink.String())
	}
}

func TestRedactorPrefersLongestOverlappingSecret(t *testing.T) {
	redactor := NewRedactor([]string{"TOKEN", "TOKENABCDEF"})

	got := redactor.Redact("value=TOKENABCDEF")
	if got != "value=[redacted]" {
		t.Fatalf("redacted value = %q", got)
	}
}

func TestStreamCollectorRedactsShellMetaSecretAndStripsCarriageReturns(t *testing.T) {
	secret := "a$b"
	var sink bytes.Buffer
	tail := NewTail(MaxTailBytes)
	collector := &StreamCollector{
		Sink:     &sink,
		Tail:     tail,
		Redactor: NewRedactor([]string{secret}),
	}

	if _, err := collector.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.Write([]byte("a$by\rprogress")); err != nil {
		t.Fatal(err)
	}
	if err := collector.Flush(); err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string]string{"sink": sink.String(), "tail": tail.String()} {
		if strings.Contains(contents, secret) {
			t.Fatalf("%s leaked shell-meta secret: %q", name, contents)
		}
		if strings.Contains(contents, "\r") {
			t.Fatalf("%s retained carriage return: %q", name, contents)
		}
		if contents != "x[redacted]yprogress" {
			t.Fatalf("%s = %q", name, contents)
		}
	}
}

func TestRenderMarkdownUsesFenceLongerThanContent(t *testing.T) {
	rendered := RenderMarkdown(Report{
		Result:    "passed",
		ExitCode:  0,
		Workspace: planning.Workspace{Mode: "temporary"},
		Runner:    planning.Runner{Kind: "local"},
		Blocks: []Block{{
			ID:          "fence",
			QualifiedID: "README.md#fence",
			File:        "README.md",
			Line:        1,
			Language:    "sh",
			Shell:       "sh",
			Source:      "printf '````'\n",
			Strict:      true,
			Stdin:       "closed",
			StateMode:   "shared",
			Runner:      "local",
			Timeout:     "120s",
			Result:      "passed",
			StdoutTail:  "````\n",
			Truncated:   Truncated{},
		}},
	}, MarkdownOptions{})

	if !strings.Contains(rendered, "`````\nprintf '````'\n`````") {
		t.Fatalf("source fence was not widened:\n%s", rendered)
	}
	if !strings.Contains(rendered, "`````text\n````\n`````") {
		t.Fatalf("tail fence was not widened:\n%s", rendered)
	}
}

func TestRenderTerminalOptionsControlStatusPrefix(t *testing.T) {
	r := Report{
		Result:    "failed",
		ExitCode:  1,
		Workspace: planning.Workspace{Mode: "temporary"},
		Runner:    planning.Runner{Kind: "local"},
		Blocks: []Block{{
			ID:          "fail",
			QualifiedID: "README.md#fail",
			File:        "README.md",
			Line:        1,
			Runner:      "local",
			Timeout:     "120s",
			Result:      "failed",
			ExitCode:    1,
			Reason:      "exit-code",
		}},
	}

	var colored bytes.Buffer
	if err := RenderTerminal(&colored, r, TerminalOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(colored.String(), "\x1b[31m!") {
		t.Fatalf("colored terminal output missing failure prefix: %q", colored.String())
	}

	var plain bytes.Buffer
	if err := RenderTerminal(&plain, r, TerminalOptions{NoColor: true, NoGlyphs: true}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain.String(), "\x1b[") {
		t.Fatalf("plain terminal output contained ANSI escapes: %q", plain.String())
	}
	if !strings.Contains(plain.String(), "[failed] README.md#fail") {
		t.Fatalf("plain terminal output missing text status prefix: %q", plain.String())
	}
}

func TestRenderGitHubStepSummaryIncludesFailingBlocks(t *testing.T) {
	r := &Report{
		Result:     "failed",
		ExitCode:   1,
		DurationMs: 42,
		Files:      []string{"README.md"},
		Blocks: []Block{{
			ID:          "fail",
			QualifiedID: "README.md#fail",
			File:        "README.md",
			Result:      "failed",
			ExitCode:    1,
			Reason:      "exit-code",
		}},
	}

	summary := RenderGitHubStepSummary(r, StepSummaryOptions{Mode: "run", Status: 1, ReportJSONPath: "report.json"})
	for _, want := range []string{"result: failed", "### Failing Blocks", "README.md#fail", "exit-code"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q:\n%s", want, summary)
		}
	}
}

func TestRenderGitHubStepSummaryReviewModeDoesNotNeedReport(t *testing.T) {
	summary := RenderGitHubStepSummary(nil, StepSummaryOptions{Mode: "review", Status: 0, Files: []string{"README.md"}})
	for _, want := range []string{"mode: review", "Review mode is non-executing.", "files: README.md"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q:\n%s", want, summary)
		}
	}
}

func TestWriteResolvedFileTightensExistingPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteResolvedFile(path, []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != `{"ok":true}` {
		t.Fatalf("contents = %q", string(contents))
	}
}

func TestWriteResolvedFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	link := filepath.Join(dir, "report.json")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := WriteResolvedFile(link, []byte(`{"ok":true}`)); err == nil {
		t.Fatal("expected symlink report path to be rejected")
	}
}
