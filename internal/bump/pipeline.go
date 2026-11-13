package bump

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cruffinoni/upvert/internal/changelog"
	"github.com/cruffinoni/upvert/internal/ecosystem"
	"github.com/cruffinoni/upvert/internal/fileio"
	"github.com/cruffinoni/upvert/internal/gomod"
	"github.com/cruffinoni/upvert/internal/project"
	"github.com/cruffinoni/upvert/internal/report"
	"github.com/cruffinoni/upvert/internal/semver"
	"github.com/cruffinoni/upvert/internal/ui"
	"github.com/cruffinoni/upvert/internal/vcs"
)

// service runs the bump pipeline for one invocation.
type service struct {
	cfg       Config
	root      string
	log       *ui.Logger
	progress  ui.Progress
	git       *vcs.Git
	detect    *project.Detector
	proj      *project.Layout
	rep       *report.Report
	operation operation
	kind      semver.Kind
	preLabel  string
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
	return nil
}

// writeSingleVersion updates the version file of a single-language project and
// verifies the result, restoring every written file on mismatch. It reports
// whether any file changed.
func (s *service) writeSingleVersion(version string) (bool, error) {
	if len(s.proj.Parts) == 0 || len(s.proj.Parts[0].Target.Files) == 0 {
		return false, fmt.Errorf("project has no version file")
	}
	part := &s.proj.Parts[0]
	primary := part.Target.Files[0]
	if s.cfg.NoVersionAttr && primary.InCode {
		return false, fmt.Errorf("no writable version declaration in %s", primary.Path)
	}
	snapshots, err := s.snapshotFiles(part.Target.Files)
	if err != nil {
		return false, err
	}
	changed, err := part.Language.Write(s.root, &part.Target, version, s.log)
	if err != nil {
		if restoreErr := restoreFiles(snapshots); restoreErr != nil {
			return false, fmt.Errorf("%w (restore failed: %w)", err, restoreErr)
		}
		return false, err
	}
	updated, verifyErr := part.Language.Read(s.root, &part.Target, s.log)
	if verifyErr != nil || updated != version {
		if restoreErr := restoreFiles(snapshots); restoreErr != nil {
			if verifyErr != nil {
				return false, fmt.Errorf("verify version update: %w (restore failed: %w)", verifyErr, restoreErr)
			}
			return false, fmt.Errorf("restore files after failed version update: %w", restoreErr)
		}
		if verifyErr != nil {
			return false, fmt.Errorf("verify version update: %w", verifyErr)
		}
		return false, fmt.Errorf("version update verification failed (expected %s, got %s)", version, updated)
	}
	return changed, nil
}

// updateVersionFiles writes every version declaration, or reports the writes a
// dry run would perform. It reports whether any version declaration changed.
func (s *service) updateVersionFiles(version string, dryRun bool) (bool, error) {
	if dryRun {
		return true, nil
	}
	if s.proj.IsMulti() {
		return true, s.syncMultiFiles(version)
	}
	if s.proj.UsesTags() {
		return false, nil
	}
	return s.writeSingleVersion(version)
}

// updateChangelog prepends the release entry unless the changelog is disabled.
func (s *service) updateChangelog(version string, dryRun bool) error {
	if s.cfg.NoChangelog {
		return nil
	}
	path := fileio.Resolve(s.root, s.cfg.ChangelogFile)
	return changelog.Update(path, s.cfg.ChangelogFile, version, dryRun)
}

// versionChanges lists the version files a bump writes.
func (s *service) versionChanges() []report.Change {
	if s.proj == nil {
		return nil
	}
	var changes []report.Change
	for _, part := range s.proj.Parts {
		for _, file := range s.writableFiles(part.Target.Files) {
			changes = append(changes, report.Change{Path: file.Path})
		}
	}
	return changes
}

