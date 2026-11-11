package cli_test

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

// A forwarded client reaches the upstream that its forwarding records. A
// provider edit that moves that upstream clears the forwarding in the same
// write and says so; the client's file then reaches the edited endpoint
// directly, and the client's retry budgets stay.
func TestProviderEditClearsTheForwardingOfAMovedUpstream(t *testing.T) {
	for _, test := range []struct {
		name      string
		selection []string
	}{
		{"native retries", nil},
		{"retry policy", []string{"--request-retries", "2"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, output, secretStore, _, _ := testApp(t, "")
			target := filepath.Join(t.TempDir(), "config.toml")
			writeFile(t, target, []byte("model = \"gpt-native\"\n"), 0o600)
			cfg := domain.NewConfig(clients.Admission())
			cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Responses: "https://gateway.test/v1"}}
			cfg.Models["sol"] = qualifiedModel("Sol", "gateway", "gpt-sol", domain.ProtocolResponses)
			cfg.SetSelectedModel(codex.ID, "sol", "")
			cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), []string{target})
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := secretStore.Set("gateway", "fixture-token"); err != nil {
				t.Fatal(err)
			}
			forwarding := append([]string{"client", "edit", "codex", "--via", "http://127.0.0.1:8792/v1"}, test.selection...)
			if err := cli.Execute(app, forwarding); err != nil {
				t.Fatalf("forward Codex: %v\n%s", err, output)
			}

			output.Reset()
			if err := cli.Execute(app, []string{"provider", "edit", "gateway", "--endpoint", "responses=https://moved.test/v1", "--json"}); err != nil {
				t.Fatalf("the provider edit: %v\n%s", err, output)
			}
			var document struct {
				ClearedForwarding []string `json:"cleared_forwarding"`
			}
			if err := json.Unmarshal(output.Bytes(), &document); err != nil || !slices.Equal(document.ClearedForwarding, []string{codex.ID}) {
				t.Fatalf("the edit's cleared forwarding = %q, %v\n%s", document.ClearedForwarding, err, output)
			}
			if projected := string(readFile(t, target)); !strings.Contains(projected, `base_url = "https://moved.test/v1"`) {
				t.Fatalf("Codex does not reach the edited endpoint directly:\n%s", projected)
			}
			saved, err := app.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			if saved.Clients[codex.ID].ForwardingEndpoint != "" {
				t.Fatalf("the forwarding stays: %q", saved.Clients[codex.ID].ForwardingEndpoint)
			}
			if policy := saved.Clients[codex.ID].RetryPolicy; test.selection != nil && (policy == nil || policy.RequestMaxRetries == nil || *policy.RequestMaxRetries != 2) {
				t.Fatalf("the edit dropped the retry budget: %#v", policy)
			}
		})
	}
}
