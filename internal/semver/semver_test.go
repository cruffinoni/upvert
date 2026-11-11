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
		{"continue prerelease", "1.0.1-alpha.1", Patch, "alpha", "1.0.2-alpha.2"},
		{"label change", "1.0.0-alpha.2", Minor, "beta", "1.1.0-beta.1"},
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
