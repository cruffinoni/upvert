package golang

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGoVersionDirect(t *testing.T) {
	content := "package demo\n\nconst Version = \"1.2.3\"\n"
	found, err := parseDeclaration("version.go", []byte(content))
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", found.Value)
	assert.Equal(t, "Version", found.TargetName)
	assert.False(t, found.Indirect)
}

func TestParseGoVersionIndirectChain(t *testing.T) {
	content := "package demo\n\nvar versionString = \"1.2.3\"\nvar buildVersion = versionString\nconst Version = buildVersion\n"
	found, err := parseDeclaration("version.go", []byte(content))
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", found.Value)
	assert.Equal(t, "versionString", found.TargetName)
	assert.True(t, found.Indirect)
}

func TestParseGoVersionMultipleSemver(t *testing.T) {
	content := "package demo\n\nconst Version = \"1.2.3\"\n\nfunc other() {\n\tconst Version = \"1.2.4\"\n}\n"
	_, err := parseDeclaration("version.go", []byte(content))
	require.Error(t, err)
	assert.True(t, errors.Is(err, errVersionAmbiguous))
}

func TestParseGoVersionUsesOnlySemverDeclaration(t *testing.T) {
	content := "package demo\n\nconst Version = \"dev\"\n\nfunc other() {\n\tconst Version = \"1.2.4\"\n}\n"
	found, err := parseDeclaration("version.go", []byte(content))
	require.NoError(t, err)
	assert.Equal(t, "1.2.4", found.Value)
	assert.Equal(t, "Version", found.TargetName)
}

func TestParseGoVersionRejectsInjectedVariable(t *testing.T) {
	content := "package demo\n\n//go:embed version.txt\nvar buildVersion string\nconst Version = buildVersion\n"
	_, err := parseDeclaration("version.go", []byte(content))
	require.Error(t, err)
	assert.True(t, errors.Is(err, errVersionInjected))
}

func TestParseGoVersionReportsParseFailure(t *testing.T) {
	_, err := parseDeclaration("version.go", []byte("package demo\nconst Version = \n"))
	require.Error(t, err)
	assert.True(t, isParseError(err))
}

func TestParseGoVersionAllowsNormalTrailingComment(t *testing.T) {
	content := "package demo\n\nvar buildVersion = \"1.0.0\" // current version\nconst Version = buildVersion\n"
	found, err := parseDeclaration("version.go", []byte(content))
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", found.Value)
	assert.Equal(t, "buildVersion", found.TargetName)
}

func TestParseGoVersionRejectsDirectTrailingInjectionComment(t *testing.T) {
	content := "package demo\n\nconst Version = \"1.0.0\" // set via -ldflags\n"
	_, err := parseDeclaration("version.go", []byte(content))
	require.Error(t, err)
	assert.True(t, errors.Is(err, errVersionInjected))
}

func TestParseGoVersionRejectsTrailingInjectionComment(t *testing.T) {
	content := "package demo\n\nvar buildVersion = \"1.0.0\" // set via -ldflags\nconst Version = buildVersion\n"
	_, err := parseDeclaration("version.go", []byte(content))
	require.Error(t, err)
	assert.True(t, errors.Is(err, errVersionInjected))
}
