//go:build darwin

package native

func observeCredential(service, account string) (bool, error) {
	return observeCredentialInKeychain(service, account, "")
}

func readCredential(service, account string) (string, error) {
	value, err := readCredentialWithMigration(service, account, "")
	return string(value), err
}

func writeCredential(service, account string, value []byte) error {
	return writeCredentialWithMigration(service, account, "", value)
}

func deleteCredential(service, account string) error {
	return deleteCredentialWithMigration(service, account, "")
}

func restoreCredential(service, account string) error {
	return restoreCredentialForPredecessor(service, account, "")
}

// Restore returns one exact AIGW-owned item to the predecessor's credential
// reader through a bounded credential subprocess, before a portable rollback.
func Restore(executable, service, account string) error {
	_, err := invoke(executable, restoreCommand, service, account, "")
	return err
}

func nativeEnvironment(getenv func(string) string) []string {
	return retainedEnvironment(getenv, "HOME")
}