// writeVersionFiles applies the version declarations and reports the step for
// real runs.
func (s *service) writeVersionFiles(version string, preview bool) (bool, error) {
	if preview {
		return s.updateVersionFiles(version, true)
	}
	changes := s.versionChanges()
	if len(changes) == 0 {
		return s.updateVersionFiles(version, false)
	}
	paths := make([]string, 0, len(changes))
	for _, change := range changes {
		paths = append(paths, change.Path)
	}
	var changed bool
	err := s.runStep("writing version files", strings.Join(paths, ", "), func() error {
		var stepErr error
		changed, stepErr = s.updateVersionFiles(version, false)
		return stepErr
	})
	return changed, err
}

// writeChangelog updates the changelog and reports the step for real runs.
func (s *service) writeChangelog(version string, preview bool) error {
	if preview {
		return s.updateChangelog(version, true)
	}
	return s.runStep("updating changelog", s.cfg.ChangelogFile, func() error {
		return s.updateChangelog(version, false)
	})
}

// sourceLabel returns the version source shown at the top of the report.
func (s *service) sourceLabel() string {
	if s.proj == nil {
		return ""
	}
	if s.proj.UsesTags() {
		return "git tags"
	}
	return s.proj.Primary().Path
}

// addHookStep records one hook result for the report.
func (s *service) addHookStep(label, value string) {
	if s.rep == nil {
		return
	}
	s.rep.Hooks = append(s.rep.Hooks, report.Step{Label: label, Value: value})
}

// addNote records a diagnostic line for the report.
func (s *service) addNote(format string, args ...any) {
	if s.rep == nil {
		return
	}
	s.rep.Notes = append(s.rep.Notes, fmt.Sprintf(format, args...))
}

// releaseSteps describes the tag and commit, and the push when requested.
func (s *service) releaseSteps(version string, push bool) []report.Step {
	steps := []report.Step{
		{Label: "tag", Value: "v" + version},
		{Label: "commit", Value: strings.ReplaceAll(s.cfg.Message, versionPlaceholder, version)},
	}
	if push {
		steps = append(steps, report.Step{Label: "push", Value: "origin"})
	}
	return steps
}

// previewFooter describes the next actions available after a preview.
func (s *service) previewFooter() string {
	if s.proj != nil && s.proj.UsesTags() {
		return "Nothing was written. Re-run with --commit or --push."
	}
	return "Nothing was written. Re-run with --write, --commit, or --push."
}

// successFooter closes a completed run.
func (s *service) successFooter(version string) string {
	tag := "v" + version
	switch s.cfg.Mode {
	case ModePush:
		return "Committed, tagged, and pushed " + tag + "."
	case ModeCommit:
		return "Committed and tagged " + tag + ".\nPush with: git push origin HEAD && git push origin " + tag
	default:
		return "Files updated. Git was left untouched."
	}
}

// renderReport writes the global view for the completed pipeline.
func (s *service) renderReport() error {
	if s.rep == nil {
		return nil
	}
	return report.Render(s.cfg.Stdout, *s.rep, report.Options{
		Color: s.cfg.Color,
		ASCII: report.ASCII(),
	})
}

// commandLabel names the resolution in the report header.
func (s *service) commandLabel() string {
	switch s.operation {
	case opContinuePreRelease:
		return "pre"
	case opPromotePreRelease:
		return "pre --release"
	default:
		return string(s.kind)
	}
}

// nextVersion resolves the version transition requested by the invocation.
func (s *service) nextVersion(current string) (string, error) {
	switch s.operation {
	case opContinuePreRelease:
		return semver.ContinuePreRelease(current, s.preLabel)
	case opPromotePreRelease:
		return semver.PromotePreRelease(current)
	default:
		return semver.Next(current, s.kind, s.cfg.PreRelease)
	}
}

