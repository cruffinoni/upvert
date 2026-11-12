package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNext(t *testing.T) {
	tests := []struct {
		name    string
		current string
		kind    Kind
		pre     string
		want    string
	}{
		{"patch", "1.0.0", Patch, "", "1.0.1"},
		{"minor", "1.0.0", Minor, "", "1.1.0"},
		{"major", "1.0.0", Major, "", "2.0.0"},
		{"v prefix", "v1.2.3", Patch, "", "1.2.4"},
		{"prerelease stripped", "1.2.3-alpha.1", Patch, "", "1.2.4"},
		{"new prerelease", "1.0.0", Patch, "alpha", "1.0.1-alpha.1"},
		{"start prerelease on new core", "1.0.1-alpha.1", Patch, "alpha", "1.0.2-alpha.1"},
		{"label change starts new core", "1.0.1-alpha.2", Patch, "beta", "1.0.2-beta.1"},
		{"label change", "1.0.0-alpha.2", Minor, "beta", "1.1.0-beta.1"},
		{"minor restarts counter", "1.0.1-alpha.2", Minor, "alpha", "1.1.0-alpha.1"},
		{"major prerelease", "1.9.9", Major, "rc", "2.0.0-rc.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Next(tt.current, tt.kind, tt.pre)
			require.NoErrorf(t, err, "Next(%q, %q, %q)", tt.current, tt.kind, tt.pre)
			assert.Equalf(t, tt.want, got, "Next(%q, %q, %q)", tt.current, tt.kind, tt.pre)
		})
	}
}

func TestContinuePreRelease(t *testing.T) {
	tests := []struct {
		name    string
		current string
		label   string
		want    string
	}{
		{"same label", "1.0.1-alpha.1", "alpha", "1.0.1-alpha.2"},
		{"reported case", "0.0.4-alpha.4", "alpha", "0.0.4-alpha.5"},
		{"label change", "1.0.1-alpha.2", "beta", "1.0.1-beta.1"},
		{"no counter", "1.0.1-rc", "rc", "1.0.1-rc.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ContinuePreRelease(tt.current, tt.label)
			require.NoErrorf(t, err, "ContinuePreRelease(%q, %q)", tt.current, tt.label)
			assert.Equalf(t, tt.want, got, "ContinuePreRelease(%q, %q)", tt.current, tt.label)
		})
	}
}

func TestContinuePreReleaseErrors(t *testing.T) {
	_, err := ContinuePreRelease("1.0.0", "alpha")
	require.ErrorContains(t, err, "not a prerelease")
	_, err = ContinuePreRelease("1.0.1-alpha.1", "bad label")
	require.ErrorContains(t, err, "invalid pre-release label")
	_, err = ContinuePreRelease("not-a-version", "alpha")
	require.Error(t, err)
}

func TestPromotePreRelease(t *testing.T) {
	tests := []struct {
		name    string
		current string
		want    string
	}{
		{"alpha", "1.0.1-alpha.2", "1.0.1"},
		{"rc", "0.0.4-rc.1", "0.0.4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PromotePreRelease(tt.current)
			require.NoErrorf(t, err, "PromotePreRelease(%q)", tt.current)
			assert.Equalf(t, tt.want, got, "PromotePreRelease(%q)", tt.current)
		})
	}
}

func TestPromotePreReleaseErrors(t *testing.T) {
	_, err := PromotePreRelease("1.0.0")
	require.ErrorContains(t, err, "not a prerelease")
	_, err = PromotePreRelease("not-a-version")
	require.Error(t, err)
}

func TestNextErrors(t *testing.T) {
	tests := []struct {
		name    string
		current string
		kind    Kind
	}{
		{"invalid version", "not-a-version", Patch},
		{"unknown kind", "1.0.0", Kind("nope")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Next(tt.current, tt.kind, "")
			require.Error(t, err)
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"2.0.0", "1.9.9", 1},
		{"v1.2.3", "1.2.3", 0},
		{"1.0.0", "1.0.0-alpha", 1},
		{"1.0.0-alpha", "1.0.0", -1},
		{"1.0.0-alpha", "1.0.0-beta", -1},
		{"1.0.0-alpha.2", "1.0.0-alpha.10", -1},
		{"1.0.0-alpha.1", "1.0.0-alpha.1", 0},
	}
	for _, tt := range tests {
		got, err := Compare(tt.a, tt.b)
		require.NoErrorf(t, err, "Compare(%q, %q)", tt.a, tt.b)
		assert.Equalf(t, tt.want, got, "Compare(%q, %q)", tt.a, tt.b)
	}
}

func TestSplitPreRelease(t *testing.T) {
	tests := []struct {
		pre     string
		label   string
		counter int
	}{
		{"", "", 0},
		{"alpha", "alpha", 0},
		{"alpha.4", "alpha", 4},
		{"rc.10", "rc", 10},
	}
	for _, tt := range tests {
		label, counter := splitPreRelease(tt.pre)
		assert.Equal(t, tt.label, label)
		assert.Equal(t, tt.counter, counter)
	}
}
