package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/setupproof/setupproof/internal/diag"
	"github.com/setupproof/setupproof/internal/planning"
	"github.com/setupproof/setupproof/internal/platform"
	"github.com/setupproof/setupproof/internal/project"
	"github.com/setupproof/setupproof/internal/report"
	"github.com/setupproof/setupproof/internal/shellquote"
)

type Options struct {
	FailFast      bool
	KeepWorkspace bool
	NoColor       bool
	NoGlyphs      bool
}

type fileState struct {
	cwd string
	env []string
}

type blockOutcome struct {
	result           string
	exitCode         int
	reason           string
	interactive      string
	cleanupCompleted *bool
}

type executionRun struct {
	code        int
	blocks      []report.Block
	warnings    []string
	runnerError string
}

type shellResult struct {
	exitCode         int
	timedOut         bool
	cleanupCompleted bool
	output           report.Output
	err              error
}

func Run(req planning.Request, opts Options, stdout io.Writer, stderr io.Writer) int {
	executionReport, code := RunWithReport(req, opts, stderr)
	if executionReport.Kind != "" {
		_ = report.RenderTerminal(stdout, executionReport, report.TerminalOptions{NoColor: opts.NoColor, NoGlyphs: opts.NoGlyphs})
	}
	return code
}

func RunWithReport(req planning.Request, opts Options, stderr io.Writer) (report.Report, int) {
	started := time.Now()
	result, code, ok := buildExecutionPlan(req, stderr)
	if !ok {
		return report.Report{}, code
	}
	executionReport := report.New(result.Plan, started)
	if len(result.Plan.Blocks) == 0 {
		report.Finalize(&executionReport, 0, started, time.Now())
		return executionReport, 0
	}
	if platform.NativeWindowsUnsupported() {
		fmt.Fprintln(stderr, platform.NativeWindowsUnsupportedMessage)
		report.SetRunnerError(&executionReport, "unsupported_platform")
		report.Finalize(&executionReport, 3, started, time.Now())
		return executionReport, 3
	}
	runnerKind, err := executionRunner(result.Plan)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return report.Report{}, 2
	}
	var run executionRun
	switch runnerKind {
	case "local", "action-local":
		run = runPreparedLocal(req, result.Plan, opts, stderr)
	case "docker":
		run = runPreparedDocker(req, result.Plan, opts, stderr)
	default:
		fmt.Fprintf(stderr, "runner %q is not implemented\n", runnerKind)
		return report.Report{}, 2
	}
	executionReport.Blocks = run.blocks
	executionReport.Warnings = append(executionReport.Warnings, run.warnings...)
	if run.code == 3 {
		report.SetRunnerError(&executionReport, run.runnerError)
	}
	report.Finalize(&executionReport, run.code, started, time.Now())
	return executionReport, run.code
}

func buildExecutionPlan(req planning.Request, stderr io.Writer) (planning.Result, int, bool) {
	result, err := planning.Build(req)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return planning.Result{}, 2, false
	}
	diag.EmitPlan(result.Plan, stderr)
	if result.ExitCode != 0 {
		return planning.Result{}, result.ExitCode, false
	}
	return result, 0, true
}

func runPreparedLocal(req planning.Request, plan planning.Plan, opts Options, stderr io.Writer) executionRun {
	source, warnings, code, runnerError, ok := prepareWorkspaceSource(req, plan, stderr)
	if !ok {
		return executionRun{code: code, warnings: warnings, runnerError: runnerError}
	}

	ctx, stop, signalReason := interruptContext()
	defer stop()

	exitCode := 0
	var blockReports []report.Block
	blocksByFile := groupBlocksByFile(plan.Blocks)
	for _, file := range plan.Files {
		blocks := blocksByFile[file]
		if len(blocks) == 0 {
			continue
		}
		fileCode, fileReports, fileRunnerError := runFile(ctx, file, blocks, plan.Env, source, opts, stderr, signalReason)
		blockReports = append(blockReports, fileReports...)
		if fileCode == 2 || fileCode == 3 {
			return executionRun{code: fileCode, blocks: blockReports, warnings: warnings, runnerError: fileRunnerError}
		}
		if fileCode == 1 {
			exitCode = 1
		}
		if ctx.Err() != nil {
			return executionRun{code: 3, blocks: blockReports, warnings: warnings, runnerError: signalReason()}
		}
	}
	return executionRun{code: exitCode, blocks: blockReports, warnings: warnings}
}

