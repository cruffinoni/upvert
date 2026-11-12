// Package golang implements the Go language strategy for version bumping.
package golang

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/cruffinoni/upvert/internal/ecosystem"
	"github.com/cruffinoni/upvert/internal/fileio"
)

// Adapter implements ecosystem.Language for Go projects.
type Adapter struct{}

var _ ecosystem.Language = (*Adapter)(nil)

// New returns a Go language adapter.
func New() *Adapter {
	return &Adapter{}
}

// Kind returns the Go ecosystem identifier.
func (a *Adapter) Kind() ecosystem.Kind {
	return ecosystem.Go
}

// fileSet returns files as a lookup set.
func fileSet(files []string) map[string]struct{} {
	set := make(map[string]struct{}, len(files))
	for _, file := range files {
		set[file] = struct{}{}
	}
	return set
}

// inspectFile parses one candidate Go file and reports whether it declares a
// semantic version. Unparseable files and files without a declaration are
// skipped with a warning.
func inspectFile(file, content string, log ecosystem.Logger) (*declaration, error) {
	if !strings.Contains(content, "Version") {
		return nil, nil
	}
	found, err := parseDeclaration(file, []byte(content))
	if err != nil {
		if isParseError(err) {
			log.Warnf("Skipping unparseable Go file %s: %v", file, err)
			return nil, nil
		}
		if errors.Is(err, errVersionNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return found, nil
}

// detectedTarget builds the single-language target for file.
func detectedTarget(file string) *ecosystem.Target {
	return &ecosystem.Target{
		Kind:   ecosystem.Go,
		Single: true,
		Files:  []ecosystem.File{{Path: file, InCode: true}},
	}
}

// Detect reports whether root hosts a Go project and returns its version file.
func (a *Adapter) Detect(root string, files []string, log ecosystem.Logger) (*ecosystem.Target, bool, error) {
	set := fileSet(files)
	if _, ok := set["go.mod"]; !ok {
		return nil, false, nil
	}
	candidates := []string{
		"internal/project/constants.go",
		"pkg/version/version.go",
		"version/version.go",
		"internal/version/version.go",
		"cmd/version.go",
	}
	checked := make(map[string]bool)
	for _, candidate := range candidates {
		checked[candidate] = true
		if _, ok := set[candidate]; !ok {
			continue
		}
		content, err := fileio.ReadText(fileio.Resolve(root, candidate))
		if err != nil {
			continue
		}
		found, err := inspectFile(candidate, content, log)
		if err != nil {
			if errors.Is(err, errVersionInjected) {
				continue
			}
			return nil, false, err
		}
		if found != nil {
			return detectedTarget(candidate), true, nil
		}
	}
	for _, rel := range files {
		if checked[rel] || !strings.HasSuffix(rel, ".go") {
			continue
		}
		content, err := fileio.ReadText(fileio.Resolve(root, rel))
		if err != nil {
			return nil, false, err
		}
		found, err := inspectFile(rel, content, log)
		if err != nil {
			if errors.Is(err, errVersionInjected) {
				continue
			}
			return nil, false, err
		}
		if found != nil {
			return detectedTarget(rel), true, nil
		}
	}
	return &ecosystem.Target{Kind: ecosystem.Go, Single: true, TagSource: true}, true, nil
}

// Scan collects the Go version declarations from the project snapshot.
func (a *Adapter) Scan(root string, files []string, log ecosystem.Logger) (*ecosystem.Target, bool, error) {
	present := false
	for _, rel := range files {
		if filepath.Base(rel) == "go.mod" {
			present = true
			break
		}
	}
	var target []ecosystem.File
	for _, rel := range files {
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		content, err := fileio.ReadText(fileio.Resolve(root, rel))
		if err != nil {
			return nil, false, err
		}
		found, err := inspectFile(rel, content, log)
		if err != nil {
			if errors.Is(err, errVersionInjected) {
				continue
			}
			return nil, false, err
		}
		if found == nil {
			continue
		}
		present = true
		target = append(target, ecosystem.File{Path: rel, InCode: true})
	}
	return &ecosystem.Target{Kind: ecosystem.Go, Files: target}, present, nil
}

// Read returns the version declared in the target's primary Go file.
func (a *Adapter) Read(root string, target *ecosystem.Target, log ecosystem.Logger) (string, error) {
	if len(target.Files) == 0 {
		return "", errors.New("go project has no version file")
	}
	file := target.Files[0].Path
	content, err := fileio.ReadText(fileio.Resolve(root, file))
	if err != nil {
		return "", err
	}
	found, err := parseDeclaration(file, []byte(content))
	if err != nil {
		if isParseError(err) {
			log.Warnf("Cannot read unparseable Go file %s: %v", file, err)
		}
		return "", err
	}
	return found.Value, nil
}

// Write updates every Go version declaration in target.
func (a *Adapter) Write(root string, target *ecosystem.Target, version string, log ecosystem.Logger) (bool, error) {
	changed := false
	for _, file := range target.Files {
		updated, err := writeVersionFile(fileio.Resolve(root, file.Path), version, log)
		if err != nil {
			return false, err
		}
		changed = changed || updated
	}
	return changed, nil
}
