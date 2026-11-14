package bump

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitFailureRestoresWorkTree(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n",
	})
	hook := filepath.Join(dir, ".git", "hooks", "pre-commit")
	require.NoError(t, os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755))

	cfg := Config{Dir: dir, Mode: ModeCommit, Stdout: io.Discard, Stderr: io.Discard}
	require.Error(t, Apply(t.Context(), cfg, Patch))

	assert.Equal(t, "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n", readTestFile(t, filepath.Join(dir, "package.json")))
	assert.NoFileExists(t, filepath.Join(dir, "CHANGELOG.md"))
	assert.Empty(t, gitRun(t, dir, "status", "--porcelain"))
	assert.Empty(t, gitRun(t, dir, "tag", "--list"))
	assert.Equal(t, "initial commit", gitRun(t, dir, "log", "-1", "--pretty=%s"))
}

func TestWriteFailureRestoresWorkTree(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n",
	})
	cfg := Config{Dir: dir, Mode: ModeWrite, PostBump: "exit 1", Stdout: io.Discard, Stderr: io.Discard}

	require.Error(t, Apply(t.Context(), cfg, Patch))

	assert.Equal(t, "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n", readTestFile(t, filepath.Join(dir, "package.json")))
	assert.NoFileExists(t, filepath.Join(dir, "CHANGELOG.md"))
	assert.Empty(t, gitRun(t, dir, "status", "--porcelain"))
}