func prepareWorkspaceSource(req planning.Request, plan planning.Plan, stderr io.Writer) (workspaceSource, []string, int, string, bool) {
	resolver, err := project.NewResolver(req.CWD)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return workspaceSource{}, nil, 2, "", false
	}
	gitRoot, err := discoverGitRoot(resolver.CWD)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return workspaceSource{}, nil, 2, "", false
	}

	source, err := loadWorkspaceSource(gitRoot, plan.Workspace.IncludedUntracked)
	if err != nil {
		fmt.Fprintf(stderr, "workspace setup failed: %v\n", err)
		return workspaceSource{}, nil, 3, "workspace_setup_failed", false
	}
	var warnings []string
	if source.trackedChanged {
		warning := "copied workspace includes tracked changes that differ from HEAD"
		fmt.Fprintf(stderr, "warning: %s\n", warning)
		warnings = append(warnings, warning)
	}
	if source.untrackedIncluded {
		warning := fmt.Sprintf("--include-untracked copied %d untracked non-ignored file(s)", source.untrackedFileCount)
		fmt.Fprintf(stderr, "warning: %s\n", warning)
		warnings = append(warnings, warning)
	}
	return source, warnings, 0, "", true
}

func executionRunner(plan planning.Plan) (string, error) {
	var runnerKind string
	for _, block := range plan.Blocks {
		if runnerKind == "" {
			runnerKind = block.Options.Runner
			continue
		}
		if block.Options.Runner != runnerKind {
			return "", fmt.Errorf("mixed runners in one execution are not implemented; pass --runner=local or --runner=docker for the whole run")
		}
	}
	return runnerKind, nil
}

func groupBlocksByFile(blocks []planning.Block) map[string][]planning.Block {
	grouped := make(map[string][]planning.Block)
	for _, block := range blocks {
		grouped[block.File] = append(grouped[block.File], block)
	}
	return grouped
}

