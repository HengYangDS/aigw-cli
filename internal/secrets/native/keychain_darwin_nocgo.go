//go:build darwin && !cgo

package native

func readCredentialFromKeychain(string, string, string) ([]byte, error) {
	return nil, ErrUnavailable
}
