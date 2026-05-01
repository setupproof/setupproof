package adoption

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/setupproof/setupproof/internal/diag"
	"github.com/setupproof/setupproof/internal/markdown"
	"github.com/setupproof/setupproof/internal/planning"
	"github.com/setupproof/setupproof/internal/project"
)

type InitOptions struct {
	Force    bool
	Workflow bool
}

func Init(req planning.Request, opts InitOptions, stdout io.Writer, stderr io.Writer) int {
	resolver, err := project.NewResolver(req.CWD)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	files, err := initFiles(req)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	var writes []initWrite
	configPath := filepath.Join(resolver.Root, project.ConfigFileName)
	writes = append(writes, initWrite{
		Path:    configPath,
		Display: project.ConfigFileName,
		Data:    []byte(defaultConfig(files)),
	})
	if opts.Workflow {
		if err := validateSourceTreeWorkflowRoot(resolver.Root); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		fmt.Fprintln(stderr, "warning: generated workflow is source-tree-only; use it only in this repository or a root that vendors SetupProof action files")
		workflowPath := filepath.Join(resolver.Root, ".github", "workflows", "setupproof.yml")
		writes = append(writes, initWrite{
			Path:    workflowPath,
			Display: filepath.ToSlash(filepath.Join(".github", "workflows", "setupproof.yml")),
			Data:    []byte(workflowContent(files)),
		})
	}

	for _, write := range writes {
		if err := preflightInitWrite(write.Path, opts.Force); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	for _, write := range writes {
		if err := writeInitFile(write.Path, write.Data, opts.Force); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		fmt.Fprintf(stdout, "wrote %s\n", write.Display)
	}
	fmt.Fprintln(stdout)
	fmt.Fprint(stdout, initNextCommand(resolver.Root, files))
	return 0
}

func InitCheck(req planning.Request, stdout io.Writer, stderr io.Writer) int {
	result, err := planning.Build(req)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintln(stdout, "init check: ok")
	fmt.Fprintf(stdout, "config path: %s\n", valueOrNone(result.Plan.Invocation.ConfigPath))
	fmt.Fprintf(stdout, "files: %s\n", strings.Join(result.Plan.Files, ", "))
	fmt.Fprintf(stdout, "marked blocks: %d\n", len(result.Plan.Blocks))
	fmt.Fprintln(stdout, "config write: not attempted")
	fmt.Fprintln(stdout, "workflow write: not attempted")
	diag.EmitPlan(result.Plan, stderr)
	return result.ExitCode
}

func PrintWorkflow(req planning.Request, stdout io.Writer, stderr io.Writer) int {
	files, err := workflowFiles(req)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprint(stdout, workflowContent(files))
	return 0
}

func workflowContent(files []string) string {
	fileInput := workflowFilesInput(files)
	return fmt.Sprintf(`name: SetupProof

on:
  pull_request:
  push:
    branches:
      - main

permissions:
  contents: read

jobs:
  readme-quickstart:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      # Source-tree workflow: see docs/adr/0009-github-actions-checkout-strategy.md.
      - name: Checkout repository
        shell: bash
        run: |
          git init .
          git remote add origin "$GITHUB_SERVER_URL/$GITHUB_REPOSITORY"
          git fetch --depth=1 origin "$GITHUB_SHA"
          git checkout --detach FETCH_HEAD
      - name: Build SetupProof CLI
        shell: bash
        run: go build -o "$RUNNER_TEMP/setupproof" ./cmd/setupproof
      - name: Review marked quickstarts
        uses: ./
        with:
          cli-path: ${{ runner.temp }}/setupproof
          mode: review
          require-blocks: "true"
          files:%s
      - name: Run marked quickstarts
        uses: ./
        with:
          cli-path: ${{ runner.temp }}/setupproof
          require-blocks: "true"
          files:%s
`, fileInput, fileInput)
}

func workflowFiles(req planning.Request) ([]string, error) {
	targets, err := planning.ResolveTargets(req)
	if err != nil {
		if errors.Is(err, planning.ErrNoTarget) && len(req.Positional) == 0 && req.ConfigPath == "" {
			return []string{"README.md"}, nil
		}
		return nil, err
	}
	files := make([]string, 0, len(targets))
	for _, target := range targets {
		files = append(files, target.Rel)
	}
	return files, nil
}

func validateSourceTreeWorkflowRoot(root string) error {
	required := []string{
		filepath.Join(root, "action.yml"),
		filepath.Join(root, "cmd", "setupproof", "main.go"),
		filepath.Join(root, "scripts", "github-action.sh"),
	}
	for _, path := range required {
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("init --workflow is source-tree-only until release archives exist; expected %s", filepath.ToSlash(path))
			}
			return err
		}
		if info.IsDir() {
			return fmt.Errorf("init --workflow is source-tree-only until release archives exist; expected file %s", filepath.ToSlash(path))
		}
	}
	return nil
}

