package bump

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBumpCommit(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\n  \"name\": \"demo\",\n  \"version\": \"1.2.3\"\n}\n",
	})
	require.NoError(t, Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Patch))
	manifest := readTestFile(t, filepath.Join(dir, "package.json"))
	assert.Contains(t, manifest, "\"version\": \"1.2.4\"")
	changelog := readTestFile(t, filepath.Join(dir, "CHANGELOG.md"))
	assert.Contains(t, changelog, "## [1.2.4]")
	assert.Equal(t, "v1.2.4", gitRun(t, dir, "tag"))
	assert.Equal(t, "chore: bump version to 1.2.4", gitRun(t, dir, "log", "-1", "--pretty=%s"))
	assert.Empty(t, gitRun(t, dir, "status", "--porcelain"))
}

func TestBumpDryRunLeavesRepositoryUntouched(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n",
	})
	cfg := Config{Dir: dir, Stdout: io.Discard, Stderr: io.Discard}
	require.NoError(t, Apply(t.Context(), cfg, Patch))
	manifest := readTestFile(t, filepath.Join(dir, "package.json"))
	assert.Contains(t, manifest, "\"1.2.3\"")
	_, err := os.Stat(filepath.Join(dir, "CHANGELOG.md"))
	assert.True(t, os.IsNotExist(err))
	assert.Empty(t, gitRun(t, dir, "tag"))
	assert.Equal(t, "initial commit", gitRun(t, dir, "log", "-1", "--pretty=%s"))
}

func TestBumpGoWithoutVersionUsesTags(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"go.mod":  "module example.com/demo\n\ngo 1.27\n",
		"main.go": "package main\n\nvar version = \"dev\"\n\nfunc main() {}\n",
	})
	require.NoError(t, Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Patch))
	source := readTestFile(t, filepath.Join(dir, "main.go"))
	assert.Contains(t, source, "var version = \"dev\"")
	changelog := readTestFile(t, filepath.Join(dir, "CHANGELOG.md"))
	assert.Contains(t, changelog, "## [0.0.1]")
	assert.Equal(t, "v0.0.1", gitRun(t, dir, "tag"))
	require.NoError(t, Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Patch))
	assert.Contains(t, gitRun(t, dir, "tag"), "v0.0.2")
}

func TestBumpGoTagSourceWriteIsRejected(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"go.mod":  "module example.com/demo\n\ngo 1.27\n",
		"main.go": "package main\n\nvar version = \"dev\"\n\nfunc main() {}\n",
	})
	err := Apply(t.Context(), Config{Dir: dir, Mode: ModeWrite}, Patch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tag-sourced projects require a tag")
	source := readTestFile(t, filepath.Join(dir, "main.go"))
	assert.Contains(t, source, "var version = \"dev\"")
	assert.Empty(t, gitRun(t, dir, "tag"))
}

func TestBumpWriteLeavesGitUntouched(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n",
	})
	require.NoError(t, Apply(t.Context(), Config{Dir: dir, Mode: ModeWrite}, Patch))
	manifest := readTestFile(t, filepath.Join(dir, "package.json"))
	assert.Contains(t, manifest, "\"1.2.4\"")
	_, err := os.Stat(filepath.Join(dir, "CHANGELOG.md"))
	assert.NoError(t, err)
	assert.Empty(t, gitRun(t, dir, "tag"))
	assert.Equal(t, "initial commit", gitRun(t, dir, "log", "-1", "--pretty=%s"))
}

func TestBumpNoVersionAttrWithoutWritableDeclaration(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"go.mod":     "module example.com/demo\n\ngo 1.27\n",
		"version.go": "package demo\n\nconst Version = \"1.0.0\"\n",
	})
	cfg := Config{Dir: dir, Mode: ModeCommit, NoVersionAttr: true}
	err := Apply(t.Context(), cfg, Patch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no writable version declaration")
	source := readTestFile(t, filepath.Join(dir, "version.go"))
	assert.Contains(t, source, "Version = \"1.0.0\"")
	assert.Empty(t, gitRun(t, dir, "tag"))
}

