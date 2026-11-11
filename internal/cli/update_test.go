package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateFlagsRegistered(t *testing.T) {
	cmd := newUpdateCommand()
	for _, name := range []string{"version", "bin-dir", "no-path-update", "quiet"} {
		assert.NotNil(t, cmd.Flags().Lookup(name), "flag --%s is not registered", name)
	}
}

func TestUpdateInvalidVersion(t *testing.T) {
	cmd := New()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"update", "--version=not-a-version"})
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid release version")
	assert.Empty(t, stdout.String())
}

func TestUpdateRejectsArguments(t *testing.T) {
	cmd := New()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"update", "extra"})
	assert.Error(t, cmd.ExecuteContext(t.Context()))
}
