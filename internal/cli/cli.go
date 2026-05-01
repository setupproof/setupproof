package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/setupproof/setupproof/internal/adoption"
	"github.com/setupproof/setupproof/internal/app"
	"github.com/setupproof/setupproof/internal/diag"
	"github.com/setupproof/setupproof/internal/planning"
	"github.com/setupproof/setupproof/internal/report"
	"github.com/setupproof/setupproof/internal/runner"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if wantsHelp(args) {
		printHelp(stdout)
		return 0
	}
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintf(stdout, "%s %s\n", app.CommandName, app.Version)
		return 0
	}

	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	if opts.mode == "list" {
		return runList(opts.request, stdout, stderr)
	}
	if opts.mode == "suggest" {
		return runSuggest(opts.request, stdout, stderr)
	}
	if opts.mode == "review" {
		return adoption.Review(opts.request, reportSinks(opts), stdout, stderr)
	}
	if opts.mode == "doctor" {
		return adoption.Doctor(opts.request, stdout, stderr)
	}
	if opts.mode == "init" {
		return adoption.Init(opts.request, adoption.InitOptions{Force: opts.initForce, Workflow: opts.initWorkflow}, stdout, stderr)
	}
	if opts.mode == "init-check" {
		return adoption.InitCheck(opts.request, stdout, stderr)
	}
	if opts.mode == "init-workflow-print" {
		return adoption.PrintWorkflow(opts.request, stdout, stderr)
	}
	if opts.mode == "report-github-step-summary" {
		return runReportGitHubStepSummary(opts.summary, stdout, stderr)
	}

	if opts.dryRun {
		return runDryRun(opts.request, opts.json, stdout, stderr)
	}

	return runExecution(opts, stdout, stderr)
}

type parsedArgs struct {
	mode          string
	dryRun        bool
	json          bool
	failFast      bool
	keepWorkspace bool
	noColor       bool
	noGlyphs      bool
	initForce     bool
	initWorkflow  bool

	reportJSON string
	reportFile string
	request    planning.Request
	summary    report.StepSummaryOptions
}

