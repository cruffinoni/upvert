package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"
)

const (
	// checksumName is the release asset that lists every archive hash.
	checksumName = "checksums.txt"
	// maxArchiveBytes bounds an archive download.
	maxArchiveBytes = 128 << 20
	// maxChecksumBytes bounds a checksums.txt download.
	maxChecksumBytes = 1 << 20
	// userAgent identifies the updater to GitHub.
	userAgent = "upvert-updater"
)

// isReleaseTag reports whether tag is a stable or prerelease semantic version.
// Build metadata is rejected because release assets do not encode it.
func isReleaseTag(tag string) bool {
	return semver.IsValid(tag) && semver.Canonical(tag) == tag && !strings.Contains(tag, "+")
}

// get performs a context-aware GET and rejects non-200 responses.
func get(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", url, err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, &HTTPStatusError{URL: url, Code: resp.StatusCode, Status: resp.Status}
	}
	return resp, nil
}

// normalizeTag adds a leading v when needed and validates the release tag.
func normalizeTag(tag string) (string, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", ErrVersionEmpty
	}
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	if !isReleaseTag(tag) {
		return "", &VersionError{Tag: tag, Err: ErrInvalidVersion}
	}
	return tag, nil
}

// latestTag resolves the latest stable release from the release-page
// redirect, avoiding the unauthenticated GitHub API rate limit.
func latestTag(ctx context.Context, client *http.Client, baseURL string) (string, error) {
	resp, err := get(ctx, client, baseURL+"/latest")
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrLatestRelease, err)
	}
	defer resp.Body.Close()

	tag, err := normalizeTag(path.Base(resp.Request.URL.Path))
	if err != nil {
		return "", fmt.Errorf("%w from %s: %w", ErrLatestRelease, resp.Request.URL, err)
	}
	return tag, nil
}

// binaryName returns the installed executable name for goos.
func binaryName(goos string) string {
	if goos == "windows" {
		return "upvert.exe"
	}
	return "upvert"
}

// assetName returns the release archive name for tag, goos, and goarch.
func assetName(tag, goos, goarch string) (string, error) {
	if goos != "linux" && goos != "darwin" && goos != "windows" {
		return "", &PlatformError{GOOS: goos, Err: ErrUnsupportedOS}
	}
	if goarch != "amd64" && goarch != "arm64" {
		return "", &PlatformError{GOARCH: goarch, Err: ErrUnsupportedArch}
	}
	extension := "tar.gz"
	if goos == "windows" {
		extension = "zip"
	}
	return fmt.Sprintf("upvert_%s_%s_%s.%s", strings.TrimPrefix(tag, "v"), goos, goarch, extension), nil
}

// downloadURL returns the URL of one release asset.
func downloadURL(baseURL, tag, name string) string {
	return baseURL + "/download/" + tag + "/" + name
}

// progressReader counts downloaded bytes and forwards cumulative progress.
type progressReader struct {
	reader   io.Reader
	reporter Reporter
	done     int64
}

// Read reads from the underlying reader and reports the cumulative count.
func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.done += int64(n)
		r.reporter.Advance(r.done)
	}
	return n, err
}

// downloader streams release archives and reports their progress.
type downloader struct {
	client   *http.Client
	reporter Reporter
}

// file downloads url into path, rejecting oversized responses.
func (d downloader) file(ctx context.Context, url, path, label string) error {
	d.reporter.Start(label)
	resp, err := get(ctx, d.client, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	d.reporter.Total(resp.ContentLength)

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	reader := &progressReader{
		reader:   io.LimitReader(resp.Body, maxArchiveBytes+1),
		reporter: d.reporter,
	}
	written, copyErr := io.Copy(file, reader)
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("download %s: %w", url, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("write %s: %w", path, closeErr)
	}
	if written > maxArchiveBytes {
		return fmt.Errorf("%w: download %s exceeds %d bytes", ErrTooLarge, url, maxArchiveBytes)
	}
	d.reporter.Finish("")
	return nil
}

// downloadBytes fetches one small release asset into memory.
func downloadBytes(ctx context.Context, client *http.Client, url string, limit int64) ([]byte, error) {
	resp, err := get(ctx, client, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: download %s exceeds %d bytes", ErrTooLarge, url, limit)
	}
	return data, nil
}

// checksumFor returns the SHA-256 recorded for asset in checksums.
func checksumFor(checksums []byte, asset string) (string, error) {
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[1] != asset {
			continue
		}
		if len(fields[0]) != hex.EncodedLen(sha256.Size) {
			return "", &ChecksumError{Asset: asset, Err: ErrChecksumInvalid}
		}
		return strings.ToLower(fields[0]), nil
	}
	return "", &ChecksumError{Asset: asset, Err: ErrChecksumNotListed}
}

// verifyChecksum checks the SHA-256 of path against expected.
func verifyChecksum(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("hash %s: %w", path, err)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return &ChecksumError{Asset: filepath.Base(path), Err: ErrChecksumMismatch}
	}
	return nil
}
