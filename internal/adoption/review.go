package adoption

import (
	"fmt"
	"io"
	"strings"

	"github.com/setupproof/setupproof/internal/app"
	"github.com/setupproof/setupproof/internal/diag"
	"github.com/setupproof/setupproof/internal/planning"
	"github.com/setupproof/setupproof/internal/shellquote"
)

func Review(req planning.Request, reportSinks []string, stdout io.Writer, stderr io.Writer) int {
	result, err := planning.Build(req)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	if len(result.Plan.Blocks) == 0 {
		fmt.Fprintln(stdout, "No marked blocks found.")
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Add one above a shell block:")
		fmt.Fprintln(stdout, "  <!-- setupproof id=quickstart -->")
		fmt.Fprintln(stdout)
		fmt.Fprintf(stdout, "Next command:\n  %s suggest %s\n", app.CommandName, reviewTargetArgs(result.Plan.Files))
		diag.EmitPlan(result.Plan, stderr)
		return result.ExitCode
	}

	for index, block := range result.Plan.Blocks {
		if index > 0 {
			fmt.Fprintln(stdout)
		}
		fmt.Fprintf(stdout, "%s\n", block.QualifiedID)
		fmt.Fprintf(stdout, "  location: %s:%d\n", block.File, block.Line)
		fmt.Fprintf(stdout, "  block id: %s\n\n", block.ID)

		fmt.Fprintf(stdout, "  execution\n")
		fmt.Fprintf(stdout, "    runner: %s\n", block.Options.Runner)
		fmt.Fprintf(stdout, "    shell: %s\n", block.Shell)
		fmt.Fprintf(stdout, "    timeout: %s\n", block.Options.Timeout)
		fmt.Fprintf(stdout, "    strict mode: %t\n", block.Options.Strict)
		fmt.Fprintf(stdout, "    stdin mode: %s\n", block.Options.Stdin)
		fmt.Fprintf(stdout, "    tty mode: %t\n", block.Options.TTY)
		if block.Options.DockerImage != "" {
			fmt.Fprintf(stdout, "    docker image: %s\n", block.Options.DockerImage)
		}

		fmt.Fprintf(stdout, "\n  workspace\n")
		fmt.Fprintf(stdout, "    workspace copy mode: %s\n", result.Plan.Workspace.Source)
		fmt.Fprintf(stdout, "    state mode: %s\n", block.Options.StateMode)
		fmt.Fprintf(stdout, "    network policy: %s\n", block.Options.NetworkPolicy)
		fmt.Fprintf(stdout, "    network enforced: %t\n", block.Options.NetworkEnforced)

		fmt.Fprintf(stdout, "\n  environment\n")
		fmt.Fprintf(stdout, "    environment variables: %s\n", strings.Join(envNames(result.Plan.Env), ", "))
		fmt.Fprintf(stdout, "    secret environment variables: %s\n", strings.Join(secretEnvNames(result.Plan.Env), ", "))

		fmt.Fprintf(stdout, "\n  reports\n")
		fmt.Fprintf(stdout, "    report sinks: %s\n", strings.Join(normalizeReportSinks(reportSinks), ", "))

		fmt.Fprintf(stdout, "\n  source\n")
		for _, line := range strings.Split(block.Source, "\n") {
			fmt.Fprintf(stdout, "    %s\n", line)
		}
	}

	diag.EmitPlan(result.Plan, stderr)
	return result.ExitCode
}

func envNames(env planning.Env) []string {
	names := append([]string{}, env.Allow...)
	for _, pass := range env.Pass {
		names = append(names, pass.Name)
	}
	if len(names) == 0 {
		return []string{"none"}
	}
	return names
}

func secretEnvNames(env planning.Env) []string {
	var names []string
	for _, pass := range env.Pass {
		if pass.Secret {
			names = append(names, pass.Name)
		}
	}
	if len(names) == 0 {
		return []string{"none"}
	}
	return names
}

func normalizeReportSinks(reportSinks []string) []string {
	if len(reportSinks) == 0 {
		return []string{"terminal"}
	}
	return reportSinks
}

func reviewTargetArgs(files []string) string {
	if len(files) == 0 {
		return "<markdown-file>"
	}
	var quoted []string
	for _, file := range files {
		if file != "" {
			quoted = append(quoted, shellquote.Quote(file))
		}
	}
	if len(quoted) == 0 {
		return "<markdown-file>"
	}
	return strings.Join(quoted, " ")
}
