package bump

import (
	"context"
	"fmt"
)

// removeTag deletes the local tag and, when origin exists, the remote tag.
func (s *service) removeTag(ctx context.Context, tag string) error {
	if !s.git.TagExists(ctx, tag) {
		s.log.Warnf("Git tag %s not found locally", tag)
		return nil
	}
	s.log.Infof("Removing git tag: %s", tag)
	if err := s.git.DeleteTag(ctx, tag); err != nil {
		return err
	}
	if !s.git.RemoteExists(ctx, "origin") {
		return nil
	}
	s.log.Infof("Removing tag from remote...")
	if err := s.git.DeleteRemoteTag(ctx, tag); err != nil {
		s.log.Warnf("Could not remove tag from remote (may not exist there)")
		return nil
	}
	s.log.Successf("Removed tag %s from remote", tag)
	return nil
}

// reportRolledBackVersion prints the version restored by the reset.
func (s *service) reportRolledBackVersion(ctx context.Context) {
	proj, err := s.detect.Detect(s.root, s.cfg.VersionFile)
	if err != nil {
		return
	}
	s.proj = proj
	version, err := s.currentVersion(ctx)
	if err != nil {
		return
	}
	s.log.Successf("Rolled back to version: %s", version)
}

// rollback reverses the most recent version bump.
func (s *service) rollback(ctx context.Context) error {
	s.log.Infof("Checking for last version bump to rollback...")
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
	s.log.Infof("Found version bump commit for: %s", version)
	if s.cfg.Mode != ModeApply {
		s.log.Infof("Would roll back commit: %s", subject)
		s.log.Infof("Would remove git tag: %s", tag)
		if s.proj != nil {
			s.log.Infof("Would restore the previous version in %s", s.proj.Primary().Path)
		}
		return nil
	}
	if err := s.removeTag(ctx, tag); err != nil {
		return err
	}
	s.log.Infof("Reverting the version bump commit...")
	if err := s.git.ResetHard(ctx, "HEAD~1"); err != nil {
		return err
	}
	s.reportRolledBackVersion(ctx)
	s.log.Warnf("If the commit was already pushed, you may need:")
	s.log.Warnf("  git push origin HEAD --force-with-lease")
	return nil
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
