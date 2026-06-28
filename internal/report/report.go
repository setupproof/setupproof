package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/setupproof/setupproof/internal/app"
	"github.com/setupproof/setupproof/internal/planning"
	"github.com/setupproof/setupproof/internal/shellquote"
)

const SchemaVersion = "1.0.0"
const MaxTailBytes = 64 * 1024

type Report struct {
	Kind              string             `json:"kind"`
	SchemaVersion     string             `json:"schemaVersion"`
	SetupproofVersion string             `json:"setupproofVersion"`
	StartedAt         string             `json:"startedAt"`
	DurationMs        int64              `json:"durationMs"`
	Result            string             `json:"result"`
	ExitCode          int                `json:"exitCode"`
	Invocation        Invocation         `json:"invocation"`
	Workspace         planning.Workspace `json:"workspace"`
	Runner            planning.Runner    `json:"runner"`
	Files             []string           `json:"files"`
	Warnings          []string           `json:"warnings"`
	Blocks            []Block            `json:"blocks"`
}

type Invocation struct {
	Args       []string `json:"args"`
	ConfigPath string   `json:"configPath,omitempty"`
}

type Block struct {
	ID                 string    `json:"id"`
	QualifiedID        string    `json:"qualifiedId"`
	File               string    `json:"file"`
	Line               int       `json:"line"`
	Language           string    `json:"language"`
	Shell              string    `json:"shell"`
	Source             string    `json:"source"`
	Strict             bool      `json:"strict"`
	Stdin              string    `json:"stdin"`
	TTY                bool      `json:"tty"`
	StateMode          string    `json:"stateMode"`
	Isolated           bool      `json:"isolated"`
	Runner             string    `json:"runner"`
	DockerImage        string    `json:"dockerImage,omitempty"`
	Timeout            string    `json:"timeout"`
	TimeoutMs          int64     `json:"timeoutMs"`
	Result             string    `json:"result"`
	ExitCode           int       `json:"exitCode"`
	Reason             string    `json:"reason,omitempty"`
	InteractiveCommand string    `json:"interactiveCommand,omitempty"`
	CleanupCompleted   *bool     `json:"cleanupCompleted,omitempty"`
	DurationMs         int64     `json:"durationMs"`
	StdoutTail         string    `json:"stdoutTail"`
	StderrTail         string    `json:"stderrTail"`
	Truncated          Truncated `json:"truncated"`
}

type Truncated struct {
	Stdout bool `json:"stdout"`
	Stderr bool `json:"stderr"`
}

type TerminalOptions struct {
	NoColor  bool
	NoGlyphs bool
}

type MarkdownOptions struct {
	StripANSI bool
}

type StepSummaryOptions struct {
	Mode           string
	Status         int
	ReportJSONPath string
	Files          []string
}

func New(plan planning.Plan, started time.Time) Report {
	return Report{
		Kind:              "report",
		SchemaVersion:     SchemaVersion,
		SetupproofVersion: app.Version,
		StartedAt:         started.UTC().Format(time.RFC3339Nano),
		Invocation: Invocation{
			Args:       append([]string(nil), plan.Invocation.Args...),
			ConfigPath: plan.Invocation.ConfigPath,
		},
		Workspace: plan.Workspace,
		Runner:    plan.Runner,
		Files:     append([]string(nil), plan.Files...),
		Warnings:  append([]string(nil), plan.Warnings...),
		Blocks:    []Block{},
	}
}

func Finalize(r *Report, exitCode int, started time.Time, finished time.Time) {
	if r.Warnings == nil {
		r.Warnings = []string{}
	}
	if r.Blocks == nil {
		r.Blocks = []Block{}
	}
	r.ExitCode = exitCode
	r.DurationMs = finished.Sub(started).Milliseconds()
	if len(r.Blocks) == 0 && exitCode == 0 {
		r.Result = "noop"
		return
	}
	switch exitCode {
	case 0:
		r.Result = "passed"
	case 1:
		r.Result = "failed"
	default:
		r.Result = "error"
	}
}

