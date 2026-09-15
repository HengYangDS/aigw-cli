//go:build !darwin

package keychain

func queryNative(string, string, string, []byte) ([]byte, error) {
	return nil, ErrUnavailable
}
