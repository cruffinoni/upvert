// Package semver implements semantic-version parsing, bump arithmetic, and
// comparison.
package semver

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// Kind selects which component of a version is incremented.
type Kind string

// Bump kinds supported by the CLI.
const (
	Patch Kind = "patch"
	Minor Kind = "minor"
	Major Kind = "major"
)

// Parse parses a semantic version, accepting an optional leading v prefix.
func Parse(s string) (*semver.Version, error) {
	v, err := semver.NewVersion(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("invalid semantic version %q: %w", s, err)
	}
	return v, nil
}

// bumpCore increments the requested component, resets the lower ones, and
// drops any pre-release suffix. The intermediate version has no pre-release
// suffix so IncPatch still advances the patch component.
func bumpCore(v *semver.Version, kind Kind) *semver.Version {
	base := semver.New(v.Major(), v.Minor(), v.Patch(), "", "")
	switch kind {
	case Major:
		bumped := base.IncMajor()
		return &bumped
	case Minor:
		bumped := base.IncMinor()
		return &bumped
	default:
		bumped := base.IncPatch()
		return &bumped
	}
}

// splitPreRelease splits a pre-release suffix such as "alpha.2" into its
// label and numeric counter.
func splitPreRelease(pre string) (string, int) {
	if pre == "" {
		return "", 0
	}
	label := pre
	counter := 0
	if idx := strings.LastIndex(pre, "."); idx >= 0 {
		label = pre[:idx]
		if n, err := strconv.Atoi(pre[idx+1:]); err == nil {
			counter = n
		}
	}
	return label, counter
}

// nextPreRelease advances the numeric core and applies the requested
// pre-release label. The counter continues when the current version already
// uses the same label.
func nextPreRelease(v *semver.Version, kind Kind, label string) (string, error) {
	base := bumpCore(v, kind)
	counter := 1
	if currentLabel, currentCounter := splitPreRelease(v.Prerelease()); currentLabel == label {
		counter = currentCounter + 1
	}
	next, err := base.SetPrerelease(fmt.Sprintf("%s.%d", label, counter))
	if err != nil {
		return "", fmt.Errorf("invalid pre-release label %q: %w", label, err)
	}
	return next.String(), nil
}

// ValidatePreReleaseLabel checks that label can be used as a pre-release
// identifier without creating an invalid version.
func ValidatePreReleaseLabel(label string) error {
	v, err := semver.NewVersion("0.0.0-" + label)
	if err != nil {
		return fmt.Errorf("invalid pre-release label %q: %w", label, err)
	}
	if v.Prerelease() != label || v.Metadata() != "" {
		return fmt.Errorf("invalid pre-release label %q", label)
	}
	return nil
}

// Valid reports whether k is a supported bump kind.
func (k Kind) Valid() bool {
	switch k {
	case Patch, Minor, Major:
		return true
	default:
		return false
	}
}

// Next returns the version that follows current for kind. When preRelease is
// set, the result carries a -label.counter suffix.
func Next(current string, kind Kind, preRelease string) (string, error) {
	v, err := Parse(current)
	if err != nil {
		return "", err
	}
	if !kind.Valid() {
		return "", fmt.Errorf("invalid bump kind %q", kind)
	}
	if preRelease != "" {
		return nextPreRelease(v, kind, preRelease)
	}
	return bumpCore(v, kind).String(), nil
}

// Compare returns -1, 0, or 1 depending on whether a is lower than, equal to,
// or higher than b under semantic version precedence.
func Compare(a, b string) (int, error) {
	va, err := Parse(a)
	if err != nil {
		return 0, err
	}
	vb, err := Parse(b)
	if err != nil {
		return 0, err
	}
	return va.Compare(vb), nil
}