// run executes the full bump pipeline for the configured mode.
func (s *service) run(ctx context.Context) error {
	preview := s.cfg.Mode == ModePreview
	proj, err := s.detect.Detect(s.root, s.cfg.VersionFile)
	if err != nil {
		return err
	}
	s.proj = proj
	if proj.UsesTags() && s.cfg.Mode == ModeWrite {
		label := "tag-sourced"
		if proj.IsMulti() {
			label = "multi-language"
		}
		return fmt.Errorf("%s projects require a tag; use --commit or --push", label)
	}
	s.applyNoVersionAttr()
	if !s.proj.UsesTags() && s.cfg.NoVersionAttr && !s.hasWritableVersion() {
		return fmt.Errorf("--no-version-attr leaves no writable version declaration")
	}
	current, err := s.currentVersion(ctx)
	if err != nil {
		return err
	}
	next, err := s.nextVersion(current)
	if err != nil {
		return err
	}
	latest, err := s.git.LatestTag(ctx)
	if err != nil {
		return err
	}
	s.rep = &report.Report{
		Command: s.commandLabel(),
		Mode:    s.cfg.Mode.String(),
		From:    current,
		To:      next,
		Source:  s.sourceLabel(),
	}
	if latest != "" && latest != current {
		if preview {
			s.addNote("current version %s differs from latest tag %s", current, latest)
		} else {
			return fmt.Errorf("current version (%s) differs from latest tag (%s); align them before bumping", current, latest)
		}
	}
	if latest != "" {
		comparison, err := semver.Compare(next, latest)
		if err != nil {
			return err
		}
		if comparison <= 0 {
			if preview {
				s.addNote("new version %s is not greater than latest tag v%s", next, latest)
			} else {
				return fmt.Errorf("new version %s is not greater than latest tag v%s", next, latest)
			}
		}
	}
	if err := s.ensureTagAvailable(ctx, next); err != nil {
		return err
	}
	if s.cfg.Mode == ModeCommit || s.cfg.Mode == ModePush {
		if err := s.validateEnvironment(ctx); err != nil {
			return err
		}
	}
	if preview {
		if s.cfg.PreBump != "" {
			s.addHookStep("pre-bump", "skipped (preview)")
		}
		if s.cfg.PostBump != "" {
			s.addHookStep("post-bump", "skipped (preview)")
		}
	}
	if err := s.runPreBumpHook(ctx, preview); err != nil {
		return err
	}
	if !preview && s.cfg.PreBump != "" {
		s.addHookStep("pre-bump", "ran")
	}
	changed, err := s.writeVersionFiles(next, preview)
	if err != nil {
		return err
	}
	if changed || preview {
		s.rep.Changes = append(s.rep.Changes, s.versionChanges()...)
	} else if !s.proj.UsesTags() {
		s.addNote("no static version declaration in %s; left unchanged", s.proj.Primary().Path)
	}
	if s.cfg.NoChangelog {
		s.addNote("changelog skipped (--no-changelog)")
	} else {
		if err := s.writeChangelog(next, preview); err != nil {
			return err
		}
		s.rep.Changes = append(s.rep.Changes, report.Change{Path: s.cfg.ChangelogFile, Detail: "entry for " + next})
	}
	if err := gomod.ValidateModulePath(s.root, next, s.log); err != nil {
		return err
	}
	if preview {
		s.rep.Release = s.releaseSteps(next, false)
		s.rep.Footer = s.previewFooter()
		return s.renderReport()
	}
	if s.cfg.Mode == ModeWrite {
		if err := s.runPostBumpHook(ctx); err != nil {
			return err
		}
		if s.cfg.PostBump != "" {
			s.addHookStep("post-bump", "ran")
		}
		if len(s.rep.Changes) == 0 {
			s.rep.Footer = "No files were updated."
		} else {
			s.rep.Footer = s.successFooter(next)
		}
		return s.renderReport()
	}
	if err := s.runStep("committing", s.commitMessage(next), func() error {
		return s.commit(ctx, next)
	}); err != nil {
		return err
	}
	if err := s.runStep("tagging", "v"+next, func() error {
		return s.createTag(ctx, next)
	}); err != nil {
		return err
	}
	if s.cfg.Mode == ModePush {
		if err := s.push(ctx, next); err != nil {
			return err
		}
	}
	if err := s.runPostBumpHook(ctx); err != nil {
		return err
	}
	if s.cfg.PostBump != "" {
		s.addHookStep("post-bump", "ran")
	}
	s.rep.Release = s.releaseSteps(next, s.cfg.Mode == ModePush)
	s.rep.Footer = s.successFooter(next)
	return s.renderReport()
}