func TestBumpRejectsMessageWithoutPlaceholder(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n",
	})
	cfg := Config{Dir: dir, Mode: ModeCommit, Message: "release"}
	err := Apply(t.Context(), cfg, Patch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must contain {version}")
	manifest := readTestFile(t, filepath.Join(dir, "package.json"))
	assert.Contains(t, manifest, "1.0.0")
}

func TestBumpMultiLanguageWriteIsRejected(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"go.mod":       "module example.com/demo\n",
		"version.go":   "package demo\n\nconst Version = \"1.2.3\"\n",
		"package.json": "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n",
	})
	err := Apply(t.Context(), Config{Dir: dir, Mode: ModeWrite}, Patch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multi-language projects require a tag")
	manifest := readTestFile(t, filepath.Join(dir, "package.json"))
	assert.Contains(t, manifest, "1.2.3")
}

func TestBumpPushRequiresOrigin(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n",
	})
	err := Apply(t.Context(), Config{Dir: dir, Mode: ModePush}, Patch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no origin remote")
	manifest := readTestFile(t, filepath.Join(dir, "package.json"))
	assert.Contains(t, manifest, "1.0.0")
	assert.Empty(t, gitRun(t, dir, "tag"))
}

func TestBumpPreRelease(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n",
	})
	cfg := Config{Dir: dir, Mode: ModeCommit, PreRelease: "alpha"}
	require.NoError(t, Apply(t.Context(), cfg, Minor))
	manifest := readTestFile(t, filepath.Join(dir, "package.json"))
	assert.Contains(t, manifest, "\"version\":\"1.1.0-alpha.1\"")
	assert.Equal(t, "v1.1.0-alpha.1", gitRun(t, dir, "tag"))
}

func TestBumpRejectsDirtyWorkTree(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n",
	})
	writeTestFile(t, dir, "package.json", "{\"name\":\"demo\",\"version\":\"1.2.3\",\"extra\":true}\n")
	err := Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Patch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uncommitted changes")
}

func TestBumpRejectsVersionTagMismatch(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n",
	})
	gitRun(t, dir, "tag", "-a", "v1.3.0", "-m", "later release")
	err := Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Patch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "differs from latest tag")
	manifest := readTestFile(t, filepath.Join(dir, "package.json"))
	assert.Contains(t, manifest, "\"1.2.3\"")
}

func TestBumpRejectsInvalidPreReleaseLabel(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n",
	})
	cfg := Config{Dir: dir, Mode: ModeCommit, PreRelease: "not a label"}
	require.Error(t, Apply(t.Context(), cfg, Patch))
}

func TestRollback(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n",
		"README.md":    "# demo\n",
	})
	require.NoError(t, Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Patch))
	require.NoError(t, Rollback(t.Context(), Config{Dir: dir, Mode: ModeApply}))
	manifest := readTestFile(t, filepath.Join(dir, "package.json"))
	assert.Contains(t, manifest, "\"1.0.0\"")
	assert.Empty(t, gitRun(t, dir, "tag"))
	assert.Equal(t, "initial commit", gitRun(t, dir, "log", "-1", "--pretty=%s"))
	_, err := os.Stat(filepath.Join(dir, "CHANGELOG.md"))
	assert.True(t, os.IsNotExist(err))
}

func TestRollbackCustomCommitMessage(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n",
	})
	cfg := Config{Dir: dir, Mode: ModeCommit, Message: "release {version}"}
	require.NoError(t, Apply(t.Context(), cfg, Patch))
	assert.Equal(t, "release 1.0.1", gitRun(t, dir, "log", "-1", "--pretty=%s"))
	rollbackCfg := cfg
	rollbackCfg.Mode = ModeApply
	require.NoError(t, Rollback(t.Context(), rollbackCfg))
	assert.Empty(t, gitRun(t, dir, "tag"))
}

