package golang

import (
	"strconv"
	"strings"

	"github.com/cruffinoni/upvert/internal/ecosystem"
	"github.com/cruffinoni/upvert/internal/fileio"
)

// writeVersionFile updates the Version declaration in a Go source file.
// When Version references another variable, that variable's literal is
// updated instead and the caller is told about the indirection.
func writeVersionFile(path, version string, log ecosystem.Logger) (bool, error) {
	var resolved *declaration
	changed, err := fileio.Update(path, func(data []byte) ([]byte, error) {
		found, err := parseDeclaration(path, data)
		if err != nil {
			return nil, err
		}
		resolved = found
		if found.Value == version {
			return data, nil
		}
		replacement := []byte(strconv.Quote(version))
		if strings.HasPrefix(found.Raw, "`") {
			replacement = []byte("`" + version + "`")
		}
		return fileio.ReplaceSpan(data, found.Start, found.End, replacement), nil
	})
	if err != nil {
		if isParseError(err) && log != nil {
			log.Warnf("Cannot update unparseable Go file %s: %v", path, err)
		}
		return false, err
	}
	if changed && resolved != nil && resolved.Indirect && log != nil {
		log.Infof("Version is defined by %s (line %d); updated that declaration", resolved.TargetName, resolved.Line)
	}
	return changed, nil
}
