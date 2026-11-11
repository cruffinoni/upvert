package update

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractTarGz(t *testing.T) {
	content := []byte("binary")
	path := filepath.Join(t.TempDir(), "release.tar.gz")
	require.NoError(t, os.WriteFile(path, tarGzArchive(t, "nested/upvert", content), 0o644))

	got, err := extractTarGz(path)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestExtractTarGzErrors(t *testing.T) {
	t.Run("missing binary", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "release.tar.gz")
		require.NoError(t, os.WriteFile(path, tarGzArchive(t, "other", []byte("data")), 0o644))
		_, err := extractTarGz(path)
		require.ErrorIs(t, err, ErrBinaryNotFound)
		archiveErr, ok := errors.AsType[*ArchiveError](err)
		require.True(t, ok)
		assert.Equal(t, "upvert", archiveErr.Entry)
	})

	t.Run("corrupt archive", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "release.tar.gz")
		require.NoError(t, os.WriteFile(path, []byte("not gzip"), 0o644))
		_, err := extractTarGz(path)
		require.Error(t, err)
	})
}

func TestExtractZip(t *testing.T) {
	content := []byte("binary")
	path := filepath.Join(t.TempDir(), "release.zip")
	require.NoError(t, os.WriteFile(path, zipArchive(t, "nested/upvert.exe", content), 0o644))

	got, err := extractZip(path)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestExtractZipErrors(t *testing.T) {
	t.Run("missing binary", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "release.zip")
		require.NoError(t, os.WriteFile(path, zipArchive(t, "other.exe", []byte("data")), 0o644))
		_, err := extractZip(path)
		require.ErrorIs(t, err, ErrBinaryNotFound)
		archiveErr, ok := errors.AsType[*ArchiveError](err)
		require.True(t, ok)
		assert.Equal(t, "upvert.exe", archiveErr.Entry)
	})

	t.Run("corrupt archive", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "release.zip")
		require.NoError(t, os.WriteFile(path, []byte("not zip"), 0o644))
		_, err := extractZip(path)
		require.Error(t, err)
	})
}

func TestExtractBinary(t *testing.T) {
	content := []byte("binary")
	dir := t.TempDir()

	tarPath := filepath.Join(dir, "release.tar.gz")
	require.NoError(t, os.WriteFile(tarPath, tarGzArchive(t, "upvert", content), 0o644))
	got, err := extractBinary(tarPath, "upvert_1.2.3_linux_amd64.tar.gz")
	require.NoError(t, err)
	assert.Equal(t, content, got)

	zipPath := filepath.Join(dir, "release.zip")
	require.NoError(t, os.WriteFile(zipPath, zipArchive(t, "upvert.exe", content), 0o644))
	got, err = extractBinary(zipPath, "upvert_1.2.3_windows_amd64.zip")
	require.NoError(t, err)
	assert.Equal(t, content, got)

	_, err = extractBinary(tarPath, "release.tar")
	require.ErrorIs(t, err, ErrUnsupportedArchive)

	emptyPath := filepath.Join(dir, "empty.tar.gz")
	require.NoError(t, os.WriteFile(emptyPath, tarGzArchive(t, "upvert", nil), 0o644))
	_, err = extractBinary(emptyPath, "upvert_1.2.3_linux_amd64.tar.gz")
	require.ErrorIs(t, err, ErrEmptyBinary)
}
