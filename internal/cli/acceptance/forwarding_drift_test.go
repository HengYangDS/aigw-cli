package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

// A forwarded client reaches the upstream that its forwarding records. An
// Account edit that would move that upstream is refused before any write, and
// the advised direct connection, which keeps the client's retry budgets, lets
// the edit succeed.
func TestProviderEditRefusesToMoveAForwardedUpstream(t *testing.T) {
	for _, test := range []struct {
		name      string
		selection []string
		advice    string
	}{
		{"native retries", nil, "aigw client edit codex --direct"},
		{"retry policy", []string{"--request-retries", "2"}, "aigw client edit codex --direct"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, output, secretStore, _, _ := testApp(t, "")
			target := filepath.Join(t.TempDir(), "config.toml")
			writeFile(t, target, []byte("model = \"gpt-native\"\n"), 0o600)
			cfg := domain.NewConfig(clients.Admission())
			cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
			cfg.Models["sol"] = qualifiedModel("Sol", "gateway", "gpt-sol", domain.ProtocolOpenAIResponses)
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
			forwarded := ownedFileSnapshot(t, filepath.Dir(target), app.Config.Path())

			output.Reset()
			edit := []string{"account", "edit", "gateway", "--openai-url", "https://moved.test/v1"}
			if err := cli.Execute(app, edit); err == nil {
				t.Fatalf("an Account edit silently moved the forwarded upstream:\n%s", output)
			}
			if !strings.Contains(output.String(), test.advice) {
				t.Fatalf("refusal does not advise %q:\n%s", test.advice, output)
			}
			if after := ownedFileSnapshot(t, filepath.Dir(target), app.Config.Path()); !equalSnapshots(after, forwarded) {
				t.Fatal("the refused Account edit changed client files or owned state")
			}

			output.Reset()
			if err := cli.Execute(app, strings.Fields(test.advice)[1:]); err != nil {
				t.Fatalf("the advised direct connection failed: %v\n%s", err, output)
			}
			output.Reset()
			if err := cli.Execute(app, edit); err != nil {
				t.Fatalf("the Account edit after the direct connection: %v\n%s", err, output)
			}
			if projected := string(readFile(t, target)); !strings.Contains(projected, `base_url = "https://moved.test/v1"`) {
				t.Fatalf("the direct connection does not reach the edited Account:\n%s", projected)
			}
			saved, err := app.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			if policy := saved.Clients[codex.ID].RetryPolicy; test.selection != nil && (policy == nil || policy.RequestMaxRetries == nil || *policy.RequestMaxRetries != 2) {
				t.Fatalf("the direct connection and the Account edit dropped the retry budget: %#v", policy)
			}
		})
	}
}
