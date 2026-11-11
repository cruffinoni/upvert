package update

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunLatestStable(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	tag := "v0.0.2"
	content := []byte("new binary")
	asset, err := assetName(tag, goos, goarch)
	require.NoError(t, err)

	archive := archiveFor(t, goos, executableName(goos), content)
	server, stats := newReleaseServer(t, tag, asset, sha256Hex(archive), archive)
	binDir := writeBinDir(t, []byte("old binary"))
	target := filepath.Join(binDir, executableName(goos))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	result, err := Run(t.Context(), Options{
		CurrentVersion: "0.0.1",
		BaseURL:        server.URL,
		GOOS:           goos,
		GOARCH:         goarch,
		BinDir:         binDir,
		NoPathUpdate:   true,
	})
	require.NoError(t, err)
	assert.True(t, result.Updated)
	assert.Equal(t, "0.0.1", result.Previous)
	assert.Equal(t, "0.0.2", result.Version)
	assert.Equal(t, target, result.Path)
	assert.True(t, result.OnPath)
	assert.False(t, result.PathUpdated)
	assert.Empty(t, result.PathWarning)
	assert.Equal(t, int32(1), stats.latest.Load())
	assert.Equal(t, int32(1), stats.downloads.Load())

	installed, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, content, installed)
	if runtime.GOOS != "windows" {
		info, err := os.Stat(target)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	}
}

func TestRunInstallsIntoNewDirectory(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	tag := "v0.0.2"
	content := []byte("new binary")
	asset, err := assetName(tag, goos, goarch)
	require.NoError(t, err)

	archive := archiveFor(t, goos, executableName(goos), content)
	server, _ := newReleaseServer(t, tag, asset, sha256Hex(archive), archive)
	binDir := filepath.Join(t.TempDir(), "nested", "bin")
	target := filepath.Join(binDir, executableName(goos))

	result, err := Run(t.Context(), Options{
		CurrentVersion: "0.0.1",
		BaseURL:        server.URL,
		GOOS:           goos,
		GOARCH:         goarch,
		BinDir:         binDir,
		NoPathUpdate:   true,
	})
	require.NoError(t, err)
	assert.True(t, result.Updated)
	assert.Equal(t, target, result.Path)

	info, err := os.Stat(binDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	installed, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, content, installed)
}

func TestRunAlreadyCurrent(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	tag := "v0.0.2"
	content := []byte("new binary")
	asset, err := assetName(tag, goos, goarch)
	require.NoError(t, err)

	archive := archiveFor(t, goos, executableName(goos), content)
	server, stats := newReleaseServer(t, tag, asset, sha256Hex(archive), archive)
	binDir := writeBinDir(t, []byte("old binary"))
	target := filepath.Join(binDir, executableName(goos))

	result, err := Run(t.Context(), Options{
		CurrentVersion: "0.0.2",
		BaseURL:        server.URL,
		GOOS:           goos,
		GOARCH:         goarch,
		BinDir:         binDir,
		NoPathUpdate:   true,
	})
	require.NoError(t, err)
	assert.False(t, result.Updated)
	assert.Equal(t, "0.0.2", result.Previous)
	assert.Equal(t, "0.0.2", result.Version)
	assert.Empty(t, result.Path)
	assert.Equal(t, int32(0), stats.downloads.Load())

	installed, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, []byte("old binary"), installed)
}

func TestRunReportsProgress(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	tag := "v0.0.2"
	content := []byte("new binary")
	asset, err := assetName(tag, goos, goarch)
	require.NoError(t, err)

	archive := archiveFor(t, goos, executableName(goos), content)
	server, _ := newReleaseServer(t, tag, asset, sha256Hex(archive), archive)
	binDir := writeBinDir(t, []byte("old binary"))
	reporter := &recordingReporter{}

	result, err := Run(t.Context(), Options{
		CurrentVersion: "0.0.1",
		BaseURL:        server.URL,
		GOOS:           goos,
		GOARCH:         goarch,
		BinDir:         binDir,
		NoPathUpdate:   true,
		Reporter:       reporter,
	})
	require.NoError(t, err)
	require.True(t, result.Updated)

	var starts, finishes int
	var total, lastAdvance int64
	for _, call := range reporter.calls {
		switch call.kind {
		case "start":
			starts++
		case "finish":
			finishes++
		case "total":
			total = call.total
		case "advance":
			lastAdvance = call.done
		case "abort":
			require.FailNow(t, "unexpected abort")
		}
	}
	assert.Equal(t, 5, starts)
	assert.Equal(t, 5, finishes)
	assert.Equal(t, int64(len(archive)), total)
	assert.Equal(t, int64(len(archive)), lastAdvance)
}

func TestRunInstallsCurrentIntoMissingDirectory(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	tag := "v0.0.2"
	content := []byte("new binary")
	asset, err := assetName(tag, goos, goarch)
	require.NoError(t, err)

	archive := archiveFor(t, goos, executableName(goos), content)
	server, _ := newReleaseServer(t, tag, asset, sha256Hex(archive), archive)
	binDir := filepath.Join(t.TempDir(), "install")
	target := filepath.Join(binDir, executableName(goos))

	result, err := Run(t.Context(), Options{
		CurrentVersion: "0.0.2",
		BaseURL:        server.URL,
		GOOS:           goos,
		GOARCH:         goarch,
		BinDir:         binDir,
		NoPathUpdate:   true,
	})
	require.NoError(t, err)
	assert.True(t, result.Updated)
	assert.Equal(t, target, result.Path)

	installed, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, content, installed)
}

