package bump

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBumpCommit(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\n  \"name\": \"demo\",\n  \"version\": \"1.2.3\"\n}\n",
	})
	if err := Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Patch); err != nil {
		t.Fatalf("Bump: %v", err)
	}
	manifest := readTestFile(t, filepath.Join(dir, "package.json"))
	if !strings.Contains(manifest, "\"version\": \"1.2.4\"") {
		t.Fatalf("package.json was not bumped:\n%s", manifest)
	}
	changelog := readTestFile(t, filepath.Join(dir, "CHANGELOG.md"))
	if !strings.Contains(changelog, "## [1.2.4]") {
		t.Fatalf("changelog was not updated:\n%s", changelog)
	}
	if tags := gitRun(t, dir, "tag"); tags != "v1.2.4" {
		t.Fatalf("tags = %q, want v1.2.4", tags)
	}
	if subject := gitRun(t, dir, "log", "-1", "--pretty=%s"); subject != "chore: bump version to 1.2.4" {
		t.Fatalf("commit subject = %q", subject)
	}
	if status := gitRun(t, dir, "status", "--porcelain"); status != "" {
		t.Fatalf("work tree is dirty after the bump:\n%s", status)
	}
}

func TestBumpDryRunLeavesRepositoryUntouched(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n",
	})
	cfg := Config{Dir: dir, Stdout: io.Discard, Stderr: io.Discard}
	if err := Apply(t.Context(), cfg, Patch); err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if manifest := readTestFile(t, filepath.Join(dir, "package.json")); !strings.Contains(manifest, "\"1.2.3\"") {
		t.Fatalf("dry run modified package.json:\n%s", manifest)
	}
	if _, err := os.Stat(filepath.Join(dir, "CHANGELOG.md")); err == nil {
		t.Fatal("dry run created a changelog")
	}
	if tags := gitRun(t, dir, "tag"); tags != "" {
		t.Fatalf("dry run created tags: %q", tags)
	}
	if subject := gitRun(t, dir, "log", "-1", "--pretty=%s"); subject != "initial commit" {
		t.Fatalf("dry run created a commit: %q", subject)
	}
}

func TestBumpGoWithoutVersionUsesTags(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"go.mod":  "module example.com/demo\n\ngo 1.27\n",
		"main.go": "package main\n\nvar version = \"dev\"\n\nfunc main() {}\n",
	})
	if err := Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Patch); err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if source := readTestFile(t, filepath.Join(dir, "main.go")); !strings.Contains(source, "var version = \"dev\"") {
		t.Fatalf("tag-sourced bump modified main.go:\n%s", source)
	}
	if changelog := readTestFile(t, filepath.Join(dir, "CHANGELOG.md")); !strings.Contains(changelog, "## [0.0.1]") {
		t.Fatalf("changelog was not updated:\n%s", changelog)
	}
	if tags := gitRun(t, dir, "tag"); tags != "v0.0.1" {
		t.Fatalf("tags = %q, want v0.0.1", tags)
	}
	if err := Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Patch); err != nil {
		t.Fatalf("second Bump: %v", err)
	}
	if tags := gitRun(t, dir, "tag"); !strings.Contains(tags, "v0.0.2") {
		t.Fatalf("tags = %q, want v0.0.2", tags)
	}
}

func TestBumpGoTagSourceWriteIsRejected(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"go.mod":  "module example.com/demo\n\ngo 1.27\n",
		"main.go": "package main\n\nvar version = \"dev\"\n\nfunc main() {}\n",
	})
	err := Apply(t.Context(), Config{Dir: dir, Mode: ModeWrite}, Patch)
	if err == nil || !strings.Contains(err.Error(), "tag-sourced projects require a tag") {
		t.Fatalf("error = %v, want a tag-source error", err)
	}
	if source := readTestFile(t, filepath.Join(dir, "main.go")); !strings.Contains(source, "var version = \"dev\"") {
		t.Fatalf("rejected write modified main.go:\n%s", source)
	}
	if tags := gitRun(t, dir, "tag"); tags != "" {
		t.Fatalf("rejected write created tags: %q", tags)
	}
}

