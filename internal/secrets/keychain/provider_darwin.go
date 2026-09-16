//go:build darwin

package keychain

import (
	"errors"

	keyring "github.com/zalando/go-keyring"
)

func queryKeyring(operation, service, account string, input []byte) ([]byte, error) {
	switch operation {
	case workerCommand:
		value, err := keyring.Get(service, account)
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, ErrNotFound
		}
		return []byte(value), err
	case writeCommand:
		return nil, keyring.Set(service, account, string(input))
	case deleteCommand:
		err := keyring.Delete(service, account)
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	default:
		return nil, ErrUnavailable
	}
}
