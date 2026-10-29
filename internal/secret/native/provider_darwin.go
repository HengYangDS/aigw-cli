//go:build darwin

package native

func observeCredential(service, account string) (bool, error) {
	return observeCredentialInKeychain(service, account, "")
}

func readCredential(service, account string, role Role) (string, error) {
	value, err := readCredentialWithMigration(service, account, "", role)
	return string(value), err
}

func writeCredential(service, account string, value []byte, role Role) error {
	return writeCredentialWithMigration(service, account, "", value, role)
}

func deleteCredential(service, account string) error {
	return deleteCredentialWithMigration(service, account, "")
}

func restoreCredential(service, account string) error {
	return restoreCredentialForPredecessor(service, account, "")
}

func reclaimCredential(service, account string) error {
	return reclaimCredentialFromPredecessor(service, account, "")
}

// Restore returns one exact AIGW-owned item to the predecessor's credential
// reader through a bounded credential subprocess, before a portable rollback.
func Restore(executable, service, account string) error {
	_, err := invoke(executable, restoreCommand, service, account, "")
	return err
}

// Reclaim returns one exact item that Restore gave to the predecessor's reader
// to AIGW ownership through a bounded credential subprocess, when the rollback
// does not activate the predecessor.
func Reclaim(executable, service, account string) error {
	_, err := invoke(executable, reclaimCommand, service, account, "")
	return err
}

func nativeEnvironment(getenv func(string) string) []string {
	return retainedEnvironment(getenv, "HOME")
}
