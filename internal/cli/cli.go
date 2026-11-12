// Package cli wires the upvert command tree to the bump engine.
package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cruffinoni/upvert/internal/buildinfo"
	"github.com/cruffinoni/upvert/internal/bump"
	"github.com/cruffinoni/upvert/internal/semver"
	"github.com/cruffinoni/upvert/internal/ui"
)

// applyOutput binds the report and logger to the writers configured on cmd,
// which keeps the engine testable.
func applyOutput(cmd *cobra.Command, cfg *bump.Config) {
	cfg.Stdout = cmd.OutOrStdout()
	cfg.Stderr = cmd.ErrOrStderr()
	cfg.Color = ui.ColorEnabled(cmd.OutOrStdout())
	cfg.LogColor = ui.ColorEnabled(cmd.ErrOrStderr())
}

// bumpMode resolves the mutually exclusive bump actions.
func bumpMode(write, commit, push bool) (bump.Mode, error) {
	actions := 0
	mode := bump.ModePreview
	if write {
		actions++
		mode = bump.ModeWrite
	}
	if commit {
		actions++
		mode = bump.ModeCommit
	}
	if push {
		actions++
		mode = bump.ModePush
	}
	if actions > 1 {
		return bump.ModePreview, errors.New("choose one of --write, --commit, or --push")
	}
	return mode, nil
}

// addActionFlags registers the mutually exclusive file and git actions.
func addActionFlags(cmd *cobra.Command, write, commit, push *bool) {
	cmd.Flags().BoolVar(write, "write", false, "update files without committing or tagging")
	cmd.Flags().BoolVar(commit, "commit", false, "update files, create a release commit, and tag")
	cmd.Flags().BoolVar(push, "push", false, "commit, tag, and push to origin")
}

// addModifierFlags registers the content flags shared by every release command.
func addModifierFlags(cmd *cobra.Command, cfg *bump.Config) {
	cmd.Flags().StringVar(&cfg.VersionFile, "version-file", "", "file that contains the version string")
	cmd.Flags().StringVar(&cfg.ChangelogFile, "changelog-file", "", "changelog path (default CHANGELOG.md)")
	cmd.Flags().BoolVar(&cfg.NoChangelog, "no-changelog", false, "skip the changelog update")
	cmd.Flags().BoolVar(&cfg.NoVersionAttr, "no-version-attr", false, "skip in-code version declarations")
	cmd.Flags().StringVar(&cfg.Message, "message", "", "commit message template with a {version} placeholder")
	cmd.Flags().StringVar(&cfg.PreBump, "pre-bump", "", "shell command to run before bumping")
	cmd.Flags().StringVar(&cfg.PostBump, "post-bump", "", "shell command to run after bumping")
}

// addBumpFlags registers the action and content flags shared by patch, minor,
// and major.
func addBumpFlags(cmd *cobra.Command, cfg *bump.Config, write, commit, push *bool) {
	addActionFlags(cmd, write, commit, push)
	cmd.Flags().StringVar(&cfg.PreRelease, "pre", "", "pre-release label (for example alpha, beta, or rc)")
	addModifierFlags(cmd, cfg)
}

// newBumpCommand returns one of the patch, minor, or major subcommands.
func newBumpCommand(cfg *bump.Config, kind bump.Kind, use, short string) *cobra.Command {
	var write, commit, push bool
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			applyOutput(cmd, cfg)
			mode, err := bumpMode(write, commit, push)
			if err != nil {
				return err
			}
			cfg.Mode = mode
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return bump.Apply(cmd.Context(), *cfg, kind)
		},
	}
	addBumpFlags(cmd, cfg, &write, &commit, &push)
	cmd.Flags().StringVarP(&cfg.Dir, "dir", "C", ".", "run as if started in this directory")
	return cmd
}