type initWrite struct {
	Path    string
	Display string
	Data    []byte
}

func initFiles(req planning.Request) ([]string, error) {
	if len(req.Positional) == 0 {
		return []string{"README.md"}, nil
	}
	files := make([]string, 0, len(req.Positional))
	for _, input := range req.Positional {
		file, err := cleanInitFile(input)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func cleanInitFile(input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("init file entries must not be empty")
	}
	if strings.ContainsAny(input, "\r\n") {
		return "", fmt.Errorf("init file entry %q must not contain newlines", input)
	}
	if filepath.IsAbs(input) {
		return "", fmt.Errorf("init file entry %q must be repository-root-relative", input)
	}
	cleaned := filepath.Clean(filepath.FromSlash(input))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("init file entry %q escapes the repository root", input)
	}
	return filepath.ToSlash(cleaned), nil
}

func defaultConfig(files []string) string {
	var builder strings.Builder
	builder.WriteString("version: 1\n\n")
	builder.WriteString("defaults:\n")
	builder.WriteString("  runner: local\n")
	builder.WriteString("  timeout: 120s\n")
	builder.WriteString("  requireBlocks: true\n")
	builder.WriteString("  strict: true\n")
	builder.WriteString("  isolated: false\n\n")
	builder.WriteString("files:\n")
	for _, file := range files {
		builder.WriteString("  - ")
		builder.WriteString(file)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func initNextCommand(root string, files []string) string {
	joined := strings.Join(files, " ")
	if initFilesHaveMarkedBlocks(root, files) {
		return fmt.Sprintf("next command: setupproof review %s\n", joined)
	}
	return fmt.Sprintf("no marked blocks detected; next command: setupproof suggest %s\n", joined)
}

func initFilesHaveMarkedBlocks(root string, files []string) bool {
	for _, file := range files {
		path := filepath.Join(root, filepath.FromSlash(file))
		contents, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if len(markdown.Discover(file, contents)) > 0 {
			return true
		}
	}
	return false
}

func preflightInitWrite(path string, force bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !force {
		return fmt.Errorf("%s already exists; pass --force to overwrite", filepath.ToSlash(path))
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", filepath.ToSlash(path))
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symlink; refusing to overwrite", filepath.ToSlash(path))
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", filepath.ToSlash(path))
	}
	return nil
}

func writeInitFile(path string, data []byte, force bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	flag := os.O_WRONLY | os.O_CREATE
	if force {
		flag |= os.O_TRUNC
	} else {
		flag |= os.O_EXCL
	}
	file, err := os.OpenFile(path, flag, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s already exists; pass --force to overwrite", filepath.ToSlash(path))
		}
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func valueOrNone(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

func workflowFilesInput(files []string) string {
	var builder strings.Builder
	builder.WriteString(" |\n")
	for i, file := range files {
		if i > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString("            ")
		builder.WriteString(file)
	}
	return builder.String()
}
