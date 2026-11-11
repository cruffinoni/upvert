package project

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/dayrion/upvert/internal/ecosystem"
	"github.com/dayrion/upvert/internal/ecosystem/golang"
	"github.com/dayrion/upvert/internal/ecosystem/node"
	"github.com/dayrion/upvert/internal/ecosystem/python"
	"github.com/dayrion/upvert/internal/ecosystem/rust"
)

var (
	// singleOrder is the priority used for single-language detection.
	singleOrder = []ecosystem.Kind{ecosystem.Go, ecosystem.Rust, ecosystem.Python, ecosystem.Node}

	// multiOrder is the report and stage order used for multi-language projects.
	multiOrder = []ecosystem.Kind{ecosystem.Node, ecosystem.Python, ecosystem.Rust, ecosystem.Go}

	// skipDirs holds directory names that never carry release metadata.
	skipDirs = map[string]struct{}{
		".git":          {},
		".claude":       {},
		".next":         {},
		".tox":          {},
		".venv":         {},
		"build":         {},
		"dist":          {},
		"env":           {},
		"node_modules":  {},
		"site-packages": {},
		"vendor":        {},
		"venv":          {},
	}
)

// skipDir reports whether name is a directory that never carries release
// metadata.
func skipDir(name string) bool {
	if _, ok := skipDirs[name]; ok {
		return true
	}
	return strings.HasSuffix(name, ".egg-info")
}

// listFiles returns every file below root that can carry release metadata, as
// slash-separated paths relative to root.
func listFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && skipDir(entry.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list project files: %w", err)
	}
	return files, nil
}

// dedupeLogger suppresses repeated warning messages while delegating every
// message to the underlying logger.
type dedupeLogger struct {
	base ecosystem.Logger
	seen map[string]bool
}

// Infof forwards an informational message.
func (l *dedupeLogger) Infof(format string, args ...any) {
	if l.base != nil {
		l.base.Infof(format, args...)
	}
}

// Warnf forwards a warning message unless it was already emitted.
func (l *dedupeLogger) Warnf(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	if l.seen[message] {
		return
	}
	l.seen[message] = true
	if l.base != nil {
		l.base.Warnf(format, args...)
	}
}

// Detector applies an ordered language registry to detect a project layout.
type Detector struct {
	log       ecosystem.Logger
	languages map[ecosystem.Kind]ecosystem.Language
}

// NewDetector returns a Detector over langs. With no languages it uses the
// default Go, Node.js, Python, and Rust adapters.
func NewDetector(log ecosystem.Logger, langs ...ecosystem.Language) *Detector {
	if len(langs) == 0 {
		langs = []ecosystem.Language{golang.New(), node.New(), python.New(), rust.New()}
	}
	byKind := make(map[ecosystem.Kind]ecosystem.Language, len(langs))
	for _, lang := range langs {
		byKind[lang.Kind()] = lang
	}
	return &Detector{log: log, languages: byKind}
}

// explicitKind maps an explicit version file to its ecosystem and reports
// whether that file embeds the version in source code.
func explicitKind(file string) (ecosystem.Kind, bool, bool) {
	base := filepath.Base(file)
	switch {
	case strings.HasSuffix(file, ".json"):
		return ecosystem.Node, false, true
	case base == "Cargo.toml":
		return ecosystem.Rust, false, true
	case strings.HasSuffix(file, ".go"):
		return ecosystem.Go, true, true
	case base == "setup.py":
		return ecosystem.Python, false, true
	case strings.HasSuffix(file, ".toml"), strings.HasSuffix(file, ".cfg"):
		return ecosystem.Python, false, true
	case strings.HasSuffix(file, ".py"):
		return ecosystem.Python, true, true
	default:
		return "", false, false
	}
}

// detectExplicit builds a project from an explicit version file.
func (d *Detector) detectExplicit(root, file string, log ecosystem.Logger) (*Layout, error) {
	kind, inCode, ok := explicitKind(file)
	if !ok {
		files, err := listFiles(root)
		if err != nil {
			return nil, err
		}
		rustLang, exists := d.languages[ecosystem.Rust]
		if !exists {
			return nil, fmt.Errorf("could not determine project type for version file %q", file)
		}
		if _, present, err := rustLang.Detect(root, files, log); err != nil {
			return nil, err
		} else if present {
			kind = ecosystem.Rust
		}
	}
	if kind == "" {
		return nil, fmt.Errorf("could not determine project type for version file %q", file)
	}
	lang, ok := d.languages[kind]
	if !ok {
		return nil, fmt.Errorf("could not determine project type for version file %q", file)
	}
	target := ecosystem.Target{
		Kind:   kind,
		Single: true,
		Files:  []ecosystem.File{{Path: file, InCode: inCode}},
	}
	if extender, ok := lang.(ecosystem.Extender); ok {
		files, err := listFiles(root)
		if err != nil {
			return nil, err
		}
		extra, err := extender.Extras(root, files, &target)
		if err != nil {
			return nil, err
		}
		target.Files = append(target.Files, extra...)
	}
	return &Layout{Kind: kind, Parts: []Part{{Target: target, Language: lang}}}, nil
}

// detectMulti reports whether root mixes several ecosystems and, if so,
// collects every contributing language.
func (d *Detector) detectMulti(root string, files []string, log ecosystem.Logger) (*Layout, bool, error) {
	present := make(map[ecosystem.Kind]bool, len(multiOrder))
	parts := make([]Part, 0, len(multiOrder))
	fileCount := 0
	for _, kind := range multiOrder {
		lang, ok := d.languages[kind]
		if !ok {
			continue
		}
		target, found, err := lang.Scan(root, files, log)
		if err != nil {
			return nil, false, err
		}
		if found {
			present[kind] = true
		}
		if target == nil || len(target.Files) == 0 {
			continue
		}
		fileCount += len(target.Files)
		parts = append(parts, Part{Target: *target, Language: lang})
	}
	ecosystems := 0
	for _, found := range present {
		if found {
			ecosystems++
		}
	}
	if ecosystems < 2 {
		return nil, false, nil
	}
	if fileCount == 0 {
		return nil, false, errors.New("multi-language project detected but no editable version declarations were found")
	}
	return &Layout{Kind: ecosystem.Multi, Parts: parts}, true, nil
}

// Detect identifies the project layout below root. An explicit version file
// bypasses auto-detection.
func (d *Detector) Detect(root, explicitFile string) (*Layout, error) {
	log := &dedupeLogger{base: d.log, seen: make(map[string]bool)}
	if explicitFile != "" {
		return d.detectExplicit(root, explicitFile, log)
	}
	files, err := listFiles(root)
	if err != nil {
		return nil, err
	}
	if multi, ok, err := d.detectMulti(root, files, log); err != nil {
		return nil, err
	} else if ok {
		return multi, nil
	}
	for _, kind := range singleOrder {
		lang, ok := d.languages[kind]
		if !ok {
			continue
		}
		target, found, err := lang.Detect(root, files, log)
		if err != nil {
			return nil, err
		}
		if found {
			return &Layout{Kind: kind, Parts: []Part{{Target: *target, Language: lang}}}, nil
		}
	}
	return nil, errors.New("could not auto-detect project type; use --version-file to specify the version file")
}
