// Package ui renders levelled, optionally coloured messages for the upvert
// commands.
package ui

import (
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

// Logger writes levelled messages to a single stream.
type Logger struct {
	w        io.Writer
	info     *color.Color
	success  *color.Color
	warning  *color.Color
	err      *color.Color
	progress Progress
}

// newColor returns a colour that is explicitly enabled or disabled, so logging
// to a buffer never depends on the terminal attached to stdout.
func newColor(attr color.Attribute, enabled bool) *color.Color {
	c := color.New(attr)
	if enabled {
		c.EnableColor()
	} else {
		c.DisableColor()
	}
	return c
}

// New returns a Logger that writes to w and uses ANSI colours when colorEnabled
// is true.
func New(w io.Writer, colorEnabled bool) *Logger {
	return &Logger{
		w:       w,
		info:    newColor(color.FgBlue, colorEnabled),
		success: newColor(color.FgGreen, colorEnabled),
		warning: newColor(color.FgYellow, colorEnabled),
		err:     newColor(color.FgRed, colorEnabled),
	}
}

// SetProgress makes the logger clear an active progress line before writing
// and restore it afterwards. A nil progress disables the coordination.
func (l *Logger) SetProgress(progress Progress) {
	l.progress = progress
}

// write renders one prefixed line without corrupting an active progress line.
func (l *Logger) write(c *color.Color, prefix, format string, args []any) {
	progress := l.progress
	if progress != nil {
		progress.Pause()
	}
	_, _ = fmt.Fprint(l.w, c.Sprint(prefix), " ")
	_, _ = fmt.Fprintln(l.w, fmt.Sprintf(format, args...))
	if progress != nil {
		progress.Resume()
	}
}

// Infof formats its arguments according to format and writes an informational
// message.
func (l *Logger) Infof(format string, args ...any) {
	l.write(l.info, "[INFO]   ", format, args)
}

// Successf formats its arguments according to format and writes a success
// message.
func (l *Logger) Successf(format string, args ...any) {
	l.write(l.success, "[SUCCESS]", format, args)
}

// Warnf formats its arguments according to format and writes a warning
// message.
func (l *Logger) Warnf(format string, args ...any) {
	l.write(l.warning, "[WARNING]", format, args)
}

// Errorf formats its arguments according to format and writes an error
// message.
func (l *Logger) Errorf(format string, args ...any) {
	l.write(l.err, "[ERROR]  ", format, args)
}

// ColorEnabled reports whether w should receive ANSI colours. Colours are
// disabled when NO_COLOR is set, when TERM is "dumb", or when w is not a
// terminal.
func ColorEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fd := f.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}
