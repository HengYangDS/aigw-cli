package update

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/secrets"
)

func TestHomebrewRollbackDoesNotReleaseCredentialsBeforeRetryCapability(t *testing.T) {
	cfg := configuration.NewConfig(clients.Admission())
	cfg.Accounts["test"] = configuration.Account{Label: "Test", Endpoints: configuration.Endpoints{OpenAIResponses: "https://endpoint.test/v1"}}
	cfg.Routes["test"] = configuration.Route{Label: "Test", Account: "test", Model: "gpt-6.1-sol", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}}}
	zero := uint64(0)
	cfg.Clients[configuration.ClientCodex] = configuration.ClientBinding{Route: "test", RetryPolicy: &configuration.RetryPolicy{RequestMaxRetries: &zero}}
	store := configuration.NewStore(filepath.Join(t.TempDir(), "aigw.toml"), clients.Admission())
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	reads := 0
	runtime := invocation.Context{Clients: clients.Registry(), Config: store, Secrets: secrets.NewMemoryStore(), Out: io.Discard, ResolveProgram: func() (credential.Program, error) {
		reads++
		return credential.Program{}, errors.New("credential reader must not be resolved")
	}}
	err = prepareHomebrewRollback(runtime)
	if err == nil || !strings.Contains(err.Error(), "retry") || reads != 0 {
		t.Fatalf("unproved rollback reached credential custody or lost its retry boundary: %v (reads=%d)", err, reads)
	}
	after, err := store.CaptureSnapshot()
	if err != nil || !before.Config.Equal(after.Config) {
		t.Fatalf("refused rollback changed policy: %v", err)
	}
}

func TestHomebrewRollbackRefusesUnreadableRetryPolicyBeforeCustody(t *testing.T) {
	store := configuration.NewStore(filepath.Join(t.TempDir(), "config.toml"), clients.Admission())
	if err := os.WriteFile(filepath.Join(filepath.Dir(store.Path()), "config.retries.toml"), []byte("["), 0o600); err != nil {
		t.Fatal(err)
	}
	reads := 0
	runtime := invocation.Context{Clients: clients.Registry(), Config: store, Secrets: secrets.NewMemoryStore(), Out: io.Discard, ResolveProgram: func() (credential.Program, error) {
		reads++
		return credential.Program{}, errors.New("credential reader must not be resolved")
	}}
	if err := prepareHomebrewRollback(runtime); err == nil || reads != 0 {
		t.Fatalf("unreadable policy reached custody: %v (reads=%d)", err, reads)
	}
}
