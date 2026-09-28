//go:build darwin

package native

func observeCredential(service, account string) (bool, error) {
	return observeCredentialInKeychain(service, nativeKeychainSlot(account), "")
}

func readCredential(service, account string) (string, error) {
	value, err := readCredentialFromKeychain(service, nativeKeychainSlot(account), "")
	return string(value), err
}

func writeCredential(service, account string, value []byte) error {
	return writeCredentialToKeychain(service, nativeKeychainSlot(account), "", value)
}

func deleteCredential(service, account string) error {
	return deleteCredentialFromKeychain(service, nativeKeychainSlot(account), "")
}

func nativeKeychainSlot(account string) string {
	return "native@" + account
}

func nativeEnvironment(getenv func(string) string) []string {
	return retainedEnvironment(getenv, "HOME")
}
