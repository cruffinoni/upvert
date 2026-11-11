package project

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cruffinoni/upvert/internal/ecosystem"
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
	require.NoErrorf(t, os.MkdirAll(filepath.Dir(path), 0o755), "create directories for %s", name)
	require.NoErrorf(t, os.WriteFile(path, []byte(content), 0o644), "write %s", name)
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
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantKind, proj.Kind)
			assert.Equal(t, tt.wantFile, proj.Primary().Path)
		})
	}
}

func TestDetectExplicitVersionFile(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "custom.json", "{\"version\":\"2.0.0\"}")
	proj, err := NewDetector(nil).Detect(dir, "custom.json")
	require.NoError(t, err)
	assert.Equal(t, ecosystem.Node, proj.Kind)
	assert.Equal(t, "custom.json", proj.Primary().Path)
}

func TestDetectGoWithoutVersionUsesTags(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "go.mod", "module example.com/demo\n")
	writeProjectFile(t, dir, "version.go", "package demo\n\nvar version = \"dev\"\n")

	proj, err := NewDetector(nil).Detect(dir, "")
	require.NoError(t, err)
	assert.Equal(t, ecosystem.Go, proj.Kind)
	assert.True(t, proj.UsesTags())
	assert.Empty(t, proj.Files())
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
	require.NoError(t, err)
	assert.Equal(t, ecosystem.Multi, proj.Kind)
	files := proj.Files()
	want := []string{"package.json", "package-lock.json", "pyproject.toml", "pkg/__init__.py", "version.go"}
	require.Len(t, files, len(want))
	for i := range want {
		assert.Equal(t, want[i], files[i].Path)
	}
}

func TestDetectSkipsGeneratedDirectories(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "go.mod", "module example.com/demo\n")
	writeProjectFile(t, dir, "version.go", "package demo\n\nconst Version = \"1.2.3\"\n")
	writeProjectFile(t, dir, "node_modules/pkg/package.json", "{\"version\":\"9.9.9\"}")

	proj, err := NewDetector(nil).Detect(dir, "")
	require.NoError(t, err)
	assert.Equal(t, ecosystem.Go, proj.Kind)
}

func TestDetectPythonModules(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "pyproject.toml", "[project]\nversion = \"1.0.0\"\n")
	writeProjectFile(t, dir, "src/pkg/__init__.py", "__version__ = \"1.0.0\"\n")
	writeProjectFile(t, dir, "node_modules/pkg/index.py", "__version__ = \"9.9.9\"\n")

	proj, err := NewDetector(nil).Detect(dir, "")
	require.NoError(t, err)
	files := proj.Files()
	want := []string{"pyproject.toml", "src/pkg/__init__.py"}
	require.Len(t, files, len(want))
	for i := range want {
		assert.Equal(t, want[i], files[i].Path)
	}
}

func TestListProjectFilesReportsRelativePaths(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "a/b/c.txt", "content")
	files, err := listFiles(dir)
	require.NoError(t, err)
	want := filepath.ToSlash(filepath.Join("a", "b", "c.txt"))
	require.Len(t, files, 1)
	assert.Equal(t, want, files[0])
	_, err = os.Stat(filepath.Join(dir, want))
	assert.NoError(t, err)
}

func TestDetectExplicitPythonModuleMarksInCode(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "src/pkg/__init__.py", "__version__ = \"1.0.0\"\n")

	proj, err := NewDetector(nil).Detect(dir, "src/pkg/__init__.py")
	require.NoError(t, err)
	files := proj.Files()
	require.Len(t, files, 1)
	assert.Equal(t, "src/pkg/__init__.py", files[0].Path)
	assert.True(t, files[0].InCode)
}

func TestDetectExplicitPythonProjectIncludesModules(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "pyproject.toml", "[project]\nversion = \"1.0.0\"\n")
	writeProjectFile(t, dir, "src/pkg/__init__.py", "__version__ = \"1.0.0\"\n")

	proj, err := NewDetector(nil).Detect(dir, "pyproject.toml")
	require.NoError(t, err)
	files := proj.Files()
	want := []string{"pyproject.toml", "src/pkg/__init__.py"}
	require.Len(t, files, len(want))
	for i := range want {
		assert.Equal(t, want[i], files[i].Path)
	}
}
