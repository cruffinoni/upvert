package report

import (
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/fatih/color"
	"golang.org/x/term"
)

// defaultWidth is used when the output is not a terminal.
const defaultWidth = 100

// minPathWidth keeps details aligned even for a single short path.
const minPathWidth = 24

// ASCII reports whether the environment selects a non-UTF-8 locale.
func ASCII() bool {
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		value := strings.ToUpper(os.Getenv(key))
		if value == "" {
			continue
		}
		if value == "C" || value == "POSIX" {
			return true
		}
		if strings.Contains(value, "UTF-8") || strings.Contains(value, "UTF8") {
			return false
		}
	}
	return false
}

// resolveWidth returns the wrapping width for w.
func resolveWidth(w io.Writer, override int) int {
	if override > 0 {
		return max(override, 40)
	}
	if file, ok := w.(*os.File); ok {
		fd := int(file.Fd())
		if term.IsTerminal(fd) {
			if width, _, err := term.GetSize(fd); err == nil && width > 0 {
				return max(width, 40)
			}
		}
	}
	return defaultWidth
}

// paint wraps s in attr when colour is enabled.
func paint(s string, attr color.Attribute, enabled bool) string {
	if !enabled || s == "" {
		return s
	}
	c := color.New(attr)
	c.EnableColor()
	return c.Sprint(s)
}

// padRight pads s with spaces to width runes.
func padRight(s string, width int) string {
	if n := utf8.RuneCountInString(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}

// wrap splits text into lines of at most limit runes, breaking long words.
func wrap(text string, limit int) []string {
	limit = max(limit, 1)
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	current := ""
	for _, word := range words {
		runes := []rune(word)
		for len(runes) > limit {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
			lines = append(lines, string(runes[:limit]))
			runes = runes[limit:]
		}
		word = string(runes)
		if current == "" {
			current = word
			continue
		}
		if utf8.RuneCountInString(current)+1+utf8.RuneCountInString(word) <= limit {
			current += " " + word
			continue
		}
		lines = append(lines, current)
		current = word
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// textStyle controls one painted text fragment.
type textStyle struct {
	attr    color.Attribute
	enabled bool
}

// writeWrapped writes text at indent, wrapping on word boundaries.
func writeWrapped(b *strings.Builder, text string, indent, width int, style textStyle) {
	for _, line := range wrap(text, max(width-indent, 10)) {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(paint(line, style.attr, style.enabled))
		b.WriteString("\n")
	}
}

// writeChanges renders one row per file, aligned on the longest path.
func writeChanges(b *strings.Builder, changes []Change, width int, opts Options) {
	maxPath := 0
	for _, change := range changes {
		if n := utf8.RuneCountInString(change.Path); n > maxPath {
			maxPath = n
		}
	}
	pathWidth := max(maxPath, minPathWidth)
	if limit := max(width-4-2-20, minPathWidth); pathWidth > limit {
		pathWidth = limit
	}
	for _, change := range changes {
		if change.Detail == "" {
			b.WriteString("    " + change.Path + "\n")
			continue
		}
		if utf8.RuneCountInString(change.Path) > pathWidth {
			b.WriteString("    " + change.Path + "\n")
			writeWrapped(b, change.Detail, 6, width, textStyle{color.Faint, opts.Color})
			continue
		}
		pad := pathWidth - utf8.RuneCountInString(change.Path)
		prefixWidth := 4 + pathWidth + 2
		lines := wrap(change.Detail, max(width-prefixWidth, 10))
		b.WriteString("    " + change.Path + strings.Repeat(" ", pad) + "  ")
		b.WriteString(paint(lines[0], color.Faint, opts.Color))
		b.WriteString("\n")
		for _, line := range lines[1:] {
			b.WriteString(strings.Repeat(" ", prefixWidth))
			b.WriteString(paint(line, color.Faint, opts.Color))
			b.WriteString("\n")
		}
	}
}

// writeSteps renders a label/value block with a shared label column.
func writeSteps(b *strings.Builder, steps []Step, indent, width int, opts Options) {
	if len(steps) == 0 {
		return
	}
	labelWidth := 0
	for _, step := range steps {
		if n := utf8.RuneCountInString(step.Label); n > labelWidth {
			labelWidth = n
		}
	}
	labelWidth += 2
	for _, step := range steps {
		valueWidth := max(width-indent-labelWidth-1, 10)
		lines := wrap(step.Value, valueWidth)
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(paint(padRight(step.Label, labelWidth), color.Faint, opts.Color))
		b.WriteString(" ")
		b.WriteString(lines[0])
		b.WriteString("\n")
		for _, line := range lines[1:] {
			b.WriteString(strings.Repeat(" ", indent+labelWidth+1))
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
}

// writeSection starts a report section with a blank line before its title.
func writeSection(b *strings.Builder, title string, opts Options) {
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(paint(title, color.Bold, opts.Color))
	b.WriteString("\n")
}

// Render writes r to w using the global, ecosystem-agnostic layout.
func Render(w io.Writer, r Report, opts Options) error {
	width := resolveWidth(w, opts.Width)
	arrow := "→"
	if opts.ASCII {
		arrow = "->"
	}

	var b strings.Builder
	b.WriteString(paint("upvert "+r.Command+" ("+r.Mode+")", color.Bold, opts.Color))
	b.WriteString("\n")

	var meta []Step
	if r.From != "" || r.To != "" {
		meta = append(meta, Step{Label: "Version", Value: r.From + " " + arrow + " " + r.To})
	}
	if r.Source != "" {
		meta = append(meta, Step{Label: "Source", Value: r.Source})
	}
	writeSteps(&b, meta, 2, width, opts)

	writeSection(&b, "Changes", opts)
	if len(r.Changes) == 0 {
		b.WriteString("    (none)\n")
	} else {
		writeChanges(&b, r.Changes, width, opts)
	}
	if len(r.Hooks) > 0 {
		writeSection(&b, "Hooks", opts)
		writeSteps(&b, r.Hooks, 4, width, opts)
	}
	if len(r.Release) > 0 {
		writeSection(&b, "Release", opts)
		writeSteps(&b, r.Release, 4, width, opts)
	}
	if len(r.Notes) > 0 {
		writeSection(&b, "Notes", opts)
		for _, note := range r.Notes {
			writeWrapped(&b, note, 4, width, textStyle{color.FgYellow, opts.Color})
		}
	}
	if r.Footer != "" {
		b.WriteString("\n")
		attr := color.FgGreen
		if r.Mode == "preview" {
			attr = color.FgYellow
		}
		for _, line := range strings.Split(r.Footer, "\n") {
			if line == "" {
				b.WriteString("\n")
				continue
			}
			writeWrapped(&b, line, 2, width, textStyle{attr, opts.Color})
		}
	}

	_, err := io.WriteString(w, b.String())
	return err
}
