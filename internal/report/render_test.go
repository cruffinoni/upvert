package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func render(t *testing.T, r Report, opts Options) string {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, Render(&b, r, opts))
	return b.String()
}

func TestRenderPreviewTagSourced(t *testing.T) {
	got := render(t, Report{
		Command: "patch",
		Mode:    "preview",
		From:    "0.0.4-alpha.4",
		To:      "0.0.5",
		Source:  "git tags",
		Changes: []Change{
			{Path: "CHANGELOG.md", Detail: "entry for 0.0.5"},
		},
		Release: []Step{
			{Label: "tag", Value: "v0.0.5"},
			{Label: "commit", Value: "chore: bump version to 0.0.5"},
		},
		Footer: "Nothing was written. Re-run with --commit or --push.",
	}, Options{Width: 80})

	want := "" +
		"upvert patch (preview)\n" +
		"  Version   0.0.4-alpha.4 → 0.0.5\n" +
		"  Source    git tags\n" +
		"\n" +
		"  Changes\n" +
		"    CHANGELOG.md              entry for 0.0.5\n" +
		"\n" +
		"  Release\n" +
		"    tag      v0.0.5\n" +
		"    commit   chore: bump version to 0.0.5\n" +
		"\n" +
		"  Nothing was written. Re-run with --commit or --push.\n"
	assert.Equal(t, want, got)
}

func TestRenderCommitWithHooksAndNotes(t *testing.T) {
	got := render(t, Report{
		Command: "minor",
		Mode:    "commit",
		From:    "1.2.3",
		To:      "1.3.0",
		Source:  "package.json",
		Changes: []Change{
			{Path: "package.json"},
			{Path: "CHANGELOG.md", Detail: "entry for 1.3.0"},
		},
		Hooks: []Step{
			{Label: "pre-bump", Value: "ran"},
			{Label: "post-bump", Value: "ran"},
		},
		Release: []Step{
			{Label: "tag", Value: "v1.3.0"},
			{Label: "commit", Value: "chore: bump version to 1.3.0"},
		},
		Notes:  []string{"changelog skipped (--no-changelog)"},
		Footer: "Committed and tagged v1.3.0.\nPush with: git push origin HEAD && git push origin v1.3.0",
	}, Options{Width: 80})

	for _, want := range []string{
		"upvert minor (commit)",
		"    package.json\n",
		"    CHANGELOG.md              entry for 1.3.0\n",
		"  Hooks\n",
		"    pre-bump    ran\n",
		"    post-bump   ran\n",
		"  Release\n",
		"    tag      v1.3.0\n",
		"  Notes\n",
		"    changelog skipped (--no-changelog)\n",
		"  Committed and tagged v1.3.0.\n",
		"  Push with: git push origin HEAD && git push origin v1.3.0\n",
	} {
		assert.Contains(t, got, want)
	}
}

func TestRenderASCII(t *testing.T) {
	got := render(t, Report{
		Command: "patch",
		Mode:    "write",
		From:    "1.0.0",
		To:      "1.0.1",
		Source:  "Cargo.toml",
		Changes: []Change{{Path: "Cargo.toml"}},
		Footer:  "Files updated. Git was left untouched.",
	}, Options{Width: 80, ASCII: true})
	assert.Contains(t, got, "  Version   1.0.0 -> 1.0.1\n")
	assert.NotContains(t, got, "→")
}

func TestRenderEmptyChanges(t *testing.T) {
	got := render(t, Report{
		Command: "patch",
		Mode:    "preview",
		From:    "1.0.0",
		To:      "1.0.1",
		Source:  "git tags",
		Footer:  "Nothing was written. Re-run with --commit or --push.",
	}, Options{Width: 80})
	assert.Contains(t, got, "  Changes\n    (none)\n")
}

func TestRenderWrapsLongDetail(t *testing.T) {
	got := render(t, Report{
		Command: "major",
		Mode:    "preview",
		From:    "1.0.0",
		To:      "2.0.0",
		Source:  "package.json",
		Changes: []Change{{Path: "package.json", Detail: "this detail is deliberately much longer than the remaining column width allows"}},
		Footer:  "Nothing was written.",
	}, Options{Width: 60})
	for _, line := range strings.Split(got, "\n") {
		assert.LessOrEqual(t, len([]rune(line)), 60, line)
	}
}
