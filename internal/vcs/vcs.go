// Package vcs wraps the git command-line operations used by the bump
// pipeline.
package vcs

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	msemver "github.com/Masterminds/semver/v3"

	"github.com/dayrion/upvert/internal/semver"
)

// Git runs git commands against one repository root.
type Git struct {
	root string
}

// New returns a Git bound to the repository at root.
func New(root string) *Git {
	return &Git{root: root}
}

// run executes git with args below the repository root and returns its trimmed
// standard output.
func (g *Git) run(ctx context.Context, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", g.root}, args...)...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			return strings.TrimSpace(stdout.String()), fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		return strings.TrimSpace(stdout.String()), fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), detail, err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// ok reports whether git exits successfully with args below the repository
// root.
func (g *Git) ok(ctx context.Context, args ...string) bool {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", g.root}, args...)...)
	return command.Run() == nil
}

// IsRepo reports whether the repository root is inside a git work tree.
func (g *Git) IsRepo(ctx context.Context) bool {
	output, err := g.run(ctx, "rev-parse", "--is-inside-work-tree")
	return err == nil && output == "true"
}

// Status returns the porcelain status excluding untracked files.
func (g *Git) Status(ctx context.Context) (string, error) {
	return g.run(ctx, "status", "--porcelain", "--untracked-files=no")
}

// LogSubject returns the subject line of the most recent commit.
func (g *Git) LogSubject(ctx context.Context) (string, error) {
	return g.run(ctx, "log", "-1", "--pretty=format:%s")
}

// TagExists reports whether a local tag exists.
func (g *Git) TagExists(ctx context.Context, tag string) bool {
	return g.ok(ctx, "rev-parse", "-q", "--verify", "refs/tags/"+tag)
}

// RemoteExists reports whether the named git remote is configured.
func (g *Git) RemoteExists(ctx context.Context, name string) bool {
	return g.ok(ctx, "remote", "get-url", name)
}

// HighestVersionTag returns the highest semantic-version tag matching the
// optional git tag pattern, or an empty string when there is none.
func (g *Git) HighestVersionTag(ctx context.Context, pattern ...string) (string, error) {
	args := append([]string{"tag", "--list"}, pattern...)
	output, err := g.run(ctx, args...)
	if err != nil {
		return "", err
	}
	var (
		best    *msemver.Version
		bestTag string
	)
	for _, line := range strings.Split(output, "\n") {
		tag := strings.TrimSpace(line)
		version, err := semver.Parse(strings.TrimPrefix(tag, "v"))
		if err != nil {
			continue
		}
		if best == nil || version.GreaterThan(best) {
			best = version
			bestTag = tag
		}
	}
	return bestTag, nil
}

// LatestTag returns the highest v-prefixed version tag in the repository, or
// an empty string when there is none.
func (g *Git) LatestTag(ctx context.Context) (string, error) {
	tag, err := g.HighestVersionTag(ctx, "v*")
	if err != nil {
		return "", nil
	}
	return strings.TrimPrefix(tag, "v"), nil
}

// VersionFromTags returns the version encoded by the highest v-prefixed tag,
// or 0.0.0 when the repository has no such tag.
func (g *Git) VersionFromTags(ctx context.Context) (string, error) {
	tag, err := g.HighestVersionTag(ctx, "v*")
	if err != nil || tag == "" {
		return "0.0.0", nil
	}
	return strings.TrimPrefix(tag, "v"), nil
}

// Add stages files for the next commit.
func (g *Git) Add(ctx context.Context, files ...string) error {
	_, err := g.run(ctx, append([]string{"add", "--"}, files...)...)
	return err
}

// Commit creates a commit with message.
func (g *Git) Commit(ctx context.Context, message string) error {
	_, err := g.run(ctx, "commit", "-m", message)
	return err
}

// CreateTag creates an annotated tag.
func (g *Git) CreateTag(ctx context.Context, tag, message string) error {
	_, err := g.run(ctx, "tag", "-a", tag, "-m", message)
	return err
}

// DeleteTag deletes the local tag.
func (g *Git) DeleteTag(ctx context.Context, tag string) error {
	_, err := g.run(ctx, "tag", "-d", tag)
	return err
}

// DeleteRemoteTag deletes the tag from origin.
func (g *Git) DeleteRemoteTag(ctx context.Context, tag string) error {
	_, err := g.run(ctx, "push", "origin", ":refs/tags/"+tag)
	return err
}

// Push pushes ref to origin.
func (g *Git) Push(ctx context.Context, ref string) error {
	_, err := g.run(ctx, "push", "origin", ref)
	return err
}

// ResetHard resets the work tree and HEAD to ref.
func (g *Git) ResetHard(ctx context.Context, ref string) error {
	_, err := g.run(ctx, "reset", "--hard", ref)
	return err
}