func SetRunnerError(r *Report, reason string) {
	if reason == "" {
		reason = "other"
	}
	r.Runner.Error = &planning.RunnerError{Reason: reason}
}

func WriteJSON(w io.Writer, r Report) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(r)
}

func RenderTerminal(w io.Writer, r Report, opts TerminalOptions) error {
	if r.Result == "noop" {
		return renderTerminalNoop(w, r, opts)
	}
	if opts.NoGlyphs {
		return renderTerminalRows(w, r, opts)
	}
	if len(r.Blocks) == 0 && r.Result == "error" {
		if _, err := fmt.Fprintf(w, "%sSetupProof %s  %s\n", terminalStatusPrefix(r.Result, opts), r.Result, terminalMuted(terminalDuration(r.DurationMs), opts)); err != nil {
			return err
		}
		if r.Runner.Kind != "" {
			if _, err := fmt.Fprintf(w, "  runner: %s\n", r.Runner.Kind); err != nil {
				return err
			}
		}
		if r.Runner.Error != nil && r.Runner.Error.Reason != "" {
			if _, err := fmt.Fprintf(w, "  reason: %s\n", r.Runner.Error.Reason); err != nil {
				return err
			}
		}
		if next := doctorCommand(r.Files); next != "" {
			_, err := fmt.Fprintf(w, "  next: %s\n", next)
			return err
		}
		return nil
	}
	if _, err := fmt.Fprintf(w, "%sSetupProof %s  %s\n", terminalStatusPrefix(r.Result, opts), r.Result, terminalMuted(terminalDuration(r.DurationMs), opts)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  %s, %s\n\n", terminalCount(len(r.Blocks), "block"), terminalCount(len(r.Files), "file")); err != nil {
		return err
	}
	for index, block := range r.Blocks {
		if index > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "%s%s %s  %s\n", terminalStatusPrefix(block.Result, opts), summaryBlockID(block), terminalResultText(block.Result), terminalMuted(terminalDuration(block.DurationMs), opts)); err != nil {
			return err
		}
		for _, details := range terminalBlockDetailLines(block, opts) {
			if _, err := fmt.Fprintf(w, "  %s\n", details); err != nil {
				return err
			}
		}
		if needsTerminalGuidance(block.Result) {
			for _, line := range terminalFailureDetailLines(block, opts) {
				if _, err := fmt.Fprintf(w, "  %s\n", line); err != nil {
					return err
				}
			}
			if hint := terminalFailureHint(block); hint != "" {
				if _, err := fmt.Fprintf(w, "  hint: %s\n", hint); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(w, "  next: %s\n", reviewCommand(block.File)); err != nil {
				return err
			}
		}
	}
	return nil
}

func renderTerminalRows(w io.Writer, r Report, opts TerminalOptions) error {
	if len(r.Blocks) == 0 && r.Result == "error" {
		line := "SetupProof runner error"
		if r.Runner.Kind != "" {
			line += " runner=" + r.Runner.Kind
		}
		if r.Runner.Error != nil && r.Runner.Error.Reason != "" {
			line += " reason=" + r.Runner.Error.Reason
		}
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
		if next := doctorCommand(r.Files); next != "" {
			_, err := fmt.Fprintf(w, "  next command: %s\n", next)
			return err
		}
		return nil
	}
	for _, block := range r.Blocks {
		line := terminalStatusPrefix(block.Result, opts) + fmt.Sprintf("%s file=%s:%d runner=%s", block.QualifiedID, block.File, block.Line, block.Runner)
		if block.DockerImage != "" {
			line += " image=" + block.DockerImage
		}
		if block.Timeout != "" {
			line += " timeout=" + block.Timeout
		}
		line += " result=" + block.Result
		if block.Result == "failed" || block.Result == "timeout" {
			line += fmt.Sprintf(" exit=%d", block.ExitCode)
		}
		if block.Reason != "" {
			line += " reason=" + block.Reason
		}
		if block.InteractiveCommand != "" {
			line += " command=" + block.InteractiveCommand
		}
		if block.CleanupCompleted != nil {
			line += fmt.Sprintf(" cleanupCompleted=%t", *block.CleanupCompleted)
		}
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
		if needsTerminalGuidance(block.Result) {
			if _, err := fmt.Fprintf(w, "  next command: %s\n", reviewCommand(block.File)); err != nil {
				return err
			}
		}
	}
	return nil
}

