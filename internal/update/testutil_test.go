package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serverStats counts the release requests an update makes.
type serverStats struct {
	latest    atomic.Int32
	downloads atomic.Int32
}

// progressCall records one Reporter method invocation.
type progressCall struct {
	kind   string
	label  string
	detail string
	total  int64
	done   int64
}

// recordingReporter records update progress for assertions.
type recordingReporter struct {
	calls []progressCall
}

func (r *recordingReporter) Start(label string) {
	r.calls = append(r.calls, progressCall{kind: "start", label: label})
}

func (r *recordingReporter) Total(total int64) {
	r.calls = append(r.calls, progressCall{kind: "total", total: total})
}

func (r *recordingReporter) Advance(done int64) {
	r.calls = append(r.calls, progressCall{kind: "advance", done: done})
}

func (r *recordingReporter) Finish(detail string) {
	r.calls = append(r.calls, progressCall{kind: "finish", detail: detail})
}

func (r *recordingReporter) Abort() {
	r.calls = append(r.calls, progressCall{kind: "abort"})
}

// sha256Hex returns the lowercase SHA-256 hex digest of data.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// tarGzArchive builds a tar.gz release archive containing name and content.
func tarGzArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(compressed)

	license := []byte("license")
	require.NoError(t, archive.WriteHeader(&tar.Header{Name: "LICENSE", Mode: 0o644, Size: int64(len(license))}))
	_, err := archive.Write(license)
	require.NoError(t, err)
	require.NoError(t, archive.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}))
	_, err = archive.Write(content)
	require.NoError(t, err)
	require.NoError(t, archive.Close())
	require.NoError(t, compressed.Close())
	return buffer.Bytes()
}

// zipArchive builds a zip release archive containing name and content.
func zipArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	file, err := archive.Create(name)
	require.NoError(t, err)
	_, err = file.Write(content)
	require.NoError(t, err)
	require.NoError(t, archive.Close())
	return buffer.Bytes()
}

// archiveFor builds the archive shape a platform releases.
func archiveFor(t *testing.T, goos, name string, content []byte) []byte {
	t.Helper()
	if goos == "windows" {
		return zipArchive(t, name, content)
	}
	return tarGzArchive(t, name, content)
}

// executableName returns the binary name for goos.
func executableName(goos string) string {
	if goos == "windows" {
		return "upvert.exe"
	}
	return "upvert"
}

// writeBinDir creates a stand-in install directory with an existing upvert.
func writeBinDir(t *testing.T, content []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, executableName(runtime.GOOS))
	require.NoError(t, os.WriteFile(path, content, 0o755))
	return dir
}

// newReleaseServer serves a latest-release redirect plus one archive and
// its checksums.txt.
func newReleaseServer(t *testing.T, tag, asset, checksum string, archive []byte) (*httptest.Server, *serverStats) {
	t.Helper()
	stats := &serverStats{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			stats.latest.Add(1)
			http.Redirect(w, r, "/releases/tag/"+tag, http.StatusFound)
		case "/releases/tag/" + tag:
			w.WriteHeader(http.StatusOK)
		case "/download/" + tag + "/" + asset:
			stats.downloads.Add(1)
			w.Header().Set("Content-Length", strconv.Itoa(len(archive)))
			_, err := w.Write(archive)
			assert.NoError(t, err)
		case "/download/" + tag + "/checksums.txt":
			_, err := fmt.Fprintf(w, "%s  %s\n", checksum, asset)
			assert.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server, stats
}
