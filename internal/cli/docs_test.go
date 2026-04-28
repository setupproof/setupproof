package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallDocCISnippetsAndDeferredClaims(t *testing.T) {
	root := repositoryRoot(t)
	installPath := filepath.Join(root, "docs", "INSTALL.md")
	data, err := os.ReadFile(installPath)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(data)

	for _, want := range []string{
		"SetupProof v0.1 runs from this source tree.",
		"Native Windows execution is unsupported in v0.1",
		"SetupProof through WSL2",
		"PowerShell fenced blocks are unsupported in v0.1.",
		"`sh`, `bash`, or\n  `shell` fenced blocks",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("docs/INSTALL.md missing %q", want)
		}
	}

	for _, forbidden := range []string{
		"pull_request_target",
		"${{ secrets.",
		"@" + "v1",
		"npx setup" + "proof",
		"npm install -g setup" + "proof",
		"npm i -g setup" + "proof",
		"brew install setup" + "proof",
		"winget install setup" + "proof",
		"choco install setup" + "proof",
		"scoop install setup" + "proof",
	} {
		if strings.Contains(doc, forbidden) {
			t.Fatalf("docs/INSTALL.md contains deferred or unsafe claim %q", forbidden)
		}
	}

	github := docSnippet(t, doc, "github-actions")
	for _, want := range []string{
		"pull_request:",
		"permissions:\n  contents: read",
		"timeout-minutes: 10",
		"uses: ./",
		"cli-path: ${{ runner.temp }}/setupproof",
		"mode: review",
		"require-blocks: \"true\"",
	} {
		if !strings.Contains(github, want) {
			t.Fatalf("GitHub Actions snippet missing %q:\n%s", want, github)
		}
	}
	for _, forbidden := range []string{"pull_request_target", "${{ secrets.", "@" + "v1"} {
		if strings.Contains(github, forbidden) {
			t.Fatalf("GitHub Actions snippet contains %q:\n%s", forbidden, github)
		}
	}

	for _, label := range []string{"gitlab-ci", "circleci", "generic-shell"} {
		snippet := docSnippet(t, doc, label)
		for _, want := range []string{
			"setupproof review README.md",
			"setupproof --report-json setupproof-report.json --require-blocks --no-color --no-glyphs README.md",
		} {
			if !strings.Contains(snippet, want) {
				t.Fatalf("%s snippet missing %q:\n%s", label, want, snippet)
			}
		}
		for _, forbidden := range []string{"uses:", "pull_request_target", "${{ secrets.", "@" + "v1"} {
			if strings.Contains(snippet, forbidden) {
				t.Fatalf("%s snippet contains %q:\n%s", label, forbidden, snippet)
			}
		}
	}
}

func TestAgentProtocolDocsDoNotDrift(t *testing.T) {
	root := repositoryRoot(t)
	for _, rel := range []string{"llms.txt", filepath.Join("docs", "AGENT_USAGE.md")} {
		path := filepath.Join(root, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		doc := string(data)
		for _, want := range []string{
			"setupproof --list README.md",
			"setupproof review README.md",
			"setupproof --dry-run --json --require-blocks README.md",
			"setupproof --require-blocks --no-color --no-glyphs README.md",
			"Never execute unmarked Markdown shell blocks as SetupProof targets.",
			"schemas/setupproof-plan.schema.json",
			"schemas/setupproof-report.schema.json",
			"schemas/setupproof-config.schema.json",
			"`0`:",
			"`1`:",
			"`2`:",
			"`3`:",
			"`4`:",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing agent protocol text %q", rel, want)
			}
		}
	}
}

func docSnippet(t *testing.T, doc string, label string) string {
	t.Helper()
	marker := "<!-- ci-snippet:" + label + " -->"
	start := strings.Index(doc, marker)
	if start < 0 {
		t.Fatalf("missing snippet marker %s", marker)
	}
	rest := doc[start+len(marker):]
	open := strings.Index(rest, "```")
	if open < 0 {
		t.Fatalf("missing opening fence after %s", marker)
	}
	afterOpen := rest[open+3:]
	if newline := strings.Index(afterOpen, "\n"); newline >= 0 {
		afterOpen = afterOpen[newline+1:]
	} else {
		t.Fatalf("missing fenced block body after %s", marker)
	}
	close := strings.Index(afterOpen, "```")
	if close < 0 {
		t.Fatalf("missing closing fence after %s", marker)
	}
	return afterOpen[:close]
}
