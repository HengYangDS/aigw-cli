//go:build !darwin && !linux && !windows

package native

func observeCredential(string, string) (bool, error) { return false, ErrUnavailable }

func nativeEnvironment(func(string) string) []string { return nil }
