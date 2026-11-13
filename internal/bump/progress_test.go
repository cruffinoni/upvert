package bump

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type progressCall struct {
	kind   string
	label  string
	detail string
}

type recordingProgress struct {
	calls []progressCall
}

func (r *recordingProgress) Start(label string) {
	r.calls = append(r.calls, progressCall{kind: "start", label: label})
}

func (r *recordingProgress) Total(int64)   {}
func (r *recordingProgress) Advance(int64) {}
func (r *recordingProgress) Pause()        {}
func (r *recordingProgress) Resume()       {}

func (r *recordingProgress) Finish(detail string) {
	r.calls = append(r.calls, progressCall{kind: "finish", detail: detail})
}

func (r *recordingProgress) Abort() {
	r.calls = append(r.calls, progressCall{kind: "abort"})
}

func TestBumpWriteReportsProgress(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n",
	})
	progress := &recordingProgress{}
	cfg := Config{Dir: dir, Mode: ModeWrite, Progress: progress}

	require.NoError(t, Apply(t.Context(), cfg, Patch))

	assert.Equal(t, []progressCall{
		{kind: "start", label: "writing version files"},
		{kind: "finish", detail: "package.json"},
		{kind: "start", label: "updating changelog"},
		{kind: "finish", detail: "CHANGELOG.md"},
	}, progress.calls)
}

func TestBumpCommitReportsProgress(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n",
	})
	progress := &recordingProgress{}
	cfg := Config{Dir: dir, Mode: ModeCommit, Progress: progress}

	require.NoError(t, Apply(t.Context(), cfg, Patch))

	assert.Equal(t, []progressCall{
		{kind: "start", label: "writing version files"},
		{kind: "finish", detail: "package.json"},
		{kind: "start", label: "updating changelog"},
		{kind: "finish", detail: "CHANGELOG.md"},
		{kind: "start", label: "committing"},
		{kind: "finish", detail: "chore: bump version to 1.2.4"},
		{kind: "start", label: "tagging"},
		{kind: "finish", detail: "v1.2.4"},
	}, progress.calls)
}

func TestBumpPushReportsProgress(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.0.0\"}\n",
	})
	remote := t.TempDir()
	gitRun(t, remote, "init", "-q", "--bare")
	gitRun(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")
	gitRun(t, dir, "remote", "add", "origin", remote)

	progress := &recordingProgress{}
	cfg := Config{Dir: dir, Mode: ModePush, Progress: progress}
	require.NoError(t, Apply(t.Context(), cfg, Patch))

	assert.Equal(t, []progressCall{
		{kind: "start", label: "writing version files"},
		{kind: "finish", detail: "package.json"},
		{kind: "start", label: "updating changelog"},
		{kind: "finish", detail: "CHANGELOG.md"},
		{kind: "start", label: "committing"},
		{kind: "finish", detail: "chore: bump version to 1.0.1"},
		{kind: "start", label: "tagging"},
		{kind: "finish", detail: "v1.0.1"},
		{kind: "start", label: "pushing commit"},
		{kind: "finish", detail: "origin HEAD"},
		{kind: "start", label: "pushing tag"},
		{kind: "finish", detail: "origin v1.0.1"},
	}, progress.calls)
}

func TestBumpPreviewDoesNotReportProgress(t *testing.T) {
	dir := initTestRepo(t, map[string]string{
		"package.json": "{\"name\":\"demo\",\"version\":\"1.2.3\"}\n",
	})
	progress := &recordingProgress{}
	cfg := Config{Dir: dir, Mode: ModePreview, Progress: progress, Stdout: io.Discard, Stderr: io.Discard}

	require.NoError(t, Apply(t.Context(), cfg, Patch))

	assert.Empty(t, progress.calls)
}

func TestRunStepAbortsOnError(t *testing.T) {
	progress := &recordingProgress{}
	svc := &service{progress: progress}
	wantErr := errors.New("boom")

	err := svc.runStep("tagging", "v1.0.0", func() error {
		return wantErr
	})

	require.ErrorIs(t, err, wantErr)
	assert.Equal(t, []progressCall{
		{kind: "start", label: "tagging"},
		{kind: "abort"},
	}, progress.calls)
}