func renderTerminalNoop(w io.Writer, r Report, opts TerminalOptions) error {
	if opts.NoGlyphs {
		_, err := fmt.Fprintln(w, "No marked blocks found.")
		return err
	}
	if _, err := fmt.Fprintln(w, "No marked blocks found."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Add one above a shell block:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "  <!-- setupproof id=quickstart -->"); err != nil {
		return err
	}
	if next := suggestCommand(r.Files); next != "" {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		_, err := fmt.Fprintf(w, "Next command:\n  %s\n", next)
		return err
	}
	return nil
}

func terminalBlockDetailLines(block Block, opts TerminalOptions) []string {
	var lines []string
	var placement []string
	if location := summaryLocation(block); location != "" {
		placement = append(placement, location)
	}
	if block.Runner != "" {
		placement = append(placement, "runner="+block.Runner)
	}
	if block.Timeout != "" {
		placement = append(placement, "timeout="+block.Timeout)
	}
	if block.Result != "" {
		placement = append(placement, "result="+block.Result)
	}
	if block.Result == "failed" || block.Result == "timeout" {
		placement = append(placement, fmt.Sprintf("exit=%d", block.ExitCode))
	}
	if block.Reason != "" {
		placement = append(placement, "reason="+block.Reason)
	}
	if block.InteractiveCommand != "" {
		placement = append(placement, "command="+block.InteractiveCommand)
	}
	if block.CleanupCompleted != nil {
		cleanup := "incomplete"
		if *block.CleanupCompleted {
			cleanup = "completed"
		}
		placement = append(placement, "cleanup="+cleanup)
	}
	if len(placement) > 0 {
		lines = append(lines, terminalMuted(strings.Join(placement, " "), opts))
	}
	if block.DockerImage != "" {
		lines = append(lines, "image: "+block.DockerImage)
	}
	return lines
}

func terminalFailureDetailLines(block Block, opts TerminalOptions) []string {
	tail, title := summaryBestTail(block)
	lines := terminalTailLines(tail, 6, 120)
	if len(lines) == 0 {
		return nil
	}
	label := strings.ToLower(strings.TrimSuffix(title, " (truncated)"))
	if label == "" {
		label = "output"
	}
	var details []string
	details = append(details, label+":")
	for _, line := range lines {
		details = append(details, terminalMuted("  "+line, opts))
	}
	return details
}

func terminalTailLines(value string, maxLines int, maxWidth int) []string {
	value = strings.ReplaceAll(StripANSI(value), "\r", "")
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, terminalTruncateMiddle(line, maxWidth))
	}
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines
}

func terminalTruncateMiddle(value string, maxWidth int) string {
	if maxWidth <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxWidth {
		return value
	}
	if maxWidth <= 3 {
		return string(runes[:maxWidth])
	}
	left := (maxWidth - 3) / 2
	right := maxWidth - 3 - left
	return string(runes[:left]) + "..." + string(runes[len(runes)-right:])
}

func terminalFailureHint(block Block) string {
	switch block.Result {
	case "timeout":
		return "raise the timeout or split the setup block"
	case "failed":
		if block.InteractiveCommand != "" {
			return "replace interactive input with non-interactive flags or fixtures"
		}
	case "error":
		switch block.Reason {
		case "shell_unavailable":
			return "install the requested shell or change the block language"
		case "image_pull_failed":
			return "check the Docker image name, tag, and registry access"
		}
	}
	return ""
}

func terminalCount(count int, singular string) string {
	label := singular
	if count != 1 {
		label += "s"
	}
	return fmt.Sprintf("%d %s", count, label)
}

func terminalDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return (time.Duration(ms) * time.Millisecond).String()
}

func terminalResultText(result string) string {
	if result == "timeout" {
		return "timed out"
	}
	if strings.TrimSpace(result) == "" {
		return "finished"
	}
	return result
}

