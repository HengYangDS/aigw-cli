//go:build darwin

package native

import (
	"errors"
	"strings"
)

func queryCredential(operation, service, account string, input []byte) ([]byte, error) {
	switch operation {
	case readCommand:
		return readCredentialFromKeychain(service, account, "")
	case writeCommand:
		return nil, writeCredentialToKeychain(service, account, "", input)
	case deleteCommand:
		err := deleteCredentialFromKeychain(service, account, "")
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	case listCommand:
		names, err := listAccountsInKeychain(service, "")
		return []byte(strings.Join(names, "\n")), err
	case existsCommand:
		present, err := observeCredentialInKeychain(service, account, "")
		if err != nil {
			return nil, err
		}
		if present {
			return []byte("1"), nil
		}
		return []byte("0"), nil
	default:
		return nil, ErrUnavailable
	}
}

func nativeEnvironment(getenv func(string) string) []string {
	return retainedEnvironment(getenv, "HOME")
}
