//go:build darwin || windows

package native

import (
	"errors"
)

func queryCredential(operation, service, account string, input []byte) ([]byte, error) {
	switch operation {
	case readCommand:
		value, err := readCredential(service, account)
		return []byte(value), err
	case writeCommand:
		return nil, writeCredential(service, account, input)
	case deleteCommand:
		err := deleteCredential(service, account)
		if errors.Is(err, ErrNotFound) {
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
