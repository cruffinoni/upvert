// Package fileio provides filesystem queries and content-preserving file
// updates shared by the version writers.
package fileio

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/renameio/v2"
)

// Exists reports whether path exists and is not a directory.
func Exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// ReadText returns the contents of the UTF-8 text file at path.
func ReadText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(data), nil
}

// Resolve returns the absolute path for rel below root. An absolute rel is
// returned unchanged.
func Resolve(root, rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(root, filepath.FromSlash(rel))
}

// WriteAtomic replaces path with data while preserving its permissions.
func WriteAtomic(path string, data []byte, mode fs.FileMode) error {
	if err := renameio.WriteFile(path, data, mode); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

// Update rewrites a file when transform changes its content and reports
// whether the content changed.
func Update(path string, transform func([]byte) ([]byte, error)) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	updated, err := transform(data)
	if err != nil {
		return false, fmt.Errorf("update %s: %w", path, err)
	}
	if bytes.Equal(updated, data) {
		return false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
	if err := WriteAtomic(path, updated, info.Mode().Perm()); err != nil {
		return false, err
	}
	return true, nil
}

// UpdateText rewrites a text file when transform changes its content and
// reports whether the content changed.
func UpdateText(path string, transform func(string) string) (bool, error) {
	return Update(path, func(data []byte) ([]byte, error) {
		return []byte(transform(string(data))), nil
	})
}

// ReplaceSpan substitutes data[start:end] with replacement.
func ReplaceSpan(data []byte, start, end int, replacement []byte) []byte {
	out := make([]byte, 0, len(data)-(end-start)+len(replacement))
	out = append(out, data[:start]...)
	out = append(out, replacement...)
	out = append(out, data[end:]...)
	return out
}

// ReplaceLines applies repl to every matching line of content.
func ReplaceLines(content string, re *regexp.Regexp, repl string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if re.MatchString(line) {
			lines[i] = re.ReplaceAllString(line, repl)
		}
	}
	return strings.Join(lines, "\n")
}

// QuoteLike returns value wrapped in the quote style of previous.
func QuoteLike(previous, value string) string {
	switch {
	case strings.HasPrefix(previous, "\""):
		return "\"" + value + "\""
	case strings.HasPrefix(previous, "'"):
		return "'" + value + "'"
	default:
		return value
	}
}
