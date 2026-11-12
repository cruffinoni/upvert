package bump

import (
	"context"
	"fmt"
	"os/exec"
)

// runHook executes a user-provided shell hook below the repository root.
func (s *service) runHook(ctx context.Context, command, label string) error {
	if command == "" {
		return nil
	}
	hook := exec.CommandContext(ctx, "sh", "-c", command)
	hook.Dir = s.root
	hook.Stdout = s.cfg.Stdout
	hook.Stderr = s.cfg.Stderr
	if err := hook.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", label, err)
	}
	return nil
}

// runPreBumpHook runs the pre-bump hook for real runs and skips it during a
// preview.
func (s *service) runPreBumpHook(ctx context.Context, dryRun bool) error {
	if dryRun {
		return nil
	}
	return s.runHook(ctx, s.cfg.PreBump, "pre-bump hook")
}
