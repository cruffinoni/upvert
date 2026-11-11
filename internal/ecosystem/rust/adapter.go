// Package rust implements the Rust language strategy for version bumping.
package rust

import (
	"fmt"
	"path/filepath"

	"github.com/cruffinoni/upvert/internal/ecosystem"
	"github.com/cruffinoni/upvert/internal/fileio"
	"github.com/cruffinoni/upvert/internal/toml"
)

var (
	// Version locations in Cargo.toml.
	cargoSections = [][]string{{"package"}, {"workspace", "package"}}

	_ ecosystem.Language = (*Adapter)(nil)
)

// Adapter implements ecosystem.Language for Rust projects.
type Adapter struct{}

// New returns a Rust language adapter.
func New() *Adapter {
	return &Adapter{}
}

// Kind returns the Rust ecosystem identifier.
func (a *Adapter) Kind() ecosystem.Kind {
	return ecosystem.Rust
}

// fileSet returns files as a lookup set.
func fileSet(files []string) map[string]struct{} {
	set := make(map[string]struct{}, len(files))
	for _, file := range files {
		set[file] = struct{}{}
	}
	return set
}

// hasPackage reports whether Cargo.toml declares a package or workspace
// package table.
func hasPackage(content string) bool {
	document, err := toml.Decode(content)
	if err != nil {
		return false
	}
	return toml.TableExists(document, "package") ||
		toml.TableExists(document, "workspace", "package")
}

// cargoVersion returns the version declared by Cargo.toml.
func cargoVersion(content string) string {
	document, err := toml.Decode(content)
	if err != nil {
		return ""
	}
	if version := toml.String(document, "package", "version"); version != "" {
		return version
	}
	return toml.String(document, "workspace", "package", "version")
}

// Detect reports whether root hosts a Rust project.
func (a *Adapter) Detect(root string, files []string, log ecosystem.Logger) (*ecosystem.Target, bool, error) {
	if _, ok := fileSet(files)["Cargo.toml"]; !ok {
		return nil, false, nil
	}
	content, err := fileio.ReadText(fileio.Resolve(root, "Cargo.toml"))
	if err != nil {
		return nil, false, nil
	}
	if !hasPackage(content) {
		return nil, false, nil
	}
	return &ecosystem.Target{
		Kind:   ecosystem.Rust,
		Single: true,
		Files:  []ecosystem.File{{Path: "Cargo.toml"}},
	}, true, nil
}

// Scan collects the Cargo manifests from the project snapshot.
func (a *Adapter) Scan(root string, files []string, log ecosystem.Logger) (*ecosystem.Target, bool, error) {
	var cargoFiles []string
	for _, rel := range files {
		if filepath.Base(rel) != "Cargo.toml" {
			continue
		}
		content, err := fileio.ReadText(fileio.Resolve(root, rel))
		if err != nil {
			return nil, false, err
		}
		if hasPackage(content) {
			cargoFiles = append(cargoFiles, rel)
		}
	}
	target := &ecosystem.Target{Kind: ecosystem.Rust}
	for _, rel := range cargoFiles {
		target.Files = append(target.Files, ecosystem.File{Path: rel})
	}
	return target, len(cargoFiles) > 0, nil
}

// Read returns the version declared by the target's Cargo manifest.
func (a *Adapter) Read(root string, target *ecosystem.Target, log ecosystem.Logger) (string, error) {
	if len(target.Files) == 0 {
		return "", fmt.Errorf("rust project has no manifest")
	}
	file := target.Files[0].Path
	content, err := fileio.ReadText(fileio.Resolve(root, file))
	if err != nil {
		return "", err
	}
	value := cargoVersion(content)
	if value == "" {
		return "", fmt.Errorf("could not extract version from %s", file)
	}
	return value, nil
}

// Write updates every Cargo manifest in target.
func (a *Adapter) Write(root string, target *ecosystem.Target, version string, log ecosystem.Logger) (bool, error) {
	changed := false
	for _, file := range target.Files {
		updated, err := toml.Write(fileio.Resolve(root, file.Path), cargoSections, version)
		if err != nil {
			return false, err
		}
		changed = changed || updated
	}
	return changed, nil
}
