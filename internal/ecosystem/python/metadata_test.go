package python

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupCfgVersionHandlesPythonFormat(t *testing.T) {
	content := "[metadata]\nname = demo\nversion = 1.2.3 # release\nclassifiers =\n    Programming Language :: Python\n\n[options.extras_require]\ndev =\n    pytest\n"
	assert.Equal(t, "1.2.3", setupCfgVersion(content))
}
