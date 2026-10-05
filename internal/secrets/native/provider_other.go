//go:build !darwin && !linux && !windows

package native

func queryCredential(string, string, string, []byte) ([]byte, error) { return nil, ErrUnavailable }

func nativeEnvironment(func(string) string) []string { return nil }
