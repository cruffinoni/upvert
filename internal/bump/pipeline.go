package bump

import (
	"context"
	"fmt"
	"os"

	"github.com/cruffinoni/upvert/internal/changelog"
	"github.com/cruffinoni/upvert/internal/ecosystem"
	"github.com/cruffinoni/upvert/internal/fileio"
	"github.com/cruffinoni/upvert/internal/gomod"
	"github.com/cruffinoni/upvert/internal/project"
	"github.com/cruffinoni/upvert/internal/semver"
	"github.com/cruffinoni/upvert/internal/ui"
	"github.com/cruffinoni/upvert/internal/vcs"
)

// service runs the bump pipeline for one invocation.
type service struct {
	cfg    Config
	root   string
	log    *ui.Logger
	git    *vcs.Git
	detect *project.Detector
	proj   *project.Layout
}

// applyNoVersionAttr drops auxiliary in-code declarations from a single-language
// project when version attributes are disabled. The primary version source is
// kept for reading.
func (s *service) applyNoVersionAttr() {
	if !s.cfg.NoVersionAttr || s.proj.UsesTags() {
		return
	}
	for i := range s.proj.Parts {
		files := s.proj.Parts[i].Target.Files
		if len(files) == 0 {
			continue
		}
		kept := []ecosystem.File{files[0]}
		for _, file := range files[1:] {
			if !file.InCode {
				kept = append(kept, file)
			}
		}
		s.proj.Parts[i].Target.Files = kept
	}
}

// hasWritableVersion reports whether any declaration survives the in-code
// filtering.
func (s *service) hasWritableVersion() bool {
	for _, part := range s.proj.Parts {
		for _, file := range part.Target.Files {
			if !file.InCode {
				return true
			}
		}
	}
	return false
}

// writableFiles filters out in-code files when version attributes are disabled.
func (s *service) writableFiles(files []ecosystem.File) []ecosystem.File {
	if !s.cfg.NoVersionAttr {
		return files
	}
	out := make([]ecosystem.File, 0, len(files))
	for _, file := range files {
		if !file.InCode {
			out = append(out, file)
		}
	}
	return out
}

// currentVersion reads the version from the project-appropriate source.
func (s *service) currentVersion(ctx context.Context) (string, error) {
	if s.proj.UsesTags() {
		return s.git.VersionFromTags(ctx)
	}
	if len(s.proj.Parts) == 0 || len(s.proj.Parts[0].Target.Files) == 0 {
		return "", fmt.Errorf("project has no version file")
	}
	part := &s.proj.Parts[0]
	return part.Language.Read(s.root, &part.Target, s.log)
}

// reportVersionUpdates reports the file writes that a real run would perform.
func (s *service) reportVersionUpdates(version string) error {
	if !s.proj.IsMulti() {
		if len(s.proj.Parts) == 0 || len(s.proj.Parts[0].Target.Files) == 0 {
			return nil
		}
		part := s.proj.Parts[0]
		file := part.Target.Files[0]
		if describer, ok := part.Language.(ecosystem.Describer); ok {
			if err := describer.Describe(s.root, file, s.log); err != nil {
				return err
			}
		}
		s.log.Infof("Would update %s: → %s", file.Path, version)
		return nil
	}
	for _, part := range s.proj.Parts {
		for _, file := range part.Target.Files {
			if s.cfg.NoVersionAttr && file.InCode {
				s.log.Infof("Would skip in-code version declaration: %s", file.Path)
				continue
			}
			if describer, ok := part.Language.(ecosystem.Describer); ok {
				if err := describer.Describe(s.root, file, s.log); err != nil {
					return err
				}
			}
			s.log.Infof("Would update %s: → %s", file.Path, version)
		}
	}
	return nil
}

// fileSnapshot stores one file's content and permissions so a failed bump can
// restore every file it touched.
type fileSnapshot struct {
	path string
	data []byte
	mode os.FileMode
}

