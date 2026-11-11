package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeProjectFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestCLIDryRun(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "package.json", "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n")

	cmd := New()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"patch", "--dir", dir})
	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("execute: %v\n%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Summary") {
		t.Fatalf("missing preview summary:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "1.0.1") {
		t.Fatalf("missing new version in summary:\n%s", stdout.String())
	}
	manifest, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	if !strings.Contains(string(manifest), "1.0.0") {
		t.Fatalf("preview modified package.json: %s", manifest)
	}
}

func TestCLIWriteUpdatesFilesOnly(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "package.json", "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n")

	cmd := New()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"patch", "--dir", dir, "--write"})
	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("execute: %v\n%s", err, stderr.String())
	}
	manifest, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	if !strings.Contains(string(manifest), "1.0.1") {
		t.Fatalf("--write did not update package.json: %s", manifest)
	}
	if _, err := os.Stat(filepath.Join(dir, "CHANGELOG.md")); err != nil {
		t.Fatalf("--write did not create a changelog: %v", err)
	}
}

func TestCLIRejectsMultipleActions(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "package.json", "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n")

	cmd := New()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"patch", "--dir", dir, "--write", "--commit"})
	err := cmd.ExecuteContext(t.Context())
	if err == nil || !strings.Contains(err.Error(), "choose one of --write, --commit, or --push") {
		t.Fatalf("error = %v, want an action conflict", err)
	}
	manifest, readErr := os.ReadFile(filepath.Join(dir, "package.json"))
	if readErr != nil {
		t.Fatalf("read package.json: %v", readErr)
	}
	if !strings.Contains(string(manifest), "1.0.0") {
		t.Fatalf("conflicting actions modified package.json: %s", manifest)
	}
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
	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("execute: %v\n%s", err, stderr.String())
	}
	if strings.Contains(stdout.String(), "alpha") {
		t.Fatalf("a .upvert file affected the run:\n%s", stdout.String())
	}
	manifest, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	if !strings.Contains(string(manifest), "1.0.0") {
		t.Fatalf(".upvert file caused a write: %s", manifest)
	}
}

func TestCLIRejectsInvalidPreReleaseLabel(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "package.json", "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n")

	cmd := New()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"patch", "--dir", dir, "--pre", "bad label"})
	err := cmd.ExecuteContext(t.Context())
	if err == nil || !strings.Contains(err.Error(), "invalid pre-release label") {
		t.Fatalf("error = %v, want invalid pre-release label", err)
	}
}

func TestCLIUnknownCommand(t *testing.T) {
	cmd := New()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"bogus"})
	if err := cmd.ExecuteContext(t.Context()); err == nil {
		t.Fatal("expected an error for an unknown command")
	}
}

func TestCLIVersion(t *testing.T) {
	cmd := New()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"version"})
	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "upvert ") {
		t.Fatalf("version output = %q", stdout.String())
	}
}

func TestCLIHelpListsCommands(t *testing.T) {
	cmd := New()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--help"})
	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	for _, command := range []string{"patch", "minor", "major", "rollback"} {
		if !strings.Contains(stdout.String(), command) {
			t.Fatalf("help does not mention %q:\n%s", command, stdout.String())
		}
	}
}

func TestCLIPatchHelpShowsActions(t *testing.T) {
	cmd := New()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"patch", "--help"})
	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	for _, flag := range []string{"--write", "--commit", "--push"} {
		if !strings.Contains(stdout.String(), flag) {
			t.Fatalf("patch help does not mention %q:\n%s", flag, stdout.String())
		}
	}
}

func TestCLIRollbackHelpShowsApply(t *testing.T) {
	cmd := New()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"rollback", "--help"})
	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "--apply") {
		t.Fatalf("rollback help does not mention --apply:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "--commit") {
		t.Fatalf("rollback help still mentions --commit:\n%s", stdout.String())
	}
}
