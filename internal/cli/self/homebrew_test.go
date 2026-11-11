package self

import (
	"errors"
	"io"
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
	cfg.Providers["test"] = domain.Provider{Label: "Test", Endpoints: domain.Endpoints{Responses: "https://endpoint.test/v1"}}
	cfg.Models[domain.NewModelRef("test", "gpt-6.1-sol")] = domain.Model{Label: "Test", Provider: "test", Family: "gpt-6.1-sol", ID: "gpt-6.1-sol", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}}
	zero := uint64(0)
	cfg.Clients[codex.ID] = domain.ClientBinding{Model: domain.NewModelRef("test", "gpt-6.1-sol"), RetryPolicy: &domain.RetryPolicy{RequestMaxRetries: &zero}}
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
	err = prepareHomebrewRollback(runtime, false)
	if err == nil || !strings.Contains(err.Error(), "retry") || reads != 0 {
		t.Fatalf("unproved rollback reached credential custody or lost its retry boundary: %v (reads=%d)", err, reads)
	}
	after, err := configStore.CaptureSnapshot()
	if err != nil || !before.Config.Equal(after.Config) {
		t.Fatalf("refused rollback changed policy: %v", err)
	}
}
