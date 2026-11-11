//go:build windows

package update

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// addUserPath adds dir to the user PATH and reports whether it changed.
func addUserPath(dir string) (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return false, fmt.Errorf("open user environment: %w", err)
	}
	defer key.Close()

	current, valueType, err := key.GetStringValue("Path")
	switch {
	case errors.Is(err, syscall.ERROR_FILE_NOT_FOUND):
		current = ""
		valueType = registry.EXPAND_SZ
	case err != nil:
		return false, fmt.Errorf("read user PATH: %w", err)
	}
	if pathContains(current, dir) {
		return false, nil
	}
	next := appendPath(current, dir)
	if valueType == registry.EXPAND_SZ {
		err = key.SetExpandStringValue("Path", next)
	} else {
		err = key.SetStringValue("Path", next)
	}
	if err != nil {
		return false, fmt.Errorf("write user PATH: %w", err)
	}
	return true, nil
}

// ensureUserPath adds dir to the user and process PATH when missing.
func ensureUserPath(dir string) (bool, error) {
	updated, err := addUserPath(dir)
	if err != nil {
		return false, err
	}
	if !pathContains(os.Getenv("PATH"), dir) {
		if err := os.Setenv("PATH", appendPath(os.Getenv("PATH"), dir)); err != nil {
			return updated, fmt.Errorf("update process PATH: %w", err)
		}
	}
	return updated, nil
}
