package update

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeTag(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "stable without v", input: "1.2.3", want: "v1.2.3"},
		{name: "stable with v", input: "v1.2.3", want: "v1.2.3"},
		{name: "trimmed", input: " v1.2.3 ", want: "v1.2.3"},
		{name: "prerelease", input: "v1.2.3-rc.1", want: "v1.2.3-rc.1"},
		{name: "prerelease without v", input: "1.2.3-alpha.2", want: "v1.2.3-alpha.2"},
		{name: "empty", input: "", wantErr: ErrVersionEmpty},
		{name: "missing patch", input: "v1.2", wantErr: ErrInvalidVersion},
		{name: "leading zero", input: "v01.2.3", wantErr: ErrInvalidVersion},
		{name: "build metadata", input: "v1.2.3+build", wantErr: ErrInvalidVersion},
		{name: "empty prerelease", input: "v1.2.3-", wantErr: ErrInvalidVersion},
		{name: "prerelease label only", input: "v1.2.3-rc", want: "v1.2.3-rc"},
		{name: "numeric prerelease", input: "v1.2.3-1.2", want: "v1.2.3-1.2"},
		{name: "hyphenated prerelease", input: "v1.2.3-rc-1", want: "v1.2.3-rc-1"},
		{name: "leading zero prerelease", input: "v1.2.3-01", wantErr: ErrInvalidVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeTag(tt.input)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAssetName(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		goos    string
		goarch  string
		want    string
		wantErr error
	}{
		{name: "linux", tag: "v1.2.3", goos: "linux", goarch: "amd64", want: "upvert_1.2.3_linux_amd64.tar.gz"},
		{name: "darwin", tag: "v1.2.3", goos: "darwin", goarch: "arm64", want: "upvert_1.2.3_darwin_arm64.tar.gz"},
		{name: "windows", tag: "v1.2.3", goos: "windows", goarch: "amd64", want: "upvert_1.2.3_windows_amd64.zip"},
		{name: "prerelease", tag: "v1.2.3-rc.1", goos: "windows", goarch: "arm64", want: "upvert_1.2.3-rc.1_windows_arm64.zip"},
		{name: "unsupported os", tag: "v1.2.3", goos: "freebsd", goarch: "amd64", wantErr: ErrUnsupportedOS},
		{name: "unsupported arch", tag: "v1.2.3", goos: "linux", goarch: "386", wantErr: ErrUnsupportedArch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := assetName(tt.tag, tt.goos, tt.goarch)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldUpdate(t *testing.T) {
	tests := []struct {
		name    string
		current string
		target  string
		want    bool
	}{
		{name: "dev build", current: "dev", target: "v0.0.2", want: true},
		{name: "empty build", current: "", target: "v0.0.2", want: true},
		{name: "newer target", current: "0.0.1", target: "v0.0.2", want: true},
		{name: "same version", current: "0.0.2", target: "v0.0.2", want: false},
		{name: "newer current", current: "0.0.3", target: "v0.0.2", want: false},
		{name: "prerelease to stable", current: "0.0.2-rc.1", target: "v0.0.2", want: true},
		{name: "stable to prerelease", current: "0.0.2", target: "v0.0.2-rc.1", want: false},
		{name: "v prefix", current: "v0.0.1", target: "v0.0.2", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shouldUpdate(tt.current, tt.target))
		})
	}
}

func TestChecksumFor(t *testing.T) {
	hash := sha256Hex([]byte("archive"))
	checksums := []byte("unrelated  other.tar.gz\n" + hash + "  upvert_1.2.3_linux_amd64.tar.gz\n")

	got, err := checksumFor(checksums, "upvert_1.2.3_linux_amd64.tar.gz")
	require.NoError(t, err)
	assert.Equal(t, hash, got)

	_, err = checksumFor(checksums, "missing.tar.gz")
	require.ErrorIs(t, err, ErrChecksumNotListed)
	checksumErr, ok := errors.AsType[*ChecksumError](err)
	require.True(t, ok)
	assert.Equal(t, "missing.tar.gz", checksumErr.Asset)

	_, err = checksumFor([]byte("short  file.tar.gz\n"), "file.tar.gz")
	require.ErrorIs(t, err, ErrChecksumInvalid)
}

func TestVerifyChecksum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.tar.gz")
	content := []byte("archive")
	require.NoError(t, os.WriteFile(path, content, 0o644))

	require.NoError(t, verifyChecksum(path, sha256Hex(content)))
	err := verifyChecksum(path, sha256Hex([]byte("other")))
	require.ErrorIs(t, err, ErrChecksumMismatch)
}

func TestResolveTagRejectsLatestPrerelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/latest" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/releases/tag/v1.2.3-rc.1", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	_, err := resolveTag(t.Context(), Options{BaseURL: server.URL, Client: server.Client()})
	require.ErrorIs(t, err, ErrPrerelease)
}

func TestLatestTag(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    string
		wantErr error
	}{
		{
			name: "redirect",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/latest" {
					w.WriteHeader(http.StatusOK)
					return
				}
				http.Redirect(w, r, "/releases/tag/v1.2.3", http.StatusFound)
			},
			want: "v1.2.3",
		},
		{
			name: "error status",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "unavailable", http.StatusServiceUnavailable)
			},
			wantErr: ErrLatestRelease,
		},
		{
			name: "not a tag",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/latest" {
					w.WriteHeader(http.StatusOK)
					return
				}
				http.Redirect(w, r, "/releases", http.StatusFound)
			},
			wantErr: ErrInvalidVersion,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			t.Cleanup(server.Close)

			got, err := latestTag(t.Context(), server.Client(), server.URL)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