func terminalStatusPrefix(result string, opts TerminalOptions) string {
	var label string
	if opts.NoGlyphs {
		label = "[" + result + "]"
	} else {
		switch result {
		case "passed":
			label = "+"
		case "skipped":
			label = "-"
		default:
			label = "!"
		}
	}
	if !opts.NoColor {
		label = terminalColor(result) + label + terminalANSIReset
	}
	return label + " "
}

const (
	terminalANSIReset = "\x1b[0m"
	terminalANSIDim   = "\x1b[2m"
)

func terminalColor(result string) string {
	switch result {
	case "passed":
		return "\x1b[32m"
	case "skipped":
		return "\x1b[90m"
	default:
		return "\x1b[31m"
	}
}

func terminalMuted(value string, opts TerminalOptions) string {
	if opts.NoColor || value == "" {
		return value
	}
	return terminalANSIDim + value + terminalANSIReset
}

func needsTerminalGuidance(result string) bool {
	switch result {
	case "failed", "timeout", "error":
		return true
	default:
		return false
	}
}

func reviewCommand(file string) string {
	if file == "" {
		return app.CommandName + " review <markdown-file>"
	}
	return app.CommandName + " review " + shellArg(file)
}

func suggestCommand(files []string) string {
	if len(files) == 0 {
		return app.CommandName + " suggest <markdown-file>"
	}
	var quoted []string
	for _, file := range files {
		if file == "" {
			continue
		}
		quoted = append(quoted, shellArg(file))
	}
	if len(quoted) == 0 {
		return app.CommandName + " suggest <markdown-file>"
	}
	return app.CommandName + " suggest " + strings.Join(quoted, " ")
}

func doctorCommand(files []string) string {
	if len(files) == 0 {
		return app.CommandName + " doctor <markdown-file>"
	}
	var quoted []string
	for _, file := range files {
		if file == "" {
			continue
		}
		quoted = append(quoted, shellArg(file))
	}
	if len(quoted) == 0 {
		return app.CommandName + " doctor <markdown-file>"
	}
	return app.CommandName + " doctor " + strings.Join(quoted, " ")
}

func shellArg(value string) string {
	if value == "" {
		return shellquote.Quote(value)
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return !(r == '/' || r == '.' || r == '_' || r == '-' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'))
	}) < 0 {
		return value
	}
	return shellquote.Quote(value)
}