func parseArgs(args []string) (parsedArgs, error) {
	opts := parsedArgs{
		request: planning.Request{Args: append([]string(nil), args...)},
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "suggest":
			if opts.mode != "" {
				return parsedArgs{}, fmt.Errorf("only one primary command may be specified")
			}
			opts.mode = "suggest"
		case arg == "review":
			if opts.mode != "" {
				return parsedArgs{}, fmt.Errorf("only one primary command may be specified")
			}
			opts.mode = "review"
		case arg == "doctor":
			if opts.mode != "" {
				return parsedArgs{}, fmt.Errorf("only one primary command may be specified")
			}
			opts.mode = "doctor"
		case arg == "report":
			if opts.mode != "" {
				return parsedArgs{}, fmt.Errorf("only one primary command may be specified")
			}
			summary, next, err := parseReportArgs(args, i)
			if err != nil {
				return parsedArgs{}, err
			}
			opts.mode = "report-github-step-summary"
			opts.summary = summary
			i = next
		case arg == "init":
			if opts.mode != "" {
				return parsedArgs{}, fmt.Errorf("only one primary command may be specified")
			}
			init, next, err := parseInitArgs(args, i)
			if err != nil {
				return parsedArgs{}, err
			}
			opts.mode = init.mode
			opts.initForce = init.force
			opts.initWorkflow = init.workflow
			opts.request.Positional = append(opts.request.Positional, init.files...)
			i = next
		case arg == "--list":
			if opts.mode != "" {
				return parsedArgs{}, fmt.Errorf("only one primary command may be specified")
			}
			opts.mode = "list"
		case arg == "--dry-run":
			opts.dryRun = true
		case arg == "--json":
			opts.json = true
		case arg == "--require-blocks":
			opts.request.HasRequireBlocks = true
			opts.request.RequireBlocks = true
		case arg == "--fail-fast":
			opts.failFast = true
		case arg == "--include-untracked":
			opts.request.IncludeUntracked = true
		case arg == "--keep-workspace":
			opts.keepWorkspace = true
		case arg == "--no-color":
			opts.noColor = true
		case arg == "--no-glyphs":
			opts.noGlyphs = true
		case arg == "--config":
			value, next, err := nextValue(args, i, arg)
			if err != nil {
				return parsedArgs{}, err
			}
			i = next
			opts.request.ConfigPath = value
		case strings.HasPrefix(arg, "--config="):
			opts.request.ConfigPath = strings.TrimPrefix(arg, "--config=")
		case arg == "--runner":
			value, next, err := nextValue(args, i, arg)
			if err != nil {
				return parsedArgs{}, err
			}
			i = next
			opts.request.HasRunner = true
			opts.request.Runner = value
		case strings.HasPrefix(arg, "--runner="):
			opts.request.HasRunner = true
			opts.request.Runner = strings.TrimPrefix(arg, "--runner=")
		case arg == "--timeout":
			value, next, err := nextValue(args, i, arg)
			if err != nil {
				return parsedArgs{}, err
			}
			i = next
			opts.request.HasTimeout = true
			opts.request.Timeout = value
		case strings.HasPrefix(arg, "--timeout="):
			opts.request.HasTimeout = true
			opts.request.Timeout = strings.TrimPrefix(arg, "--timeout=")
		case arg == "--network":
			value, next, err := nextValue(args, i, arg)
			if err != nil {
				return parsedArgs{}, err
			}
			i = next
			network, err := parseNetworkFlag(value)
			if err != nil {
				return parsedArgs{}, err
			}
			opts.request.HasNetwork = true
			opts.request.Network = network
		case strings.HasPrefix(arg, "--network="):
			network, err := parseNetworkFlag(strings.TrimPrefix(arg, "--network="))
			if err != nil {
				return parsedArgs{}, err
			}
			opts.request.HasNetwork = true
			opts.request.Network = network
		case arg == "--report-json":
			value, next, err := nextValue(args, i, arg)
			if err != nil {
				return parsedArgs{}, err
			}
			i = next
			opts.reportJSON = value
		case strings.HasPrefix(arg, "--report-json="):
			opts.reportJSON = strings.TrimPrefix(arg, "--report-json=")
		case arg == "--report-file":
			value, next, err := nextValue(args, i, arg)
			if err != nil {
				return parsedArgs{}, err
			}
			i = next
			opts.reportFile = value
		case strings.HasPrefix(arg, "--report-file="):
			opts.reportFile = strings.TrimPrefix(arg, "--report-file=")
		case strings.HasPrefix(arg, "-"):
			return parsedArgs{}, fmt.Errorf("unsupported flag %q", arg)
		default:
			opts.request.Positional = append(opts.request.Positional, arg)
		}
	}

	if opts.mode == "list" && opts.dryRun {
		return parsedArgs{}, fmt.Errorf("--list cannot be combined with --dry-run; use --list for a text inventory or --dry-run --json for a machine-readable plan")
	}
	if opts.mode == "list" && opts.json {
		return parsedArgs{}, fmt.Errorf("--list --json is not implemented; use --dry-run --json <markdown files...> for machine-readable output")
	}
	if (opts.mode == "suggest" || opts.mode == "review" || opts.mode == "doctor" || strings.HasPrefix(opts.mode, "init")) && opts.json {
		return parsedArgs{}, fmt.Errorf("--json is not implemented for %s", opts.mode)
	}
	if (opts.mode == "suggest" || opts.mode == "review" || opts.mode == "doctor" || strings.HasPrefix(opts.mode, "init")) && opts.dryRun {
		return parsedArgs{}, fmt.Errorf("--dry-run cannot be combined with %s", opts.mode)
	}
	if opts.dryRun && (opts.reportJSON != "" || opts.reportFile != "") {
		return parsedArgs{}, fmt.Errorf("--dry-run cannot be combined with report output files; use --dry-run --json > plan.json, or remove --dry-run to write execution reports")
	}
	return opts, nil
}

