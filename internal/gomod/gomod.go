// Package gomod inspects go.mod so a bump can warn about major-version module
// paths.
package gomod

import (
	"fmt"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/cruffinoni/upvert/internal/fileio"
	"github.com/cruffinoni/upvert/internal/semver"
	"github.com/cruffinoni/upvert/internal/ui"
)

// ModulePath returns the module path declared in the go.mod file at path.
func ModulePath(path string) (string, error) {
	data, err := fileio.ReadText(path)
	if err != nil {
		return "", err
	}
	file, err := modfile.ParseLax(path, []byte(data), nil)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	if file.Module == nil {
		return "", nil
	}
	return file.Module.Mod.Path, nil
}

// ValidateModulePath warns when a major bump needs a /vN module suffix.
func ValidateModulePath(root, next string, log *ui.Logger) error {
	parsed, err := semver.Parse(next)
	if err != nil {
		return nil
	}
	if parsed.Major() < 2 {
		return nil
	}
	module, err := ModulePath(fileio.Resolve(root, "go.mod"))
	if err != nil || module == "" {
		return nil
	}
	suffix := fmt.Sprintf("/v%d", parsed.Major())
	if !strings.HasSuffix(module, suffix) {
		log.Warnf("Major version %d requires a %s suffix in go.mod module path", parsed.Major(), suffix)
		log.Warnf("Consider: %s%s", module, suffix)
	}
	return nil
}