func RenderMarkdown(r Report, opts MarkdownOptions) string {
	var builder strings.Builder
	builder.WriteString("# SetupProof Report\n\n")
	builder.WriteString("- result: ")
	builder.WriteString(r.Result)
	builder.WriteString("\n- exit code: ")
	builder.WriteString(fmt.Sprint(r.ExitCode))
	builder.WriteString("\n- duration ms: ")
	builder.WriteString(fmt.Sprint(r.DurationMs))
	builder.WriteString("\n- runner: ")
	builder.WriteString(r.Runner.Kind)
	builder.WriteString("\n- workspace: ")
	builder.WriteString(r.Workspace.Mode)
	builder.WriteString("\n")
	if len(r.Warnings) > 0 {
		builder.WriteString("\n## Warnings\n\n")
		for _, warning := range r.Warnings {
			builder.WriteString("- ")
			builder.WriteString(markdownText(warning))
			builder.WriteString("\n")
		}
	}
	if len(r.Blocks) == 0 {
		if r.Result == "error" {
			builder.WriteString("\nNo block reports were produced.\n")
			if r.Runner.Error != nil && r.Runner.Error.Reason != "" {
				builder.WriteString("\n- runner error reason: ")
				builder.WriteString(r.Runner.Error.Reason)
				builder.WriteString("\n")
			}
			return builder.String()
		}
		builder.WriteString("\nNo marked blocks were found.\n")
		return builder.String()
	}
	builder.WriteString("\n## Blocks\n\n")
	for _, block := range r.Blocks {
		builder.WriteString("### ")
		builder.WriteString(markdownText(block.QualifiedID))
		builder.WriteString("\n\n")
		builder.WriteString("- result: ")
		builder.WriteString(block.Result)
		builder.WriteString("\n- file: ")
		builder.WriteString(markdownText(block.File))
		builder.WriteString(":")
		builder.WriteString(fmt.Sprint(block.Line))
		builder.WriteString("\n- runner: ")
		builder.WriteString(block.Runner)
		if block.DockerImage != "" {
			builder.WriteString("\n- docker image: ")
			builder.WriteString(markdownText(block.DockerImage))
		}
		builder.WriteString("\n- timeout: ")
		builder.WriteString(block.Timeout)
		builder.WriteString("\n- shell: ")
		builder.WriteString(block.Shell)
		builder.WriteString("\n- strict: ")
		builder.WriteString(fmt.Sprint(block.Strict))
		builder.WriteString("\n- stdin: ")
		builder.WriteString(block.Stdin)
		builder.WriteString("\n- tty: ")
		builder.WriteString(fmt.Sprint(block.TTY))
		builder.WriteString("\n- state mode: ")
		builder.WriteString(block.StateMode)
		builder.WriteString("\n- isolated: ")
		builder.WriteString(fmt.Sprint(block.Isolated))
		builder.WriteString("\n- exit code: ")
		builder.WriteString(fmt.Sprint(block.ExitCode))
		builder.WriteString("\n- duration ms: ")
		builder.WriteString(fmt.Sprint(block.DurationMs))
		if block.Reason != "" {
			builder.WriteString("\n- reason: ")
			builder.WriteString(block.Reason)
		}
		if block.CleanupCompleted != nil {
			builder.WriteString("\n- cleanup completed: ")
			builder.WriteString(fmt.Sprint(*block.CleanupCompleted))
		}
		builder.WriteString("\n\nSource:\n\n")
		writeFencedLog(&builder, "", block.Source, opts)
		writeTail(&builder, "Stdout tail", block.StdoutTail, block.Truncated.Stdout, opts)
		writeTail(&builder, "Stderr tail", block.StderrTail, block.Truncated.Stderr, opts)
	}
	return builder.String()
}

func RenderGitHubStepSummary(r *Report, opts StepSummaryOptions) string {
	var lines []string
	lines = append(lines, "## SetupProof", "")
	if opts.Mode == "review" {
		lines = append(lines,
			"- mode: review",
			fmt.Sprintf("- exit code: %d", opts.Status),
			"- report JSON: not produced in review mode",
			"",
			"Review mode is non-executing.",
		)
		if len(opts.Files) > 0 {
			lines = append(lines, "", "- files: "+summaryText(strings.Join(opts.Files, ", "), 240))
		}
		return strings.Join(lines, "\n") + "\n"
	}
	if r == nil {
		lines = append(lines,
			"- result: unavailable",
			fmt.Sprintf("- exit code: %d", opts.Status),
			"- report JSON: `"+summaryText(opts.ReportJSONPath, 240)+"`",
			"",
			"SetupProof did not produce a readable JSON report.",
		)
		return strings.Join(lines, "\n") + "\n"
	}
	lines = append(lines,
		"- result: "+summaryText(r.Result, 80),
		fmt.Sprintf("- exit code: %d", r.ExitCode),
		fmt.Sprintf("- duration ms: %d", r.DurationMs),
		"- report JSON: `"+summaryText(opts.ReportJSONPath, 240)+"`",
		"- files: "+summaryText(strings.Join(r.Files, ", "), 240),
		"",
	)
	if len(r.Warnings) > 0 {
		lines = append(lines, "### Warnings", "")
		for index, warning := range r.Warnings {
			if index >= 10 {
				lines = append(lines, fmt.Sprintf("- ... %d more", len(r.Warnings)-index))
				break
			}
			lines = append(lines, "- "+summaryText(warning, 240))
		}
		lines = append(lines, "")
	}
	if len(r.Blocks) == 0 {
		lines = append(lines, "No marked blocks were reported.", "")
		return strings.Join(lines, "\n") + "\n"
	}
	failed := nonPassingBlocks(r.Blocks)
	selected := failed
	title := "Failing Blocks"
	if len(selected) == 0 {
		selected = r.Blocks
		title = "Blocks"
	}
	if len(selected) > 15 {
		selected = selected[:15]
	}
	lines = append(lines,
		"### "+title,
		"",
		"| Block | Location | Result | Exit | Reason |",
		"| --- | --- | --- | ---: | --- |",
	)
	for _, block := range selected {
		lines = append(lines, fmt.Sprintf("| %s | %s | %s | %d | %s |",
			summaryText(summaryBlockID(block), 160),
			summaryText(summaryLocation(block), 120),
			summaryText(block.Result, 80),
			block.ExitCode,
			summaryText(block.Reason, 120),
		))
	}
	omitted := len(failed) - len(selected)
	if len(failed) == 0 {
		omitted = len(r.Blocks) - len(selected)
	}
	if omitted > 0 {
		lines = append(lines, fmt.Sprintf("| ... %d more |  |  |  |  |", omitted))
	}
	lines = append(lines, "")
	if len(failed) > 0 {
		lines = append(lines, summaryFailureDetails(failed)...)
	}
	return strings.Join(lines, "\n") + "\n"
}

