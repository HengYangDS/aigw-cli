package app

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	clientactivation "aigw-cli/internal/app/activation"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// refusedToken is a credential store that answers the observation of every
// Token slot as the system keyring answers for a Keychain item that macOS
// refuses AIGW: there when it is observed, and no Token that AIGW can use.
type refusedToken struct {
	secret.Store
	refusal *secret.KeychainItemRefusedError
}

func (store refusedToken) Exists(string) (bool, error) { return true, nil }

func (store refusedToken) ObserveToken(string) secret.TokenObservation {
	return secret.TokenObservation{Present: true, Refusal: store.refusal}
}

// An enabled client whose provider the activation did not observe is
// classified from the same observation of its Token as every other client. A
// slot that is there and holds nothing is no Token, with the step that stores
// one, and a Keychain item that macOS refuses AIGW names itself and its fix,
// as the activation's own observation of such an item does.
func TestCredentialFactsObserveATokenThatTheActivationDidNotObserve(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	files, err := secret.Select(secret.Selection{Backend: "file", GOOS: runtime.GOOS, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := files.Set("gateway", "public-token"); err != nil {
		t.Fatal(err)
	}
	// It observed no provider: its configuration holds none.
	activation := clientactivation.AssessActivation(domain.NewConfig(clients.Admission()), files)
	observe := func(store secret.Store) domainreadiness.ClientFacts {
		facts := domainreadiness.ClientFacts{CredentialRequired: true, ProjectionEnabled: true}
		applyCredentialFacts(&facts, &activation, store, "codex", "gateway", "")
		return facts
	}

	if facts := observe(files); !facts.CredentialAvailable || facts.CredentialAction != "" || facts.CredentialObservationIssue != "" {
		t.Fatalf("a stored Token = %+v", facts)
	}
	// The selected file backend keeps its Token files in the folder tokens.
	if err := os.Truncate(filepath.Join(root, "tokens", "gateway"), 0); err != nil {
		t.Fatal(err)
	}
	if facts := observe(files); facts.CredentialAvailable || facts.CredentialAction != "run `aigw provider rotate gateway`" || facts.CredentialObservationIssue != "" {
		t.Fatalf("an empty Token = %+v", facts)
	}
	refusal := &secret.KeychainItemRefusedError{Kind: secret.APIToken, Provider: "gateway", Status: -25293}
	if facts := observe(refusedToken{refusal: refusal}); facts.CredentialAvailable || facts.CredentialAction != "" ||
		facts.CredentialObservationIssue != refusal.Title() || facts.CredentialObservationAction != refusal.Fix() {
		t.Fatalf("an item that macOS refuses = %+v", facts)
	}
}
