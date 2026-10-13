//go:build darwin && !cgo

package native

func observeCredentialInKeychain(string, string, string) (bool, error) {
	return false, ErrUnavailable
}

func readCredentialWithMigration(string, string, string) ([]byte, error) {
	return nil, ErrUnavailable
}

func writeCredentialWithMigration(string, string, string, []byte) error {
	return ErrUnavailable
}

func deleteCredentialWithMigration(string, string, string) error {
	return ErrUnavailable
}

func restoreCredentialForPredecessor(string, string, string) error {
	return ErrUnavailable
}