func runFile(ctx context.Context, file string, blocks []planning.Block, envPlan planning.Env, source workspaceSource, opts Options, stderr io.Writer, signalReason func() string) (code int, blockReports []report.Block, runnerError string) {
	sharedWorkspace, err := createWorkspace(source)
	if err != nil {
		fmt.Fprintf(stderr, "%s: workspace setup failed: %v\n", file, err)
		return 3, blockReports, "workspace_setup_failed"
	}
	if opts.KeepWorkspace {
		fmt.Fprint(stderr, keepWorkspaceWarning(file, sharedWorkspace.repoRoot))
	}
	defer func() {
		if err := sharedWorkspace.cleanup(opts.KeepWorkspace); err != nil {
			fmt.Fprintf(stderr, "%s: cleanup failed: %v\n", file, err)
			code, runnerError = codeWithCleanupFailure(code, runnerError)
		}
	}()

	baseEnv, secretValues, err := baselineEnv(envPlan, sharedWorkspace, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2, blockReports, ""
	}
	state := fileState{cwd: sharedWorkspace.repoRoot, env: baseEnv}
	exitCode := 0
	stopped := false

	for _, block := range blocks {
		if stopped {
			blockReports = append(blockReports, reportBlock(block, blockOutcome{result: "skipped", reason: "fail-fast"}, report.Output{}, 0))
			continue
		}

		activeWorkspace := sharedWorkspace
		activeState := state
		blockSecrets := secretValues
		if block.Options.Isolated {
			activeWorkspace, err = createWorkspace(source)
			if err != nil {
				fmt.Fprintf(stderr, "%s: workspace setup failed: %v\n", block.QualifiedID, err)
				return 3, blockReports, "workspace_setup_failed"
			}
			if opts.KeepWorkspace {
				fmt.Fprint(stderr, keepWorkspaceWarning(block.QualifiedID, activeWorkspace.repoRoot))
			}
			isolatedEnv, isolatedSecrets, err := baselineEnv(envPlan, activeWorkspace, stderr)
			if err != nil {
				_ = activeWorkspace.cleanup(opts.KeepWorkspace)
				fmt.Fprintln(stderr, err)
				return 2, blockReports, ""
			}
			blockSecrets = append(append([]string(nil), secretValues...), isolatedSecrets...)
			activeState = fileState{cwd: activeWorkspace.repoRoot, env: isolatedEnv}
		}

		outcome, nextState, output, durationMs := runBlock(ctx, block, activeWorkspace, activeState, stderr, blockSecrets)
		blockReports = append(blockReports, reportBlock(block, outcome, output, durationMs))
		if block.Options.Isolated {
			if err := activeWorkspace.cleanup(opts.KeepWorkspace); err != nil {
				fmt.Fprintf(stderr, "%s: cleanup failed: %v\n", block.QualifiedID, err)
				return 3, blockReports, "cleanup_failed"
			}
		}

		switch outcome.result {
		case "passed":
			if !block.Options.Isolated {
				state = nextState
			}
		case "error":
			return 3, blockReports, outcome.reason
		default:
			exitCode = 1
			if opts.FailFast {
				stopped = true
			}
		}

		if ctx.Err() != nil {
			return 3, blockReports, signalReason()
		}
	}
	return exitCode, blockReports, ""
}

func keepWorkspaceWarning(target string, path string) string {
	return fmt.Sprintf("warning: keeping temporary workspace for %s at %s; it may contain command output, generated files, and unredacted environment captures from configured env.pass entries; do not share without inspection\n", target, path)
}

func runBlock(ctx context.Context, block planning.Block, ws *workspace, state fileState, stderr io.Writer, secretValues []string) (blockOutcome, fileState, report.Output, int64) {
	started := time.Now()
	if command, ok := classifyInteractive(block.Source); ok {
		fmt.Fprintf(stderr, "%s: non-interactive execution cannot run common interactive command %q\n", block.QualifiedID, command)
		return blockOutcome{result: "failed", exitCode: 1, reason: "interactive-command", interactive: command}, state, report.Output{}, elapsedMillis(started)
	}

	controlDir, err := os.MkdirTemp(ws.tempRoot, "control-")
	if err != nil {
		return blockOutcome{result: "error", reason: "workspace_setup_failed"}, state, report.Output{}, elapsedMillis(started)
	}
	stateCWD := filepath.Join(controlDir, "cwd")
	stateEnv := filepath.Join(controlDir, "env")
	scriptPath := filepath.Join(controlDir, "block.sh")
	if err := os.WriteFile(scriptPath, []byte(shellScript(block, stateCWD, stateEnv)), 0o700); err != nil {
		return blockOutcome{result: "error", reason: "workspace_setup_failed"}, state, report.Output{}, elapsedMillis(started)
	}

	timeout := time.Duration(block.Options.TimeoutMs) * time.Millisecond
	blockCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := runShell(blockCtx, block.Shell, scriptPath, state.cwd, state.env, stderr, secretValues)
	if result.timedOut {
		return blockOutcome{result: "timeout", exitCode: 1, reason: "timeout", cleanupCompleted: boolPointer(result.cleanupCompleted)}, state, result.output, elapsedMillis(started)
	}
	if result.err != nil {
		if errors.Is(result.err, exec.ErrNotFound) {
			return blockOutcome{result: "error", reason: "shell_unavailable"}, state, result.output, elapsedMillis(started)
		}
		return blockOutcome{result: "error", reason: "process_start_failed"}, state, result.output, elapsedMillis(started)
	}
	if result.exitCode != 0 {
		return blockOutcome{result: "failed", exitCode: result.exitCode, reason: "exit-code"}, state, result.output, elapsedMillis(started)
	}

	nextState, err := readStateFiles(stateCWD, stateEnv, state)
	if err != nil {
		fmt.Fprintf(stderr, "%s: warning: could not capture shell state: %v\n", block.QualifiedID, err)
		return blockOutcome{result: "passed", exitCode: 0}, state, result.output, elapsedMillis(started)
	}
	return blockOutcome{result: "passed", exitCode: 0}, nextState, result.output, elapsedMillis(started)
}