func wantsHelp(args []string) bool {
	if len(args) == 1 && args[0] == "help" {
		return true
	}
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

type initArgs struct {
	mode     string
	force    bool
	workflow bool
	files    []string
}

func parseInitArgs(args []string, index int) (initArgs, int, error) {
	next := index + 1
	check := false
	workflow := false
	printOnly := false
	force := false
	var files []string
	for next < len(args) {
		switch args[next] {
		case "--check":
			check = true
		case "--workflow":
			workflow = true
		case "--print":
			printOnly = true
		case "--force":
			force = true
		default:
			if strings.HasPrefix(args[next], "-") {
				return initArgs{}, next, fmt.Errorf("unsupported init argument %q", args[next])
			}
			files = append(files, args[next])
		}
		next++
	}
	if check && (workflow || printOnly || force || len(files) > 0) {
		return initArgs{}, next, fmt.Errorf("init --check cannot be combined with workflow, force, or file arguments")
	}
	if check {
		return initArgs{mode: "init-check"}, next - 1, nil
	}
	if workflow && printOnly {
		if force {
			return initArgs{}, next, fmt.Errorf("init --workflow --print cannot be combined with --force")
		}
		return initArgs{mode: "init-workflow-print", files: files}, next - 1, nil
	}
	if printOnly {
		return initArgs{}, next, fmt.Errorf("init --print requires --workflow")
	}
	if workflow {
		return initArgs{mode: "init", force: force, workflow: true, files: files}, next - 1, nil
	}
	return initArgs{mode: "init", force: force, files: files}, next - 1, nil
}

func parseReportArgs(args []string, index int) (report.StepSummaryOptions, int, error) {
	next := index + 1
	format := ""
	opts := report.StepSummaryOptions{Mode: "run"}
	for next < len(args) {
		arg := args[next]
		switch {
		case arg == "--format":
			value, valueIndex, err := nextValue(args, next, arg)
			if err != nil {
				return report.StepSummaryOptions{}, next, err
			}
			format = value
			next = valueIndex
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		case arg == "--mode":
			value, valueIndex, err := nextValue(args, next, arg)
			if err != nil {
				return report.StepSummaryOptions{}, next, err
			}
			opts.Mode = value
			next = valueIndex
		case strings.HasPrefix(arg, "--mode="):
			opts.Mode = strings.TrimPrefix(arg, "--mode=")
		case arg == "--status":
			value, valueIndex, err := nextValue(args, next, arg)
			if err != nil {
				return report.StepSummaryOptions{}, next, err
			}
			status, err := strconv.Atoi(value)
			if err != nil || status < 0 {
				return report.StepSummaryOptions{}, next, fmt.Errorf("--status must be a non-negative integer")
			}
			opts.Status = status
			next = valueIndex
		case strings.HasPrefix(arg, "--status="):
			status, err := strconv.Atoi(strings.TrimPrefix(arg, "--status="))
			if err != nil || status < 0 {
				return report.StepSummaryOptions{}, next, fmt.Errorf("--status must be a non-negative integer")
			}
			opts.Status = status
		case arg == "--report-json":
			value, valueIndex, err := nextValue(args, next, arg)
			if err != nil {
				return report.StepSummaryOptions{}, next, err
			}
			opts.ReportJSONPath = value
			next = valueIndex
		case strings.HasPrefix(arg, "--report-json="):
			opts.ReportJSONPath = strings.TrimPrefix(arg, "--report-json=")
		case arg == "--files":
			value, valueIndex, err := nextValue(args, next, arg)
			if err != nil {
				return report.StepSummaryOptions{}, next, err
			}
			opts.Files = splitReportFiles(value)
			next = valueIndex
		case strings.HasPrefix(arg, "--files="):
			opts.Files = splitReportFiles(strings.TrimPrefix(arg, "--files="))
		default:
			return report.StepSummaryOptions{}, next, fmt.Errorf("unsupported report argument %q", arg)
		}
		next++
	}
	if format != "github-step-summary" {
		return report.StepSummaryOptions{}, next, fmt.Errorf("report --format must be github-step-summary")
	}
	switch opts.Mode {
	case "run", "review":
	default:
		return report.StepSummaryOptions{}, next, fmt.Errorf("--mode must be run or review")
	}
	return opts, next - 1, nil
}

func splitReportFiles(value string) []string {
	var files []string
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimRight(line, "\r")
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

func runList(req planning.Request, stdout io.Writer, stderr io.Writer) int {
	result, err := planning.Build(req)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	for _, block := range result.Plan.Blocks {
		if block.Language == block.Shell {
			fmt.Fprintf(stdout, "%s:%d id=%s language=%s marker=%s\n", block.File, block.Line, block.ID, block.Language, block.MarkerForm)
		} else {
			fmt.Fprintf(stdout, "%s:%d id=%s language=%s shell=%s marker=%s\n", block.File, block.Line, block.ID, block.Language, block.Shell, block.MarkerForm)
		}
	}
	if len(result.Plan.Blocks) == 0 {
		fmt.Fprintln(stdout, "No marked blocks found.")
	}
	diag.EmitPlan(result.Plan, stderr)
	return result.ExitCode
}

func runDryRun(req planning.Request, jsonOutput bool, stdout io.Writer, stderr io.Writer) int {
	if !jsonOutput {
		fmt.Fprintln(stderr, "--dry-run currently requires --json; use --dry-run --json <markdown files...>")
		return 2
	}

	result, err := planning.Build(req)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(result.Plan); err != nil {
		fmt.Fprintln(stderr, err)
		return 3
	}
	diag.EmitPlan(result.Plan, stderr)
	return result.ExitCode
}

func runReportGitHubStepSummary(opts report.StepSummaryOptions, stdout io.Writer, stderr io.Writer) int {
	var executionReport *report.Report
	if opts.Mode == "run" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if len(bytes.TrimSpace(data)) > 0 {
			var parsed report.Report
			if err := json.Unmarshal(data, &parsed); err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			executionReport = &parsed
		}
	}
	_, err := io.WriteString(stdout, report.RenderGitHubStepSummary(executionReport, opts))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 3
	}
	return 0
}