func TestBumpWriteLeavesGitUntouched(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n",
	})
	if err := Apply(t.Context(), Config{Dir: dir, Mode: ModeWrite}, Patch); err != nil {
		t.Fatalf("Bump: %v", err)
	}
	manifest := readTestFile(t, filepath.Join(dir, "package.json"))
	if !strings.Contains(manifest, "\"1.2.4\"") {
		t.Fatalf("package.json was not bumped:\n%s", manifest)
	}
	if _, err := os.Stat(filepath.Join(dir, "CHANGELOG.md")); err != nil {
		t.Fatalf("changelog was not created: %v", err)
	}
	if tags := gitRun(t, dir, "tag"); tags != "" {
		t.Fatalf("write mode created tags: %q", tags)
	}
	if subject := gitRun(t, dir, "log", "-1", "--pretty=%s"); subject != "initial commit" {
		t.Fatalf("write mode created a commit: %q", subject)
	}
}

func TestBumpNoVersionAttrWithoutWritableDeclaration(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"go.mod":     "module example.com/demo\n\ngo 1.27\n",
		"version.go": "package demo\n\nconst Version = \"1.0.0\"\n",
	})
	cfg := Config{Dir: dir, Mode: ModeCommit, NoVersionAttr: true}
	err := Apply(t.Context(), cfg, Patch)
	if err == nil || !strings.Contains(err.Error(), "no writable version declaration") {
		t.Fatalf("error = %v, want no writable version declaration", err)
	}
	if source := readTestFile(t, filepath.Join(dir, "version.go")); !strings.Contains(source, "Version = \"1.0.0\"") {
		t.Fatalf("failed bump modified version.go:\n%s", source)
	}
	if tags := gitRun(t, dir, "tag"); tags != "" {
		t.Fatalf("failed bump created tags: %q", tags)
	}
}

func TestBumpRejectsMessageWithoutPlaceholder(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n",
	})
	cfg := Config{Dir: dir, Mode: ModeCommit, Message: "release"}
	err := Apply(t.Context(), cfg, Patch)
	if err == nil || !strings.Contains(err.Error(), "must contain {version}") {
		t.Fatalf("error = %v, want a placeholder error", err)
	}
	if manifest := readTestFile(t, filepath.Join(dir, "package.json")); !strings.Contains(manifest, "1.0.0") {
		t.Fatalf("failed bump modified package.json:\n%s", manifest)
	}
}

func TestBumpMultiLanguageWriteIsRejected(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"go.mod":       "module example.com/demo\n",
		"version.go":   "package demo\n\nconst Version = \"1.2.3\"\n",
		"package.json": "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n",
	})
	err := Apply(t.Context(), Config{Dir: dir, Mode: ModeWrite}, Patch)
	if err == nil || !strings.Contains(err.Error(), "multi-language projects require a tag") {
		t.Fatalf("error = %v, want a multi-language tag error", err)
	}
	if manifest := readTestFile(t, filepath.Join(dir, "package.json")); !strings.Contains(manifest, "1.2.3") {
		t.Fatalf("rejected write modified package.json:\n%s", manifest)
	}
}

func TestBumpPushRequiresOrigin(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n",
	})
	err := Apply(t.Context(), Config{Dir: dir, Mode: ModePush}, Patch)
	if err == nil || !strings.Contains(err.Error(), "no origin remote") {
		t.Fatalf("error = %v, want a missing origin error", err)
	}
	if manifest := readTestFile(t, filepath.Join(dir, "package.json")); !strings.Contains(manifest, "1.0.0") {
		t.Fatalf("failed push modified package.json:\n%s", manifest)
	}
	if tags := gitRun(t, dir, "tag"); tags != "" {
		t.Fatalf("failed push created tags: %q", tags)
	}
}

