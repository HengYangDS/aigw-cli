package native

import (
	"errors"

	keyring "github.com/zalando/go-keyring"
)

func queryCredential(operation, service, account string, input []byte) ([]byte, error) {
	switch operation {
	case readCommand:
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
	case existsCommand:
		present, err := observeCredential(service, account)
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