func runShell(ctx context.Context, shell string, scriptPath string, cwd string, env []string, stderr io.Writer, secretValues []string) shellResult {
	cmd := exec.Command(shell, scriptPath)
	cmd.Dir = cwd
	cmd.Env = append([]string(nil), env...)
	configureProcess(cmd)

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return shellResult{err: err}
	}
	defer devNull.Close()
	cmd.Stdin = devNull

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return shellResult{err: err}
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return shellResult{err: err}
	}

	if err := cmd.Start(); err != nil {
		return shellResult{err: err}
	}

	stdoutTail := report.NewTail(report.MaxTailBytes)
	stderrTail := report.NewTail(report.MaxTailBytes)
	redactor := report.NewRedactor(secretValues)
	stderrSink := synchronizedWriter(stderr)
	var copies sync.WaitGroup
	copies.Add(2)
	go func() {
		defer copies.Done()
		collector := &report.StreamCollector{Sink: stderrSink, Tail: stdoutTail, Redactor: redactor}
		_, _ = io.Copy(collector, stdoutPipe)
		_ = collector.Flush()
	}()
	go func() {
		defer copies.Done()
		collector := &report.StreamCollector{Sink: stderrSink, Tail: stderrTail, Redactor: redactor}
		_, _ = io.Copy(collector, stderrPipe)
		_ = collector.Flush()
	}()

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	var waitErr error
	select {
	case waitErr = <-waitCh:
	case <-ctx.Done():
		cleanupCompleted := terminateProcessTree(cmd)
		<-waitCh
		copies.Wait()
		return shellResult{exitCode: 1, timedOut: true, cleanupCompleted: cleanupCompleted, output: report.OutputFromTails(stdoutTail, stderrTail)}
	}
	copies.Wait()
	if waitErr == nil {
		return shellResult{exitCode: 0, output: report.OutputFromTails(stdoutTail, stderrTail)}
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		return shellResult{exitCode: exitErr.ExitCode(), output: report.OutputFromTails(stdoutTail, stderrTail)}
	}
	return shellResult{err: waitErr, output: report.OutputFromTails(stdoutTail, stderrTail)}
}

func shellScript(block planning.Block, stateCWD string, stateEnv string) string {
	return shellScriptWithStart(block, stateCWD, stateEnv, "")
}

