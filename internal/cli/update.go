package cli

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/dayrion/upvert/internal/buildinfo"
	"github.com/dayrion/upvert/internal/update"
)

// pathHelp returns the PATH guidance the installers print.
func pathHelp(result update.Result) string {
	binDir := filepath.Dir(result.Path)
	if result.PathUpdated {
		return fmt.Sprintf("Added %s to the user PATH.\nRestart your terminal to use upvert from other shells.\n", binDir)
	}
	if result.OnPath {
		return ""
	}
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("Add %s to your PATH to run upvert from other shells.\n", binDir)
	}
	return fmt.Sprintf("\nAdd %s to your PATH:\n  export PATH=\"%s:$PATH\"\n", binDir, binDir)
}

// newUpdateCommand builds the update command, which installs a GitHub release
// into the same directory as the install scripts.
func newUpdateCommand() *cobra.Command {
	var tag, binDir string
	var noPathUpdate, quiet bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update upvert from a GitHub release.",
		Long: "Update upvert by downloading a GitHub release for the current operating system and CPU, " +
			"verifying its SHA-256 checksum, and installing it into the standard install directory. " +
			"Without --version, only the latest stable release is considered. Pass --version to " +
			"install a specific tag, including prereleases such as v1.2.3-rc.1.",
		Example: "  upvert update\n" +
			"  upvert update --version v1.2.3-rc.1\n" +
			"  upvert update --bin-dir \"$HOME/.local/bin\"",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			reporter := newUpdateReporter(cmd.ErrOrStderr(), quiet)
			result, err := update.Run(cmd.Context(), update.Options{
				CurrentVersion: buildinfo.Current(),
				Version:        tag,
				BinDir:         binDir,
				NoPathUpdate:   noPathUpdate,
				Reporter:       reporter,
			})
			if err != nil {
				reporter.Abort()
				return err
			}
			if result.PathWarning != "" {
				if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "upvert: warning: %s\n", result.PathWarning); err != nil {
					return err
				}
			}
			if !result.Updated {
				_, err = fmt.Fprintf(out, "upvert %s is already up to date (latest stable is %s).\n", result.Previous, result.Version)
				return err
			}
			if tag == "" && result.Previous != result.Version {
				_, err = fmt.Fprintf(out, "Updated upvert %s to %s at %s.\n", result.Previous, result.Version, result.Path)
			} else {
				_, err = fmt.Fprintf(out, "Installed upvert %s at %s.\n", result.Version, result.Path)
			}
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(out, pathHelp(result))
			return err
		},
	}
	cmd.Flags().StringVar(&tag, "version", tag, "release tag to install, including prereleases (default: latest stable)")
	cmd.Flags().StringVar(&binDir, "bin-dir", binDir, "install directory (default: the install scripts' directory)")
	cmd.Flags().BoolVar(&noPathUpdate, "no-path-update", noPathUpdate, "Windows only: do not add the install directory to the user PATH")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress step progress; print only the final result and errors")
	return cmd
}
