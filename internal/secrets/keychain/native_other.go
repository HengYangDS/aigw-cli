//go:build !darwin

package keychain

func queryNative(string, string, bool) ([]byte, error) {
	return nil, ErrUnavailable
}
