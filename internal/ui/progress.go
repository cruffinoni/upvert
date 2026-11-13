package ui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/term"
)

const (
	progressInitialDelay    = 100 * time.Millisecond
	progressRefresh         = 100 * time.Millisecond
	defaultProgressBarWidth = 20
)

var progressFrames = []string{"|", "/", "-", "\\"}

// Progress reports at most one active step. Start begins a step, Total and
// Advance set its optional measurable progress, and Finish or Abort completes
// it. Pause and Resume let callers write other output without corrupting the
// active line.
type Progress interface {
	Start(label string)
	Total(total int64)
	Advance(done int64)
	Finish(detail string)
	Abort()
	Pause()
	Resume()
}

// ProgressOptions controls how NewProgress renders steps.
type ProgressOptions struct {
	// Quiet discards all progress.
	Quiet bool
	// Transient clears completed steps on a terminal instead of leaving them
	// on screen. Use it when a final report will list the same work.
	Transient bool
	// Width overrides the terminal width used for animation.
	Width int
}

// NewProgress selects a quiet, animated terminal, or plain line reporter.
// Animation is used only when out is an interactive terminal.
func NewProgress(out io.Writer, opts ProgressOptions) Progress {
	if opts.Quiet {
		return quietProgress{}
	}
	if file, ok := out.(*os.File); ok {
		fd := int(file.Fd())
		if term.IsTerminal(fd) {
			width := opts.Width
			if width <= 0 {
				width = 80
				if w, _, err := term.GetSize(fd); err == nil && w > 0 {
					width = w
				}
			}
			return &terminalProgress{out: out, width: width, transient: opts.Transient}
		}
	}
	return &lineProgress{out: out}
}

// NopProgress discards all progress.
func NopProgress() Progress { return quietProgress{} }

// formatBytes formats n using binary units.
func formatBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for value := n / unit; value >= unit; value /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// progressBarFill returns the number of filled cells for done out of total at the given width.
// It reports false when the bar should not be rendered.
func progressBarFill(done, total int64, width int) (int, bool) {
	if total <= 0 || width <= 0 {
		return 0, false
	}
	ratio := float64(done) / float64(total)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	return int(ratio*float64(width) + 0.5), true
}

// progressBarWithWidth renders an ASCII progress bar of the requested width.
func progressBarWithWidth(done, total int64, width int) string {
	filled, ok := progressBarFill(done, total, width)
	if !ok {
		return ""
	}
	return "[" + strings.Repeat("=", filled) + strings.Repeat(" ", width-filled) + "]"
}

// progressBar renders a progress bar at the default width.
func progressBar(done, total int64) string {
	return progressBarWithWidth(done, total, defaultProgressBarWidth)
}

// progressDetail describes a completed download using its byte count.
func progressDetail(done, total int64) string {
	if done > 0 {
		return formatBytes(done)
	}
	if total > 0 {
		return formatBytes(total)
	}
	return ""
}

// stepLine formats a completed step for failure output.
func stepLine(label, detail string) string {
	if detail == "" {
		return label + "..."
	}
	return label + "... " + detail
}

// quietProgress discards all progress.
type quietProgress struct{}

func (quietProgress) Start(string)  {}
func (quietProgress) Total(int64)   {}
func (quietProgress) Advance(int64) {}
func (quietProgress) Finish(string) {}
func (quietProgress) Abort()        {}
func (quietProgress) Pause()        {}
func (quietProgress) Resume()       {}

// lineProgress prints one persistent line per step for non-terminals.
type lineProgress struct {
	out         io.Writer
	label       string
	total       int64
	done        int64
	interrupted bool
}

// finish completes the active line, reprinting its label when other output
// interrupted it.
func (r *lineProgress) finish(detail string, failed bool) {
	if r.label == "" {
		return
	}
	label, done, total := r.label, r.done, r.total
	interrupted := r.interrupted
	r.label = ""
	r.total = 0
	r.done = 0
	r.interrupted = false
	if failed {
		if interrupted {
			fmt.Fprintf(r.out, "%s... failed\n", label)
		} else {
			fmt.Fprintln(r.out, " failed")
		}
		return
	}
	if detail == "" {
		detail = progressDetail(done, total)
	}
	if interrupted {
		if detail == "" {
			fmt.Fprintf(r.out, "%s...\n", label)
		} else {
			fmt.Fprintf(r.out, "%s... %s\n", label, detail)
		}
		return
	}
	if detail == "" {
		fmt.Fprintln(r.out)
	} else {
		fmt.Fprintf(r.out, " %s\n", detail)
	}
}

// Start begins a plain step line.
func (r *lineProgress) Start(label string) {
	if r.label != "" {
		r.finish("", true)
	}
	r.label = label
	r.total = 0
	r.done = 0
	r.interrupted = false
	fmt.Fprintf(r.out, "%s...", label)
}

func (r *lineProgress) Total(total int64) {
	r.total = total
}

func (r *lineProgress) Advance(done int64) {
	r.done = done
}

// Finish completes the active step line.
func (r *lineProgress) Finish(detail string) {
	r.finish(detail, false)
}

// Abort marks the active step line failed.
func (r *lineProgress) Abort() {
	r.finish("", true)
}

// Pause ends the current line so other output starts cleanly.
func (r *lineProgress) Pause() {
	if r.label == "" || r.interrupted {
		return
	}
	fmt.Fprintln(r.out)
	r.interrupted = true
}

// Resume is a no-op for plain lines; Finish reprints an interrupted label.
func (r *lineProgress) Resume() {}

