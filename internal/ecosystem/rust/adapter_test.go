package rust

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dayrion/upvert/internal/ecosystem"
)

func TestWriteCargoVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Cargo.toml")
	initial := "[package]\nname = \"demo\"\nversion = \"1.0.0\"\n\n[dependencies]\nserde = \"1\"\n"
	require.NoError(t, os.WriteFile(path, []byte(initial), 0o644))
	target := &ecosystem.Target{
		Kind:   ecosystem.Rust,
		Single: true,
		Files:  []ecosystem.File{{Path: "Cargo.toml"}},
	}
	_, err := New().Write(dir, target, "1.1.0", nil)
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	want := "[package]\nname = \"demo\"\nversion = \"1.1.0\"\n\n[dependencies]\nserde = \"1\"\n"
	assert.Equal(t, want, string(data))
}
