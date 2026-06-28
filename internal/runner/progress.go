package runner

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/setupproof/setupproof/internal/planning"
)

const (
	progressStartDelay = 160 * time.Millisecond
	progressTick       = 120 * time.Millisecond
	progressPhaseWidth = 15
	progressCountWidth = 3
	progressTimeWidth  = 7
)

const (
	progressANSIReset = "\x1b[0m"
	progressANSIBold  = "\x1b[1m"
	progressANSIDim   = "\x1b[2m"
	progressANSICyan  = "\x1b[36m"
)

type progressSpanKind int

const (
	progressSpanBlock progressSpanKind = iota
	progressSpanPhase
)

type terminalProgress struct {
	enabled bool
	noColor bool
	w       io.Writer
	total   int

	mu      sync.Mutex
	current int
	active  *progressSpan
}

type progressSpan struct {
	progress               *terminalProgress
	kind                   progressSpanKind
	blockID                string
	label                  string
	phase                  string
	count                  string
	started                time.Time
	done                   chan struct{}
	stopOnce               sync.Once
	rendered               bool
	outputSeen             bool
	outputEndedWithNewline bool
}

type progressOutputWriter struct {
	progress *terminalProgress
}

func newTerminalProgress(w io.Writer, opts Options, total int) *terminalProgress {
	return &terminalProgress{
		enabled: opts.Progress && !opts.NoColor && !opts.NoGlyphs && total > 0,
		noColor: opts.NoColor,
		w:       w,
		total:   total,
	}
}

func (p *terminalProgress) Start(block planning.Block) *progressSpan {
	return p.StartBlock(block)
}

func (p *terminalProgress) StartBlock(block planning.Block) *progressSpan {
	span := &progressSpan{progress: p}
	if p == nil || !p.enabled {
		return span
	}
	p.mu.Lock()
	p.current++
	span.kind = progressSpanBlock
	span.blockID = block.QualifiedID
	span.label = block.QualifiedID
	span.phase = "Running"
	span.count = progressCount(p.current, p.total)
	span.started = time.Now()
	span.done = make(chan struct{})
	p.active = span
	p.mu.Unlock()

	go span.animate()
	return span
}

func (p *terminalProgress) StartPhase(label string) *progressSpan {
	span := &progressSpan{progress: p}
	if p == nil || !p.enabled {
		return span
	}
	p.mu.Lock()
	span.kind = progressSpanPhase
	span.label = label
	span.phase = label
	span.started = time.Now()
	span.done = make(chan struct{})
	p.active = span
	p.mu.Unlock()

	go span.animate()
	return span
}

func (p *terminalProgress) OutputWriter() io.Writer {
	if p == nil || !p.enabled {
		if p == nil {
			return io.Discard
		}
		return p.w
	}
	return progressOutputWriter{progress: p}
}

func (w progressOutputWriter) Write(data []byte) (int, error) {
	return w.progress.writeOutput(data)
}

func (p *terminalProgress) writeOutput(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if span := p.active; span != nil {
		if !span.outputSeen {
			if span.rendered {
				p.clearLocked()
			}
			fmt.Fprintf(p.w, "%s %s\n", progressPrefix(p.noColor), span.outputLabel(p.noColor))
			span.outputSeen = true
			span.stop()
		}
		if len(data) > 0 {
			span.outputEndedWithNewline = data[len(data)-1] == '\n'
		}
	}
	return p.w.Write(data)
}

func (s *progressSpan) Phase(phase string) {
	if s == nil || s.progress == nil || !s.progress.enabled || strings.TrimSpace(phase) == "" {
		return
	}
	p := s.progress
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.active != s || s.outputSeen {
		return
	}
	s.phase = phase
	if s.rendered {
		p.renderLocked(s)
	}
}

func (s *progressSpan) Finish(result string, durationMs int64) {
	if s == nil || s.progress == nil || !s.progress.enabled {
		return
	}
	s.stop()
	if durationMs == 0 && !s.started.IsZero() {
		durationMs = time.Since(s.started).Milliseconds()
	}

	p := s.progress
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.active != s {
		return
	}
	if s.outputSeen && !s.outputEndedWithNewline {
		fmt.Fprintln(p.w)
	}
	if !s.outputSeen && s.rendered {
		p.clearLocked()
	}
	if s.kind == progressSpanBlock {
		fmt.Fprintln(p.w, progressCompletionLine(result, s.blockID, durationMs, p.noColor))
	}
	p.active = nil
}

