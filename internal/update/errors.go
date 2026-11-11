package update

import (
	"errors"
	"fmt"
)

// Sentinel errors identify update failures that callers and tests need to
// distinguish without depending on message text.
var (
	ErrVersionEmpty       = errors.New("release version is empty")
	ErrInvalidVersion     = errors.New("invalid release version")
	ErrUnsupportedOS      = errors.New("unsupported operating system")
	ErrUnsupportedArch    = errors.New("unsupported CPU architecture")
	ErrPrerelease         = errors.New("latest release is a prerelease")
	ErrChecksumNotListed  = errors.New("checksum not listed")
	ErrChecksumInvalid    = errors.New("invalid checksum")
	ErrChecksumMismatch   = errors.New("checksum mismatch")
	ErrUnsupportedArchive = errors.New("unsupported archive format")
	ErrBinaryNotFound     = errors.New("archive does not contain binary")
	ErrEmptyBinary        = errors.New("archive contains an empty binary")
	ErrTooLarge           = errors.New("content exceeds size limit")
	ErrHTTPStatus         = errors.New("unexpected HTTP status")
	ErrLatestRelease      = errors.New("resolve latest release")
	ErrEnvNotSet          = errors.New("required environment variable is not set")
)

// VersionError describes an invalid release tag while preserving the tag in
// the message.
type VersionError struct {
	Tag string
	Err error
}

// Error returns a string representation of the version error.
func (e *VersionError) Error() string { return fmt.Sprintf("%v %q", e.Err, e.Tag) }

// Unwrap returns the underlying version error.
func (e *VersionError) Unwrap() error { return e.Err }

// PrereleaseError describes a latest release that is a prerelease.
type PrereleaseError struct {
	Tag string
}

// Error returns a string representation of the prerelease error.
func (e *PrereleaseError) Error() string {
	return fmt.Sprintf("latest release %s is a prerelease; pass --version to install it explicitly", e.Tag)
}

// Unwrap returns ErrPrerelease.
func (e *PrereleaseError) Unwrap() error { return ErrPrerelease }

// PlatformError describes an unsupported release platform.
type PlatformError struct {
	GOOS   string
	GOARCH string
	Err    error
}

// Error returns a string representation of the platform error.
func (e *PlatformError) Error() string {
	if e.GOOS != "" {
		return fmt.Sprintf("%v %q (supported: linux, darwin, windows)", e.Err, e.GOOS)
	}
	return fmt.Sprintf("%v %q (supported: amd64, arm64)", e.Err, e.GOARCH)
}

// Unwrap returns the underlying platform error.
func (e *PlatformError) Unwrap() error { return e.Err }

// ChecksumError describes a checksum failure for one asset.
type ChecksumError struct {
	Asset string
	Err   error
}

// Error returns a string representation of the checksum error.
func (e *ChecksumError) Error() string {
	switch {
	case errors.Is(e.Err, ErrChecksumNotListed):
		return fmt.Sprintf("%s does not list %s", checksumName, e.Asset)
	case errors.Is(e.Err, ErrChecksumInvalid):
		return fmt.Sprintf("invalid checksum for %s", e.Asset)
	case errors.Is(e.Err, ErrChecksumMismatch):
		return fmt.Sprintf("checksum mismatch for %s", e.Asset)
	default:
		return fmt.Sprintf("%v for %s", e.Err, e.Asset)
	}
}

// Unwrap returns the underlying checksum error.
func (e *ChecksumError) Unwrap() error { return e.Err }

// ArchiveError describes an archive failure for one archive or entry.
type ArchiveError struct {
	Archive string
	Entry   string
	Err     error
}

// Error returns a string representation of the archive error.
func (e *ArchiveError) Error() string {
	switch {
	case errors.Is(e.Err, ErrBinaryNotFound):
		return fmt.Sprintf("archive does not contain %s", e.Entry)
	case errors.Is(e.Err, ErrEmptyBinary):
		return fmt.Sprintf("archive %s contains an empty binary", e.Archive)
	case errors.Is(e.Err, ErrUnsupportedArchive):
		return fmt.Sprintf("unsupported archive format: %s", e.Archive)
	case errors.Is(e.Err, ErrTooLarge):
		return fmt.Sprintf("%s is too large", e.Entry)
	default:
		return fmt.Sprintf("%v: %s", e.Err, e.Archive)
	}
}

// Unwrap returns the underlying archive error.
func (e *ArchiveError) Unwrap() error { return e.Err }

// HTTPStatusError reports a non-200 release request.
type HTTPStatusError struct {
	URL    string
	Code   int
	Status string
}

// Error returns a string representation of the HTTP status error.
func (e *HTTPStatusError) Error() string { return fmt.Sprintf("request %s: %s", e.URL, e.Status) }

// Unwrap returns ErrHTTPStatus.
func (e *HTTPStatusError) Unwrap() error { return ErrHTTPStatus }

// EnvError reports a missing environment variable and the fix.
type EnvError struct {
	Name string
	Hint string
}

// Error returns a string representation of the environment error.
func (e *EnvError) Error() string {
	return fmt.Sprintf("the %s environment variable is not set; %s", e.Name, e.Hint)
}

// Unwrap returns ErrEnvNotSet.
func (e *EnvError) Unwrap() error { return ErrEnvNotSet }