func shellScriptWithStart(block planning.Block, stateCWD string, stateEnv string, startMarker string) string {
	names := shellScriptHelperNames(stateCWD, stateEnv, startMarker)
	var builder strings.Builder
	builder.WriteString(names.stateCWD)
	builder.WriteString("=")
	builder.WriteString(shellquote.Quote(stateCWD))
	builder.WriteString("\n")
	builder.WriteString(names.stateEnv)
	builder.WriteString("=")
	builder.WriteString(shellquote.Quote(stateEnv))
	builder.WriteString("\n")
	if startMarker != "" {
		builder.WriteString(names.startMarker)
		builder.WriteString("=")
		builder.WriteString(shellquote.Quote(startMarker))
		builder.WriteString("\nprintf 'started\\n' > \"$")
		builder.WriteString(names.startMarker)
		builder.WriteString("\"\n")
	}
	builder.WriteString(names.capture)
	builder.WriteString("() {\n")
	builder.WriteString("  pwd > \"$")
	builder.WriteString(names.stateCWD)
	builder.WriteString("\"\n")
	builder.WriteString("  awk 'BEGIN { for (name in ENVIRON) printf \"%s=%s%c\", name, ENVIRON[name], 0 }' > \"$")
	builder.WriteString(names.stateEnv)
	builder.WriteString("\"\n")
	builder.WriteString("}\n")
	builder.WriteString("trap '")
	builder.WriteString(names.status)
	builder.WriteString("=$?; if [ \"$")
	builder.WriteString(names.status)
	builder.WriteString("\" -eq 0 ] && [ ! -s \"$")
	builder.WriteString(names.stateEnv)
	builder.WriteString("\" ]; then ")
	builder.WriteString(names.capture)
	builder.WriteString("; fi' EXIT\n")
	if block.Options.Strict {
		if block.Shell == "bash" {
			builder.WriteString("set -e -o pipefail\n")
		} else {
			builder.WriteString("set -e\n")
		}
	}
	builder.WriteString(block.Source)
	if !strings.HasSuffix(block.Source, "\n") {
		builder.WriteByte('\n')
	}
	builder.WriteString(names.status)
	builder.WriteString("=$?\n")
	builder.WriteString("if [ \"$")
	builder.WriteString(names.status)
	builder.WriteString("\" -eq 0 ]; then ")
	builder.WriteString(names.capture)
	builder.WriteString("; fi\n")
	builder.WriteString("exit \"$")
	builder.WriteString(names.status)
	builder.WriteString("\"\n")
	return builder.String()
}

type shellScriptNames struct {
	stateCWD    string
	stateEnv    string
	startMarker string
	status      string
	capture     string
}

func shellScriptHelperNames(stateCWD string, stateEnv string, startMarker string) shellScriptNames {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(stateCWD))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(stateEnv))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(startMarker))
	suffix := fmt.Sprintf("%x", hash.Sum64())
	prefix := "__setupproof_" + suffix + "_"
	return shellScriptNames{
		stateCWD:    prefix + "state_cwd",
		stateEnv:    prefix + "state_env",
		startMarker: prefix + "start_marker",
		status:      prefix + "status",
		capture:     prefix + "capture",
	}
}

func codeWithCleanupFailure(code int, runnerError string) (int, string) {
	if code == 0 {
		return 3, "cleanup_failed"
	}
	return code, runnerError
}

func readStateFiles(cwdPath string, envPath string, previous fileState) (fileState, error) {
	cwdBytes, err := os.ReadFile(cwdPath)
	if err != nil {
		return previous, err
	}
	envBytes, err := os.ReadFile(envPath)
	if err != nil {
		return previous, err
	}
	cwd := strings.TrimRight(string(cwdBytes), "\r\n")
	if cwd == "" {
		return previous, fmt.Errorf("captured cwd was empty")
	}
	env := parseNullEnv(envBytes)
	if len(env) == 0 {
		return previous, fmt.Errorf("captured environment was empty")
	}
	return fileState{cwd: cwd, env: env}, nil
}

func parseNullEnv(data []byte) []string {
	parts := bytes.Split(data, []byte{0})
	env := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 || !bytes.Contains(part, []byte("=")) {
			continue
		}
		env = append(env, string(part))
	}
	sort.Strings(env)
	return env
}

