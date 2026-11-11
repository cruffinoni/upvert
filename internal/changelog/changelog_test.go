package changelog

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dayrion/upvert/internal/ui"
)

// readFile returns the content of path.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// writeFile creates a file below dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestUpdateCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")
	log := ui.New(io.Discard, false)
	if err := Update(path, "CHANGELOG.md", "1.2.3", false, log); err != nil {
		t.Fatalf("Update: %v", err)
	}
	content := readFile(t, path)
	if got := strings.Count(content, "# Changelog"); got != 1 {
		t.Fatalf("changelog header occurs %d times, want 1:\n%s", got, content)
	}
	wantEntry := "## [1.2.3] - " + time.Now().Format(time.DateOnly)
	if !strings.Contains(content, wantEntry) {
		t.Fatalf("changelog missing %q:\n%s", wantEntry, content)
	}
	if !strings.Contains(content, "- Version bump to 1.2.3") {
		t.Fatalf("changelog missing bump note:\n%s", content)
	}
}

func TestUpdatePrependsEntry(t *testing.T) {
	dir := t.TempDir()
	initial := "# Changelog\n\nAll notable changes to this project will be documented in this file.\n\n## [1.0.0] - 2024-01-01\n\n### Added\n- Initial release\n"
	writeFile(t, dir, "CHANGELOG.md", initial)
	path := filepath.Join(dir, "CHANGELOG.md")
	log := ui.New(io.Discard, false)
	if err := Update(path, "CHANGELOG.md", "1.1.0", false, log); err != nil {
		t.Fatalf("Update: %v", err)
	}
	content := readFile(t, path)
	if strings.Count(content, "# Changelog") != 1 {
		t.Fatalf("changelog header was duplicated:\n%s", content)
	}
	newIndex := strings.Index(content, "## [1.1.0]")
	oldIndex := strings.Index(content, "## [1.0.0]")
	if newIndex < 0 || oldIndex < 0 || newIndex > oldIndex {
		t.Fatalf("entries are out of order:\n%s", content)
	}
	if !strings.Contains(content, "- Initial release") {
		t.Fatalf("existing entry was dropped:\n%s", content)
	}
}

func TestUpdatePreservesMinimalHeader(t *testing.T) {
	dir := t.TempDir()
	initial := "# Changelog\n\n## [1.0.0] - 2024-01-01\n\n- Old release note\n"
	writeFile(t, dir, "CHANGELOG.md", initial)
	path := filepath.Join(dir, "CHANGELOG.md")
	log := ui.New(io.Discard, false)
	if err := Update(path, "CHANGELOG.md", "1.1.0", false, log); err != nil {
		t.Fatalf("Update: %v", err)
	}
	content := readFile(t, path)
	if !strings.Contains(content, "## [1.0.0]") || !strings.Contains(content, "- Old release note") {
		t.Fatalf("existing entry was dropped:\n%s", content)
	}
	if strings.Index(content, "## [1.1.0]") > strings.Index(content, "## [1.0.0]") {
		t.Fatalf("new entry is not before the old one:\n%s", content)
	}
}

func TestUpdatePreservesCustomPreamble(t *testing.T) {
	dir := t.TempDir()
	initial := "# Releases\n\nCustom introduction.\n\n## [1.0.0] - 2024-01-01\n\n- Old note\n"
	writeFile(t, dir, "CHANGELOG.md", initial)
	path := filepath.Join(dir, "CHANGELOG.md")
	log := ui.New(io.Discard, false)
	if err := Update(path, "CHANGELOG.md", "1.1.0", false, log); err != nil {
		t.Fatalf("Update: %v", err)
	}
	content := readFile(t, path)
	for _, want := range []string{"# Releases", "Custom introduction.", "## [1.0.0]", "- Old note", "## [1.1.0]"} {
		if !strings.Contains(content, want) {
			t.Fatalf("changelog missing %q:\n%s", want, content)
		}
	}
}

func TestUpdateAppendsWithoutSections(t *testing.T) {
	dir := t.TempDir()
	initial := "# Changelog\n\nNo sections yet.\n"
	writeFile(t, dir, "CHANGELOG.md", initial)
	path := filepath.Join(dir, "CHANGELOG.md")
	log := ui.New(io.Discard, false)
	if err := Update(path, "CHANGELOG.md", "1.1.0", false, log); err != nil {
		t.Fatalf("Update: %v", err)
	}
	content := readFile(t, path)
	for _, want := range []string{"No sections yet.", "## [1.1.0]"} {
		if !strings.Contains(content, want) {
			t.Fatalf("changelog missing %q:\n%s", want, content)
		}
	}
}

func TestUpdateDryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")
	log := ui.New(io.Discard, false)
	if err := Update(path, "CHANGELOG.md", "1.2.3", true, log); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("dry run created a changelog: %v", err)
	}
}
