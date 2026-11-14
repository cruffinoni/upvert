package bump

import "context"

// validateStageable rejects ignored files before the pipeline writes anything,
// because git would refuse to stage them after the version files changed.
func (s *service) validateStageable(ctx context.Context) error {
	files := s.filesToCommit()
	if !s.cfg.NoChangelog {
		// The changelog may not exist yet, but the pipeline creates it.
		files = append(files, s.cfg.ChangelogFile)
	}
	for _, file := range files {
		if err := s.git.EnsureStageable(ctx, file); err != nil {
			return err
		}
	}
	return nil
}
