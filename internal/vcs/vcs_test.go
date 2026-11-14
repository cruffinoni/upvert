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

// initWorkTree creates an initialized repository below a temporary directory.
func initWorkTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitTest(t, dir, "init", "-q", "-b", "main")
	return dir
}

// commitWorkTree stages every file and commits it with a test identity.
func commitWorkTree(t *testing.T, dir, message string) {
	t.Helper()
	gitTest(t, dir, "add", "-A")
	gitTest(t, dir, "-c", "user.name=Upvert Test", "-c", "user.email=upvert@example.com", "commit", "-q", "-m", message)
}

func TestEnsureStageableRejectsIgnoredUntrackedFile(t *testing.T) {
	dir := initWorkTree(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("scratch/\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.go"), []byte("package demo\n"), 0o644))
	commitWorkTree(t, dir, "initial commit")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "scratch"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "scratch", "gen.go"), []byte("package scratch\n"), 0o644))

	git := New(dir)
	ctx := t.Context()
	err := git.EnsureStageable(ctx, "scratch/gen.go")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ignored")
	require.NoError(t, git.EnsureStageable(ctx, "tracked.go"))
}

func TestEnsureStageableAcceptsTrackedIgnoredFile(t *testing.T) {
	dir := initWorkTree(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keep.log"), []byte("kept\n"), 0o644))
	commitWorkTree(t, dir, "track the log")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644))
	commitWorkTree(t, dir, "ignore logs")

	git := New(dir)
	ctx := t.Context()
	ignored, err := git.IsIgnored(ctx, "keep.log")
	require.NoError(t, err)
	assert.False(t, ignored, "tracked files are stageable even when an exclude rule matches")
	require.NoError(t, git.EnsureStageable(ctx, "keep.log"))
}
