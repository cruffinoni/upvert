package project

import (
	"fmt"
	"os"
	"os/exec"
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

// gitProject runs git in dir and fails the test on error.
func gitProject(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "git %v\n%s", args, output)
}

// initGitProject creates a committed repository below a temporary directory.
func initGitProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		writeProjectFile(t, dir, name, content)
	}
	gitProject(t, dir, "init", "-q", "-b", "main")
	gitProject(t, dir, "add", "-A")
	gitProject(t, dir, "-c", "user.name=Upvert Test", "-c", "user.email=upvert@example.com", "commit", "-q", "-m", "initial commit")
	return dir
}

func TestDetectIgnoresGitIgnoredManifests(t *testing.T) {
	dir := initGitProject(t, map[string]string{
		"go.mod":     "module example.com/demo\n",
		"version.go": "package demo\n\nconst Version = \"1.2.3\"\n",
		".gitignore": ".tmp/\n",
	})
	writeProjectFile(t, dir, ".tmp/smoke/package.json", "{\"name\":\"demo\",\"version\":\"0.0.2\"}")

	proj, err := NewDetector(nil).Detect(dir, "")
	require.NoError(t, err)
	assert.Equal(t, ecosystem.Go, proj.Kind)
	assert.False(t, proj.IsMulti())
	for _, file := range proj.Files() {
		assert.NotContains(t, file.Path, ".tmp")
	}
}

func TestDetectKeepsTrackedNestedManifests(t *testing.T) {
	dir := initGitProject(t, map[string]string{
		"go.mod":                    "module example.com/demo\n",
		"version.go":                "package demo\n\nconst Version = \"1.2.3\"\n",
		"packages/web/package.json": "{\"name\":\"web\",\"version\":\"1.2.3\"}",
	})

	proj, err := NewDetector(nil).Detect(dir, "")
	require.NoError(t, err)
	assert.True(t, proj.IsMulti())
	assert.Contains(t, proj.Files(), ecosystem.File{Path: "packages/web/package.json"})
	assert.Contains(t, proj.Files(), ecosystem.File{Path: "version.go", InCode: true})
}

func TestDetectIncludesUntrackedManifests(t *testing.T) {
	dir := initGitProject(t, map[string]string{
		"go.mod":     "module example.com/demo\n",
		"version.go": "package demo\n\nconst Version = \"1.2.3\"\n",
	})
	writeProjectFile(t, dir, "packages/web/package.json", "{\"name\":\"web\",\"version\":\"1.2.3\"}")

	proj, err := NewDetector(nil).Detect(dir, "")
	require.NoError(t, err)
	assert.True(t, proj.IsMulti())
	assert.Contains(t, proj.Files(), ecosystem.File{Path: "packages/web/package.json"})
}

func TestDetectSkipsHiddenDirectoriesOutsideGit(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "go.mod", "module example.com/demo\n")
	writeProjectFile(t, dir, "version.go", "package demo\n\nconst Version = \"1.2.3\"\n")
	writeProjectFile(t, dir, ".tmp/smoke/package.json", "{\"version\":\"0.0.2\"}")
	writeProjectFile(t, dir, ".cache/work/package.json", "{\"version\":\"0.0.2\"}")

	proj, err := NewDetector(nil).Detect(dir, "")
	require.NoError(t, err)
	assert.Equal(t, ecosystem.Go, proj.Kind)
	assert.False(t, proj.IsMulti())
}