func TestRunExplicitPrerelease(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	tag := "v0.0.2-rc.1"
	content := []byte("prerelease binary")
	asset, err := assetName(tag, goos, goarch)
	require.NoError(t, err)

	archive := archiveFor(t, goos, executableName(goos), content)
	server, stats := newReleaseServer(t, tag, asset, sha256Hex(archive), archive)
	binDir := writeBinDir(t, []byte("old binary"))
	target := filepath.Join(binDir, executableName(goos))

	result, err := Run(t.Context(), Options{
		CurrentVersion: "0.0.2",
		Version:        "0.0.2-rc.1",
		BaseURL:        server.URL,
		GOOS:           goos,
		GOARCH:         goarch,
		BinDir:         binDir,
		NoPathUpdate:   true,
	})
	require.NoError(t, err)
	assert.True(t, result.Updated)
	assert.Equal(t, "0.0.2-rc.1", result.Version)
	assert.Equal(t, int32(0), stats.latest.Load())
	assert.Equal(t, int32(1), stats.downloads.Load())

	installed, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, content, installed)
}

func TestRunChecksumMismatchKeepsBinary(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	tag := "v0.0.2"
	content := []byte("new binary")
	asset, err := assetName(tag, goos, goarch)
	require.NoError(t, err)

	archive := archiveFor(t, goos, executableName(goos), content)
	server, _ := newReleaseServer(t, tag, asset, sha256Hex([]byte("other")), archive)
	binDir := writeBinDir(t, []byte("old binary"))
	target := filepath.Join(binDir, executableName(goos))

	_, err = Run(t.Context(), Options{
		CurrentVersion: "0.0.1",
		BaseURL:        server.URL,
		GOOS:           goos,
		GOARCH:         goarch,
		BinDir:         binDir,
		NoPathUpdate:   true,
	})
	require.ErrorIs(t, err, ErrChecksumMismatch)

	installed, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, []byte("old binary"), installed)
}

func TestRunMissingAssetKeepsBinary(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	tag := "v0.0.2"
	server, _ := newReleaseServer(t, tag, "other-asset.tar.gz", sha256Hex([]byte("other")), []byte("other"))
	binDir := writeBinDir(t, []byte("old binary"))
	target := filepath.Join(binDir, executableName(goos))

	_, err := Run(t.Context(), Options{
		CurrentVersion: "0.0.1",
		BaseURL:        server.URL,
		GOOS:           goos,
		GOARCH:         goarch,
		BinDir:         binDir,
		NoPathUpdate:   true,
	})
	require.ErrorIs(t, err, ErrHTTPStatus)
	httpErr, ok := errors.AsType[*HTTPStatusError](err)
	require.True(t, ok)
	assert.Equal(t, 404, httpErr.Code)

	installed, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, []byte("old binary"), installed)
}

func TestRunLatestFailureKeepsBinary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)
	binDir := writeBinDir(t, []byte("old binary"))
	target := filepath.Join(binDir, executableName(runtime.GOOS))

	_, err := Run(t.Context(), Options{
		CurrentVersion: "0.0.1",
		BaseURL:        server.URL,
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
		BinDir:         binDir,
		NoPathUpdate:   true,
	})
	require.ErrorIs(t, err, ErrLatestRelease)

	installed, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, []byte("old binary"), installed)
}

func TestRunUnsupportedPlatform(t *testing.T) {
	tests := []struct {
		name    string
		goos    string
		goarch  string
		wantErr error
	}{
		{name: "os", goos: "plan9", goarch: "amd64", wantErr: ErrUnsupportedOS},
		{name: "arch", goos: "linux", goarch: "386", wantErr: ErrUnsupportedArch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Run(t.Context(), Options{
				Version: "v0.0.2",
				GOOS:    tt.goos,
				GOARCH:  tt.goarch,
				BinDir:  t.TempDir(),
			})
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestDefaultBinDir(t *testing.T) {
	t.Run("unix xdg", func(t *testing.T) {
		t.Setenv("XDG_BIN_HOME", "/custom/bin")
		dir, err := defaultBinDir("linux")
		require.NoError(t, err)
		assert.Equal(t, "/custom/bin", dir)
	})

	t.Run("unix home", func(t *testing.T) {
		t.Setenv("XDG_BIN_HOME", "")
		t.Setenv("HOME", "/home/test")
		dir, err := defaultBinDir("darwin")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join("/home/test", ".local", "bin"), dir)
	})

	t.Run("unix missing home", func(t *testing.T) {
		t.Setenv("XDG_BIN_HOME", "")
		t.Setenv("HOME", "")
		_, err := defaultBinDir("linux")
		envErr, ok := errors.AsType[*EnvError](err)
		require.True(t, ok)
		assert.Equal(t, "HOME", envErr.Name)
	})

	t.Run("windows localappdata", func(t *testing.T) {
		root := filepath.Join("C:", "Users", "test", "AppData", "Local")
		t.Setenv("LOCALAPPDATA", root)
		dir, err := defaultBinDir("windows")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(root, "Programs", "upvert"), dir)
	})

	t.Run("windows missing localappdata", func(t *testing.T) {
		t.Setenv("LOCALAPPDATA", "")
		_, err := defaultBinDir("windows")
		envErr, ok := errors.AsType[*EnvError](err)
		require.True(t, ok)
		assert.Equal(t, "LOCALAPPDATA", envErr.Name)
	})
}

func TestPathContains(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bin")
	other := filepath.Join(t.TempDir(), "other")
	pathEnv := strings.Join([]string{other, dir}, string(os.PathListSeparator))

	assert.True(t, pathContains(pathEnv, dir))
	assert.False(t, pathContains(pathEnv, filepath.Join(t.TempDir(), "missing")))
	assert.False(t, pathContains("", dir))
}
