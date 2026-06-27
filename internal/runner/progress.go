package runner

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/setupproof/setupproof/internal/planning"
)

var progressFrames = []string{"-", "\\", "|", "/"}

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
	blockID                string
	index                  int
	total                  int
	started                time.Time
	done                   chan struct{}
	stopOnce               sync.Once
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
	span := &progressSpan{progress: p}
	if p == nil || !p.enabled {
		return span
	}
	p.mu.Lock()
	p.current++
	span.blockID = block.QualifiedID
	span.index = p.current
	span.total = p.total
	span.started = time.Now()
	span.done = make(chan struct{})
	p.active = span
	p.renderLocked(span, 0)
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
			p.clearLocked()
			fmt.Fprintf(p.w, "==> %s\n", progressLabel(span))
			span.outputSeen = true
			span.stop()
		}
		if len(data) > 0 {
			span.outputEndedWithNewline = data[len(data)-1] == '\n'
		}
	}
	return p.w.Write(data)
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
	if !s.outputSeen {
		p.clearLocked()
	}
	fmt.Fprintf(p.w, "%s %s %s in %s\n", progressStatus(result, p.noColor), s.blockID, progressResultText(result), progressDuration(durationMs))
	p.active = nil
}

func (s *progressSpan) animate() {
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()
	frame := 0
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			frame = (frame + 1) % len(progressFrames)
			s.progress.mu.Lock()
			if s.progress.active == s && !s.outputSeen {
				s.progress.renderLocked(s, frame)
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

func (p *terminalProgress) renderLocked(s *progressSpan, frame int) {
	fmt.Fprintf(p.w, "\r\x1b[2K%s Running %s %s", progressFrames[frame], progressLabel(s), progressDuration(time.Since(s.started).Milliseconds()))
}

func (p *terminalProgress) clearLocked() {
	fmt.Fprint(p.w, "\r\x1b[2K")
}

func progressLabel(s *progressSpan) string {
	if s.total <= 1 {
		return s.blockID
	}
	return fmt.Sprintf("%s (%d/%d)", s.blockID, s.index, s.total)
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