func summaryFailureDetails(blocks []Block) []string {
	selected := blocks
	if len(selected) > 3 {
		selected = selected[:3]
	}
	lines := []string{"### Failure Details", ""}
	for _, block := range selected {
		lines = append(lines,
			"#### "+summaryText(summaryBlockID(block), 160),
			"",
			"- location: "+summaryText(summaryLocation(block), 160),
			"- runner: "+summaryText(block.Runner, 80),
			"- timeout: "+summaryText(block.Timeout, 80),
			"- next command: "+summaryText(reviewCommand(block.File), 240),
		)
		if block.InteractiveCommand != "" {
			lines = append(lines, "- interactive command: "+summaryText(block.InteractiveCommand, 120))
		}
		if strings.TrimSpace(block.Source) != "" {
			lines = append(lines, "", "Source:", "", summaryFence("", block.Source, 1200))
		}
		if tail, title := summaryBestTail(block); tail != "" {
			lines = append(lines, title+":", "", summaryFence("text", tail, 1600))
		}
		lines = append(lines, "")
	}
	if len(blocks) > len(selected) {
		lines = append(lines, fmt.Sprintf("_%d more failing block(s) omitted from details._", len(blocks)-len(selected)), "")
	}
	return lines
}

func summaryBlockID(block Block) string {
	if block.QualifiedID != "" {
		return block.QualifiedID
	}
	if block.File == "" {
		return block.ID
	}
	return block.File + "#" + block.ID
}

func summaryLocation(block Block) string {
	if block.File == "" {
		return ""
	}
	if block.Line <= 0 {
		return block.File
	}
	return fmt.Sprintf("%s:%d", block.File, block.Line)
}

func summaryBestTail(block Block) (string, string) {
	if strings.TrimSpace(block.StderrTail) != "" {
		title := "Stderr tail"
		if block.Truncated.Stderr {
			title += " (truncated)"
		}
		return block.StderrTail, title
	}
	if strings.TrimSpace(block.StdoutTail) != "" {
		title := "Stdout tail"
		if block.Truncated.Stdout {
			title += " (truncated)"
		}
		return block.StdoutTail, title
	}
	return "", ""
}

func summaryFence(info string, value string, maxRunes int) string {
	value = summaryLog(value, maxRunes)
	fence := markdownFence(value)
	var builder strings.Builder
	builder.WriteString(fence)
	if info != "" {
		builder.WriteString(info)
	}
	builder.WriteString("\n")
	builder.WriteString(value)
	if !strings.HasSuffix(value, "\n") {
		builder.WriteString("\n")
	}
	builder.WriteString(fence)
	return builder.String()
}

func summaryLog(value string, maxRunes int) string {
	value = StripANSI(strings.ReplaceAll(value, "\r", ""))
	if maxRunes <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "\n... truncated\n"
}

func nonPassingBlocks(blocks []Block) []Block {
	failed := make([]Block, 0)
	for _, block := range blocks {
		if block.Result != "passed" {
			failed = append(failed, block)
		}
	}
	return failed
}

