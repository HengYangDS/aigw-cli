//go:build darwin && keychain_integration

package main

import (
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/secrets/keychain"
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseCredentialWorkerObservesOnlyAnAbsentItem(t *testing.T) {
	if os.Getenv("AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE") != "ephemeral-host" {
		t.Fatal("Keychain integration requires an explicitly admitted ephemeral-host")
	}
	account := "aigw-release-absent-" + filepath.Base(t.TempDir())
	if exists, err := keychain.Exists(secrets.Service, account); err != nil || exists {
		t.Fatalf("release credential worker observation: exists=%t error=%v", exists, err)
	}
}
