package cli_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"
)

// A forwarded client's destination follows its provider's endpoint: a
// provider edit that moves the endpoint keeps the forwarding, Codex's file
// names the destination composed from the edited endpoint, and the client's
// retry budgets stay. An endpoint that the forwarding cannot name is refused
// before anything changes, naming the direct connection.
func TestProviderEditMovesTheDestinationOfAForwardedClient(t *testing.T) {
	t.Parallel()
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
			cfg.Models[domain.NewModelRef("gateway", "gpt-sol")] = qualifiedModel("Sol", "gateway", "gpt-sol", domain.ProtocolResponses)
			cfg.SetSelectedModel(codex.ID, domain.NewModelRef("gateway", "gpt-sol"), "")
			cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), []string{target})
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := secretStore.Set("gateway", "fixture-token"); err != nil {
				t.Fatal(err)
			}
			forwarding := append([]string{"client", "edit", "codex", "--via", "http://127.0.0.1:8792"}, test.selection...)
			if err := cli.Execute(app, forwarding); err != nil {
				t.Fatalf("forward Codex: %v\n%s", err, output)
			}
			if projected := string(readFile(t, target)); !strings.Contains(projected, `base_url = "http://127.0.0.1:8792/https/gateway.test/v1"`) {
				t.Fatalf("Codex does not forward to its provider's upstream:\n%s", projected)
			}

			output.Reset()
			if err := cli.Execute(app, []string{"provider", "edit", "gateway", "--endpoint", "responses=https://moved.test:8443/v1", "--json"}); err != nil {
				t.Fatalf("the provider edit: %v\n%s", err, output)
			}
			var document map[string]any
			if err := json.Unmarshal(output.Bytes(), &document); err != nil || document["cleared_forwarding"] != nil {
				t.Fatalf("the edit's document = %v, %v\n%s", document, err, output)
			}
			if projected := string(readFile(t, target)); !strings.Contains(projected, `base_url = "http://127.0.0.1:8792/https/moved.test:8443/v1"`) {
				t.Fatalf("Codex's destination does not follow the edited endpoint:\n%s", projected)
			}
			saved, err := app.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			if saved.Clients[codex.ID].ForwardingRoot != "http://127.0.0.1:8792" {
				t.Fatalf("the forwarding = %q", saved.Clients[codex.ID].ForwardingRoot)
			}
			if policy := saved.Clients[codex.ID].RetryPolicy; test.selection != nil && (policy == nil || policy.RequestMaxRetries == nil || *policy.RequestMaxRetries != 2) {
				t.Fatalf("the edit dropped the retry budget: %#v", policy)
			}

			file, stored := readFile(t, target), readFile(t, app.Config.Path())
			err = cli.Execute(app, []string{"provider", "edit", "gateway", "--endpoint", "responses=https://moved.test/v1/../v2"})
			if problem := terminal.ProblemFor(err); !strings.Contains(problem.Title, "empty or dot path segment") || problem.Fix != "aigw client edit codex --direct" {
				t.Fatalf("an endpoint that the forwarding cannot name = %q, fix %q", problem.Title, problem.Fix)
			}
			if !bytes.Equal(readFile(t, target), file) || !bytes.Equal(readFile(t, app.Config.Path()), stored) {
				t.Fatal("a refused provider edit changed the configuration or Codex's file")
			}
		})
	}
}
