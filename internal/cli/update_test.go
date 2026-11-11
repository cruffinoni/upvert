package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestUpdateFlagsRegistered(t *testing.T) {
	cmd := newUpdateCommand()
	for _, name := range []string{"version", "bin-dir", "no-path-update", "quiet"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s is not registered", name)
		}
	}
}

func TestUpdateInvalidVersion(t *testing.T) {
	cmd := New()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"update", "--version=not-a-version"})
	err := cmd.ExecuteContext(t.Context())
	if err == nil || !strings.Contains(err.Error(), "invalid release version") {
		t.Fatalf("error = %v, want an invalid release version", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestUpdateRejectsArguments(t *testing.T) {
	cmd := New()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"update", "extra"})
	if err := cmd.ExecuteContext(t.Context()); err == nil {
		t.Fatal("expected an error for positional arguments")
	}
}
