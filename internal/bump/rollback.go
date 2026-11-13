package bump

import (
	"context"
	"fmt"

	"github.com/cruffinoni/upvert/internal/report"
)

// removeTag deletes the local tag and, when origin exists, the remote tag. It
// reports whether the local tag existed.
func (s *service) removeTag(ctx context.Context, tag string) (bool, error) {
	if !s.git.TagExists(ctx, tag) {
		return false, nil
	}
	if err := s.git.DeleteTag(ctx, tag); err != nil {
		return false, err
	}
	if s.git.RemoteExists(ctx, "origin") {
		if err := s.git.DeleteRemoteTag(ctx, tag); err != nil {
			s.log.Warnf("Could not remove tag %s from remote (may not exist there)", tag)
		}
	}
	return true, nil
}

// rolledBackVersion returns the version restored by the reset, or an empty
// string when it cannot be detected.
func (s *service) rolledBackVersion(ctx context.Context) string {
	proj, err := s.detect.Detect(s.root, s.cfg.VersionFile)
	if err != nil {
		return ""
	}
	s.proj = proj
	version, err := s.currentVersion(ctx)
	if err != nil {
		return ""
	}
	return version
}

// rollback reverses the most recent version bump.
func (s *service) rollback(ctx context.Context) error {
	if !s.git.IsRepo(ctx) {
		return fmt.Errorf("not in a git repository")
	}
	subject, err := s.git.LogSubject(ctx)
	if err != nil {
		return err
	}
	pattern, err := s.messagePattern()
	if err != nil {
		return err
	}
	match := pattern.FindStringSubmatch(subject)
	if match == nil {
		return fmt.Errorf("last commit does not appear to be a version bump: %q", subject)
	}
	version := match[1]
	tag := "v" + version
	if proj, err := s.detect.Detect(s.root, s.cfg.VersionFile); err == nil {
		s.proj = proj
	}
	s.rep = &report.Report{
		Command: "rollback",
		Mode:    s.cfg.Mode.String(),
		Source:  s.sourceLabel(),
	}
	s.rep.Changes = append(s.rep.Changes, report.Change{Path: "work tree", Detail: "reset to HEAD~1"})
	if s.proj != nil {
		if primary := s.proj.Primary(); primary.Path != "" {
			s.rep.Changes = append(s.rep.Changes, report.Change{Path: primary.Path, Detail: "restore previous version"})
		}
	}
	if s.cfg.Mode != ModeApply {
		s.rep.Release = []report.Step{{Label: "tag", Value: "remove " + tag}}
		s.rep.Footer = "Nothing was changed. Re-run with --apply to apply."
		return s.renderReport()
	}
	var removed bool
	if err := s.runStep("removing tag", tag, func() error {
		var stepErr error
		removed, stepErr = s.removeTag(ctx, tag)
		return stepErr
	}); err != nil {
		return err
	}
	if err := s.runStep("resetting work tree", "HEAD~1", func() error {
		return s.git.ResetHard(ctx, "HEAD~1")
	}); err != nil {
		return err
	}
	restored := s.rolledBackVersion(ctx)
	tagStep := "removed " + tag
	if !removed {
		tagStep = "not found (" + tag + ")"
	}
	s.rep.Release = []report.Step{{Label: "tag", Value: tagStep}}
	footer := "Rolled back."
	if restored != "" {
		footer = "Rolled back to " + restored + "."
	}
	s.rep.Footer = footer + "\nIf the commit was already pushed, run: git push origin HEAD --force-with-lease"
	return s.renderReport()
}

// Rollback reverses the most recent version bump commit, removing its tag and
// resetting the work tree to the parent commit.
func Rollback(ctx context.Context, cfg Config) error {
	normalised, err := cfg.normalised()
	if err != nil {
		return err
	}
	switch normalised.Mode {
	case ModePreview, ModeApply:
	default:
		return fmt.Errorf("rollback supports --apply, not %q", normalised.Mode)
	}
	return newService(normalised).rollback(ctx)
}
