//go:build !windows

package update

// ensureUserPath leaves PATH unchanged outside Windows.
func ensureUserPath(_ string) (bool, error) {
	return false, nil
}
