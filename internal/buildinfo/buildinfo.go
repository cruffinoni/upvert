// Package buildinfo exposes metadata injected at build time.
package buildinfo

import (
	"runtime/debug"
	"strings"
)

// Version, Commit, and BuiltAt are set through -ldflags by GoReleaser.
var (
	// Version is the released semantic version.
	Version = "dev"
	// Commit is the source revision used for the release build.
	Commit = "unknown"
	// BuiltAt is the timestamp of the release build.
	BuiltAt = "unknown"
)

// Current returns the injected version, falling back to the module version
// embedded by the Go toolchain for local installs.
func Current() string {
	if version := strings.TrimSpace(Version); version != "" && version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if version := strings.TrimSpace(info.Main.Version); version != "" && version != "(devel)" {
			return version
		}
	}
	return "dev"
}