// newPreCommand returns the prerelease lifecycle subcommand.
func newPreCommand(cfg *bump.Config) *cobra.Command {
	var write, commit, push, release bool
	cmd := &cobra.Command{
		Use:   "pre [<label>]",
		Short: "Continue, relabel, or promote the current prerelease",
		Long: `Continue the current prerelease counter, change its label, or promote it to
its stable version. The numeric core never changes; use patch, minor, or major
with --pre to start a prerelease on a new core.`,
		Args: cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			applyOutput(cmd, cfg)
			mode, err := bumpMode(write, commit, push)
			if err != nil {
				return err
			}
			cfg.Mode = mode
			if release {
				if len(args) != 0 {
					return errors.New("--release does not take a prerelease label")
				}
				return nil
			}
			if len(args) != 1 {
				return errors.New("provide a prerelease label or --release")
			}
			return semver.ValidatePreReleaseLabel(args[0])
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if release {
				return bump.ApplyRelease(cmd.Context(), *cfg)
			}
			return bump.ApplyPre(cmd.Context(), *cfg, args[0])
		},
	}
	addActionFlags(cmd, &write, &commit, &push)
	addModifierFlags(cmd, cfg)
	cmd.Flags().BoolVar(&release, "release", false, "promote the current prerelease to its stable version")
	cmd.Flags().StringVarP(&cfg.Dir, "dir", "C", ".", "run as if started in this directory")
	return cmd
}

// newRollbackCommand returns the rollback subcommand.
func newRollbackCommand(cfg *bump.Config) *cobra.Command {
	var apply bool
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Undo the most recent version bump commit",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			applyOutput(cmd, cfg)
			cfg.Mode = bump.ModePreview
			if apply {
				cfg.Mode = bump.ModeApply
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return bump.Rollback(cmd.Context(), *cfg)
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "apply the rollback (default is a preview)")
	cmd.Flags().StringVarP(&cfg.Dir, "dir", "C", ".", "run as if started in this directory")
	cmd.Flags().StringVar(&cfg.VersionFile, "version-file", "", "file that contains the version string")
	cmd.Flags().StringVar(&cfg.Message, "message", "", "commit message template used by the bump")
	return cmd
}

// newVersionCommand returns the version subcommand.
func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print upvert version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "upvert %s\ncommit: %s\nbuilt: %s\n", buildinfo.Current(), buildinfo.Commit, buildinfo.BuiltAt)
			return err
		},
	}
}

// New returns the upvert root command and its subcommands.
func New() *cobra.Command {
	cfg := &bump.Config{}
	root := &cobra.Command{
		Use:   "upvert",
		Short: "Bump semantic versions across project ecosystems",
		Long: `upvert bumps the version of a project and optionally commits, tags,
and pushes the result. It understands Go, Node.js, Python, Rust, and
repositories that mix several of those ecosystems.

Use "upvert pre" to continue, relabel, or promote the current prerelease.
The default is a preview: upvert reports what it would change and writes
nothing until --write, --commit, or --push is supplied. Rollback previews
until --apply is supplied. Use "upvert update" to install a GitHub release.`,
		Example: `  upvert patch                     # preview
  upvert patch --write             # update files only
  upvert minor --commit            # update, commit, and tag
  upvert patch --pre alpha --commit
  upvert pre alpha --commit        # continue the current prerelease
  upvert pre --release --push      # promote it to stable
  upvert minor --push              # update, commit, tag, and push
  upvert rollback --apply          # undo the last bump
  upvert update                    # install the latest stable release`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newBumpCommand(cfg, bump.Patch, "patch", "Increment the patch version (1.0.0 -> 1.0.1)"),
		newBumpCommand(cfg, bump.Minor, "minor", "Increment the minor version, resetting patch (1.0.0 -> 1.1.0)"),
		newBumpCommand(cfg, bump.Major, "major", "Increment the major version, resetting minor and patch (1.0.0 -> 2.0.0)"),
		newPreCommand(cfg),
		newRollbackCommand(cfg),
		newUpdateCommand(),
		newVersionCommand(),
	)
	return root
}
