package node

import (
	"encoding/json"
	"errors"
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

func TestSetJSONString(t *testing.T) {
	input := []byte(`{
  "name": "demo",
  "version": "1.0.0",
  "nested": {"version": "9.9.9"}
}
`)
	updated, err := setJSONString(input, []string{"version"}, "1.1.0")
	require.NoError(t, err)
	assert.True(t, json.Valid(updated))
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(updated, &parsed))
	assert.Equal(t, "1.1.0", parsed["version"])
	assert.Equal(t, "demo", parsed["name"])
	nested, ok := parsed["nested"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "9.9.9", nested["version"])
}

func TestSetJSONStringNestedEmptyKey(t *testing.T) {
	input := []byte(`{"packages":{"":{"version":"1.0.0"}}}`)
	updated, err := setJSONString(input, []string{"packages", "", "version"}, "1.1.0")
	require.NoError(t, err)
	want := `{"packages":{"":{"version":"1.1.0"}}}`
	assert.Equal(t, want, string(updated))
}

func TestSetJSONStringMissingKey(t *testing.T) {
	_, err := setJSONString([]byte(`{"name":"demo"}`), []string{"version"}, "1.1.0")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errJSONKeyNotFound))
}

func TestSetJSONStringRejectsInvalidInput(t *testing.T) {
	_, err := setJSONString([]byte("not json"), []string{"version"}, "1.1.0")
	require.Error(t, err)
}

func TestWriteNodeManifestPreservesFormatting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "package.json")
	writeFile(t, dir, "package.json", `{
  "name": "demo",
  "version": "1.0.0",
  "nested": {"version": "9.9.9"}
}
`)

	target := &ecosystem.Target{
		Kind:   ecosystem.Node,
		Single: true,
		Files:  []ecosystem.File{{Path: "package.json"}},
	}
	changed, err := New().Write(dir, target, "1.1.0", nil)
	require.NoError(t, err)
	assert.True(t, changed)
	want := `{
  "name": "demo",
  "version": "1.1.0",
  "nested": {"version": "9.9.9"}
}
`
	assert.Equal(t, want, readFile(t, path))
}

func TestWriteNodeLockfileUpdatesBothVersions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "package-lock.json")
	writeFile(t, dir, "package-lock.json", `{
  "name": "demo",
  "version": "1.0.0",
  "packages": {
    "": {
      "version": "1.0.0"
    }
  }
}
`)

	target := &ecosystem.Target{
		Kind:  ecosystem.Node,
		Files: []ecosystem.File{{Path: "package-lock.json"}},
	}
	_, err := New().Write(dir, target, "1.1.0", nil)
	require.NoError(t, err)
	want := `{
  "name": "demo",
  "version": "1.1.0",
  "packages": {
    "": {
      "version": "1.1.0"
    }
  }
}
`
	assert.Equal(t, want, readFile(t, path))
}
