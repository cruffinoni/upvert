package changelog

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cruffinoni/upvert/internal/ui"
)

// readFile returns the content of path.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoErrorf(t, err, "read %s", path)
	return string(data)
}

// writeFile creates a file below dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoErrorf(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644), "write %s", name)
}

func TestUpdateCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")
	log := ui.New(io.Discard, false)
	require.NoError(t, Update(path, "CHANGELOG.md", "1.2.3", false, log))
	content := readFile(t, path)
	assert.Equal(t, 1, strings.Count(content, "# Changelog"))
	wantEntry := "## [1.2.3] - " + time.Now().Format(time.DateOnly)
	assert.Contains(t, content, wantEntry)
	assert.Contains(t, content, "- Version bump to 1.2.3")
}

func TestUpdatePrependsEntry(t *testing.T) {
	dir := t.TempDir()
	initial := "# Changelog\n\nAll notable changes to this project will be documented in this file.\n\n## [1.0.0] - 2024-01-01\n\n### Added\n- Initial release\n"
	writeFile(t, dir, "CHANGELOG.md", initial)
	path := filepath.Join(dir, "CHANGELOG.md")
	log := ui.New(io.Discard, false)
	require.NoError(t, Update(path, "CHANGELOG.md", "1.1.0", false, log))
	content := readFile(t, path)
	assert.Equal(t, 1, strings.Count(content, "# Changelog"))
	newIndex := strings.Index(content, "## [1.1.0]")
	oldIndex := strings.Index(content, "## [1.0.0]")
	assert.GreaterOrEqual(t, newIndex, 0)
	assert.GreaterOrEqual(t, oldIndex, 0)
	assert.Less(t, newIndex, oldIndex)
	assert.Contains(t, content, "- Initial release")
}

func TestUpdatePreservesMinimalHeader(t *testing.T) {
	dir := t.TempDir()
	initial := "# Changelog\n\n## [1.0.0] - 2024-01-01\n\n- Old release note\n"
	writeFile(t, dir, "CHANGELOG.md", initial)
	path := filepath.Join(dir, "CHANGELOG.md")
	log := ui.New(io.Discard, false)
	require.NoError(t, Update(path, "CHANGELOG.md", "1.1.0", false, log))
	content := readFile(t, path)
	assert.Contains(t, content, "## [1.0.0]")
	assert.Contains(t, content, "- Old release note")
	assert.Less(t, strings.Index(content, "## [1.1.0]"), strings.Index(content, "## [1.0.0]"))
}

func TestUpdatePreservesCustomPreamble(t *testing.T) {
	dir := t.TempDir()
	initial := "# Releases\n\nCustom introduction.\n\n## [1.0.0] - 2024-01-01\n\n- Old note\n"
	writeFile(t, dir, "CHANGELOG.md", initial)
	path := filepath.Join(dir, "CHANGELOG.md")
	log := ui.New(io.Discard, false)
	require.NoError(t, Update(path, "CHANGELOG.md", "1.1.0", false, log))
	content := readFile(t, path)
	for _, want := range []string{"# Releases", "Custom introduction.", "## [1.0.0]", "- Old note", "## [1.1.0]"} {
		assert.Contains(t, content, want)
	}
}

func TestUpdateAppendsWithoutSections(t *testing.T) {
	dir := t.TempDir()
	initial := "# Changelog\n\nNo sections yet.\n"
	writeFile(t, dir, "CHANGELOG.md", initial)
	path := filepath.Join(dir, "CHANGELOG.md")
	log := ui.New(io.Discard, false)
	require.NoError(t, Update(path, "CHANGELOG.md", "1.1.0", false, log))
	content := readFile(t, path)
	for _, want := range []string{"No sections yet.", "## [1.1.0]"} {
		assert.Contains(t, content, want)
	}
}

func TestUpdateDryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")
	log := ui.New(io.Discard, false)
	require.NoError(t, Update(path, "CHANGELOG.md", "1.2.3", true, log))
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}
