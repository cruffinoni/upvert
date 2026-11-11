package update

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// samePath compares two cleaned path entries using platform semantics.
func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// appendPath appends dir to a PATH-style list when it is not empty.
func appendPath(pathEnv, dir string) string {
	pathEnv = strings.TrimRight(pathEnv, string(os.PathListSeparator))
	if pathEnv == "" {
		return dir
	}
	return pathEnv + string(os.PathListSeparator) + dir
}

// pathContains reports whether dir is one of the entries in pathEnv.
func pathContains(pathEnv, dir string) bool {
	if dir == "" {
		return false
	}
	dir = filepath.Clean(dir)
	for _, entry := range filepath.SplitList(pathEnv) {
		if entry == "" {
			continue
		}
		if samePath(filepath.Clean(entry), dir) {
			return true
		}
	}
	return false
}
