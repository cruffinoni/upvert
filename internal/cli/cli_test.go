package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cruffinoni/upvert/internal/buildinfo"
)

func writeProjectFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoErrorf(t, os.WriteFile(path, []byte(content), 0o644), "write %s", name)
}

func TestCLIDryRun(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "package.json", "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n")

	cmd := New()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"patch", "--dir", dir})
	require.NoErrorf(t, cmd.ExecuteContext(t.Context()), "execute:\n%s", stderr.String())
	assert.Contains(t, stdout.String(), "upvert patch (preview)")
	assert.Contains(t, stdout.String(), "1.0.1")
	assert.Contains(t, stdout.String(), "Nothing was written")
	manifest, err := os.ReadFile(filepath.Join(dir, "package.json"))
	require.NoError(t, err)
	assert.Contains(t, string(manifest), "1.0.0")
}

func TestCLIWriteUpdatesFilesOnly(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "package.json", "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n")

	cmd := New()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"patch", "--dir", dir, "--write"})
	require.NoErrorf(t, cmd.ExecuteContext(t.Context()), "execute:\n%s", stderr.String())
	manifest, err := os.ReadFile(filepath.Join(dir, "package.json"))
	require.NoError(t, err)
	assert.Contains(t, string(manifest), "1.0.1")
	_, err = os.Stat(filepath.Join(dir, "CHANGELOG.md"))
	assert.NoError(t, err)
}

func TestCLIRejectsMultipleActions(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "package.json", "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n")

	cmd := New()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"patch", "--dir", dir, "--write", "--commit"})
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "choose one of --write, --commit, or --push")
	manifest, readErr := os.ReadFile(filepath.Join(dir, "package.json"))
	require.NoError(t, readErr)
	assert.Contains(t, string(manifest), "1.0.0")
}

func TestCLIIgnoresUpvertFile(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "package.json", "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n")
	writeProjectFile(t, dir, ".upvert.yaml", "commit: true\npre: alpha\n")

	cmd := New()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"patch", "--dir", dir})
	require.NoErrorf(t, cmd.ExecuteContext(t.Context()), "execute:\n%s", stderr.String())
	assert.NotContains(t, stdout.String(), "alpha")
	manifest, err := os.ReadFile(filepath.Join(dir, "package.json"))
	require.NoError(t, err)
	assert.Contains(t, string(manifest), "1.0.0")
}

func TestCLIRejectsInvalidPreReleaseLabel(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "package.json", "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n")

	cmd := New()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"patch", "--dir", dir, "--pre", "bad label"})
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pre-release label")
}

func TestCLIUnknownCommand(t *testing.T) {
	cmd := New()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"bogus"})
	assert.Error(t, cmd.ExecuteContext(t.Context()))
}

func TestCLIVersion(t *testing.T) {
	version, commit, builtAt := buildinfo.Version, buildinfo.Commit, buildinfo.BuiltAt
	buildinfo.Version = "1.2.3"
	buildinfo.Commit = "abc123"
	buildinfo.BuiltAt = "2026-01-02T03:04:05Z"
	t.Cleanup(func() {
		buildinfo.Version = version
		buildinfo.Commit = commit
		buildinfo.BuiltAt = builtAt
	})

	cmd := New()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"version"})
	require.NoError(t, cmd.ExecuteContext(t.Context()))
	for _, want := range []string{"upvert 1.2.3", "commit: abc123", "built: 2026-01-02T03:04:05Z"} {
		assert.Contains(t, stdout.String(), want)
	}
}

func TestCLIHelpListsCommands(t *testing.T) {
	cmd := New()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--help"})
	require.NoError(t, cmd.ExecuteContext(t.Context()))
	for _, command := range []string{"patch", "minor", "major", "rollback", "update"} {
		assert.Contains(t, stdout.String(), command)
	}
}

func TestCLIPatchHelpShowsActions(t *testing.T) {
	cmd := New()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"patch", "--help"})
	require.NoError(t, cmd.ExecuteContext(t.Context()))
	for _, flag := range []string{"--write", "--commit", "--push"} {
		assert.Contains(t, stdout.String(), flag)
	}
}

func TestCLIDirFlagIsScopedToBumpAndRollback(t *testing.T) {
	check := func(args []string, want bool) {
		t.Helper()
		cmd := New()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(args)
		require.NoError(t, cmd.ExecuteContext(t.Context()))
		assert.Equal(t, want, strings.Contains(stdout.String(), "--dir"), "%v", args)
	}
	check([]string{"patch", "--help"}, true)
	check([]string{"rollback", "--help"}, true)
	check([]string{"update", "--help"}, false)
	check([]string{"version", "--help"}, false)
}

func TestCLIRollbackHelpShowsApply(t *testing.T) {
	cmd := New()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"rollback", "--help"})
	require.NoError(t, cmd.ExecuteContext(t.Context()))
	assert.Contains(t, stdout.String(), "--apply")
	assert.NotContains(t, stdout.String(), "--commit")
}
