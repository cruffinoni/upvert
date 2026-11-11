package golang

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cruffinoni/upvert/internal/ui"
)

// testLogger records warnings for detection assertions.
type testLogger struct {
	warnings []string
}

// Infof discards informational messages.
func (l *testLogger) Infof(string, ...any) {}

// Warnf records a warning.
func (l *testLogger) Warnf(format string, args ...any) {
	l.warnings = append(l.warnings, fmt.Sprintf(format, args...))
}

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

func TestWriteGoVersionFileIndirect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "version.go")
	writeFile(t, dir, "version.go", "package demo\n\nvar buildVersion = \"1.0.0\"\nconst Version = buildVersion\n")

	var output bytes.Buffer
	changed, err := writeVersionFile(path, "1.1.0", ui.New(&output, false))
	require.NoError(t, err)
	assert.True(t, changed)
	got := readFile(t, path)
	assert.Contains(t, got, "var buildVersion = \"1.1.0\"")
	assert.Contains(t, got, "const Version = buildVersion")
	assert.Contains(t, output.String(), "buildVersion")
}

func TestWriteGoVersionFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "version.go")
	writeFile(t, dir, "version.go", "package demo\n\nconst Version = \"1.0.0\"\n")

	_, err := writeVersionFile(path, "1.1.0", nil)
	require.NoError(t, err)
	want := "package demo\n\nconst Version = \"1.1.0\"\n"
	assert.Equal(t, want, readFile(t, path))
}

func TestDetectGoProjectSkipsUnparseableFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/demo\n")
	writeFile(t, dir, "bad.go", "package demo\n\nconst Version = \"1.0.0\"\n\nfunc broken( {\n")
	writeFile(t, dir, "version.go", "package demo\n\nconst Version = \"1.0.0\"\n")

	log := &testLogger{}
	adapter := New()
	target, found, err := adapter.Detect(dir, []string{"go.mod", "bad.go", "version.go"}, log)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, target.Files, 1)
	assert.Equal(t, "version.go", target.Files[0].Path)
	require.NotEmpty(t, log.warnings)
	assert.Contains(t, log.warnings[0], "bad.go")
}

func TestDetectGoProjectWithoutVersionUsesTags(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/demo\n")
	writeFile(t, dir, "version.go", "package demo\n\nvar version = \"dev\"\n")

	log := &testLogger{}
	target, found, err := New().Detect(dir, []string{"go.mod", "version.go"}, log)
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, target.TagSource)
	assert.Empty(t, target.Files)
	require.NotEmpty(t, log.warnings)
	assert.Contains(t, log.warnings[0], "using git tags")
}

func TestDetectGoProjectWithInjectedVersionUsesTags(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/demo\n")
	writeFile(t, dir, "version.go", "package demo\n\nconst Version = \"1.0.0\" // set via -ldflags\n")

	target, found, err := New().Detect(dir, []string{"go.mod", "version.go"}, &testLogger{})
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, target.TagSource)
	assert.Empty(t, target.Files)
}

func TestDetectGoProjectRejectsMultipleSemver(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/demo\n")
	writeFile(t, dir, "version.go", "package demo\n\nconst Version = \"1.0.0\"\n\nfunc other() {\n\tconst Version = \"1.0.1\"\n}\n")

	_, _, err := New().Detect(dir, []string{"go.mod", "version.go"}, &testLogger{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, errVersionAmbiguous))
}
