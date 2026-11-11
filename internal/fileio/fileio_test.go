package fileio

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateReportsChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version.txt")
	require.NoError(t, os.WriteFile(path, []byte("1.0.0"), 0o644))
	changed, err := Update(path, func([]byte) ([]byte, error) {
		return []byte("1.1.0"), nil
	})
	require.NoError(t, err)
	assert.True(t, changed)
	changed, err = Update(path, func(data []byte) ([]byte, error) {
		return data, nil
	})
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestReplaceLinesAndQuoteLike(t *testing.T) {
	content := "version = 1.0.0\nother = 2.0.0\n"
	got := ReplaceLines(content, regexp.MustCompile("^version = .*$"), "version = 1.1.0")
	want := "version = 1.1.0\nother = 2.0.0\n"
	assert.Equal(t, want, got)
	assert.Equal(t, "\"1.1.0\"", QuoteLike("\"1.0.0\"", "1.1.0"))
	assert.Equal(t, "1.1.0", QuoteLike("1.0.0", "1.1.0"))
}
