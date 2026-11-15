package hermes

import (
	"path/filepath"
	"slices"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// observedTokens is a credential store that answers the observation of each
// Token slot from a table, as the system keyring answers: a slot of the table
// is there, and whether AIGW can use it is the table's answer.
type observedTokens struct {
	secret.Store
	slots map[string]secret.TokenObservation
}

func (store observedTokens) Exists(account string) (bool, error) {
	return store.slots[account].Present, nil
}

func (store observedTokens) ObserveToken(account string) (secret.TokenObservation, error) {
	return store.slots[account], nil
}

// Hermes lists the providers that it can reach beside the selected one: those
// whose Token AIGW can use. A provider whose slot is there and holds nothing,
// or holds a Keychain item that macOS refuses AIGW, is left out, as one
// without a Token is: its credential command would return no Token. The
// selected provider is listed whatever its slot holds, because the readiness
// of the client's own Token decides whether Hermes's file is written at all.
func TestHermesListsOnlyProvidersWhoseTokenAIGWCanUse(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	for _, provider := range []string{"selected", "stored", "empty", "refused", "missing"} {
		cfg.Providers[provider] = domain.Provider{Label: provider, Endpoints: domain.Endpoints{Responses: "https://" + provider + ".test/v1"}}
		cfg.Models[domain.NewModelRef(provider, "gpt-"+provider)] = qualifiedModel(provider, provider, "gpt-"+provider, domain.ProtocolResponses)
	}
	cfg.SetSelectedModel(ID, domain.NewModelRef("selected", "gpt-selected"), "")
	selected, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	refusal := &secret.KeychainItemRefusedError{Kind: secret.APIToken, Provider: "refused", Status: -25293}
	store := observedTokens{slots: map[string]secret.TokenObservation{
		"selected": {Present: true, Empty: true},
		"stored":   {Present: true},
		"empty":    {Present: true, Empty: true},
		"refused":  {Present: true, Refusal: refusal},
	}}
	desired, err := hermesDesired(adapter.Dependencies{Secrets: store, AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}, cfg, selected)
	if err != nil {
		t.Fatal(err)
	}
	var listed []string
	for _, provider := range desired.Providers {
		listed = append(listed, provider.ID)
	}
	slices.Sort(listed)
	if want := []string{"aigw-selected-responses", "aigw-stored-responses"}; !slices.Equal(listed, want) {
		t.Fatalf("Hermes lists %q; want %q", listed, want)
	}
}