func summaryText(value string, maxLen int) string {
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "<br>")
	value = strings.ReplaceAll(value, "|", `\|`)
	if maxLen > 3 && len(value) > maxLen {
		return value[:maxLen-3] + "..."
	}
	return value
}

func writeTail(builder *strings.Builder, title string, tail string, truncated bool, opts MarkdownOptions) {
	builder.WriteString(title)
	builder.WriteString(":\n\n")
	if tail == "" {
		builder.WriteString("_empty_\n\n")
		return
	}
	if truncated {
		builder.WriteString("_truncated to last 64 KiB_\n\n")
	}
	writeFencedLog(builder, "text", tail, opts)
}

func writeFencedLog(builder *strings.Builder, info string, value string, opts MarkdownOptions) {
	text := markdownLog(value, opts)
	fence := markdownFence(text)
	builder.WriteString(fence)
	if info != "" {
		builder.WriteString(info)
	}
	builder.WriteString("\n")
	builder.WriteString(text)
	if !strings.HasSuffix(text, "\n") {
		builder.WriteString("\n")
	}
	builder.WriteString(fence)
	builder.WriteString("\n\n")
}

func markdownFence(value string) string {
	longest := 0
	current := 0
	for i := 0; i < len(value); i++ {
		if value[i] == '`' {
			current++
			if current > longest {
				longest = current
			}
			continue
		}
		current = 0
	}
	fenceLength := longest + 1
	if fenceLength < 4 {
		fenceLength = 4
	}
	return strings.Repeat("`", fenceLength)
}

func markdownText(value string) string {
	return strings.ReplaceAll(value, "\r", "")
}

func markdownLog(value string, opts MarkdownOptions) string {
	value = strings.ReplaceAll(value, "\r", "")
	if opts.StripANSI {
		value = StripANSI(value)
	}
	return value
}

