//go:build !windows

package update

import (
	"fmt"
	"os"
	"path/filepath"
)

// replaceExecutable atomically replaces target with data.
func replaceExecutable(target string, data []byte) error {
	dir := filepath.Dir(target)
	file, err := os.CreateTemp(dir, ".upvert-update-*")
	if err != nil {
		return fmt.Errorf("create temporary executable in %s: %w", dir, err)
	}
	temp := file.Name()
	defer os.Remove(temp)

	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("write %s: %w", temp, err)
	}
	if err := file.Chmod(0o755); err != nil {
		file.Close()
		return fmt.Errorf("make %s executable: %w", temp, err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync %s: %w", temp, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", temp, err)
	}
	if err := os.Rename(temp, target); err != nil {
		return fmt.Errorf("move new executable into place: %w", err)
	}
	return nil
}