func TestBumpMultiLanguageUsesTags(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"go.mod":       "module example.com/demo\n",
		"version.go":   "package demo\n\nconst Version = \"1.2.3\"\n",
		"package.json": "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n",
	})
	require.NoError(t, Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Patch))
	manifest := readTestFile(t, filepath.Join(dir, "package.json"))
	assert.Contains(t, manifest, "\"version\":\"0.0.1\"")
	source := readTestFile(t, filepath.Join(dir, "version.go"))
	assert.Contains(t, source, "Version = \"0.0.1\"")
	assert.Equal(t, "v0.0.1", gitRun(t, dir, "tag"))
	require.NoError(t, Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Patch))
	manifest = readTestFile(t, filepath.Join(dir, "package.json"))
	assert.Contains(t, manifest, "\"version\":\"0.0.2\"")
}

func TestBumpHooks(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n",
	})
	cfg := Config{
		Dir:      dir,
		Mode:     ModeCommit,
		PreBump:  "printf pre >> hooks.log",
		PostBump: "printf post >> hooks.log",
	}
	require.NoError(t, Apply(t.Context(), cfg, Patch))
	assert.Equal(t, "prepost", readTestFile(t, filepath.Join(dir, "hooks.log")))
}

func TestDryRunSkipsPreBumpHook(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n",
	})
	cfg := Config{
		Dir:     dir,
		PreBump: "printf pre > hook-ran",
		Stdout:  io.Discard,
		Stderr:  io.Discard,
	}
	require.NoError(t, Apply(t.Context(), cfg, Patch))
	_, err := os.Stat(filepath.Join(dir, "hook-ran"))
	assert.True(t, os.IsNotExist(err))
}

func TestBumpGoProject(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"go.mod":     "module example.com/demo\n\ngo 1.27\n",
		"version.go": "package demo\n\nconst Version = \"1.0.0\"\n",
	})
	require.NoError(t, Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Minor))
	source := readTestFile(t, filepath.Join(dir, "version.go"))
	assert.Contains(t, source, "Version = \"1.1.0\"")
	assert.Equal(t, "v1.1.0", gitRun(t, dir, "tag"))
}

func TestBumpPushToOrigin(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n",
	})
	remote := t.TempDir()
	gitRun(t, remote, "init", "-q", "--bare")
	gitRun(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")
	gitRun(t, dir, "remote", "add", "origin", remote)

	require.NoError(t, Apply(t.Context(), Config{Dir: dir, Mode: ModePush}, Patch))
	assert.Equal(t, "v1.0.1", gitRun(t, remote, "tag"))
	assert.Equal(t, "chore: bump version to 1.0.1", gitRun(t, remote, "log", "-1", "--pretty=%s"))
}

func TestBumpPythonProjectUpdatesModules(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"pyproject.toml": `[project]
name = 'demo'
version = '1.0.0'
`,
		"src/pkg/__init__.py": `__version__ = '1.0.0'
`,
	})
	require.NoError(t, Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Patch))
	manifest := readTestFile(t, filepath.Join(dir, "pyproject.toml"))
	assert.Contains(t, manifest, "version = '1.0.1'")
	source := readTestFile(t, filepath.Join(dir, "src/pkg/__init__.py"))
	assert.Contains(t, source, "__version__ = '1.0.1'")
	assert.Equal(t, "v1.0.1", gitRun(t, dir, "tag"))
}

func TestBumpRustProject(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"Cargo.toml": `[package]
name = "demo"
version = "1.0.0"
`,
	})
	require.NoError(t, Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Minor))
	manifest := readTestFile(t, filepath.Join(dir, "Cargo.toml"))
	assert.Contains(t, manifest, "version = \"1.1.0\"")
	assert.Equal(t, "v1.1.0", gitRun(t, dir, "tag"))
}

func TestBumpPythonNoVersionAttrSkipsModules(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"pyproject.toml": `[project]
name = 'demo'
version = '1.0.0'
`,
		"src/pkg/__init__.py": `__version__ = '1.0.0'
`,
	})
	cfg := Config{Dir: dir, Mode: ModeCommit, NoVersionAttr: true}
	require.NoError(t, Apply(t.Context(), cfg, Patch))
	manifest := readTestFile(t, filepath.Join(dir, "pyproject.toml"))
	assert.Contains(t, manifest, "version = '1.0.1'")
	source := readTestFile(t, filepath.Join(dir, "src/pkg/__init__.py"))
	assert.Contains(t, source, "__version__ = '1.0.0'")
}
