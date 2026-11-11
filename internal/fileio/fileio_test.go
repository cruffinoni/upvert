package fileio

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestUpdateReportsChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version.txt")
	if err := os.WriteFile(path, []byte("1.0.0"), 0o644); err != nil {
		t.Fatalf("write version.txt: %v", err)
	}
	changed, err := Update(path, func([]byte) ([]byte, error) {
		return []byte("1.1.0"), nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !changed {
		t.Fatal("expected the file to change")
	}
	changed, err = Update(path, func(data []byte) ([]byte, error) {
		return data, nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if changed {
		t.Fatal("expected no change")
	}
}

func TestReplaceLinesAndQuoteLike(t *testing.T) {
	content := "version = 1.0.0\nother = 2.0.0\n"
	got := ReplaceLines(content, regexp.MustCompile("^version = .*$"), "version = 1.1.0")
	want := "version = 1.1.0\nother = 2.0.0\n"
	if got != want {
		t.Fatalf("ReplaceLines = %q, want %q", got, want)
	}
	if quoted := QuoteLike("\"1.0.0\"", "1.1.0"); quoted != "\"1.1.0\"" {
		t.Fatalf("QuoteLike = %q, want quoted value", quoted)
	}
	if quoted := QuoteLike("1.0.0", "1.1.0"); quoted != "1.1.0" {
		t.Fatalf("QuoteLike = %q, want unquoted value", quoted)
	}
}
