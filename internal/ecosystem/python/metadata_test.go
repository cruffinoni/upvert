package python

import "testing"

func TestSetupCfgVersionHandlesPythonFormat(t *testing.T) {
	content := "[metadata]\nname = demo\nversion = 1.2.3 # release\nclassifiers =\n    Programming Language :: Python\n\n[options.extras_require]\ndev =\n    pytest\n"
	if got := setupCfgVersion(content); got != "1.2.3" {
		t.Fatalf("setupCfgVersion = %q, want 1.2.3", got)
	}
}
