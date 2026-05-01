package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

var exampleMarkdownFiles = []string{
	"examples/node-npm/README.md",
	"examples/python-pip/README.md",
	"examples/docker-compose/README.md",
	"examples/monorepo/docs/web.md",
	"examples/monorepo/docs/api.md",
	"examples/go/README.md",
	"examples/rust/README.md",
}

func TestExamplesPlanListAndReviewWithoutExecuting(t *testing.T) {
	root := repositoryRoot(t)
	chdir(t, root)

	args := append([]string{"--dry-run", "--json", "--require-blocks"}, exampleMarkdownFiles...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dry-run code = %d, stderr = %q", code, stderr.String())
	}

	plan := decodeExamplePlan(t, stdout.Bytes())
	if plan.Kind != "plan" {
		t.Fatalf("kind = %q", plan.Kind)
	}

	wantIDs := map[string]string{
		"node-npm-test":        "examples/node-npm/README.md",
		"python-pip-test":      "examples/python-pip/README.md",
		"docker-compose-smoke": "examples/docker-compose/README.md",
		"web-package":          "examples/monorepo/docs/web.md",
		"api-service":          "examples/monorepo/docs/api.md",
		"go-test":              "examples/go/README.md",
		"rust-cargo-test":      "examples/rust/README.md",
	}
	gotIDs := map[string]string{}
	for _, block := range plan.Blocks {
		gotIDs[block.ID] = block.File
		if strings.TrimSpace(block.Source) == "" {
			t.Fatalf("%s has empty source", block.ID)
		}
		if block.Options.Runner != "local" {
			t.Fatalf("%s runner = %q", block.ID, block.Options.Runner)
		}
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("example blocks = %#v", gotIDs)
	}

	stdout.Reset()
	stderr.Reset()
	listArgs := append([]string{"--list"}, exampleMarkdownFiles...)
	code = Run(listArgs, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("list code = %d, stderr = %q", code, stderr.String())
	}
	for id := range wantIDs {
		if !strings.Contains(stdout.String(), "id="+id) {
			t.Fatalf("list output missing %q:\n%s", id, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	reviewArgs := append([]string{"review"}, exampleMarkdownFiles...)
	code = Run(reviewArgs, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("review code = %d, stderr = %q", code, stderr.String())
	}
	for id := range wantIDs {
		if !strings.Contains(stdout.String(), "#"+id) {
			t.Fatalf("review output missing %q:\n%s", id, stdout.String())
		}
	}
}

func TestExampleNoConfigAndConfiguredSelection(t *testing.T) {
	root := repositoryRoot(t)

	t.Run("no config README default", func(t *testing.T) {
		dir := gitRepoForCLI(t)
		writeCLIFile(t, dir, "README.md", readExampleFile(t, root, "examples/go/README.md"))
		chdir(t, dir)

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{"--dry-run", "--json", "--require-blocks"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("dry-run code = %d, stderr = %q", code, stderr.String())
		}
		plan := decodeExamplePlan(t, stdout.Bytes())
		if plan.Invocation.ConfigPath != "" {
			t.Fatalf("config path = %q", plan.Invocation.ConfigPath)
		}
		if !reflect.DeepEqual(plan.Files, []string{"README.md"}) {
			t.Fatalf("files = %#v", plan.Files)
		}
		if len(plan.Blocks) != 1 || plan.Blocks[0].ID != "go-test" {
			t.Fatalf("blocks = %#v", plan.Blocks)
		}
	})

	t.Run("configured monorepo files", func(t *testing.T) {
		dir := gitRepoForCLI(t)
		writeCLIFile(t, dir, "setupproof.yml", readExampleFile(t, root, "examples/monorepo/setupproof.yml"))
		writeCLIFile(t, dir, "docs/web.md", readExampleFile(t, root, "examples/monorepo/docs/web.md"))
		writeCLIFile(t, dir, "docs/api.md", readExampleFile(t, root, "examples/monorepo/docs/api.md"))
		chdir(t, dir)

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{"--dry-run", "--json"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("dry-run code = %d, stderr = %q", code, stderr.String())
		}
		plan := decodeExamplePlan(t, stdout.Bytes())
		if plan.Invocation.ConfigPath != "setupproof.yml" {
			t.Fatalf("config path = %q", plan.Invocation.ConfigPath)
		}
		if !plan.Invocation.RequireBlocks {
			t.Fatalf("requireBlocks should come from config")
		}
		if !reflect.DeepEqual(plan.Files, []string{"docs/web.md", "docs/api.md"}) {
			t.Fatalf("files = %#v", plan.Files)
		}
		if len(plan.Blocks) != 2 || plan.Blocks[0].ID != "web-package" || plan.Blocks[1].ID != "api-service" {
			t.Fatalf("blocks = %#v", plan.Blocks)
		}
	})
}

func readExampleFile(t *testing.T, root string, rel string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repository root = %s: %v", root, err)
	}
	return root
}

type examplePlan struct {
	Kind       string   `json:"kind"`
	Files      []string `json:"files"`
	Invocation struct {
		ConfigPath    string `json:"configPath"`
		RequireBlocks bool   `json:"requireBlocks"`
	} `json:"invocation"`
	Blocks []struct {
		ID      string `json:"id"`
		File    string `json:"file"`
		Source  string `json:"source"`
		Options struct {
			Runner string `json:"runner"`
		} `json:"options"`
	} `json:"blocks"`
}

func decodeExamplePlan(t *testing.T, data []byte) examplePlan {
	t.Helper()
	var plan examplePlan
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatalf("invalid plan JSON: %v\n%s", err, string(data))
	}
	return plan
}