func runExecution(opts parsedArgs, stdout io.Writer, stderr io.Writer) int {
	reportJSONPath, reportFilePath, err := resolveReportPaths(opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	executionReport, code := runner.RunWithReport(opts.request, runner.Options{
		FailFast:      opts.failFast,
		KeepWorkspace: opts.keepWorkspace,
		NoColor:       opts.noColor || os.Getenv("NO_COLOR") != "",
		NoGlyphs:      opts.noGlyphs,
	}, stderr)
	if executionReport.Kind == "" {
		return code
	}

	if reportJSONPath != "" {
		data, err := report.JSONBytes(executionReport)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 3
		}
		if err := report.WriteResolvedFile(reportJSONPath, data); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	if reportFilePath != "" {
		markdown := report.RenderMarkdown(executionReport, report.MarkdownOptions{StripANSI: true})
		if err := report.WriteResolvedFile(reportFilePath, []byte(markdown)); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	if opts.json {
		if err := report.WriteJSON(stdout, executionReport); err != nil {
			fmt.Fprintln(stderr, err)
			return 3
		}
		return code
	}
	if err := report.RenderTerminal(stdout, executionReport, report.TerminalOptions{
		NoColor:  opts.noColor || os.Getenv("NO_COLOR") != "",
		NoGlyphs: opts.noGlyphs,
	}); err != nil {
		fmt.Fprintln(stderr, err)
		return 3
	}
	return code
}

func resolveReportPaths(opts parsedArgs) (string, string, error) {
	cwd := opts.request.CWD
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return "", "", err
		}
	}
	var reportJSONPath string
	var reportFilePath string
	var err error
	if opts.reportJSON != "" {
		reportJSONPath, err = report.ResolveOutputPath(cwd, opts.reportJSON)
		if err != nil {
			return "", "", err
		}
	}
	if opts.reportFile != "" {
		reportFilePath, err = report.ResolveOutputPath(cwd, opts.reportFile)
		if err != nil {
			return "", "", err
		}
	}
	if reportJSONPath != "" && reportFilePath != "" && sameOutputFile(reportJSONPath, reportFilePath) {
		return "", "", fmt.Errorf("--report-json and --report-file must use different paths")
	}
	return reportJSONPath, reportFilePath, nil
}

