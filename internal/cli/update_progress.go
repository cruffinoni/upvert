package cli

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

	"github.com/cruffinoni/upvert/internal/update"
)

const (
	progressInitialDelay    = 100 * time.Millisecond
	progressRefresh         = 100 * time.Millisecond
	defaultProgressBarWidth = 20
)

var progressFrames = []string{"|", "/", "-", "\\"}

// newUpdateReporter selects quiet, animated terminal, or plain line output.
func newUpdateReporter(out io.Writer, quiet bool) update.Reporter {
	if quiet {
		return quietUpdateReporter{}
	}
	file, ok := out.(*os.File)
	if ok && term.IsTerminal(int(file.Fd())) {
		width := 80
		if w, _, err := term.GetSize(int(file.Fd())); err == nil && w > 0 {
			width = w
		}
		return &terminalUpdateReporter{out: out, width: width}
	}
	return &lineUpdateReporter{out: out}
}

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

// quietUpdateReporter discards progress in --quiet mode.
type quietUpdateReporter struct{}

func (quietUpdateReporter) Start(string)  {}
func (quietUpdateReporter) Total(int64)   {}
func (quietUpdateReporter) Advance(int64) {}
func (quietUpdateReporter) Finish(string) {}
func (quietUpdateReporter) Abort()        {}

// lineUpdateReporter prints one persistent line per step for non-terminals.
type lineUpdateReporter struct {
	out   io.Writer
	label string
	total int64
	done  int64
}

// finish completes the active line with detail or a failed marker.
func (r *lineUpdateReporter) finish(detail string, failed bool) {
	if r.label == "" {
		return
	}
	if failed {
		fmt.Fprintf(r.out, " failed\n")
	} else {
		if detail == "" {
			detail = progressDetail(r.done, r.total)
		}
		if detail == "" {
			fmt.Fprintln(r.out)
		} else {
			fmt.Fprintf(r.out, " %s\n", detail)
		}
	}
	r.label = ""
	r.total = 0
	r.done = 0
}

// Start begins a plain step line.
func (r *lineUpdateReporter) Start(label string) {
	r.label = label
	r.total = 0
	r.done = 0
	fmt.Fprintf(r.out, "%s...", label)
}

func (r *lineUpdateReporter) Total(total int64) {
	r.total = total
}

func (r *lineUpdateReporter) Advance(done int64) {
	r.done = done
}

// Finish completes the active step line.
func (r *lineUpdateReporter) Finish(detail string) {
	r.finish(detail, false)
}

// Abort marks the active step line failed.
func (r *lineUpdateReporter) Abort() {
	r.finish("", true)
}

// terminalUpdateReporter animates one line on an interactive terminal.
type terminalUpdateReporter struct {
	out io.Writer
	// width is the terminal width, or a fallback when it is unknown.
	width int

	mu        sync.Mutex
	label     string
	total     int64
	done      int64
	frame     int
	active    bool
	cancel    context.CancelFunc
	group     *errgroup.Group
	lastWidth int
}

// writeLineLocked repaints the active line without ANSI escapes.
func (r *terminalUpdateReporter) writeLineLocked(line string) {
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
func (r *terminalUpdateReporter) renderLocked() {
	if !r.active || r.label == "" {
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
func (r *terminalUpdateReporter) animate(ctx context.Context) {
	timer := time.NewTimer(progressInitialDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	r.mu.Lock()
	if r.active {
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
			if r.active {
				r.frame++
				r.renderLocked()
			}
			r.mu.Unlock()
		}
	}
}

// Start begins an animated step line.
func (r *terminalUpdateReporter) Start(label string) {
	ctx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.label = label
	r.total = 0
	r.done = 0
	r.frame = 0
	r.active = true
	r.cancel = cancel
	r.group = &errgroup.Group{}
	group := r.group
	group.Go(func() error {
		r.animate(ctx)
		return nil
	})
	r.mu.Unlock()
}

func (r *terminalUpdateReporter) Total(total int64) {
	r.mu.Lock()
	r.total = total
	r.mu.Unlock()
}

func (r *terminalUpdateReporter) Advance(done int64) {
	r.mu.Lock()
	r.done = done
	r.mu.Unlock()
}

// stopAnimation stops the redraw loop and waits for it to exit.
func (r *terminalUpdateReporter) stopAnimation() {
	r.mu.Lock()
	if !r.active {
		r.mu.Unlock()
		return
	}
	cancel := r.cancel
	group := r.group
	r.active = false
	r.cancel = nil
	r.group = nil
	r.mu.Unlock()
	cancel()
	_ = group.Wait()
}

// clearLineLocked erases the active line without ANSI escapes.
func (r *terminalUpdateReporter) clearLineLocked() {
	if r.lastWidth == 0 {
		return
	}
	fmt.Fprintf(r.out, "\r%s\r", strings.Repeat(" ", r.lastWidth))
	r.lastWidth = 0
}

// finish stops animation and prints the final line for the active step.
func (r *terminalUpdateReporter) finish(detail string, failed bool) {
	r.stopAnimation()
	r.mu.Lock()
	defer r.mu.Unlock()
	label, done, total := r.label, r.done, r.total
	r.clearLineLocked()
	r.label = ""
	r.total = 0
	r.done = 0
	if label == "" {
		return
	}
	if failed {
		fmt.Fprintf(r.out, "%s... failed\n", label)
		return
	}
	if detail == "" {
		detail = progressDetail(done, total)
	}
	if detail == "" {
		fmt.Fprintf(r.out, "%s...\n", label)
	} else {
		fmt.Fprintf(r.out, "%s... %s\n", label, detail)
	}
}

// Finish completes the active step line.
func (r *terminalUpdateReporter) Finish(detail string) {
	r.finish(detail, false)
}

// Abort marks the active step line failed.
func (r *terminalUpdateReporter) Abort() {
	r.finish("", true)
}
