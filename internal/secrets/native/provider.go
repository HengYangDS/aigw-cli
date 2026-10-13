//go:build darwin || windows

package native

import (
	"errors"
)

func queryCredential(operation, service, account string, input []byte, role Role) ([]byte, error) {
	switch operation {
	case readCommand:
		value, err := readCredential(service, account, role)
		return []byte(value), err
	case writeCommand:
		return nil, writeCredential(service, account, input, role)
	case deleteCommand:
		err := deleteCredential(service, account)
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	case restoreCommand, reclaimCommand:
		if role == OtherProgram {
			return nil, ErrUnavailable
		}
		if operation == restoreCommand {
			return nil, restoreCredential(service, account)
		}
		return nil, reclaimCredential(service, account)
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