// snapshotFiles reads every file that the next write may modify.
func (s *service) snapshotFiles(files []ecosystem.File) ([]fileSnapshot, error) {
	snapshots := make([]fileSnapshot, 0, len(files))
	for _, file := range files {
		path := fileio.Resolve(s.root, file.Path)
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", file.Path, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file.Path, err)
		}
		snapshots = append(snapshots, fileSnapshot{
			path: path,
			data: data,
			mode: info.Mode().Perm(),
		})
	}
	return snapshots, nil
}

// restoreFiles writes every snapshot back and reports the first failure.
func restoreFiles(snapshots []fileSnapshot) error {
	var firstErr error
	for _, snapshot := range snapshots {
		if err := fileio.WriteAtomic(snapshot.path, snapshot.data, snapshot.mode); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// syncMultiFiles rewrites every version declaration in a multi-language
// repository, restoring all files when a later write fails.
func (s *service) syncMultiFiles(version string) error {
	var writable []ecosystem.File
	for i := range s.proj.Parts {
		writable = append(writable, s.writableFiles(s.proj.Parts[i].Target.Files)...)
	}
	snapshots, err := s.snapshotFiles(writable)
	if err != nil {
		return err
	}
	for i := range s.proj.Parts {
		part := &s.proj.Parts[i]
		files := s.writableFiles(part.Target.Files)
		if len(files) == 0 {
			continue
		}
		target := part.Target
		target.Files = files
		if _, err := part.Language.Write(s.root, &target, version, s.log); err != nil {
			if restoreErr := restoreFiles(snapshots); restoreErr != nil {
				return fmt.Errorf("%w (restore failed: %w)", err, restoreErr)
			}
			return err
		}
	}
	for _, file := range s.proj.Files() {
		s.log.Successf("Synchronized %s: → %s", file.Path, version)
	}
	return nil
}

// writeSingleVersion updates the version file of a single-language project and
// verifies the result, restoring every written file on mismatch.
func (s *service) writeSingleVersion(ctx context.Context, version string) error {
	if len(s.proj.Parts) == 0 || len(s.proj.Parts[0].Target.Files) == 0 {
		return fmt.Errorf("project has no version file")
	}
	part := &s.proj.Parts[0]
	primary := part.Target.Files[0]
	if s.cfg.NoVersionAttr && primary.InCode {
		return fmt.Errorf("no writable version declaration in %s", primary.Path)
	}
	snapshots, err := s.snapshotFiles(part.Target.Files)
	if err != nil {
		return err
	}
	changed, err := part.Language.Write(s.root, &part.Target, version, s.log)
	if err != nil {
		if restoreErr := restoreFiles(snapshots); restoreErr != nil {
			return fmt.Errorf("%w (restore failed: %w)", err, restoreErr)
		}
		return err
	}
	updated, verifyErr := part.Language.Read(s.root, &part.Target, s.log)
	if verifyErr != nil || updated != version {
		if restoreErr := restoreFiles(snapshots); restoreErr != nil {
			if verifyErr != nil {
				return fmt.Errorf("verify version update: %w (restore failed: %w)", verifyErr, restoreErr)
			}
			return fmt.Errorf("restore files after failed version update: %w", restoreErr)
		}
		if verifyErr != nil {
			return fmt.Errorf("verify version update: %w", verifyErr)
		}
		return fmt.Errorf("version update verification failed (expected %s, got %s)", version, updated)
	}
	for _, file := range part.Target.Files[1:] {
		if file.InCode {
			s.log.Successf("Updated in-code version in %s: → %s", file.Path, version)
		}
	}
	if changed {
		s.log.Successf("Updated %s: → %s", primary.Path, version)
	} else {
		s.log.Infof("No static version declaration in %s; left unchanged", primary.Path)
	}
	return nil
}

// updateVersionFiles writes every version declaration, or reports the writes a
// dry run would perform.
func (s *service) updateVersionFiles(ctx context.Context, version string, dryRun bool) error {
	if dryRun {
		return s.reportVersionUpdates(version)
	}
	if s.proj.IsMulti() {
		return s.syncMultiFiles(version)
	}
	if s.proj.UsesTags() {
		return nil
	}
	return s.writeSingleVersion(ctx, version)
}

// updateChangelog prepends the release entry unless the changelog is disabled.
func (s *service) updateChangelog(version string, dryRun bool) error {
	if s.cfg.NoChangelog {
		return nil
	}
	path := fileio.Resolve(s.root, s.cfg.ChangelogFile)
	return changelog.Update(path, s.cfg.ChangelogFile, version, dryRun, s.log)
}

// bump runs the full bump pipeline for the configured mode.
func (s *service) run(ctx context.Context, kind semver.Kind) error {
	preview := s.cfg.Mode == ModePreview
	proj, err := s.detect.Detect(s.root, s.cfg.VersionFile)
	if err != nil {
		return err
	}
	s.proj = proj
	if proj.UsesTags() && s.cfg.Mode == ModeWrite {
		kind := "tag-sourced"
		if proj.IsMulti() {
			kind = "multi-language"
		}
		return fmt.Errorf("%s projects require a tag; use --commit or --push", kind)
	}
	s.applyNoVersionAttr()
	if !s.proj.UsesTags() && s.cfg.NoVersionAttr && !s.hasWritableVersion() {
		return fmt.Errorf("--no-version-attr leaves no writable version declaration")
	}
	current, err := s.currentVersion(ctx)
	if err != nil {
		return err
	}
	s.log.Infof("Current version: %s", current)
	s.log.Infof("Project type:   %s", proj.Kind)
	next, err := semver.Next(current, kind, s.cfg.PreRelease)
	if err != nil {
		return err
	}
	s.log.Infof("New version:     %s", next)
	latest, err := s.git.LatestTag(ctx)
	if err != nil {
		return err
	}
	if latest != "" && latest != current {
		if preview {
			s.log.Warnf("Current version (%s) differs from latest tag (%s)", current, latest)
		} else {
			return fmt.Errorf("current version (%s) differs from latest tag (%s); align them before bumping", current, latest)
		}
	}
	if err := s.ensureTagAvailable(ctx, next); err != nil {
		return err
	}
	if s.cfg.Mode == ModeCommit || s.cfg.Mode == ModePush {
		if err := s.validateEnvironment(ctx, next); err != nil {
			return err
		}
	}
	if err := s.runPreBumpHook(ctx, preview); err != nil {
		return err
	}
	if err := s.updateVersionFiles(ctx, next, preview); err != nil {
		return err
	}
	if err := s.updateChangelog(next, preview); err != nil {
		return err
	}
	if !proj.IsMulti() {
		if err := gomod.Tidy(ctx, gomod.TidyOptions{
			Root:   s.root,
			DryRun: preview,
			Stdout: s.cfg.Stdout,
			Stderr: s.cfg.Stderr,
			Log:    s.log,
		}); err != nil {
			return err
		}
	}
	if preview {
		s.printSummary(current, next, kind, latest)
		s.log.Infof("Preview completed. Use --write, --commit, or --push to apply.")
		return nil
	}
	if err := gomod.ValidateModulePath(s.root, next, s.log); err != nil {
		return err
	}
	if s.cfg.Mode == ModeWrite {
		if err := s.runHook(ctx, s.cfg.PostBump, "post-bump hook"); err != nil {
			return err
		}
		s.log.Successf("Version written: %s → %s", current, next)
		return nil
	}
	if err := s.commit(ctx, next); err != nil {
		return err
	}
	if err := s.createTag(ctx, next); err != nil {
		return err
	}
	if s.cfg.Mode == ModePush {
		if err := s.push(ctx, next); err != nil {
			return err
		}
	}
	if err := s.runHook(ctx, s.cfg.PostBump, "post-bump hook"); err != nil {
		return err
	}
	s.log.Successf("Version bumped: %s → %s", current, next)
	return nil
}