func baselineEnv(envPlan planning.Env, ws *workspace, stderr io.Writer) ([]string, []string, error) {
	values := map[string]string{
		"HOME":       ws.homeDir,
		"TMPDIR":     ws.tmpDir,
		"CI":         "true",
		"SETUPPROOF": "1",
	}
	if path := os.Getenv("PATH"); path != "" {
		values["PATH"] = path
	}
	if lang := os.Getenv("LANG"); safeLocale(lang) {
		values["LANG"] = lang
	} else {
		values["LANG"] = "C.UTF-8"
	}
	// LC_ALL has POSIX precedence over LANG, so unsafe host values are omitted
	// instead of defaulted to avoid overriding the sanitized LANG fallback.
	if lcAll := os.Getenv("LC_ALL"); safeLocale(lcAll) {
		values["LC_ALL"] = lcAll
	}

	var warnings []string
	for _, name := range envPlan.Allow {
		if value, ok := os.LookupEnv(name); ok {
			values[name] = value
		} else {
			warnings = append(warnings, name)
		}
	}

	var secretValues []string
	for _, pass := range envPlan.Pass {
		value, ok := os.LookupEnv(pass.Name)
		if !ok {
			if pass.Required {
				return nil, nil, fmt.Errorf("required environment variable %s is missing", pass.Name)
			}
			warnings = append(warnings, pass.Name)
			continue
		}
		values[pass.Name] = value
		if pass.Secret && value != "" {
			secretValues = append(secretValues, value)
		}
	}
	for _, name := range warnings {
		fmt.Fprintf(stderr, "warning: optional environment variable %s is not set\n", name)
	}

	env := make([]string, 0, len(values))
	for name, value := range values {
		env = append(env, name+"="+value)
	}
	sort.Strings(env)
	return env, secretValues, nil
}

func safeLocale(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "c.utf-8", "c.utf8", "en_us.utf-8", "en_us.utf8", "posix.utf-8", "posix.utf8":
		return true
	default:
		return false
	}
}

func reportBlock(block planning.Block, outcome blockOutcome, output report.Output, durationMs int64) report.Block {
	return report.Block{
		ID:                 block.ID,
		QualifiedID:        block.QualifiedID,
		File:               block.File,
		Line:               block.Line,
		Language:           block.Language,
		Shell:              block.Shell,
		Source:             block.Source,
		Strict:             block.Options.Strict,
		Stdin:              block.Options.Stdin,
		TTY:                block.Options.TTY,
		StateMode:          block.Options.StateMode,
		Isolated:           block.Options.Isolated,
		Runner:             block.Options.Runner,
		DockerImage:        block.Options.DockerImage,
		Timeout:            block.Options.Timeout,
		TimeoutMs:          block.Options.TimeoutMs,
		Result:             outcome.result,
		ExitCode:           outcome.exitCode,
		Reason:             outcome.reason,
		InteractiveCommand: outcome.interactive,
		CleanupCompleted:   outcome.cleanupCompleted,
		DurationMs:         durationMs,
		StdoutTail:         output.StdoutTail,
		StderrTail:         output.StderrTail,
		Truncated: report.Truncated{
			Stdout: output.StdoutTruncated,
			Stderr: output.StderrTruncated,
		},
	}
}

func elapsedMillis(started time.Time) int64 {
	return time.Since(started).Milliseconds()
}

func boolPointer(value bool) *bool {
	return &value
}

type lockedWriter struct {
	w  io.Writer
	mu *sync.Mutex
}

func synchronizedWriter(w io.Writer) io.Writer {
	return lockedWriter{w: w, mu: &sync.Mutex{}}
}

func (w lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

func interruptContext() (context.Context, func(), func() string) {
	if runtime.GOOS == "windows" {
		ctx, cancel := context.WithCancel(context.Background())
		return ctx, cancel, func() string { return "signal" }
	}
	ctx, cancel := context.WithCancel(context.Background())
	signals := make(chan os.Signal, 1)
	done := make(chan struct{})
	var once sync.Once
	var mu sync.Mutex
	reason := ""

	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case sig := <-signals:
			mu.Lock()
			reason = signalRunnerError(sig)
			mu.Unlock()
			cancel()
		case <-done:
		}
	}()

	stop := func() {
		once.Do(func() {
			signal.Stop(signals)
			close(done)
			cancel()
		})
	}
	currentReason := func() string {
		mu.Lock()
		defer mu.Unlock()
		if reason == "" {
			return "signal"
		}
		return reason
	}
	return ctx, stop, currentReason
}

func signalRunnerError(sig os.Signal) string {
	switch sig {
	case os.Interrupt:
		return "signal_interrupt"
	case syscall.SIGTERM:
		return "signal_terminate"
	default:
		return "signal"
	}
}
