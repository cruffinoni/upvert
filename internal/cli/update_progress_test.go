package cli

import (
	"bytes"
	"strings"
	"testing"

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

func TestLineUpdateReporterTransitions(t *testing.T) {
	var out bytes.Buffer
	reporter := &lineUpdateReporter{out: &out}

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

func TestLineUpdateReporterUsesProgressDetail(t *testing.T) {
	var out bytes.Buffer
	reporter := &lineUpdateReporter{out: &out}

	reporter.Start("download")
	reporter.Total(2048)
	reporter.Advance(1024)
	reporter.Finish("")

	assert.Contains(t, out.String(), "download")
	assert.True(t, strings.HasSuffix(out.String(), "1.0 KiB\n"))
	assert.Empty(t, reporter.label)
}

func TestLineUpdateReporterAbort(t *testing.T) {
	var out bytes.Buffer
	reporter := &lineUpdateReporter{out: &out}

	reporter.Start("install")
	reporter.Abort()

	assert.Contains(t, out.String(), "install")
	assert.True(t, strings.HasSuffix(out.String(), "failed\n"))
	assert.Empty(t, reporter.label)
	assert.Zero(t, reporter.total)
	assert.Zero(t, reporter.done)
}

func TestLineUpdateReporterIgnoresFinishWithoutStart(t *testing.T) {
	var out bytes.Buffer
	reporter := &lineUpdateReporter{out: &out}

	reporter.Finish("ok")
	reporter.Abort()

	assert.Empty(t, out.String())
}

func TestQuietUpdateReporter(t *testing.T) {
	var out bytes.Buffer
	reporter := quietUpdateReporter{}

	reporter.Start("Resolving latest stable release")
	reporter.Total(10)
	reporter.Advance(5)
	reporter.Finish("ok")
	reporter.Abort()
	assert.Empty(t, out.String())
}

func TestTerminalUpdateReporterWritesWithinWidth(t *testing.T) {
	var out bytes.Buffer
	reporter := &terminalUpdateReporter{out: &out, width: 10}

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

func TestTerminalUpdateReporterFinish(t *testing.T) {
	var out bytes.Buffer
	reporter := &terminalUpdateReporter{out: &out, width: 80, label: "verify", done: 10, total: 10}

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

func TestNewUpdateReporter(t *testing.T) {
	_, ok := newUpdateReporter(&bytes.Buffer{}, false).(*lineUpdateReporter)
	assert.True(t, ok)

	_, ok = newUpdateReporter(&bytes.Buffer{}, true).(quietUpdateReporter)
	assert.True(t, ok)
}