// terminalProgress animates one line on an interactive terminal.
type terminalProgress struct {
	out io.Writer
	// width is the terminal width, or a fallback when it is unknown.
	width int
	// transient clears completed steps instead of printing them.
	transient bool

	mu        sync.Mutex
	label     string
	total     int64
	done      int64
	frame     int
	active    bool
	paused    bool
	cancel    context.CancelFunc
	group     *errgroup.Group
	lastWidth int
	// completed holds transient steps so a later failure can show what ran.
	completed []string
}

// writeLineLocked repaints the active line without ANSI escapes.
func (r *terminalProgress) writeLineLocked(line string) {
	width := r.width
	if width <= 0 {
		width = 80
	}
	runes := []rune(line)
	if width > 1 && len(runes) >= width {
		runes = runes[:width-1]
		line = string(runes)
	}
	count := len(runes)
	padding := ""
	if count < r.lastWidth {
		padding = strings.Repeat(" ", r.lastWidth-count)
	}
	fmt.Fprintf(r.out, "\r%s%s", line, padding)
	r.lastWidth = count
}

// renderLocked paints the current frame and download state.
func (r *terminalProgress) renderLocked() {
	if !r.active || r.paused || r.label == "" {
		return
	}
	frame := progressFrames[r.frame%len(progressFrames)]
	prefix := frame + " " + r.label + "..."
	line := prefix
	switch {
	case r.total > 0:
		ratio := float64(r.done) / float64(r.total)
		if ratio < 0 {
			ratio = 0
		}
		if ratio > 1 {
			ratio = 1
		}
		suffix := fmt.Sprintf(" %3d%%  %s / %s", int(ratio*100+0.5), formatBytes(r.done), formatBytes(r.total))
		width := r.width
		if width <= 0 {
			width = 80
		}
		barWidth := width - len([]rune(prefix)) - len([]rune(suffix)) - 4
		if barWidth < 10 {
			barWidth = 10
		}
		if barWidth > 30 {
			barWidth = 30
		}
		line = prefix + " " + progressBarWithWidth(r.done, r.total, barWidth) + suffix
	case r.done > 0:
		line = prefix + " " + formatBytes(r.done)
	}
	r.writeLineLocked(line)
}

// animate repaints the active line until ctx is canceled.
func (r *terminalProgress) animate(ctx context.Context) {
	timer := time.NewTimer(progressInitialDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	r.mu.Lock()
	if r.active && !r.paused {
		r.renderLocked()
	}
	r.mu.Unlock()

	ticker := time.NewTicker(progressRefresh)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.mu.Lock()
			if r.active && !r.paused {
				r.frame++
				r.renderLocked()
			}
			r.mu.Unlock()
		}
	}
}

// stopAnimation stops the redraw loop without changing the step state.
func (r *terminalProgress) stopAnimation() {
	r.mu.Lock()
	cancel, group := r.cancel, r.group
	r.cancel, r.group = nil, nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if group != nil {
		_ = group.Wait()
	}
}

// Start begins an animated step line.
func (r *terminalProgress) Start(label string) {
	r.stopAnimation()
	ctx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.label = label
	r.total = 0
	r.done = 0
	r.frame = 0
	r.active = true
	r.paused = false
	r.cancel = cancel
	r.group = &errgroup.Group{}
	group := r.group
	group.Go(func() error {
		r.animate(ctx)
		return nil
	})
	r.mu.Unlock()
}

func (r *terminalProgress) Total(total int64) {
	r.mu.Lock()
	r.total = total
	r.mu.Unlock()
}

func (r *terminalProgress) Advance(done int64) {
	r.mu.Lock()
	r.done = done
	r.mu.Unlock()
}

// clearLineLocked erases the active line without ANSI escapes.
func (r *terminalProgress) clearLineLocked() {
	if r.lastWidth == 0 {
		return
	}
	fmt.Fprintf(r.out, "\r%s\r", strings.Repeat(" ", r.lastWidth))
	r.lastWidth = 0
}

// finish stops animation and prints or clears the final line for the active step.
func (r *terminalProgress) finish(detail string, failed bool) {
	r.stopAnimation()
	r.mu.Lock()
	defer r.mu.Unlock()
	label, done, total := r.label, r.done, r.total
	r.clearLineLocked()
	r.active = false
	r.paused = false
	r.label = ""
	r.total = 0
	r.done = 0
	if label == "" {
		return
	}
	if detail == "" {
		detail = progressDetail(done, total)
	}
	if r.transient {
		if !failed {
			r.completed = append(r.completed, stepLine(label, detail))
			return
		}
		for _, line := range r.completed {
			fmt.Fprintln(r.out, line)
		}
		r.completed = nil
		fmt.Fprintf(r.out, "%s... failed\n", label)
		return
	}
	if failed {
		fmt.Fprintf(r.out, "%s... failed\n", label)
		return
	}
	if detail == "" {
		fmt.Fprintf(r.out, "%s...\n", label)
	} else {
		fmt.Fprintf(r.out, "%s... %s\n", label, detail)
	}
}

// Finish completes the active step line.
func (r *terminalProgress) Finish(detail string) {
	r.finish(detail, false)
}

// Abort marks the active step line failed.
func (r *terminalProgress) Abort() {
	r.finish("", true)
}

// Pause stops the animation and clears the active line.
func (r *terminalProgress) Pause() {
	r.stopAnimation()
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.active || r.paused || r.label == "" {
		return
	}
	r.paused = true
	r.clearLineLocked()
}

// Resume restarts the animation for a paused step.
func (r *terminalProgress) Resume() {
	r.mu.Lock()
	if !r.active || !r.paused || r.label == "" {
		r.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.paused = false
	r.cancel = cancel
	r.group = &errgroup.Group{}
	group := r.group
	group.Go(func() error {
		r.animate(ctx)
		return nil
	})
	r.mu.Unlock()
}
