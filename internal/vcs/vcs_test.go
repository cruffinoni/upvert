package vcs

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitTest runs git in dir and fails the test on error.
func gitTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "git %v\n%s", args, output)
}

func TestHighestVersionTag(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# demo\n"), 0o644))
	gitTest(t, dir, "init", "-q", "-b", "main")
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "-c", "user.name=Upvert Test", "-c", "user.email=upvert@example.com", "commit", "-q", "-m", "initial commit")
	for _, tag := range []string{"v1.2.0", "v1.10.0", "v1.9.9", "not-a-version", "2.0.0"} {
		gitTest(t, dir, "tag", tag)
	}

	ctx := t.Context()
	git := New(dir)
	highest, err := git.HighestVersionTag(ctx, "v*")
	require.NoError(t, err)
	assert.Equal(t, "v1.10.0", highest)
	latest, err := git.LatestTag(ctx)
	require.NoError(t, err)
	assert.Equal(t, "1.10.0", latest)
	fromTags, err := git.VersionFromTags(ctx)
	require.NoError(t, err)
	assert.Equal(t, "1.10.0", fromTags)
}
