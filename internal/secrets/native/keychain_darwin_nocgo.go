//go:build darwin && !cgo

package native

func readCredentialFromKeychain(string, string, string) ([]byte, error) {
	return nil, ErrUnavailable
}

func writeCredentialToKeychain(string, string, string, []byte) error {
	return ErrUnavailable
}

func deleteCredentialFromKeychain(string, string, string) error {
	return ErrUnavailable
}
