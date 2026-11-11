package bump

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeTestFile creates a file and any parent directories below dir.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	require.NoErrorf(t, os.MkdirAll(filepath.Dir(path), 0o755), "create directories for %s", name)
	require.NoErrorf(t, os.WriteFile(path, []byte(content), 0o644), "write %s", name)
}

// readTestFile returns the content of path.
func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoErrorf(t, err, "read %s", path)
	return string(data)
}

// gitRun runs git in dir and returns trimmed standard output.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "git %s:\n%s", strings.Join(args, " "), output)
	return strings.TrimSpace(string(output))
}

// initTestRepo creates a git repository containing files and an initial
// commit, with an isolated git identity.
func initTestRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GIT_AUTHOR_NAME", "Upvert Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "upvert@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Upvert Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "upvert@example.com")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("HOME", t.TempDir())
	for name, content := range files {
		writeTestFile(t, dir, name, content)
	}
	gitRun(t, dir, "init", "-q", "-b", "main")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "initial commit")
	return dir
}
