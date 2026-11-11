package rust

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dayrion/upvert/internal/ecosystem"
)

func TestWriteCargoVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Cargo.toml")
	initial := "[package]\nname = \"demo\"\nversion = \"1.0.0\"\n\n[dependencies]\nserde = \"1\"\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatalf("write Cargo.toml: %v", err)
	}
	target := &ecosystem.Target{
		Kind:   ecosystem.Rust,
		Single: true,
		Files:  []ecosystem.File{{Path: "Cargo.toml"}},
	}
	if _, err := New().Write(dir, target, "1.1.0", nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Cargo.toml: %v", err)
	}
	want := "[package]\nname = \"demo\"\nversion = \"1.1.0\"\n\n[dependencies]\nserde = \"1\"\n"
	if got := string(data); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}
