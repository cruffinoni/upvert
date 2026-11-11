package buildinfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCurrentUsesInjectedVersion(t *testing.T) {
	previous := Version
	t.Cleanup(func() { Version = previous })
	Version = "1.2.3"
	assert.Equal(t, "1.2.3", Current())
}
