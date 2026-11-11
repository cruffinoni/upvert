package toml

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdatePreservesFormatting(t *testing.T) {
	initial := []byte("# head\n[project]\nname = 'demo'\nversion = '1.0.0' # keep\n\n[tool.poetry.dependencies]\nversion = '2.0.0'\n")
	want := "# head\n[project]\nname = 'demo'\nversion = '1.1.0' # keep\n\n[tool.poetry.dependencies]\nversion = '2.0.0'\n"
	updated, err := Update(initial, [][]string{{"project"}}, "1.1.0")
	require.NoError(t, err)
	assert.Equal(t, want, string(updated))
}

func TestUpdateMultipleSections(t *testing.T) {
	initial := []byte("[project]\nversion = \"1.0.0\"\n\n[tool.poetry]\nversion = \"1.0.0\"\n")
	want := "[project]\nversion = \"1.1.0\"\n\n[tool.poetry]\nversion = \"1.1.0\"\n"
	updated, err := Update(initial, [][]string{{"project"}, {"tool", "poetry"}}, "1.1.0")
	require.NoError(t, err)
	assert.Equal(t, want, string(updated))
}

func TestDecodeAndQueries(t *testing.T) {
	document, err := Decode("[project]\nversion = \"1.2.3\"\n")
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", String(document, "project", "version"))
	assert.True(t, TableExists(document, "project"))
}
