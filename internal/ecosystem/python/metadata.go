package python

import (
	"strings"

	"gopkg.in/ini.v1"

	"github.com/cruffinoni/upvert/internal/toml"
)

// pyProjectVersion returns the version declared by pyproject.toml.
func pyProjectVersion(content string) string {
	document, err := toml.Decode(content)
	if err != nil {
		return ""
	}
	if version := toml.String(document, "project", "version"); version != "" {
		return version
	}
	return toml.String(document, "tool", "poetry", "version")
}

// setupCfgVersion returns the version declared in [metadata] of setup.cfg.
func setupCfgVersion(content string) string {
	file, err := ini.LoadSources(ini.LoadOptions{
		AllowBooleanKeys:           true,
		AllowPythonMultilineValues: true,
		SpaceBeforeInlineComment:   true,
	}, []byte(content))
	if err != nil {
		return ""
	}
	key, err := file.Section("metadata").GetKey("version")
	if err != nil {
		return ""
	}
	return strings.Trim(strings.TrimSpace(key.String()), `"'`)
}
