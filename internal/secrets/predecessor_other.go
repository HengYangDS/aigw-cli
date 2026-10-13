//go:build !darwin

package secrets

// RestorePredecessorCredentials needs no conversion where the native store
// authorizes the user or session rather than the reading executable.
func RestorePredecessorCredentials(Store, string, []string) error { return nil }
