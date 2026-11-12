package bump

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cruffinoni/upvert/internal/semver"
	"github.com/cruffinoni/upvert/internal/ui"
)

const (
	// defaultCommitMessage is used when no --message template is supplied.
	defaultCommitMessage = "chore: bump version to {version}"
	// defaultChangelogFile is the changelog name used when none is supplied.
	defaultChangelogFile = "CHANGELOG.md"
	// versionPlaceholder marks where the new version is inserted.
	versionPlaceholder = "{version}"
)

// Mode selects what a bump or rollback invocation does. The zero value is a
// preview.
type Mode string

const (
	// ModePreview reports what would change without writing anything.
	ModePreview Mode = ""
	// ModeWrite updates files without touching git.
	ModeWrite Mode = "write"
	// ModeCommit updates files, commits, and tags.
	ModeCommit Mode = "commit"
	// ModePush updates files, commits, tags, and pushes to origin.
	ModePush Mode = "push"
	// ModeApply performs a rollback.
	ModeApply Mode = "apply"
)

// String returns the user-facing mode name.
func (m Mode) String() string {
	if m == ModePreview {
		return "preview"
	}
	return string(m)
}

// Config controls a single bump or rollback invocation. The zero value is a
// preview in the current directory that updates CHANGELOG.md when applied.
type Config struct {
	Dir           string
	Mode          Mode
	PreRelease    string
	VersionFile   string
	ChangelogFile string
	NoChangelog   bool
	NoVersionAttr bool
	Message       string
	PreBump       string
	PostBump      string
	Stdout        io.Writer
	Stderr        io.Writer
	// Color enables ANSI colours in the report written to Stdout.
	Color bool
	// LogColor enables ANSI colours in the logger written to Stderr.
	LogColor bool
	// Quiet suppresses progress written to Stderr for non-preview runs.
	Quiet bool
	// Progress receives step feedback for non-preview runs. Nil discards it.
	Progress ui.Progress
}

// normalised fills defaults, resolves the working directory, and validates the
// invocation.
func (c Config) normalised() (Config, error) {
	if c.Dir == "" {
		c.Dir = "."
	}
	root, err := filepath.Abs(c.Dir)
	if err != nil {
		return c, fmt.Errorf("resolve working directory: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return c, fmt.Errorf("open working directory %s: %w", root, err)
	}
	if !info.IsDir() {
		return c, fmt.Errorf("working directory is not a directory: %s", root)
	}
	c.Dir = root
	if c.Message == "" {
		c.Message = defaultCommitMessage
	}
	if c.ChangelogFile == "" {
		c.ChangelogFile = defaultChangelogFile
	}
	if c.Stdout == nil {
		c.Stdout = os.Stdout
	}
	if c.Stderr == nil {
		c.Stderr = os.Stderr
	}
	if c.Quiet || c.Progress == nil {
		c.Progress = ui.NopProgress()
	}
	if c.PreRelease != "" {
		if err := semver.ValidatePreReleaseLabel(c.PreRelease); err != nil {
			return c, err
		}
	}
	if !strings.Contains(c.Message, versionPlaceholder) {
		return c, fmt.Errorf("commit message template must contain %s", versionPlaceholder)
	}
	return c, nil
}