func TestBumpPreRelease(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n",
	})
	cfg := Config{Dir: dir, Mode: ModeCommit, PreRelease: "alpha"}
	if err := Apply(t.Context(), cfg, Minor); err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if manifest := readTestFile(t, filepath.Join(dir, "package.json")); !strings.Contains(manifest, "\"version\":\"1.1.0-alpha.1\"") {
		t.Fatalf("package.json was not pre-released:\n%s", manifest)
	}
	if tags := gitRun(t, dir, "tag"); tags != "v1.1.0-alpha.1" {
		t.Fatalf("tags = %q, want v1.1.0-alpha.1", tags)
	}
}

func TestBumpRejectsDirtyWorkTree(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n",
	})
	writeTestFile(t, dir, "package.json", "{\"name\":\"demo\",\"version\":\"1.2.3\",\"extra\":true}\n")
	err := Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Patch)
	if err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("error = %v, want uncommitted changes", err)
	}
}

func TestBumpRejectsVersionTagMismatch(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n",
	})
	gitRun(t, dir, "tag", "-a", "v1.3.0", "-m", "later release")
	err := Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Patch)
	if err == nil || !strings.Contains(err.Error(), "differs from latest tag") {
		t.Fatalf("error = %v, want version/tag mismatch", err)
	}
	if manifest := readTestFile(t, filepath.Join(dir, "package.json")); !strings.Contains(manifest, "\"1.2.3\"") {
		t.Fatalf("failed bump modified package.json:\n%s", manifest)
	}
}

func TestBumpRejectsInvalidPreReleaseLabel(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n",
	})
	cfg := Config{Dir: dir, Mode: ModeCommit, PreRelease: "not a label"}
	if err := Apply(t.Context(), cfg, Patch); err == nil {
		t.Fatal("expected an error for an invalid pre-release label")
	}
}

func TestRollback(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n",
		"README.md":    "# demo\n",
	})
	if err := Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Patch); err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if err := Rollback(t.Context(), Config{Dir: dir, Mode: ModeApply}); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if manifest := readTestFile(t, filepath.Join(dir, "package.json")); !strings.Contains(manifest, "\"1.0.0\"") {
		t.Fatalf("package.json was not restored:\n%s", manifest)
	}
	if tags := gitRun(t, dir, "tag"); tags != "" {
		t.Fatalf("tags after rollback = %q, want none", tags)
	}
	if subject := gitRun(t, dir, "log", "-1", "--pretty=%s"); subject != "initial commit" {
		t.Fatalf("rollback left commit %q", subject)
	}
	if _, err := os.Stat(filepath.Join(dir, "CHANGELOG.md")); err == nil {
		t.Fatal("rollback left the changelog behind")
	}
}

func TestRollbackCustomCommitMessage(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n",
	})
	cfg := Config{Dir: dir, Mode: ModeCommit, Message: "release {version}"}
	if err := Apply(t.Context(), cfg, Patch); err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if subject := gitRun(t, dir, "log", "-1", "--pretty=%s"); subject != "release 1.0.1" {
		t.Fatalf("commit subject = %q", subject)
	}
	rollbackCfg := cfg
	rollbackCfg.Mode = ModeApply
	if err := Rollback(t.Context(), rollbackCfg); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if tags := gitRun(t, dir, "tag"); tags != "" {
		t.Fatalf("tags after rollback = %q, want none", tags)
	}
}

