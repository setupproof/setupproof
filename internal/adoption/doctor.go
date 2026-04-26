package adoption

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/setupproof/setupproof/internal/config"
	"github.com/setupproof/setupproof/internal/diag"
	"github.com/setupproof/setupproof/internal/planning"
	"github.com/setupproof/setupproof/internal/platform"
	"github.com/setupproof/setupproof/internal/project"
)

func Doctor(req planning.Request, stdout io.Writer, stderr io.Writer) int {
	resolver, err := project.NewResolver(req.CWD)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	fmt.Fprintf(stdout, "root: %s\n", resolver.Root)
	if gitLayoutWarning(resolver.Root) {
		fmt.Fprintln(stdout, "git layout: .git is a file; treating this worktree or submodule root as the repository boundary")
	}
	if err := checkTempWritable(); err != nil {
		fmt.Fprintf(stderr, "temporary workspace check failed: %v\n", err)
		return 2
	}
	fmt.Fprintln(stdout, "temporary workspace: writable")

	if configFile, ok, err := doctorConfigFile(req, resolver); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	} else if ok {
		if _, err := config.Load(configFile.Abs); err != nil {
			fmt.Fprintf(stderr, "config: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "config: %s\n", configFile.Rel)
	} else {
		fmt.Fprintln(stdout, "config: not found")
	}

	result, err := planning.Build(req)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintf(stdout, "targets: %d\n", len(result.Plan.Files))
	for _, file := range result.Plan.Files {
		fmt.Fprintf(stdout, "  %s\n", file)
	}
	fmt.Fprintf(stdout, "marked blocks: %d\n", len(result.Plan.Blocks))
	fmt.Fprintf(stdout, "platform: %s/%s\n", platform.GOOS(), platform.GOARCH())
	if platform.NativeWindowsUnsupported() {
		fmt.Fprintln(stderr, platform.NativeWindowsUnsupportedMessage)
		return 3
	}
	if dockerConfigured(req, result.Plan) {
		if err := checkDockerReachable(); err != nil {
			fmt.Fprintf(stderr, "docker: %v\n", err)
			return 2
		}
		fmt.Fprintln(stdout, "docker: configured; daemon reachable")
	} else {
		fmt.Fprintln(stdout, "docker: not requested")
	}

	diag.EmitPlan(result.Plan, stderr)
	return result.ExitCode
}

func gitLayoutWarning(root string) bool {
	info, err := os.Lstat(filepath.Join(root, ".git"))
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func doctorConfigFile(req planning.Request, resolver project.Resolver) (project.ResolvedFile, bool, error) {
	if req.ConfigPath != "" {
		configFile, err := resolver.ResolveConfig(req.ConfigPath)
		if err != nil {
			return project.ResolvedFile{}, false, err
		}
		return configFile, true, nil
	}
	return resolver.DiscoverConfig()
}

func dockerConfigured(req planning.Request, plan planning.Plan) bool {
	if req.HasRunner && req.Runner == "docker" {
		return true
	}
	if plan.Defaults.Runner == "docker" {
		return true
	}
	for _, block := range plan.Blocks {
		if block.Options.Runner == "docker" {
			return true
		}
	}
	return false
}

func checkDockerReachable() error {
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("unavailable: docker command was not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, dockerPath, "info")
	cmd.Stdout = io.Discard
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if ctx.Err() != nil {
			return fmt.Errorf("unavailable: docker info timed out")
		}
		if detail == "" {
			return fmt.Errorf("unavailable: docker info failed: %w", err)
		}
		return fmt.Errorf("unavailable: %s", detail)
	}
	return nil
}

func checkTempWritable() error {
	dir, err := os.MkdirTemp("", "setupproof-doctor-")
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}
