// Package node implements the Node.js language strategy for version bumping.
package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dayrion/upvert/internal/ecosystem"
	"github.com/dayrion/upvert/internal/fileio"
)

// Adapter implements ecosystem.Language for Node.js projects.
type Adapter struct{}

var _ ecosystem.Language = (*Adapter)(nil)

// New returns a Node.js language adapter.
func New() *Adapter {
	return &Adapter{}
}

// Kind returns the Node.js ecosystem identifier.
func (a *Adapter) Kind() ecosystem.Kind {
	return ecosystem.Node
}

// fileSet returns files as a lookup set.
func fileSet(files []string) map[string]struct{} {
	set := make(map[string]struct{}, len(files))
	for _, file := range files {
		set[file] = struct{}{}
	}
	return set
}

// writeLockfile updates both version locations used by package-lock.json.
func writeLockfile(path, version string) (bool, error) {
	changed, err := updateJSONField(path, []string{"version"}, version, true)
	if err != nil {
		return false, err
	}
	nested, err := updateJSONField(path, []string{"packages", "", "version"}, version, true)
	if err != nil {
		return false, err
	}
	return changed || nested, nil
}

// Detect reports whether root hosts a Node.js project.
func (a *Adapter) Detect(root string, files []string, log ecosystem.Logger) (*ecosystem.Target, bool, error) {
	if _, ok := fileSet(files)["package.json"]; !ok {
		return nil, false, nil
	}
	return &ecosystem.Target{
		Kind:   ecosystem.Node,
		Single: true,
		Files:  []ecosystem.File{{Path: "package.json"}},
	}, true, nil
}

// Scan collects the Node.js manifests and lockfiles from the project snapshot.
func (a *Adapter) Scan(root string, files []string, log ecosystem.Logger) (*ecosystem.Target, bool, error) {
	set := fileSet(files)
	var manifests, lockfiles []string
	for _, rel := range files {
		if filepath.Base(rel) != "package.json" {
			continue
		}
		manifests = append(manifests, rel)
		lock := filepath.ToSlash(filepath.Join(filepath.Dir(rel), "package-lock.json"))
		if _, ok := set[lock]; ok {
			lockfiles = append(lockfiles, lock)
		}
	}
	target := &ecosystem.Target{Kind: ecosystem.Node}
	for _, rel := range manifests {
		target.Files = append(target.Files, ecosystem.File{Path: rel})
	}
	for _, rel := range lockfiles {
		target.Files = append(target.Files, ecosystem.File{Path: rel})
	}
	return target, len(manifests) > 0, nil
}

// Read returns the top-level version field of the target's manifest.
func (a *Adapter) Read(root string, target *ecosystem.Target, log ecosystem.Logger) (string, error) {
	if len(target.Files) == 0 {
		return "", errors.New("node project has no manifest")
	}
	file := target.Files[0].Path
	data, err := fileio.ReadText(fileio.Resolve(root, file))
	if err != nil {
		return "", err
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(data), &manifest); err != nil {
		return "", fmt.Errorf("parse %s: %w", file, err)
	}
	if manifest.Version == "" {
		return "", fmt.Errorf("could not extract version from %s", file)
	}
	return manifest.Version, nil
}

// Write updates every Node.js manifest and lockfile in target.
func (a *Adapter) Write(root string, target *ecosystem.Target, version string, log ecosystem.Logger) (bool, error) {
	changed := false
	for _, file := range target.Files {
		path := fileio.Resolve(root, file.Path)
		var (
			updated bool
			err     error
		)
		if strings.EqualFold(filepath.Base(file.Path), "package-lock.json") {
			updated, err = writeLockfile(path, version)
		} else {
			updated, err = updateJSONField(path, []string{"version"}, version, false)
		}
		if err != nil {
			return false, err
		}
		changed = changed || updated
	}
	return changed, nil
}
