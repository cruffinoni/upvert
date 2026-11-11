// Package ecosystem defines the version-source strategy implemented by each
// supported language.
package ecosystem

// Kind identifies a supported project ecosystem.
type Kind string

// Project kinds recognised by detection.
const (
	Go     Kind = "go"
	Node   Kind = "node"
	Python Kind = "python"
	Rust   Kind = "rust"
	Multi  Kind = "multi"
)

// File is one version-bearing file, relative to the project root.
type File struct {
	// Path is the slash-separated path of the file relative to the root.
	Path string
	// InCode reports whether the version is a source-code declaration that
	// --no-version-attr suppresses.
	InCode bool
}

// Target is the set of version declarations owned by one language.
type Target struct {
	// Kind identifies the language that owns the target.
	Kind Kind
	// Single reports whether the target came from single-language detection.
	Single bool
	// Files lists the version-bearing files in report and write order.
	Files []File
}

// Logger is the narrow logging surface used by language adapters.
type Logger interface {
	// Infof writes an informational message.
	Infof(format string, args ...any)
	// Warnf writes a warning message.
	Warnf(format string, args ...any)
}

// Language is the strategy implemented once per supported ecosystem.
type Language interface {
	// Kind returns the ecosystem identifier.
	Kind() Kind
	// Detect reports whether root hosts this ecosystem as a single-language
	// project. files is the pruned project file snapshot.
	Detect(root string, files []string, log Logger) (*Target, bool, error)
	// Scan collects this ecosystem's declarations from the project snapshot
	// for multi-language detection. present reports whether the ecosystem is
	// represented even when it has no editable declaration.
	Scan(root string, files []string, log Logger) (*Target, bool, error)
	// Read returns the version recorded by target's primary file.
	Read(root string, target *Target, log Logger) (string, error)
	// Write stores version in every file of target and reports whether any
	// file changed.
	Write(root string, target *Target, version string, log Logger) (bool, error)
}

// Describer is implemented by languages that report dry-run detail for a file
// before the pipeline reports it as updatable.
type Describer interface {
	// Describe logs the language-specific detail for file.
	Describe(root string, file File, log Logger) error
}

// Extender is implemented by languages that discover additional in-code version
// declarations after an explicit version file is selected.
type Extender interface {
	// Extras returns the auxiliary in-code files that belong to target.
	Extras(root string, files []string, target *Target) ([]File, error)
}
