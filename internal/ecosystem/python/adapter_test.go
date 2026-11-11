package python

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dayrion/upvert/internal/ecosystem"
)

// writeFile creates a file below dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create directories for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// readFile returns the content of path.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestWritePyProjectSinglePreservesFormatting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyproject.toml")
	initial := "# head\n[project]\nname = 'demo'\nversion = '1.0.0' # keep\n\n[tool.poetry.dependencies]\nversion = '2.0.0'\n"
	writeFile(t, dir, "pyproject.toml", initial)

	target := &ecosystem.Target{
		Kind:   ecosystem.Python,
		Single: true,
		Files:  []ecosystem.File{{Path: "pyproject.toml"}},
	}
	changed, err := New().Write(dir, target, "1.1.0", nil)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !changed {
		t.Fatal("expected the project version to change")
	}
	want := "# head\n[project]\nname = 'demo'\nversion = '1.1.0' # keep\n\n[tool.poetry.dependencies]\nversion = '2.0.0'\n"
	if got := readFile(t, path); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestWritePyProjectMultiUpdatesSections(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyproject.toml")
	writeFile(t, dir, "pyproject.toml", "[project]\nversion = \"1.0.0\"\n\n[tool.poetry]\nversion = \"1.0.0\"\n")

	target := &ecosystem.Target{
		Kind:  ecosystem.Python,
		Files: []ecosystem.File{{Path: "pyproject.toml"}},
	}
	if _, err := New().Write(dir, target, "1.1.0", nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := "[project]\nversion = \"1.1.0\"\n\n[tool.poetry]\nversion = \"1.1.0\"\n"
	if got := readFile(t, path); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestWriteSetupCfgVersionHandlesUnquotedValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "setup.cfg")
	writeFile(t, dir, "setup.cfg", "[metadata]\nname = demo\nversion = 1.0.0\n")

	target := &ecosystem.Target{
		Kind:  ecosystem.Python,
		Files: []ecosystem.File{{Path: "setup.cfg"}},
	}
	if _, err := New().Write(dir, target, "1.1.0", nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := "[metadata]\nname = demo\nversion = 1.1.0\n"
	if got := readFile(t, path); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestExplicitPythonModuleReadsAndWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "__init__.py")
	writeFile(t, dir, "__init__.py", "__version__ = '1.0.0'\n")
	target := &ecosystem.Target{
		Kind:   ecosystem.Python,
		Single: true,
		Files:  []ecosystem.File{{Path: "__init__.py", InCode: true}},
	}
	version, err := New().Read(dir, target, nil)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if version != "1.0.0" {
		t.Fatalf("Read = %q, want 1.0.0", version)
	}
	if _, err := New().Write(dir, target, "1.1.0", nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := readFile(t, path); !strings.Contains(got, "__version__ = '1.1.0'") {
		t.Fatalf("module was not rewritten: %q", got)
	}
}

func TestWritePythonModuleVersionKeepsQuoteStyle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "__init__.py")
	writeFile(t, dir, "__init__.py", "__version__ = '1.0.0'\nVERSION = \"1.0.0\"\n")

	target := &ecosystem.Target{
		Kind:  ecosystem.Python,
		Files: []ecosystem.File{{Path: "__init__.py", InCode: true}},
	}
	if _, err := New().Write(dir, target, "1.1.0", nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := "__version__ = '1.1.0'\nVERSION = \"1.1.0\"\n"
	if got := readFile(t, path); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}
