package cli

import (
	"bytes"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestJSONOutputsMatchPublishedSchemas(t *testing.T) {
	t.Run("plan", func(t *testing.T) {
		dir := t.TempDir()
		chdir(t, dir)
		if err := os.WriteFile("README.md", []byte("```sh setupproof id=quickstart\ntrue\n```\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{"--dry-run", "--json", "--require-blocks", "README.md"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
		}
		validateJSONSchema(t, "schemas/setupproof-plan.schema.json", stdout.Bytes())
	})

	t.Run("invalid plan", func(t *testing.T) {
		dir := t.TempDir()
		chdir(t, dir)
		if err := os.WriteFile("README.md", []byte("```sh setupproof id=quickstart network=false\ntrue\n```\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{"--dry-run", "--json", "README.md"}, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
		}
		validateJSONSchema(t, "schemas/setupproof-plan.schema.json", stdout.Bytes())
	})

	t.Run("mixed runner plan", func(t *testing.T) {
		dir := t.TempDir()
		chdir(t, dir)
		if err := os.WriteFile("README.md", []byte("```sh setupproof id=local\ntrue\n```\n\n```sh setupproof id=docker runner=docker\ntrue\n```\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{"--dry-run", "--json", "README.md"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
		}
		validateJSONSchema(t, "schemas/setupproof-plan.schema.json", stdout.Bytes())
	})

	t.Run("report", func(t *testing.T) {
		dir := gitRepoForCLI(t)
		chdir(t, dir)
		writeCLIFile(t, dir, "README.md", "```sh setupproof id=quickstart\ntrue\n```\n")
		runGit(t, dir, "add", "README.md")

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{"--json", "--no-color", "--no-glyphs", "README.md"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
		}
		validateJSONSchema(t, "schemas/setupproof-report.schema.json", stdout.Bytes())
	})

	t.Run("report infrastructure error", func(t *testing.T) {
		gitPath, err := exec.LookPath("git")
		if err != nil {
			t.Skip("git is not available")
		}
		dir := gitRepoForCLI(t)
		chdir(t, dir)
		writeCLIFile(t, dir, "README.md", "```bash setupproof id=missing-shell\ntrue\n```\n")
		runGit(t, dir, "add", "README.md")

		pathDir := t.TempDir()
		if err := os.Symlink(gitPath, filepath.Join(pathDir, "git")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		t.Setenv("PATH", pathDir)

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{"--json", "--no-color", "--no-glyphs", "README.md"}, &stdout, &stderr)
		if code != 3 {
			t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
		}
		validateJSONSchema(t, "schemas/setupproof-report.schema.json", stdout.Bytes())
	})
}

func TestConfigSchemaValidatesRepresentativeConfig(t *testing.T) {
	validateJSONSchema(t, "schemas/setupproof-config.schema.json", []byte(`{
  "version": 1,
  "defaults": {
    "runner": "docker",
    "image": "ubuntu:24.04",
    "timeout": 120,
    "requireBlocks": true,
    "strict": true,
    "isolated": false,
    "network": false
  },
  "files": ["README.md", "docs/quickstart.md"],
  "env": {
    "allow": ["NODE_ENV"],
    "pass": [
      {
        "name": "SDK_API_KEY",
        "secret": true,
        "required": false
      }
    ]
  },
  "blocks": [
    {
      "file": "README.md",
      "id": "quickstart",
      "runner": "local",
      "timeout": "90s"
    }
  ],
  "x-local": {
    "owner": "docs"
  }
}`))
}

func validateJSONSchema(t *testing.T, schemaRel string, data []byte) {
	t.Helper()
	root := repositoryRoot(t)
	schemaPath := filepath.Join(root, filepath.FromSlash(schemaRel))
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	schema, err := compiler.Compile((&url.URL{Scheme: "file", Path: schemaPath}).String())
	if err != nil {
		t.Fatalf("compile %s: %v", schemaRel, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, string(data))
	}
	if err := schema.Validate(doc); err != nil {
		t.Fatalf("%s validation failed: %v\n%s", schemaRel, err, string(data))
	}
}
