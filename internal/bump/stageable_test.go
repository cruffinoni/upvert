package bump

import (
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitRejectsIgnoredChangelogBeforeWriting(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n",
		".gitignore":   "CHANGELOG.md\n",
	})
	cfg := Config{Dir: dir, Mode: ModeCommit, Stdout: io.Discard, Stderr: io.Discard}

	err := Apply(t.Context(), cfg, Patch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ignored")

	assert.Equal(t, "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n", readTestFile(t, filepath.Join(dir, "package.json")))
	assert.NoFileExists(t, filepath.Join(dir, "CHANGELOG.md"))
	assert.Empty(t, gitRun(t, dir, "status", "--porcelain"))
	assert.Empty(t, gitRun(t, dir, "tag", "--list"))
}
