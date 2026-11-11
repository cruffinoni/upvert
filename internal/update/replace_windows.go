//go:build windows

package update

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// replaceExecutable replaces target with data using a rename swap, because
// Windows cannot overwrite a running executable in place.
func replaceExecutable(target string, data []byte) error {
	dir := filepath.Dir(target)
	file, err := os.CreateTemp(dir, ".upvert-update-*.exe")
	if err != nil {
		return fmt.Errorf("create temporary executable in %s: %w", dir, err)
	}
	temp := file.Name()
	defer os.Remove(temp)

	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("write %s: %w", temp, err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync %s: %w", temp, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", temp, err)
	}

	info, err := os.Stat(target)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.Rename(temp, target); err != nil {
			return fmt.Errorf("move new executable into place: %w", err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("inspect %s: %w", target, err)
	case info.IsDir():
		return fmt.Errorf("target %s is a directory", target)
	}

	backup := target + ".old"
	if err := os.Remove(backup); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale backup %s: %w", backup, err)
	}
	if err := os.Rename(target, backup); err != nil {
		return fmt.Errorf("move existing executable to %s: %w", backup, err)
	}
	if err := os.Rename(temp, target); err != nil {
		if restoreErr := os.Rename(backup, target); restoreErr != nil {
			return fmt.Errorf("move new executable into place: %w; restore %s failed: %w", err, backup, restoreErr)
		}
		return fmt.Errorf("move new executable into place: %w", err)
	}
	// The old executable is still the running process, so Windows may keep
	// it locked until this process exits. The next update removes it.
	_ = os.Remove(backup)
	return nil
}
