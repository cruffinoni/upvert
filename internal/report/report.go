// Package report renders the language-agnostic result of a bump or rollback.
package report

// Change is one file-level effect of a bump.
type Change struct {
	// Path is the file, or a synthetic row such as "work tree".
	Path string
	// Detail is a neutral description such as "entry for 1.3.0".
	Detail string
}

// Step is one labelled action in the report.
type Step struct {
	Label string
	Value string
}

// Report is the global view shared by previews and real runs.
type Report struct {
	Command string
	Mode    string
	From    string
	To      string
	Source  string
	Changes []Change
	Hooks   []Step
	Release []Step
	Notes   []string
	Footer  string
}

// Options controls rendering.
type Options struct {
	// Color enables ANSI colours.
	Color bool
	// ASCII replaces Unicode glyphs such as the version arrow.
	ASCII bool
	// Width is the wrapping width; zero detects the terminal or uses 100.
	Width int
}
