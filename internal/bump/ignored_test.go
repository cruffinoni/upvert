package bump

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitLeavesIgnoredTreesAlone(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		".gitignore": ".tmp/\n",
		"go.mod":     "module example.com/demo\n",
		"version.go": "package demo\n\nconst Version = \"1.2.3\"\n",
	})
	writeTestFile(t, dir, ".tmp/smoke/package.json", "{\"name\":\"demo\",\"version\":\"9.9.9\"}")

	cfg := Config{Dir: dir, Mode: ModeCommit, Stdout: io.Discard, Stderr: io.Discard}
	require.NoError(t, Apply(t.Context(), cfg, Patch))

	assert.Equal(t, "{\"name\":\"demo\",\"version\":\"9.9.9\"}", readTestFile(t, filepath.Join(dir, ".tmp/smoke/package.json")))
	assert.Equal(t, "v1.2.4", gitRun(t, dir, "tag", "--list"))
	assert.Equal(t, "chore: bump version to 1.2.4", gitRun(t, dir, "log", "-1", "--pretty=%s"))

	changed := strings.Fields(gitRun(t, dir, "show", "--name-only", "--pretty=format:", "HEAD"))
	assert.ElementsMatch(t, []string{"CHANGELOG.md", "version.go"}, changed)
}
