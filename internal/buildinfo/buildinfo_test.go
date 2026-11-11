package buildinfo

import "testing"

func TestCurrentUsesInjectedVersion(t *testing.T) {
	previous := Version
	t.Cleanup(func() { Version = previous })
	Version = "1.2.3"
	if got := Current(); got != "1.2.3" {
		t.Fatalf("Current() = %q, want 1.2.3", got)
	}
}
