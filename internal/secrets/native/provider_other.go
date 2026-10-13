//go:build !darwin && !linux && !windows

package native

func queryCredential(string, string, string, []byte, Role) ([]byte, error) {
	return nil, ErrUnavailable
}

func nativeEnvironment(func(string) string) []string { return nil }