func (s *progressSpan) animate() {
	delay := time.NewTimer(progressStartDelay)
	defer delay.Stop()
	select {
	case <-s.done:
		return
	case <-delay.C:
		s.progress.mu.Lock()
		if s.progress.active == s && !s.outputSeen {
			s.rendered = true
			s.progress.renderLocked(s)
		}
		s.progress.mu.Unlock()
	}

	ticker := time.NewTicker(progressTick)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.progress.mu.Lock()
			if s.progress.active == s && !s.outputSeen {
				s.progress.renderLocked(s)
			}
			s.progress.mu.Unlock()
		}
	}
}

func (s *progressSpan) stop() {
	if s.done == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.done)
	})
}

func (p *terminalProgress) renderLocked(s *progressSpan) {
	fmt.Fprintf(p.w, "\r\x1b[2K%s %s", progressPrefix(p.noColor), s.activityLabel(p.noColor, time.Since(s.started).Milliseconds()))
}

func (p *terminalProgress) clearLocked() {
	fmt.Fprint(p.w, "\r\x1b[2K")
}

func (s *progressSpan) activityLabel(noColor bool, durationMs int64) string {
	elapsed := progressDim(progressPadded(progressDuration(durationMs), progressTimeWidth), noColor)
	if s.kind == progressSpanPhase {
		return progressBold(s.label, noColor) + "  " + elapsed
	}
	phase := s.phase
	if phase == "" {
		phase = "Running"
	}
	line := progressBold(progressPaddedRight(phase, progressPhaseWidth), noColor) + "  " + s.label
	if s.count != "" {
		line += "  " + progressDim(progressPadded(s.count, progressCountWidth), noColor)
	}
	return line + "  " + elapsed
}

func (s *progressSpan) outputLabel(noColor bool) string {
	if s.kind == progressSpanPhase {
		return progressBold(s.label, noColor)
	}
	phase := s.phase
	if phase == "" {
		phase = "Running"
	}
	label := progressBold(phase, noColor) + " " + s.blockID
	if s.count != "" {
		label += " " + progressDim(s.count, noColor)
	}
	return label
}

func progressCount(index int, total int) string {
	if total <= 1 {
		return ""
	}
	return fmt.Sprintf("%d/%d", index, total)
}

func progressStatus(result string, noColor bool) string {
	label := "!"
	switch result {
	case "passed":
		label = "+"
	case "skipped":
		label = "-"
	}
	if noColor {
		return label
	}
	return progressColor(result) + label + "\x1b[0m"
}

func progressColor(result string) string {
	switch result {
	case "passed":
		return "\x1b[32m"
	case "skipped":
		return "\x1b[90m"
	default:
		return "\x1b[31m"
	}
}

func progressCompletionLine(result string, blockID string, durationMs int64, noColor bool) string {
	return fmt.Sprintf("%s %s %-7s %s", progressStatus(result, noColor), blockID, progressResultText(result), progressDim(progressPadded(progressDuration(durationMs), progressTimeWidth), noColor))
}

func progressPrefix(noColor bool) string {
	return progressCyan("==>", noColor)
}

func progressBold(value string, noColor bool) string {
	return progressStyle(value, progressANSIBold, noColor)
}

func progressDim(value string, noColor bool) string {
	return progressStyle(value, progressANSIDim, noColor)
}

func progressCyan(value string, noColor bool) string {
	return progressStyle(value, progressANSICyan, noColor)
}

func progressStyle(value string, style string, noColor bool) string {
	if noColor || value == "" {
		return value
	}
	return style + value + progressANSIReset
}

func progressPadded(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return fmt.Sprintf("%*s", width, value)
}

func progressPaddedRight(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return fmt.Sprintf("%-*s", width, value)
}

func progressResultText(result string) string {
	if strings.TrimSpace(result) == "" {
		return "finished"
	}
	return result
}

func progressDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return (time.Duration(ms) * time.Millisecond).String()
}
