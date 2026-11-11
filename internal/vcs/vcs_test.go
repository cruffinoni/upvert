package vcs

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitTest runs git in dir and fails the test on error.
func gitTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func TestHighestVersionTag(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	gitTest(t, dir, "init", "-q", "-b", "main")
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "-c", "user.name=Upvert Test", "-c", "user.email=upvert@example.com", "commit", "-q", "-m", "initial commit")
	for _, tag := range []string{"v1.2.0", "v1.10.0", "v1.9.9", "not-a-version", "2.0.0"} {
		gitTest(t, dir, "tag", tag)
	}

	ctx := t.Context()
	git := New(dir)
	highest, err := git.HighestVersionTag(ctx, "v*")
	if err != nil {
		t.Fatalf("HighestVersionTag: %v", err)
	}
	if highest != "v1.10.0" {
		t.Fatalf("HighestVersionTag = %q, want v1.10.0", highest)
	}
	latest, err := git.LatestTag(ctx)
	if err != nil {
		t.Fatalf("LatestTag: %v", err)
	}
	if latest != "1.10.0" {
		t.Fatalf("LatestTag = %q, want 1.10.0", latest)
	}
	fromTags, err := git.VersionFromTags(ctx)
	if err != nil {
		t.Fatalf("VersionFromTags: %v", err)
	}
	if fromTags != "1.10.0" {
		t.Fatalf("VersionFromTags = %q, want 1.10.0", fromTags)
	}
}