func TestBumpMultiLanguageUsesTags(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"go.mod":       "module example.com/demo\n",
		"version.go":   "package demo\n\nconst Version = \"1.2.3\"\n",
		"package.json": "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n",
	})
	if err := Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Patch); err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if manifest := readTestFile(t, filepath.Join(dir, "package.json")); !strings.Contains(manifest, "\"version\":\"0.0.1\"") {
		t.Fatalf("package.json not synced from tags:\n%s", manifest)
	}
	if source := readTestFile(t, filepath.Join(dir, "version.go")); !strings.Contains(source, "Version = \"0.0.1\"") {
		t.Fatalf("version.go not synced from tags:\n%s", source)
	}
	if tags := gitRun(t, dir, "tag"); tags != "v0.0.1" {
		t.Fatalf("tags = %q, want v0.0.1", tags)
	}
	if err := Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Patch); err != nil {
		t.Fatalf("second Bump: %v", err)
	}
	if manifest := readTestFile(t, filepath.Join(dir, "package.json")); !strings.Contains(manifest, "\"version\":\"0.0.2\"") {
		t.Fatalf("second bump did not advance from the tag:\n%s", manifest)
	}
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
	if err := Apply(t.Context(), cfg, Patch); err != nil {
		t.Fatalf("Bump: %v", err)
	}
	hooks := readTestFile(t, filepath.Join(dir, "hooks.log"))
	if hooks != "prepost" {
		t.Fatalf("hooks log = %q, want prepost", hooks)
	}
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
	if err := Apply(t.Context(), cfg, Patch); err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "hook-ran")); !os.IsNotExist(err) {
		t.Fatal("dry run executed the pre-bump hook")
	}
}
func TestBumpGoProject(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"go.mod":     "module example.com/demo\n\ngo 1.27\n",
		"version.go": "package demo\n\nconst Version = \"1.0.0\"\n",
	})
	if err := Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Minor); err != nil {
		t.Fatalf("Bump: %v", err)
	}
	source := readTestFile(t, filepath.Join(dir, "version.go"))
	if !strings.Contains(source, "Version = \"1.1.0\"") {
		t.Fatalf("version.go was not bumped:\n%s", source)
	}
	if tags := gitRun(t, dir, "tag"); tags != "v1.1.0" {
		t.Fatalf("tags = %q, want v1.1.0", tags)
	}
}

func TestBumpPushToOrigin(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n",
	})
	remote := t.TempDir()
	gitRun(t, remote, "init", "-q", "--bare")
	gitRun(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")
	gitRun(t, dir, "remote", "add", "origin", remote)

	if err := Apply(t.Context(), Config{Dir: dir, Mode: ModePush}, Patch); err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if tags := gitRun(t, remote, "tag"); tags != "v1.0.1" {
		t.Fatalf("remote tags = %q, want v1.0.1", tags)
	}
	if subject := gitRun(t, remote, "log", "-1", "--pretty=%s"); subject != "chore: bump version to 1.0.1" {
		t.Fatalf("remote subject = %q", subject)
	}
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
	if err := Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Patch); err != nil {
		t.Fatalf("Bump: %v", err)
	}
	manifest := readTestFile(t, filepath.Join(dir, "pyproject.toml"))
	if !strings.Contains(manifest, "version = '1.0.1'") {
		t.Fatalf("pyproject.toml was not bumped:\n%s", manifest)
	}
	source := readTestFile(t, filepath.Join(dir, "src/pkg/__init__.py"))
	if !strings.Contains(source, "__version__ = '1.0.1'") {
		t.Fatalf("__init__.py was not bumped:\n%s", source)
	}
	if tags := gitRun(t, dir, "tag"); tags != "v1.0.1" {
		t.Fatalf("tags = %q, want v1.0.1", tags)
	}
}

func TestBumpRustProject(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"Cargo.toml": `[package]
name = "demo"
version = "1.0.0"
`,
	})
	if err := Apply(t.Context(), Config{Dir: dir, Mode: ModeCommit}, Minor); err != nil {
		t.Fatalf("Bump: %v", err)
	}
	manifest := readTestFile(t, filepath.Join(dir, "Cargo.toml"))
	if !strings.Contains(manifest, "version = \"1.1.0\"") {
		t.Fatalf("Cargo.toml was not bumped:\n%s", manifest)
	}
	if tags := gitRun(t, dir, "tag"); tags != "v1.1.0" {
		t.Fatalf("tags = %q, want v1.1.0", tags)
	}
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
	if err := Apply(t.Context(), cfg, Patch); err != nil {
		t.Fatalf("Bump: %v", err)
	}
	manifest := readTestFile(t, filepath.Join(dir, "pyproject.toml"))
	if !strings.Contains(manifest, "version = '1.0.1'") {
		t.Fatalf("pyproject.toml was not bumped:\n%s", manifest)
	}
	source := readTestFile(t, filepath.Join(dir, "src/pkg/__init__.py"))
	if !strings.Contains(source, "__version__ = '1.0.0'") {
		t.Fatalf("__init__.py should not be bumped:\n%s", source)
	}
}
