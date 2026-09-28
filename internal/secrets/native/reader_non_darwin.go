//go:build !darwin

package native

import keyring "github.com/zalando/go-keyring"

func readCredential(service, account string) (string, error) {
	return keyring.Get(service, account)
}
