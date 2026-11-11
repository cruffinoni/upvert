package python

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dayrion/upvert/internal/ecosystem"
)

// writeFile creates a file below dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	require.NoErrorf(t, os.MkdirAll(filepath.Dir(path), 0o755), "create directories for %s", name)
	require.NoErrorf(t, os.WriteFile(path, []byte(content), 0o644), "write %s", name)
}

// readFile returns the content of path.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoErrorf(t, err, "read %s", path)
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
	require.NoError(t, err)
	assert.True(t, changed)
	want := "# head\n[project]\nname = 'demo'\nversion = '1.1.0' # keep\n\n[tool.poetry.dependencies]\nversion = '2.0.0'\n"
	assert.Equal(t, want, readFile(t, path))
}

func TestWritePyProjectMultiUpdatesSections(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyproject.toml")
	writeFile(t, dir, "pyproject.toml", "[project]\nversion = \"1.0.0\"\n\n[tool.poetry]\nversion = \"1.0.0\"\n")

	target := &ecosystem.Target{
		Kind:  ecosystem.Python,
		Files: []ecosystem.File{{Path: "pyproject.toml"}},
	}
	_, err := New().Write(dir, target, "1.1.0", nil)
	require.NoError(t, err)
	want := "[project]\nversion = \"1.1.0\"\n\n[tool.poetry]\nversion = \"1.1.0\"\n"
	assert.Equal(t, want, readFile(t, path))
}

func TestWriteSetupCfgVersionHandlesUnquotedValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "setup.cfg")
	writeFile(t, dir, "setup.cfg", "[metadata]\nname = demo\nversion = 1.0.0\n")

	target := &ecosystem.Target{
		Kind:  ecosystem.Python,
		Files: []ecosystem.File{{Path: "setup.cfg"}},
	}
	_, err := New().Write(dir, target, "1.1.0", nil)
	require.NoError(t, err)
	want := "[metadata]\nname = demo\nversion = 1.1.0\n"
	assert.Equal(t, want, readFile(t, path))
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
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", version)
	_, err = New().Write(dir, target, "1.1.0", nil)
	require.NoError(t, err)
	assert.Contains(t, readFile(t, path), "__version__ = '1.1.0'")
}

func TestWritePythonModuleVersionKeepsQuoteStyle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "__init__.py")
	writeFile(t, dir, "__init__.py", "__version__ = '1.0.0'\nVERSION = \"1.0.0\"\n")

	target := &ecosystem.Target{
		Kind:  ecosystem.Python,
		Files: []ecosystem.File{{Path: "__init__.py", InCode: true}},
	}
	_, err := New().Write(dir, target, "1.1.0", nil)
	require.NoError(t, err)
	want := "__version__ = '1.1.0'\nVERSION = \"1.1.0\"\n"
	assert.Equal(t, want, readFile(t, path))
}
