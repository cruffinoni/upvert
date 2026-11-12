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

// operation selects the version transition applied by one invocation.
type operation int

const (
	// opBump advances the numeric core and optionally starts a prerelease.
	opBump operation = iota
	// opContinuePreRelease advances the current prerelease on its core.
	opContinuePreRelease
	// opPromotePreRelease promotes the current prerelease to stable.
	opPromotePreRelease
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

// validateMode checks that mode is one of the file or git actions.
func validateMode(mode Mode) error {
	switch mode {
	case ModePreview, ModeWrite, ModeCommit, ModePush:
		return nil
	default:
		return fmt.Errorf("invalid bump mode %q", mode)
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
	if err := validateMode(normalised.Mode); err != nil {
		return err
	}
	svc := newService(normalised)
	svc.operation = opBump
	svc.kind = kind
	return svc.run(ctx)
}

// ApplyPre continues or relabels the current prerelease according to cfg.
func ApplyPre(ctx context.Context, cfg Config, label string) error {
	normalised, err := cfg.normalised()
	if err != nil {
		return err
	}
	if err := semver.ValidatePreReleaseLabel(label); err != nil {
		return err
	}
	if err := validateMode(normalised.Mode); err != nil {
		return err
	}
	svc := newService(normalised)
	svc.operation = opContinuePreRelease
	svc.preLabel = label
	return svc.run(ctx)
}

// ApplyRelease promotes the current prerelease to its stable version.
func ApplyRelease(ctx context.Context, cfg Config) error {
	normalised, err := cfg.normalised()
	if err != nil {
		return err
	}
	if err := validateMode(normalised.Mode); err != nil {
		return err
	}
	svc := newService(normalised)
	svc.operation = opPromotePreRelease
	return svc.run(ctx)
}
