// Package python implements the Python language strategy for version bumping.
package python

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cruffinoni/upvert/internal/ecosystem"
	"github.com/cruffinoni/upvert/internal/fileio"
	"github.com/cruffinoni/upvert/internal/semver"
	"github.com/cruffinoni/upvert/internal/toml"
)

var (
	versionPattern     = regexp.MustCompile(`version\s*=\s*["']([^"']*)["']`)
	moduleVersion      = regexp.MustCompile(`(?m)^\s*(?:__version__|VERSION)\s*=\s*["']([^"']*)["']`)
	versionDeclaration = regexp.MustCompile(`(?m)^[ \t]*(?:__version__|VERSION)[ \t]*=[ \t]*["']`)
	moduleDoubleQuoted = regexp.MustCompile(`^(\s*__version__\s*=\s*)"[^"]*"`)
	moduleSingleQuoted = regexp.MustCompile(`^(\s*__version__\s*=\s*)'[^']*'`)
	constDoubleQuoted  = regexp.MustCompile(`^(\s*VERSION\s*=\s*)"[^"]*"`)
	constSingleQuoted  = regexp.MustCompile(`^(\s*VERSION\s*=\s*)'[^']*'`)
	callVersionPattern = regexp.MustCompile(`(version\s*=\s*)["'][^"']*["']`)

	// Version locations in Python manifests. The empty section targets a top-level
	// version key.
	pyProjectSections       = [][]string{{"project"}, {"tool", "poetry"}}
	pyProjectSingleSections = [][]string{{"project"}, {"tool", "poetry"}, nil}

	_ ecosystem.Language = (*Adapter)(nil)
	_ ecosystem.Extender = (*Adapter)(nil)
)

// Adapter implements ecosystem.Language for Python projects.
type Adapter struct{}

// New returns a Python language adapter.
func New() *Adapter {
	return &Adapter{}
}

// Kind returns the Python ecosystem identifier.
func (a *Adapter) Kind() ecosystem.Kind {
	return ecosystem.Python
}

// fileSet returns files as a lookup set.
func fileSet(files []string) map[string]struct{} {
	set := make(map[string]struct{}, len(files))
	for _, file := range files {
		set[file] = struct{}{}
	}
	return set
}

