package project

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dayrion/upvert/internal/ecosystem"
)

// captureLogger records warnings for detection assertions.
type captureLogger struct {
	warnings []string
}

// Infof discards informational messages.
func (l *captureLogger) Infof(string, ...any) {}

// Warnf records a warning.
func (l *captureLogger) Warnf(format string, args ...any) {
	l.warnings = append(l.warnings, fmt.Sprintf(format, args...))
}

// writeProjectFile creates a file below dir.
func writeProjectFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create directories for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestDetectProject(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string
		wantKind  ecosystem.Kind
		wantFile  string
		wantError bool
	}{
		{
			name:     "node",
			files:    map[string]string{"package.json": "{\"version\":\"1.0.0\"}"},
			wantKind: ecosystem.Node,
			wantFile: "package.json",
		},
		{
			name:     "rust",
			files:    map[string]string{"Cargo.toml": "[package]\nname = \"demo\"\nversion = \"1.0.0\"\n"},
			wantKind: ecosystem.Rust,
			wantFile: "Cargo.toml",
		},
		{
			name:     "python",
			files:    map[string]string{"pyproject.toml": "[project]\nname = \"demo\"\nversion = \"1.0.0\"\n"},
			wantKind: ecosystem.Python,
			wantFile: "pyproject.toml",
		},
		{
			name:     "setup.cfg",
			files:    map[string]string{"setup.cfg": "[metadata]\nname = demo\nversion = 1.0.0\n"},
			wantKind: ecosystem.Python,
			wantFile: "setup.cfg",
		},
		{
			name:     "go constant",
			files:    map[string]string{"go.mod": "module example.com/demo\n", "version.go": "package demo\n\nconst Version = \"1.2.3\"\n"},
			wantKind: ecosystem.Go,
			wantFile: "version.go",
		},
		{
			name:      "unknown",
			files:     map[string]string{"README.md": "# demo\n"},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				writeProjectFile(t, dir, name, content)
			}
			proj, err := NewDetector(nil).Detect(dir, "")
			if tt.wantError {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			if proj.Kind != tt.wantKind || proj.Primary().Path != tt.wantFile {
				t.Fatalf("detected %s in %s, want %s in %s", proj.Kind, proj.Primary().Path, tt.wantKind, tt.wantFile)
			}
		})
	}
}

func TestDetectExplicitVersionFile(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "custom.json", "{\"version\":\"2.0.0\"}")
	proj, err := NewDetector(nil).Detect(dir, "custom.json")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if proj.Kind != ecosystem.Node || proj.Primary().Path != "custom.json" {
		t.Fatalf("detected %s in %s, want node in custom.json", proj.Kind, proj.Primary().Path)
	}
}

func TestDetectGoWithoutVersionUsesTags(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "go.mod", "module example.com/demo\n")
	writeProjectFile(t, dir, "version.go", "package demo\n\nvar version = \"dev\"\n")

	proj, err := NewDetector(nil).Detect(dir, "")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if proj.Kind != ecosystem.Go || !proj.UsesTags() {
		t.Fatalf("detected %s with UsesTags=%v, want a tag-sourced Go project", proj.Kind, proj.UsesTags())
	}
	if len(proj.Files()) != 0 {
		t.Fatalf("version files = %v, want none", proj.Files())
	}
}

func TestDetectMultiLanguageProject(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "go.mod", "module example.com/demo\n")
	writeProjectFile(t, dir, "version.go", "package demo\n\nconst Version = \"1.2.3\"\n")
	writeProjectFile(t, dir, "package.json", "{\"name\":\"demo\",\"version\":\"1.2.3\"}")
	writeProjectFile(t, dir, "package-lock.json", "{\"name\":\"demo\",\"version\":\"1.2.3\",\"packages\":{\"\":{\"version\":\"1.2.3\"}}}")
	writeProjectFile(t, dir, "pkg/__init__.py", "__version__ = \"1.2.3\"\n")
	writeProjectFile(t, dir, "pyproject.toml", "[project]\nname = \"demo\"\nversion = \"1.2.3\"\n")

	proj, err := NewDetector(nil).Detect(dir, "")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if proj.Kind != ecosystem.Multi {
		t.Fatalf("detected %s, want multi", proj.Kind)
	}
	files := proj.Files()
	want := []string{"package.json", "package-lock.json", "pyproject.toml", "pkg/__init__.py", "version.go"}
	if len(files) != len(want) {
		t.Fatalf("version files = %v, want %v", files, want)
	}
	for i := range want {
		if files[i].Path != want[i] {
			t.Fatalf("version files = %v, want %v", files, want)
		}
	}
}

func TestDetectSkipsGeneratedDirectories(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "go.mod", "module example.com/demo\n")
	writeProjectFile(t, dir, "version.go", "package demo\n\nconst Version = \"1.2.3\"\n")
	writeProjectFile(t, dir, "node_modules/pkg/package.json", "{\"version\":\"9.9.9\"}")

	proj, err := NewDetector(nil).Detect(dir, "")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if proj.Kind != ecosystem.Go {
		t.Fatalf("detected %s, want go: generated directories must be pruned", proj.Kind)
	}
}

func TestDetectPythonModules(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "pyproject.toml", "[project]\nversion = \"1.0.0\"\n")
	writeProjectFile(t, dir, "src/pkg/__init__.py", "__version__ = \"1.0.0\"\n")
	writeProjectFile(t, dir, "node_modules/pkg/index.py", "__version__ = \"9.9.9\"\n")

	proj, err := NewDetector(nil).Detect(dir, "")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	files := proj.Files()
	want := []string{"pyproject.toml", "src/pkg/__init__.py"}
	if len(files) != len(want) {
		t.Fatalf("version files = %v, want %v", files, want)
	}
	for i := range want {
		if files[i].Path != want[i] {
			t.Fatalf("version files = %v, want %v", files, want)
		}
	}
}

func TestListProjectFilesReportsRelativePaths(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "a/b/c.txt", "content")
	files, err := listFiles(dir)
	if err != nil {
		t.Fatalf("listFiles: %v", err)
	}
	want := filepath.ToSlash(filepath.Join("a", "b", "c.txt"))
	if len(files) != 1 || files[0] != want {
		t.Fatalf("files = %v, want [%s]", files, want)
	}
	if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
		t.Fatalf("reported path does not exist: %v", err)
	}
}

func TestDetectExplicitPythonModuleMarksInCode(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "src/pkg/__init__.py", "__version__ = \"1.0.0\"\n")

	proj, err := NewDetector(nil).Detect(dir, "src/pkg/__init__.py")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	files := proj.Files()
	if len(files) != 1 || files[0].Path != "src/pkg/__init__.py" || !files[0].InCode {
		t.Fatalf("files = %v, want one in-code Python module", files)
	}
}
func TestDetectExplicitPythonProjectIncludesModules(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "pyproject.toml", "[project]\nversion = \"1.0.0\"\n")
	writeProjectFile(t, dir, "src/pkg/__init__.py", "__version__ = \"1.0.0\"\n")

	proj, err := NewDetector(nil).Detect(dir, "pyproject.toml")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	files := proj.Files()
	want := []string{"pyproject.toml", "src/pkg/__init__.py"}
	if len(files) != len(want) {
		t.Fatalf("version files = %v, want %v", files, want)
	}
	for i := range want {
		if files[i].Path != want[i] {
			t.Fatalf("version files = %v, want %v", files, want)
		}
	}
}
