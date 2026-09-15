package secrets

import (
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"aigw-cli/internal/secrets/keychain"

	keyring "github.com/zalando/go-keyring"
)

// DecodeKeyringBase64 decodes one explicitly selected macOS go-keyring storage envelope.
// Native stored-value decoding is separate; raw stdin never calls this decoder.
func DecodeKeyringBase64(value string) (string, error) {
	const prefix = "go-keyring-base64:"
	encoded, present := strings.CutPrefix(value, prefix)
	if !present || encoded == "" {
		return "", errors.New("Token input requires a non-empty go-keyring-base64 envelope")
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || base64.StdEncoding.EncodeToString(decoded) != encoded {
		return "", errors.New("Token input has a noncanonical go-keyring-base64 envelope")
	}
	if strings.HasPrefix(string(decoded), prefix) {
		return "", errors.New("Token input contains a nested go-keyring-base64 envelope")
	}
	return string(decoded), nil
}

// keyringStore persists credentials in the operating system's native credential service.
type keyringStore struct {
	observe func(service, slot string) (bool, error)
	read    func(service, slot string) (string, error)
	write   func(service, slot, value string) error
	remove  func(service, slot string) error
}

func newKeyringStore() keyringStore {
	store := keyringStore{observe: observeKeyringItem, read: keyring.Get, write: keyring.Set, remove: keyring.Delete}
	if runtime.GOOS == "darwin" {
		store.read = readNativeKeychain
		store.write = func(service, slot, value string) error {
			return keychain.Write(service, slot, "go-keyring-base64:"+base64.StdEncoding.EncodeToString([]byte(value)))
		}
		store.remove = keychain.Delete
	}
	return store
}

func readNativeKeychain(service, slot string) (string, error) {
	value, err := keychain.Read(service, slot)
	if errors.Is(err, keychain.ErrNotFound) {
		return "", keyring.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return keychain.DecodeStoredValue(value)
}

func (store keyringStore) get(kind Kind, account string) (string, error) {
	slot := slotName(kind, account)
	value, err := store.read(Service, slot)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read %s/%s from system keyring: %w", Service, slot, err)
	}
	if value == "" {
		return "", ErrNotFound
	}
	return value, nil
}

func (store keyringStore) set(kind Kind, account, value string) error {
	slot := slotName(kind, account)
	if err := store.write(Service, slot, value); err != nil {
		return fmt.Errorf("write %s/%s to system keyring: %w", Service, slot, err)
	}
	return nil
}

func (store keyringStore) delete(kind Kind, account string) error {
	slot := slotName(kind, account)
	err := store.remove(Service, slot)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete %s/%s from system keyring: %w", Service, slot, err)
	}
	return nil
}

func (store keyringStore) exists(kind Kind, account string) (bool, error) {
	present, err := store.observe(Service, slotName(kind, account))
	if err != nil {
		return false, fmt.Errorf("observe %s/%s in system keyring: %w", Service, slotName(kind, account), err)
	}
	return present, nil
}
