package bump

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/cruffinoni/upvert/internal/fileio"
)

// rollbackSnapshot records every file a bump may rewrite, including the files
// that do not exist yet, so a failed run can restore the work tree.
type rollbackSnapshot struct {
	root    string
	files   []fileSnapshot
	missing []string
	paths   []string
}

// snapshotWrites captures the files the pipeline is about to rewrite.
func (s *service) snapshotWrites() (*rollbackSnapshot, error) {
	snapshot := &rollbackSnapshot{root: s.root}
	seen := make(map[string]bool)
	add := func(relative string) error {
		if relative == "" || seen[relative] {
			return nil
		}
		seen[relative] = true
		path := fileio.Resolve(s.root, relative)
		info, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			snapshot.paths = append(snapshot.paths, relative)
			snapshot.missing = append(snapshot.missing, relative)
			return nil
		}
		if err != nil {
			return fmt.Errorf("stat %s: %w", relative, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", relative, err)
		}
		snapshot.paths = append(snapshot.paths, relative)
		snapshot.files = append(snapshot.files, fileSnapshot{
			path: path,
			data: data,
			mode: info.Mode().Perm(),
		})
		return nil
	}
	for i := range s.proj.Parts {
		for _, file := range s.writableFiles(s.proj.Parts[i].Target.Files) {
			if err := add(file.Path); err != nil {
				return nil, err
			}
		}
	}
	if !s.cfg.NoChangelog {
		if err := add(s.cfg.ChangelogFile); err != nil {
			return nil, err
		}
	}
	return snapshot, nil
}

// restore rewrites the captured files and removes the ones that did not exist.
func (r *rollbackSnapshot) restore() error {
	var firstErr error
	for _, file := range r.files {
		if err := fileio.WriteAtomic(file.path, file.data, file.mode); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	for _, relative := range r.missing {
		err := os.Remove(fileio.Resolve(r.root, relative))
		if err != nil && !errors.Is(err, fs.ErrNotExist) && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// rollbackWrites undoes a partially applied bump. It removes a local tag and
// commit, restores the captured files, and unstages them. It leaves the
// repository untouched once a ref reached origin, because the remote cannot be
// rewound safely.
func (s *service) rollbackWrites(ctx context.Context, snapshot *rollbackSnapshot, version string) {
	if snapshot == nil {
		return
	}
	if s.pushed > 0 {
		s.log.Warnf("push failed after publishing refs; the commit and tag were left in place")
		return
	}
	if s.tagged {
		if err := s.git.DeleteTag(ctx, "v"+version); err != nil {
			s.log.Warnf("could not remove tag v%s: %v", version, err)
		}
	}
	if s.committed {
		if err := s.git.ResetMixed(ctx, "HEAD~1"); err != nil {
			s.log.Warnf("could not undo the bump commit: %v", err)
		}
	}
	if err := snapshot.restore(); err != nil {
		s.log.Warnf("could not restore the work tree: %v", err)
	}
	if len(snapshot.paths) > 0 {
		if err := s.git.Unstage(ctx, snapshot.paths...); err != nil {
			s.log.Warnf("could not unstage the bump files: %v", err)
		}
	}
	if !s.cfg.Quiet {
		s.log.Infof("restored the work tree after the failed bump")
	}
}
