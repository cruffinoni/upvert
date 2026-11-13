package bump

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/cruffinoni/upvert/internal/fileio"
)

// ensureTagAvailable rejects a bump whose tag already exists locally.
func (s *service) ensureTagAvailable(ctx context.Context, version string) error {
	if s.git.TagExists(ctx, "v"+version) {
		return fmt.Errorf("git tag already exists locally: v%s", version)
	}
	return nil
}

// filesToCommit lists the files a bump commit should stage.
func (s *service) filesToCommit() []string {
	var files []string
	for _, file := range s.proj.Files() {
		files = append(files, file.Path)
	}
	if !s.cfg.NoChangelog && fileio.Exists(fileio.Resolve(s.root, s.cfg.ChangelogFile)) {
		files = append(files, s.cfg.ChangelogFile)
	}
	return files
}

// commitMessage renders the configured commit message for version.
func (s *service) commitMessage(version string) string {
	return strings.ReplaceAll(s.cfg.Message, versionPlaceholder, version)
}

// commit stages the files produced by the bump and creates the commit.
func (s *service) commit(ctx context.Context, version string) error {
	message := s.commitMessage(version)
	files := s.filesToCommit()
	if len(files) == 0 {
		return fmt.Errorf("nothing to commit")
	}
	if err := s.git.Add(ctx, files...); err != nil {
		return err
	}
	return s.git.Commit(ctx, message)
}

// createTag creates the annotated release tag.
func (s *service) createTag(ctx context.Context, version string) error {
	tag := "v" + version
	return s.git.CreateTag(ctx, tag, "Release version "+version)
}

// push publishes the bump commit and its tag to origin.
func (s *service) push(ctx context.Context, version string) error {
	if err := s.runStep("pushing commit", "origin HEAD", func() error {
		return s.git.Push(ctx, "HEAD")
	}); err != nil {
		return err
	}
	return s.runStep("pushing tag", "origin v"+version, func() error {
		return s.git.Push(ctx, "v"+version)
	})
}

// validateEnvironment blocks a bump that would produce an invalid repository
// state before any file is modified.
func (s *service) validateEnvironment(ctx context.Context) error {
	if !s.git.IsRepo(ctx) {
		return fmt.Errorf("not in a git repository")
	}
	if s.cfg.Mode == ModePush && !s.git.RemoteExists(ctx, "origin") {
		return fmt.Errorf("no origin remote configured; cannot push")
	}
	if !s.proj.UsesTags() {
		primary := s.proj.Primary()
		if !fileio.Exists(fileio.Resolve(s.root, primary.Path)) {
			return fmt.Errorf("version file not found: %s", primary.Path)
		}
	}
	status, err := s.git.Status(ctx)
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("working directory has uncommitted changes; commit or stash them first")
	}
	return nil
}

// messagePattern derives the rollback pattern from the commit message
// template so custom messages remain reversible.
func (s *service) messagePattern() (*regexp.Regexp, error) {
	index := strings.Index(s.cfg.Message, versionPlaceholder)
	if index < 0 {
		return nil, fmt.Errorf("commit message template must contain %s", versionPlaceholder)
	}
	prefix := regexp.QuoteMeta(s.cfg.Message[:index])
	suffix := regexp.QuoteMeta(s.cfg.Message[index+len(versionPlaceholder):])
	pattern, err := regexp.Compile("^" + prefix + "(.+)" + suffix + "$")
	if err != nil {
		return nil, fmt.Errorf("compile rollback pattern: %w", err)
	}
	return pattern, nil
}
