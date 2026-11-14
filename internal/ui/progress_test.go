package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		value int64
		want  string
	}{
		{value: -1, want: "0 B"},
		{value: 0, want: "0 B"},
		{value: 1023, want: "1023 B"},
		{value: 1024, want: "1.0 KiB"},
		{value: 1536, want: "1.5 KiB"},
		{value: 1 << 20, want: "1.0 MiB"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, formatBytes(tt.value))
	}
}

func TestProgressBarFill(t *testing.T) {
	tests := []struct {
		name   string
		done   int64
		total  int64
		width  int
		want   int
		wantOK bool
	}{
		{name: "empty", done: 0, total: 10, width: 20, want: 0, wantOK: true},
		{name: "rounds up", done: 1, total: 3, width: 20, want: 7, wantOK: true},
		{name: "rounds down", done: 1, total: 6, width: 20, want: 3, wantOK: true},
		{name: "half", done: 5, total: 10, width: 20, want: 10, wantOK: true},
		{name: "nearly full", done: 9, total: 10, width: 20, want: 18, wantOK: true},
		{name: "full", done: 10, total: 10, width: 20, want: 20, wantOK: true},
		{name: "clamps above total", done: 20, total: 10, width: 20, want: 20, wantOK: true},
		{name: "clamps negative", done: -1, total: 10, width: 20, want: 0, wantOK: true},
		{name: "zero total", done: 1, total: 0, width: 20, wantOK: false},
		{name: "zero width", done: 1, total: 10, width: 0, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := progressBarFill(tt.done, tt.total, tt.width)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProgressBarRendering(t *testing.T) {
	// One rendering assertion pins the bracket and fill style. The arithmetic
	// is covered by TestProgressBarFill.
	assert.Equal(t, "[==========          ]", progressBar(5, 10))
}

func TestLineProgressTransitions(t *testing.T) {
	var out bytes.Buffer
	reporter := &lineProgress{out: &out}

	reporter.Start("step")
	assert.Equal(t, "step", reporter.label)
	assert.Zero(t, reporter.total)
	assert.Zero(t, reporter.done)
	assert.Contains(t, out.String(), "step")

	reporter.Finish("ok")
	assert.Contains(t, out.String(), "step")
	assert.Contains(t, out.String(), "ok")
	assert.True(t, strings.HasSuffix(out.String(), "\n"))
	assert.Empty(t, reporter.label)
	assert.Zero(t, reporter.total)
	assert.Zero(t, reporter.done)

	out.Reset()
	reporter.Finish("again")
	assert.Empty(t, out.String(), "finishing twice must not write")
}

func TestLineProgressUsesProgressDetail(t *testing.T) {
	var out bytes.Buffer
	reporter := &lineProgress{out: &out}

	reporter.Start("download")
	reporter.Total(2048)
	reporter.Advance(1024)
	reporter.Finish("")

	assert.Contains(t, out.String(), "download")
	assert.True(t, strings.HasSuffix(out.String(), "1.0 KiB\n"))
	assert.Empty(t, reporter.label)
}

func TestLineProgressAbort(t *testing.T) {
	var out bytes.Buffer
	reporter := &lineProgress{out: &out}

	reporter.Start("install")
	reporter.Abort()

	assert.Contains(t, out.String(), "install")
	assert.True(t, strings.HasSuffix(out.String(), "failed\n"))
	assert.Empty(t, reporter.label)
	assert.Zero(t, reporter.total)
	assert.Zero(t, reporter.done)
}

func TestLineProgressIgnoresFinishWithoutStart(t *testing.T) {
	var out bytes.Buffer
	reporter := &lineProgress{out: &out}

	reporter.Finish("ok")
	reporter.Abort()

	assert.Empty(t, out.String())
}

func TestLineProgressPauseReprintsLabel(t *testing.T) {
	var out bytes.Buffer
	reporter := &lineProgress{out: &out}

	reporter.Start("writing")
	reporter.Pause()
	reporter.Finish("package.json")

	assert.Equal(t, "writing...\nwriting... package.json\n", out.String())
}

func TestQuietProgress(t *testing.T) {
	var out bytes.Buffer
	reporter := quietProgress{}

	reporter.Start("Resolving latest stable release")
	reporter.Total(10)
	reporter.Advance(5)
	reporter.Pause()
	reporter.Resume()
	reporter.Finish("ok")
	reporter.Abort()
	assert.Empty(t, out.String())
}

func TestNewProgressSelectsLineAndQuiet(t *testing.T) {
	_, ok := NewProgress(&bytes.Buffer{}, ProgressOptions{}).(*lineProgress)
	assert.True(t, ok)

	_, ok = NewProgress(&bytes.Buffer{}, ProgressOptions{Quiet: true}).(quietProgress)
	assert.True(t, ok)

	_, ok = NopProgress().(quietProgress)
	assert.True(t, ok)
}

func TestTerminalProgressWritesWithinWidth(t *testing.T) {
	var out bytes.Buffer
	reporter := &terminalProgress{out: &out, width: 10}

	reporter.writeLineLocked("abcdefghijklmnop")
	assert.Equal(t, "\rabcdefghi", out.String())
	assert.Equal(t, 9, reporter.lastWidth)

	out.Reset()
	reporter.writeLineLocked("abc")
	assert.Equal(t, "\rabc      ", out.String())
	assert.Equal(t, 3, reporter.lastWidth)

	out.Reset()
	reporter.clearLineLocked()
	assert.Equal(t, "\r   \r", out.String())
	assert.Zero(t, reporter.lastWidth)
}

func TestTerminalProgressFinish(t *testing.T) {
	var out bytes.Buffer
	reporter := &terminalProgress{out: &out, width: 80, label: "verify", done: 10, total: 10}

	reporter.finish("ok", false)
	assert.Contains(t, out.String(), "verify")
	assert.Contains(t, out.String(), "ok")
	assert.Empty(t, reporter.label)

	out.Reset()
	reporter.label = "install"
	reporter.finish("", true)
	assert.Contains(t, out.String(), "install")
	assert.Contains(t, out.String(), "failed")
	assert.Empty(t, reporter.label)
}

func TestTerminalProgressTransientFinishClearsStep(t *testing.T) {
	var out bytes.Buffer
	reporter := &terminalProgress{out: &out, width: 80, transient: true, label: "push", active: true}

	reporter.writeLineLocked("| push...")
	out.Reset()
	reporter.finish("origin", false)

	assert.NotContains(t, out.String(), "push")
	assert.NotContains(t, out.String(), "origin")
	assert.Contains(t, out.String(), "\r")
}

func TestTerminalProgressTransientFailureStillPrints(t *testing.T) {
	var out bytes.Buffer
	reporter := &terminalProgress{out: &out, width: 80, transient: true, label: "tag", active: true}

	reporter.finish("", true)

	assert.Contains(t, out.String(), "tag... failed")
}

func TestTerminalProgressPauseResume(t *testing.T) {
	var out bytes.Buffer
	reporter := &terminalProgress{out: &out, width: 80, label: "push", active: true}

	reporter.Pause()
	assert.True(t, reporter.paused)
	assert.True(t, reporter.active)

	reporter.Resume()
	assert.False(t, reporter.paused)

	reporter.Finish("done")
	assert.Empty(t, reporter.label)
}

func TestLoggerPausesProgress(t *testing.T) {
	var out bytes.Buffer
	progress := NewProgress(&out, ProgressOptions{})
	logger := New(&out, false)
	logger.SetProgress(progress)

	progress.Start("writing")
	logger.Warnf("careful")
	progress.Finish("package.json")

	assert.Equal(t, "writing...\n[WARNING] careful\nwriting... package.json\n", out.String())
}

func TestTerminalProgressTransientFailurePrintsCompletedSteps(t *testing.T) {
	var out bytes.Buffer
	reporter := &terminalProgress{out: &out, width: 80, transient: true}

	reporter.Start("writing")
	reporter.Finish("package.json")
	reporter.Start("tagging")
	reporter.Abort()

	assert.Contains(t, out.String(), "writing... package.json")
	assert.Contains(t, out.String(), "tagging... failed")
}

func TestTerminalProgressPaintsFirstFrameImmediately(t *testing.T) {
	var out bytes.Buffer
	reporter := &terminalProgress{out: &out, width: 80}

	reporter.Start("committing")
	reporter.Close()

	assert.Contains(t, out.String(), "committing...")
}

func TestTerminalProgressShowsPositionAndElapsed(t *testing.T) {
	var out bytes.Buffer
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	reporter := &terminalProgress{out: &out, width: 80, clock: func() time.Time { return now }}

	reporter.Steps(4)
	reporter.Start("writing version files")
	now = now.Add(3 * time.Second)
	reporter.Start("committing")
	now = now.Add(14 * time.Second)
	reporter.mu.Lock()
	reporter.renderLocked()
	reporter.mu.Unlock()
	reporter.Close()

	rendered := out.String()
	assert.Contains(t, rendered, "(1/4)")
	assert.Contains(t, rendered, "(2/4)")
	assert.Contains(t, rendered, "14s")
}

func TestTerminalProgressDownloadsHaveNoStepCounter(t *testing.T) {
	var out bytes.Buffer
	reporter := &terminalProgress{out: &out, width: 100}

	reporter.Start("download")
	reporter.Total(2048)
	reporter.Advance(1024)
	reporter.mu.Lock()
	reporter.renderLocked()
	reporter.mu.Unlock()
	reporter.Close()

	rendered := out.String()
	assert.Contains(t, rendered, "50%")
	assert.NotContains(t, rendered, "0s")
	assert.NotContains(t, rendered, "(1/")
}

func TestTerminalProgressCloseClearsActiveLine(t *testing.T) {
	var out bytes.Buffer
	reporter := &terminalProgress{out: &out, width: 80}

	reporter.Start("tagging")
	out.Reset()
	reporter.Close()

	assert.Contains(t, out.String(), "\r")
	assert.NotContains(t, out.String(), "tagging")
	assert.False(t, reporter.active)
}
