package golang

import (
	"strconv"
	"strings"

	"github.com/cruffinoni/upvert/internal/ecosystem"
	"github.com/cruffinoni/upvert/internal/fileio"
)

// writeVersionFile updates the Version declaration in a Go source file.
// When Version references another variable, that variable's literal is
// updated instead.
func writeVersionFile(path, version string, log ecosystem.Logger) (bool, error) {
	changed, err := fileio.Update(path, func(data []byte) ([]byte, error) {
		found, err := parseDeclaration(path, data)
		if err != nil {
			return nil, err
		}
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
	return changed, nil
}
