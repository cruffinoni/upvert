package python

import (
	"regexp"
	"slices"
	"strings"

	"github.com/cruffinoni/upvert/internal/fileio"
)

var (
	versionLine       = regexp.MustCompile(`^(\s*version\s*=\s*)(.*)$`)
	versionDoubleLine = regexp.MustCompile(`^(\s*version\s*=\s*)"[^"]*"`)
	versionSingleLine = regexp.MustCompile(`^(\s*version\s*=\s*)'[^']*'`)
)

// replaceINIVersion rewrites the version key inside the named INI sections.
func replaceINIVersion(content string, sections []string, version string) string {
	lines := strings.Split(content, "\n")
	current := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			if idx := strings.Index(trimmed, "]"); idx >= 0 {
				current = trimmed[:idx+1]
			}
			continue
		}
		if !slices.Contains(sections, current) {
			continue
		}
		match := versionLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		lines[i] = match[1] + fileio.QuoteLike(strings.TrimSpace(match[2]), version)
	}
	return strings.Join(lines, "\n")
}

// replaceVersionAssignment rewrites quoted version assignments while keeping
// the original quote style.
func replaceVersionAssignment(content, version string) string {
	content = fileio.ReplaceLines(content, versionDoubleLine, "$1\""+version+"\"")
	return fileio.ReplaceLines(content, versionSingleLine, "$1'"+version+"'")
}
