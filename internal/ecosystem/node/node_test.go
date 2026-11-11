package node

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

func TestSetJSONString(t *testing.T) {
	input := []byte(`{
  "name": "demo",
  "version": "1.0.0",
  "nested": {"version": "9.9.9"}
}
`)
	updated, err := setJSONString(input, []string{"version"}, "1.1.0")
	if err != nil {
		t.Fatalf("setJSONString: %v", err)
	}
	if !json.Valid(updated) {
		t.Fatalf("result is not valid json: %s", updated)
	}
	var parsed map[string]any
	if err := json.Unmarshal(updated, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed["version"] != "1.1.0" {
		t.Fatalf("version = %v, want 1.1.0", parsed["version"])
	}
	if parsed["name"] != "demo" {
		t.Fatalf("name = %v, want demo", parsed["name"])
	}
	nested, ok := parsed["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested = %T, want object", parsed["nested"])
	}
	if nested["version"] != "9.9.9" {
		t.Fatalf("nested version = %v, want 9.9.9", nested["version"])
	}
}

func TestSetJSONStringNestedEmptyKey(t *testing.T) {
	input := []byte(`{"packages":{"":{"version":"1.0.0"}}}`)
	updated, err := setJSONString(input, []string{"packages", "", "version"}, "1.1.0")
	if err != nil {
		t.Fatalf("setJSONString: %v", err)
	}
	want := `{"packages":{"":{"version":"1.1.0"}}}`
	if string(updated) != want {
		t.Fatalf("updated = %s, want %s", updated, want)
	}
}

func TestSetJSONStringMissingKey(t *testing.T) {
	_, err := setJSONString([]byte(`{"name":"demo"}`), []string{"version"}, "1.1.0")
	if !errors.Is(err, errJSONKeyNotFound) {
		t.Fatalf("error = %v, want errJSONKeyNotFound", err)
	}
}

func TestSetJSONStringRejectsInvalidInput(t *testing.T) {
	if _, err := setJSONString([]byte("not json"), []string{"version"}, "1.1.0"); err == nil {
		t.Fatal("expected an error for invalid json")
	}
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
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !changed {
		t.Fatal("expected the manifest to change")
	}
	want := `{
  "name": "demo",
  "version": "1.1.0",
  "nested": {"version": "9.9.9"}
}
`
	if got := readFile(t, path); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
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
	if _, err := New().Write(dir, target, "1.1.0", nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
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
	if got := readFile(t, path); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}