func ResolveOutputPath(cwd string, requested string) (string, error) {
	if requested == "" {
		return "", fmt.Errorf("report path must not be empty")
	}
	resolved := requested
	if !filepath.IsAbs(resolved) {
		if cwd == "" {
			var err error
			cwd, err = os.Getwd()
			if err != nil {
				return "", err
			}
		}
		resolved = filepath.Join(cwd, requested)
	}
	resolved = filepath.Clean(resolved)
	parent := filepath.Dir(resolved)
	info, err := os.Stat(parent)
	if err != nil {
		return "", fmt.Errorf("report path parent does not exist: %s", parent)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("report path parent is not a directory: %s", parent)
	}
	if info, err := os.Lstat(resolved); err == nil {
		switch mode := info.Mode(); {
		case mode.IsDir():
			return "", fmt.Errorf("report path is a directory: %s", requested)
		case mode&os.ModeSymlink != 0:
			return "", fmt.Errorf("report path must not be a symlink: %s", requested)
		case !mode.IsRegular():
			return "", fmt.Errorf("report path is not a regular file: %s", requested)
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return resolved, nil
}

func WriteResolvedFile(path string, data []byte) error {
	if path == "" {
		return fmt.Errorf("report path must not be empty")
	}
	if info, err := os.Lstat(path); err == nil {
		switch mode := info.Mode(); {
		case mode.IsDir():
			return fmt.Errorf("report path is a directory: %s", path)
		case mode&os.ModeSymlink != 0:
			return fmt.Errorf("report path must not be a symlink: %s", path)
		case !mode.IsRegular():
			return fmt.Errorf("report path is not a regular file: %s", path)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

type Redactor struct {
	secrets          []string
	streamSuffixSize int
	multilineSecrets bool
}

func NewRedactor(secrets []string) Redactor {
	filtered := make([]string, 0, len(secrets))
	seen := make(map[string]bool)
	maxLength := 0
	multilineSecrets := false
	for _, secret := range secrets {
		if secret == "" || seen[secret] {
			continue
		}
		seen[secret] = true
		filtered = append(filtered, secret)
		if len(secret) > maxLength {
			maxLength = len(secret)
		}
		if strings.ContainsAny(secret, "\r\n") {
			multilineSecrets = true
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return len(filtered[i]) > len(filtered[j])
	})
	suffixSize := maxLength - 1
	if suffixSize < 0 {
		suffixSize = 0
	}
	return Redactor{secrets: filtered, streamSuffixSize: suffixSize, multilineSecrets: multilineSecrets}
}

func (r Redactor) Redact(value string) string {
	for _, secret := range r.secrets {
		value = strings.ReplaceAll(value, secret, "[redacted]")
	}
	return value
}

type Tail struct {
	max       int
	buffer    []byte
	truncated bool
}

func NewTail(max int) *Tail {
	if max <= 0 {
		max = MaxTailBytes
	}
	return &Tail{max: max}
}

func (t *Tail) Write(p []byte) (int, error) {
	t.AppendString(string(p))
	return len(p), nil
}

func (t *Tail) AppendString(value string) {
	if value == "" {
		return
	}
	t.buffer = append(t.buffer, []byte(value)...)
	if len(t.buffer) > t.max {
		drop := len(t.buffer) - t.max
		copy(t.buffer, t.buffer[drop:])
		t.buffer = t.buffer[:t.max]
		t.truncated = true
	}
}

func (t *Tail) String() string {
	return string(t.buffer)
}

func (t *Tail) Truncated() bool {
	return t.truncated
}

type StreamCollector struct {
	Sink     io.Writer
	Tail     *Tail
	Redactor Redactor
	pending  string
}

func (w *StreamCollector) Write(p []byte) (int, error) {
	// Keep a suffix when streaming so a secret split across writes can still
	// match on the next write.
	if w.Redactor.streamSuffixSize > 0 {
		emit, pending := w.Redactor.redactStreamPrefix(w.pending + string(p))
		w.pending = pending
		if err := w.writeString(emit); err != nil {
			return 0, err
		}
		return len(p), nil
	}
	if err := w.writeString(w.Redactor.Redact(string(p))); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *StreamCollector) Flush() error {
	if w.pending == "" {
		return nil
	}
	pending := w.pending
	w.pending = ""
	return w.writeString(w.Redactor.Redact(pending))
}

func (w *StreamCollector) writeString(text string) error {
	if text == "" {
		return nil
	}
	text = strings.ReplaceAll(text, "\r", "")
	if w.Tail != nil {
		w.Tail.AppendString(text)
	}
	if w.Sink != nil {
		if _, err := io.WriteString(w.Sink, text); err != nil {
			return err
		}
	}
	return nil
}

func (r Redactor) redactStreamPrefix(input string) (string, string) {
	safeLimit := len(input) - r.streamSuffixSize
	if safeLimit < 0 {
		safeLimit = 0
	}
	if !r.multilineSecrets {
		if newline := strings.LastIndexAny(input, "\r\n"); newline >= safeLimit {
			safeLimit = newline + 1
		}
	}
	var builder strings.Builder
	for i := 0; i < len(input); {
		if i >= safeLimit {
			return builder.String(), input[i:]
		}
		if secret, ok := r.matchSecretAt(input, i); ok {
			builder.WriteString("[redacted]")
			i += len(secret)
			continue
		}
		builder.WriteByte(input[i])
		i++
	}
	return builder.String(), ""
}

func (r Redactor) matchSecretAt(input string, index int) (string, bool) {
	var match string
	for _, secret := range r.secrets {
		if len(secret) <= len(match) {
			continue
		}
		if strings.HasPrefix(input[index:], secret) {
			match = secret
		}
	}
	return match, match != ""
}

type Output struct {
	StdoutTail      string
	StderrTail      string
	StdoutTruncated bool
	StderrTruncated bool
}

func OutputFromTails(stdoutTail *Tail, stderrTail *Tail) Output {
	var output Output
	if stdoutTail != nil {
		output.StdoutTail = stdoutTail.String()
		output.StdoutTruncated = stdoutTail.Truncated()
	}
	if stderrTail != nil {
		output.StderrTail = stderrTail.String()
		output.StderrTruncated = stderrTail.Truncated()
	}
	return output
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func StripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}

func JSONBytes(r Report) ([]byte, error) {
	var buffer bytes.Buffer
	if err := WriteJSON(&buffer, r); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
