// Package update installs a upvert GitHub release for the current
// operating system and CPU.
package update

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	// releasesURL is the base URL of the GitHub release assets.
	releasesURL = "https://github.com/cruffinoni/upvert/releases"
	// defaultTimeout bounds one complete update, including every download.
	defaultTimeout = 5 * time.Minute
)

// Options controls an update. Zero values select runtime defaults.
type Options struct {
	// CurrentVersion is the version reported by the version command.
	CurrentVersion string
	// Version is the release tag to install. Empty selects the latest stable
	// release.
	Version string
	// GOOS and GOARCH override the release platform; tests set them.
	GOOS   string
	GOARCH string
	// BinDir is the directory to install into. Empty selects the install
	// scripts' default directory for GOOS.
	BinDir string
	// NoPathUpdate skips updating the user PATH on Windows.
	NoPathUpdate bool
	// BaseURL overrides the GitHub releases base URL; tests set it.
	BaseURL string
	// Client overrides the HTTP client; tests may point it at a local server.
	Client *http.Client
	// Reporter receives step and download progress. Nil discards progress.
	Reporter Reporter
}

// Result describes a completed update attempt.
type Result struct {
	// Previous is the version that was running before the update.
	Previous string
	// Version is the release tag that was selected or installed.
	Version string
	// Path is the installed executable, when an install happened.
	Path string
	// Updated reports whether a new binary was installed.
	Updated bool
	// PathUpdated reports whether the Windows user PATH was changed.
	PathUpdated bool
	// OnPath reports whether the install directory is on the process PATH.
	OnPath bool
	// PathWarning explains why PATH could not be updated, when applicable.
	PathWarning string
}

// withDefaults fills empty options from the running process.
func (opts Options) withDefaults() Options {
	if opts.GOOS == "" {
		opts.GOOS = runtime.GOOS
	}
	if opts.GOARCH == "" {
		opts.GOARCH = runtime.GOARCH
	}
	if opts.BaseURL == "" {
		opts.BaseURL = releasesURL
	}
	opts.BaseURL = strings.TrimRight(opts.BaseURL, "/")
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: defaultTimeout}
	}
	if opts.Reporter == nil {
		opts.Reporter = nopReporter{}
	}
	return opts
}

// defaultBinDir returns the directory the install scripts use for goos.
func defaultBinDir(goos string) (string, error) {
	if goos == "windows" {
		root := os.Getenv("LOCALAPPDATA")
		if root == "" {
			return "", &EnvError{Name: "LOCALAPPDATA", Hint: "pass --bin-dir"}
		}
		return filepath.Join(root, "Programs", "upvert"), nil
	}
	if dir := os.Getenv("XDG_BIN_HOME"); dir != "" {
		return dir, nil
	}
	home := os.Getenv("HOME")
	if home == "" {
		return "", &EnvError{Name: "HOME", Hint: "pass --bin-dir"}
	}
	return filepath.Join(home, ".local", "bin"), nil
}

// installDir returns the absolute directory selected by opts.
func (opts Options) installDir() (string, error) {
	dir := opts.BinDir
	if dir == "" {
		var err error
		dir, err = defaultBinDir(opts.GOOS)
		if err != nil {
			return "", err
		}
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve install directory %s: %w", dir, err)
	}
	return absolute, nil
}

// displayVersion returns a version without its leading v for user output.
func displayVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "unknown"
	}
	return strings.TrimPrefix(version, "v")
}

// resolveTag returns the normalized tag selected by opts. An explicit tag is
// used as-is; otherwise the latest stable release is resolved from GitHub.
func resolveTag(ctx context.Context, opts Options) (string, error) {
	if opts.Version != "" {
		return normalizeTag(opts.Version)
	}
	reporter := opts.Reporter
	if reporter == nil {
		reporter = nopReporter{}
	}
	reporter.Start("Resolving latest stable release")
	tag, err := latestTag(ctx, opts.Client, opts.BaseURL)
	if err != nil {
		return "", err
	}
	reporter.Finish(displayVersion(tag))
	if strings.Contains(tag, "-") {
		return "", &PrereleaseError{Tag: tag}
	}
	return tag, nil
}

// shouldUpdate reports whether target is newer than current. An unparseable
// current version is treated as updatable.
func shouldUpdate(current, target string) bool {
	current = strings.TrimPrefix(strings.TrimSpace(current), "v")
	if !semver.IsValid("v" + current) {
		return true
	}
	return semver.Compare("v"+current, target) < 0
}

// Run installs the requested release, or the latest stable release when
// Options.Version is empty, and returns what it changed.
func Run(ctx context.Context, opts Options) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	opts = opts.withDefaults()
	tag, err := resolveTag(ctx, opts)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		Previous: displayVersion(opts.CurrentVersion),
		Version:  displayVersion(tag),
	}
	binDir, err := opts.installDir()
	if err != nil {
		return Result{}, err
	}
	target := filepath.Join(binDir, binaryName(opts.GOOS))
	if opts.Version == "" && !shouldUpdate(opts.CurrentVersion, tag) {
		if info, statErr := os.Stat(target); statErr == nil && info.Mode().IsRegular() {
			return result, nil
		}
	}
	result.Path = target

	asset, err := assetName(tag, opts.GOOS, opts.GOARCH)
	if err != nil {
		return Result{}, err
	}

	workDir, err := os.MkdirTemp("", "upvert-update-")
	if err != nil {
		return Result{}, fmt.Errorf("create update workspace: %w", err)
	}
	defer os.RemoveAll(workDir)

	archivePath := filepath.Join(workDir, asset)
	download := downloader{client: opts.Client, reporter: opts.Reporter}
	label := fmt.Sprintf("Downloading %s (%s/%s)", displayVersion(tag), opts.GOOS, opts.GOARCH)
	if err := download.file(ctx, downloadURL(opts.BaseURL, tag, asset), archivePath, label); err != nil {
		return Result{}, err
	}

	opts.Reporter.Start("Verifying SHA-256 checksum")
	checksums, err := downloadBytes(ctx, opts.Client, downloadURL(opts.BaseURL, tag, checksumName), maxChecksumBytes)
	if err != nil {
		return Result{}, err
	}
	expected, err := checksumFor(checksums, asset)
	if err != nil {
		return Result{}, err
	}
	if err := verifyChecksum(archivePath, expected); err != nil {
		return Result{}, err
	}
	opts.Reporter.Finish("ok")

	opts.Reporter.Start("Extracting upvert")
	binary, err := extractBinary(archivePath, asset)
	if err != nil {
		return Result{}, err
	}
	opts.Reporter.Finish("ok")

	opts.Reporter.Start("Installing to " + target)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create install directory %s: %w", binDir, err)
	}
	if err := replaceExecutable(target, binary); err != nil {
		return Result{}, fmt.Errorf("replace %s: %w", target, err)
	}
	opts.Reporter.Finish("ok")

	result.Updated = true
	if !opts.NoPathUpdate {
		windowsPath := opts.GOOS == "windows"
		if windowsPath {
			opts.Reporter.Start("Updating user PATH")
		}
		updated, pathErr := ensureUserPath(binDir)
		if pathErr != nil {
			result.PathWarning = fmt.Sprintf("could not update PATH: %v", pathErr)
			if windowsPath {
				opts.Reporter.Finish("not updated")
			}
		} else {
			result.PathUpdated = updated
			if windowsPath {
				if updated {
					opts.Reporter.Finish("added")
				} else {
					opts.Reporter.Finish("already set")
				}
			}
		}
	}
	result.OnPath = pathContains(os.Getenv("PATH"), binDir)
	return result, nil
}
