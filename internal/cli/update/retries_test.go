package update

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"

	"aigw-cli/internal/store"
)

func TestHomebrewRollbackDoesNotReleaseCredentialsBeforeRetryCapability(t *testing.T) {
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["test"] = domain.Account{Label: "Test", Endpoints: domain.Endpoints{OpenAIResponses: "https://endpoint.test/v1"}}
	cfg.Routes["test"] = domain.Route{Label: "Test", Account: "test", Model: "gpt-6.1-sol", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	zero := uint64(0)
	cfg.Clients[codex.ID] = domain.ClientBinding{Route: "test", RetryPolicy: &domain.RetryPolicy{RequestMaxRetries: &zero}}
	configStore := store.NewStore(filepath.Join(t.TempDir(), "aigw.toml"), clients.Admission())
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := configStore.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	reads := 0
	runtime := invocation.Context{Clients: clients.Registry(), Config: configStore, Secrets: secret.NewMemoryStore(), Out: io.Discard, ResolveProgram: func() (secret.Program, error) {
		reads++
		return secret.Program{}, errors.New("credential reader must not be resolved")
	}}
	err = prepareHomebrewRollback(runtime)
	if err == nil || !strings.Contains(err.Error(), "retry") || reads != 0 {
		t.Fatalf("unproved rollback reached credential custody or lost its retry boundary: %v (reads=%d)", err, reads)
	}
	after, err := configStore.CaptureSnapshot()
	if err != nil || !before.Config.Equal(after.Config) {
		t.Fatalf("refused rollback changed policy: %v", err)
	}
}

func TestHomebrewRollbackRefusesUnreadableRetryPolicyBeforeCustody(t *testing.T) {
	configStore := store.NewStore(filepath.Join(t.TempDir(), "config.toml"), clients.Admission())
	if err := os.WriteFile(filepath.Join(filepath.Dir(configStore.Path()), "config.retries.toml"), []byte("["), 0o600); err != nil {
		t.Fatal(err)
	}
	reads := 0
	runtime := invocation.Context{Clients: clients.Registry(), Config: configStore, Secrets: secret.NewMemoryStore(), Out: io.Discard, ResolveProgram: func() (secret.Program, error) {
		reads++
		return secret.Program{}, errors.New("credential reader must not be resolved")
	}}
	if err := prepareHomebrewRollback(runtime); err == nil || reads != 0 {
		t.Fatalf("unreadable policy reached custody: %v (reads=%d)", err, reads)
	}
}
