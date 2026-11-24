package adapter_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"aigw-cli/internal/clients"
	"aigw-cli/internal/secret"
)

// observedToken is a credential store that answers the observation of every
// Token slot with one answer, as the system keyring answers for a Keychain
// item that macOS refuses AIGW: there when it is observed, and no Token that
// AIGW can use.
type observedToken struct {
	secret.Store
	observation secret.TokenObservation
}

func (store observedToken) Exists(string) (bool, error) { return store.observation.Present, nil }

func (store observedToken) ObserveToken(string) secret.TokenObservation {
	return store.observation
}

// emptyTokenFile is the file backend with the Token file of account present
// and empty.
func emptyTokenFile(t *testing.T, account string) secret.Store {
	t.Helper()
	root := filepath.Join(t.TempDir(), "secrets")
	files, err := secret.Select(secret.Selection{Backend: "file", GOOS: runtime.GOOS, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := files.Set(account, "public-token"); err != nil {
		t.Fatal(err)
	}
	// The selected file backend keeps its Token files in the folder tokens.
	if err := os.Truncate(filepath.Join(root, "tokens", account), 0); err != nil {
		t.Fatal(err)
	}
	return files
}

// Every client adapter adopts what discovery found, here the installed
// executable, only for a client that can work: one whose provider has a Token
// that AIGW can use. A slot that is there and holds nothing, a Keychain item
// that macOS refuses AIGW, and a Token that the store cannot observe, as a
// locked keychain leaves it, leave the client's activation as it was, as a
// missing Token does, and fail nothing.
func TestConvergenceLeavesAClientWithoutAUsableTokenAsItWas(t *testing.T) {
	registry := clients.Registry()
	stored := secret.NewMemoryStore()
	if err := stored.Set("gateway", "public-token"); err != nil {
		t.Fatal(err)
	}
	for _, id := range registry.IDs() {
		for _, test := range []struct {
			name   string
			store  secret.Store
			usable bool
		}{
			{name: "a missing Token", store: secret.NewMemoryStore()},
			{name: "an empty Token", store: emptyTokenFile(t, "gateway")},
			{name: "an item that macOS refuses", store: observedToken{observation: secret.TokenObservation{
				Present: true, Refusal: &secret.KeychainItemRefusedError{Kind: secret.APIToken, Provider: "gateway", Status: -25293},
			}}},
			{name: "a Token that cannot be observed", store: observedToken{observation: secret.TokenObservation{Unobserved: errors.New("the keychain is locked")}}},
			{name: "a stored Token", store: stored, usable: true},
		} {
			t.Run(id+"/"+test.name, func(t *testing.T) {
				cfg, deps, discovered := configuredClient(t, id)
				// The configured executable is gone, so a client that is ready
				// takes the one that discovery found.
				binding := cfg.Clients[id]
				binding.Executable = filepath.Join(t.TempDir(), "gone")
				cfg.Clients[id] = binding
				deps.Secrets = test.store
				after, err := registry.Converge(deps, cfg, discovered, id)
				if err != nil {
					t.Fatal(err)
				}
				if adopted := after.Clients[id].Executable == discovered.Executables[id]; adopted != test.usable {
					t.Fatalf("the client adopted the discovered executable %t, want %t: %+v", adopted, test.usable, after.Clients[id])
				}
			})
		}
	}
}
