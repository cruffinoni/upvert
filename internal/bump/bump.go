// Package bump implements semantic version bumping for Go, Node.js, Python,
// Rust, and multi-language repositories.
package bump

import (
	"context"
	"fmt"

	"github.com/cruffinoni/upvert/internal/project"
	"github.com/cruffinoni/upvert/internal/semver"
	"github.com/cruffinoni/upvert/internal/ui"
	"github.com/cruffinoni/upvert/internal/vcs"
)

// Kind selects which component of a version is incremented.
type Kind = semver.Kind

// Kinds supported by the CLI.
const (
	Patch = semver.Patch
	Minor = semver.Minor
	Major = semver.Major
)

// newService builds the pipeline for one invocation.
func newService(cfg Config) *service {
	log := ui.New(cfg.Stderr, cfg.LogColor)
	return &service{
		cfg:    cfg,
		root:   cfg.Dir,
		log:    log,
		git:    vcs.New(cfg.Dir),
		detect: project.NewDetector(log),
	}
}

// Apply applies the requested version bump according to cfg.
func Apply(ctx context.Context, cfg Config, kind Kind) error {
	normalised, err := cfg.normalised()
	if err != nil {
		return err
	}
	if !kind.Valid() {
		return fmt.Errorf("invalid bump kind %q", kind)
	}
	switch normalised.Mode {
	case ModePreview, ModeWrite, ModeCommit, ModePush:
	default:
		return fmt.Errorf("invalid bump mode %q", normalised.Mode)
	}
	return newService(normalised).run(ctx, kind)
}
