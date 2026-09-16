//go:build !darwin

package keychain

func queryKeyring(string, string, string, []byte) ([]byte, error) {
	return nil, ErrUnavailable
}
