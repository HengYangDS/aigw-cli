//go:build darwin

package keychain

import (
	"errors"
	"testing"

	keyring "github.com/zalando/go-keyring"
)

func TestProviderPreservesLogicalValueAcrossLifecycle(t *testing.T) {
	keyring.MockInit()
	const service, account, token = "AIGW_TOKEN", "provider-test", "line one\nline two"

	if _, err := queryKeyring(writeCommand, service, account, []byte(token)); err != nil {
		t.Fatal(err)
	}
	if stored, err := keyring.Get(service, account); err != nil || stored != token {
		t.Fatalf("stored value = %q, %v", stored, err)
	}
	if value, err := queryKeyring(workerCommand, service, account, nil); err != nil || string(value) != token {
		t.Fatalf("read value = %q, %v", value, err)
	}
	if _, err := queryKeyring(deleteCommand, service, account, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := queryKeyring(deleteCommand, service, account, nil); err != nil {
		t.Fatalf("repeated delete = %v", err)
	}
	if _, err := queryKeyring(workerCommand, service, account, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing read = %v", err)
	}
}