// firstCapture returns the first submatch of re in content, or an empty string.
func firstCapture(content string, re *regexp.Regexp) string {
	match := re.FindStringSubmatch(content)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

// findModules returns the Python sources that declare an in-code version
// attribute, excluding the primary packaging file.
func findModules(root string, files []string, exclude string) ([]ecosystem.File, error) {
	var modules []ecosystem.File
	for _, rel := range files {
		if !strings.HasSuffix(rel, ".py") || filepath.ToSlash(exclude) == rel {
			continue
		}
		content, err := fileio.ReadText(fileio.Resolve(root, rel))
		if err != nil {
			return nil, err
		}
		if versionDeclaration.MatchString(content) {
			modules = append(modules, ecosystem.File{Path: rel, InCode: true})
		}
	}
	return modules, nil
}

// readVersionFromModule returns the first version attribute declared in a
// Python module.
func readVersionFromModule(path string) (string, error) {
	content, err := fileio.ReadText(path)
	if err != nil {
		return "", err
	}
	value := firstCapture(content, moduleVersion)
	if value == "" {
		return "", fmt.Errorf("could not extract version from %s", path)
	}
	return value, nil
}

// writeSetupCfg rewrites the version key in [metadata] and, for single-language
// projects, falls back to any quoted version assignment.
func writeSetupCfg(path, version string, fallback bool) (bool, error) {
	return fileio.UpdateText(path, func(content string) string {
		updated := replaceINIVersion(content, []string{"[metadata]"}, version)
		if fallback && updated == content {
			updated = replaceVersionAssignment(content, version)
		}
		return updated
	})
}

// writeSetupPy rewrites the version keyword argument in setup.py.
func writeSetupPy(path, version string) (bool, error) {
	return fileio.UpdateText(path, func(content string) string {
		return fileio.ReplaceLines(content, callVersionPattern, "$1\""+version+"\"")
	})
}

// writeModuleVersion updates __version__ and VERSION declarations while
// preserving the quote style of each declaration.
func writeModuleVersion(path, version string) (bool, error) {
	return fileio.UpdateText(path, func(content string) string {
		content = fileio.ReplaceLines(content, moduleDoubleQuoted, "$1\""+version+"\"")
		content = fileio.ReplaceLines(content, moduleSingleQuoted, "$1'"+version+"'")
		content = fileio.ReplaceLines(content, constDoubleQuoted, "$1\""+version+"\"")
		return fileio.ReplaceLines(content, constSingleQuoted, "$1'"+version+"'")
	})
}

// writeVersionFile updates one Python version file. packagingSingle selects the
// single-language manifest semantics for the primary file.
func writeVersionFile(root, rel, version string, packagingSingle bool) (bool, error) {
	path := fileio.Resolve(root, rel)
	base := strings.ToLower(filepath.Base(rel))
	if packagingSingle {
		switch base {
		case "pyproject.toml":
			return toml.Write(path, pyProjectSingleSections, version)
		case "setup.cfg":
			return writeSetupCfg(path, version, true)
		case "setup.py":
			return writeSetupPy(path, version)
		default:
			if strings.HasSuffix(base, ".py") {
				return writeModuleVersion(path, version)
			}
			return false, fmt.Errorf("unsupported python version file %s", rel)
		}
	}
	switch base {
	case "pyproject.toml":
		return toml.Write(path, pyProjectSections, version)
	case "setup.cfg":
		return writeSetupCfg(path, version, false)
	case "setup.py":
		return writeSetupPy(path, version)
	default:
		if strings.HasSuffix(base, ".py") {
			return writeModuleVersion(path, version)
		}
		return false, fmt.Errorf("unsupported python version file %s", rel)
	}
}

// Extras returns the in-code Python modules that carry a version alongside the
// target's primary packaging file.
func (a *Adapter) Extras(root string, files []string, target *ecosystem.Target) ([]ecosystem.File, error) {
	if len(target.Files) == 0 {
		return nil, nil
	}
	primary := target.Files[0].Path
	if strings.ToLower(filepath.Ext(primary)) == ".py" {
		return nil, nil
	}
	return findModules(root, files, primary)
}

// Detect reports whether root hosts a Python project and returns its version
// files, including in-code modules discovered next to the packaging metadata.
func (a *Adapter) Detect(root string, files []string, log ecosystem.Logger) (*ecosystem.Target, bool, error) {
	set := fileSet(files)
	primary := ""
	for _, candidate := range []string{"pyproject.toml", "setup.cfg", "setup.py"} {
		if _, ok := set[candidate]; ok {
			primary = candidate
			break
		}
	}
	if primary == "" {
		return nil, false, nil
	}
	target := &ecosystem.Target{
		Kind:   ecosystem.Python,
		Single: true,
		Files:  []ecosystem.File{{Path: primary}},
	}
	modules, err := a.Extras(root, files, target)
	if err != nil {
		return nil, false, err
	}
	target.Files = append(target.Files, modules...)
	return target, true, nil
}

// Scan collects the Python packaging files and in-code modules from the project
// snapshot.
func (a *Adapter) Scan(root string, files []string, log ecosystem.Logger) (*ecosystem.Target, bool, error) {
	var pyProjects, setupCfgs, setupPys, modules []string
	present := false
	for _, rel := range files {
		switch filepath.Base(rel) {
		case "pyproject.toml":
			content, err := fileio.ReadText(fileio.Resolve(root, rel))
			if err != nil {
				return nil, false, err
			}
			if pyProjectVersion(content) != "" {
				present = true
				pyProjects = append(pyProjects, rel)
			}
		case "setup.cfg":
			content, err := fileio.ReadText(fileio.Resolve(root, rel))
			if err != nil {
				return nil, false, err
			}
			if setupCfgVersion(content) != "" {
				present = true
				setupCfgs = append(setupCfgs, rel)
			}
		case "setup.py":
			content, err := fileio.ReadText(fileio.Resolve(root, rel))
			if err != nil {
				return nil, false, err
			}
			if callVersionPattern.MatchString(content) {
				present = true
				setupPys = append(setupPys, rel)
			}
		}
	}
	for _, rel := range files {
		if !strings.HasSuffix(rel, ".py") {
			continue
		}
		content, err := fileio.ReadText(fileio.Resolve(root, rel))
		if err != nil {
			return nil, false, err
		}
		if versionDeclaration.MatchString(content) {
			modules = append(modules, rel)
		}
	}
	target := &ecosystem.Target{Kind: ecosystem.Python}
	for _, rel := range pyProjects {
		target.Files = append(target.Files, ecosystem.File{Path: rel})
	}
	for _, rel := range setupCfgs {
		target.Files = append(target.Files, ecosystem.File{Path: rel})
	}
	for _, rel := range setupPys {
		target.Files = append(target.Files, ecosystem.File{Path: rel})
	}
	for _, rel := range modules {
		target.Files = append(target.Files, ecosystem.File{Path: rel, InCode: true})
	}
	return target, present, nil
}

// Read returns the version declared by the target's packaging metadata,
// falling back to the first in-code module when the metadata has no static
// version.
func (a *Adapter) Read(root string, target *ecosystem.Target, log ecosystem.Logger) (string, error) {
	if len(target.Files) == 0 {
		return "", errors.New("python project has no version file")
	}
	file := target.Files[0].Path
	content, err := fileio.ReadText(fileio.Resolve(root, file))
	if err != nil {
		return "", err
	}
	var value string
	switch strings.ToLower(filepath.Ext(file)) {
	case ".toml":
		value = pyProjectVersion(content)
	case ".cfg":
		value = setupCfgVersion(content)
	case ".py":
		if strings.EqualFold(filepath.Base(file), "setup.py") {
			value = firstCapture(content, versionPattern)
		} else {
			value, err = readVersionFromModule(fileio.Resolve(root, file))
			if err != nil {
				return "", err
			}
		}
	}
	if _, err := semver.Parse(value); err != nil && len(target.Files) > 1 {
		value, err = readVersionFromModule(fileio.Resolve(root, target.Files[1].Path))
		if err != nil {
			return "", err
		}
	}
	if value == "" {
		return "", fmt.Errorf("could not extract version from %s", file)
	}
	return value, nil
}

// Write updates every Python version declaration in target.
func (a *Adapter) Write(root string, target *ecosystem.Target, version string, log ecosystem.Logger) (bool, error) {
	changed := false
	for i, file := range target.Files {
		updated, err := writeVersionFile(root, file.Path, version, target.Single && i == 0)
		if err != nil {
			return false, err
		}
		changed = changed || updated
	}
	return changed, nil
}
