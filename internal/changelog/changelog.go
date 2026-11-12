// Package changelog renders and prepends release entries in a changelog file.
package changelog

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/cruffinoni/upvert/internal/fileio"
)

// header starts every changelog file created by upvert.
const header = "# Changelog\n\nAll notable changes to this project will be documented in this file.\n\n"

// firstSection returns the byte offset of the first level-2 heading, or -1.
func firstSection(content string) int {
	offset := 0
	for _, line := range strings.SplitAfter(content, "\n") {
		if strings.HasPrefix(line, "## ") {
			return offset
		}
		offset += len(line)
	}
	return -1
}

// insertEntry places entry before the first level-2 section while preserving
// every existing line. Files without a section heading get the entry appended.
func insertEntry(existing, entry string) string {
	if strings.TrimSpace(existing) == "" {
		return header + entry
	}
	if index := firstSection(existing); index >= 0 {
		prefix := strings.TrimRight(existing[:index], "\n")
		if prefix != "" {
			prefix += "\n\n"
		}
		return prefix + entry + existing[index:]
	}
	return strings.TrimRight(existing, "\n") + "\n\n" + entry
}

// Update prepends a release entry to the changelog at path, creating the file
// with a standard header when it does not exist yet. It does not write during a
// dry run.
func Update(path, name, version string, dryRun bool) error {
	if dryRun {
		return nil
	}
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read %s: %w", name, err)
	}
	entry := fmt.Sprintf("## [%s] - %s\n\n### Added\n- Version bump to %s\n\n",
		version, time.Now().Format(time.DateOnly), version)
	content := insertEntry(string(existing), entry)
	return fileio.WriteAtomic(path, []byte(content), mode)
}