func sameOutputFile(left string, right string) bool {
	if left == right {
		return true
	}
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	if leftErr == nil && rightErr == nil {
		return os.SameFile(leftInfo, rightInfo)
	}
	return false
}

func runSuggest(req planning.Request, stdout io.Writer, stderr io.Writer) int {
	suggestions, err := adoption.Suggest(req)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if len(suggestions) == 0 {
		fmt.Fprintln(stdout, "No candidate shell blocks found.")
		return 0
	}
	for _, suggestion := range suggestions {
		risks := strings.Join(suggestion.RiskFlags, ", ")
		if risks == "" {
			risks = "none"
		}
		fmt.Fprintf(stdout, "%s:%d\n", suggestion.File, suggestion.Line)
		fmt.Fprintf(stdout, "  language: %s\n", suggestion.Language)
		fmt.Fprintf(stdout, "  confidence: %s\n", suggestion.Confidence)
		fmt.Fprintf(stdout, "  reason: %s\n", suggestion.Reason)
		fmt.Fprintf(stdout, "  risk flags: %s\n", risks)
		fmt.Fprintf(stdout, "  suggested canonical marker: %s\n", suggestion.SuggestedMarker)
		fmt.Fprintf(stdout, "  next command: %s\n", suggestion.NextCommand)
	}
	return 0
}

func nextValue(args []string, index int, flag string) (string, int, error) {
	if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
		return "", index, fmt.Errorf("%s requires a value", flag)
	}
	return args[index+1], index + 1, nil
}

func parseNetworkFlag(value string) (bool, error) {
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("--network must be true or false")
	}
}

func reportSinks(opts parsedArgs) []string {
	var sinks []string
	if opts.reportFile != "" {
		sinks = append(sinks, "markdown:"+opts.reportFile)
	}
	if opts.reportJSON != "" {
		sinks = append(sinks, "json:"+opts.reportJSON)
	}
	return sinks
}

func printHelp(stdout io.Writer) {
	var buffer bytes.Buffer
	fmt.Fprintf(&buffer, `%s

Usage:
  %s --dry-run --json <markdown files...>
  %s --list <markdown files...>
  %s suggest <markdown files...>
  %s review <markdown files...>
  %s doctor <markdown files...>
  %s init [--force] [markdown files...]
  %s init --check
  %s init --workflow --print
  %s report --format=github-step-summary
  %s --help
  %s --version

Flags:
  --config <path>          Read an explicit setupproof.yml.
  --runner <kind>          Use local, action-local, or docker.
  --timeout <duration>     Override the default block timeout.
  --network <true|false>   Request network policy for supported runners.
  --require-blocks         Exit 4 when no marked blocks are found.
  --fail-fast              Stop after the first failing block in a file.
  --include-untracked      Copy untracked non-ignored files into workspaces.
  --keep-workspace         Keep temporary workspaces after execution.
  --report-json <path>     Write an execution report as JSON.
  --report-file <path>     Write an execution report as Markdown.
  --json                   Emit JSON for dry runs or execution reports.
  --no-color               Disable ANSI color in terminal output.
  --no-glyphs              Use text status labels instead of symbols.

SetupProof supports local, action-local, and Docker runners with terminal, Markdown, and JSON execution reports.
`, app.DisplayName, app.CommandName, app.CommandName, app.CommandName, app.CommandName, app.CommandName, app.CommandName, app.CommandName, app.CommandName, app.CommandName, app.CommandName, app.CommandName)
	_, _ = stdout.Write(buffer.Bytes())
}
