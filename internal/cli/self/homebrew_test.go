package self

import (
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

// A Homebrew rollback prepares the credentials of a configuration whose
// client declares retry budgets as of any other: no retry policy stops it,
// and the configuration stays as it was.
func TestHomebrewRollbackPreparesCredentialsWithRetryBudgets(t *testing.T) {
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
	var out strings.Builder
	runtime := invocation.Context{Clients: clients.Registry(), Config: configStore, Secrets: secret.NewMemoryStore(), Out: &out}
	if err := prepareHomebrewRollback(runtime, false); err != nil || !strings.Contains(out.String(), "Credentials are ready for an earlier AIGW release.") {
		t.Fatalf("a Homebrew rollback with retry budgets = %v\n%s", err, out.String())
	}
	after, err := configStore.CaptureSnapshot()
	if err != nil || !before.Config.Equal(after.Config) {
		t.Fatalf("the rollback changed the configuration: %v", err)
	}
}
