package golang

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dayrion/upvert/internal/ui"
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

func TestWriteGoVersionFileIndirect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "version.go")
	writeFile(t, dir, "version.go", "package demo\n\nvar buildVersion = \"1.0.0\"\nconst Version = buildVersion\n")

	var output bytes.Buffer
	changed, err := writeVersionFile(path, "1.1.0", ui.New(&output, false))
	if err != nil {
		t.Fatalf("writeVersionFile: %v", err)
	}
	if !changed {
		t.Fatal("expected the file to change")
	}
	got := readFile(t, path)
	if !strings.Contains(got, "var buildVersion = \"1.1.0\"") {
		t.Fatalf("variable was not updated:\n%s", got)
	}
	if !strings.Contains(got, "const Version = buildVersion") {
		t.Fatalf("reference was changed:\n%s", got)
	}
	if !strings.Contains(output.String(), "buildVersion") {
		t.Fatalf("indirection was not reported: %q", output.String())
	}
}

func TestWriteGoVersionFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "version.go")
	writeFile(t, dir, "version.go", "package demo\n\nconst Version = \"1.0.0\"\n")

	if _, err := writeVersionFile(path, "1.1.0", nil); err != nil {
		t.Fatalf("writeVersionFile: %v", err)
	}
	want := "package demo\n\nconst Version = \"1.1.0\"\n"
	if got := readFile(t, path); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestDetectGoProjectSkipsUnparseableFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/demo\n")
	writeFile(t, dir, "bad.go", "package demo\n\nconst Version = \"1.0.0\"\n\nfunc broken( {\n")
	writeFile(t, dir, "version.go", "package demo\n\nconst Version = \"1.0.0\"\n")

	log := &testLogger{}
	adapter := New()
	target, found, err := adapter.Detect(dir, []string{"go.mod", "bad.go", "version.go"}, log)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !found || target.Files[0].Path != "version.go" {
		t.Fatalf("target = %+v, want version.go", target)
	}
	if len(log.warnings) == 0 || !strings.Contains(log.warnings[0], "bad.go") {
		t.Fatalf("warnings = %v, want one mentioning bad.go", log.warnings)
	}
}

func TestDetectGoProjectRejectsMultipleSemver(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/demo\n")
	writeFile(t, dir, "version.go", "package demo\n\nconst Version = \"1.0.0\"\n\nfunc other() {\n\tconst Version = \"1.0.1\"\n}\n")

	_, _, err := New().Detect(dir, []string{"go.mod", "version.go"}, &testLogger{})
	if !errors.Is(err, errVersionAmbiguous) {
		t.Fatalf("error = %v, want errVersionAmbiguous", err)
	}
}
