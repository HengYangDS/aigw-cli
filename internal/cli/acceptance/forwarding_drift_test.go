package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/configuration"
)

// A forwarded client reaches the upstream it was selected with. An Account edit
// that would move that upstream is refused before any write, and the advised
// direct selection, which passes its own guards, lets the edit succeed.
func TestAccountEditRefusesToMoveAForwardedUpstream(t *testing.T) {
	for _, test := range []struct {
		name      string
		selection []string
		advice    string
	}{
		{"native retries", nil, "aigw use --for codex sol --direct"},
		{"retry policy", []string{"--request-max-retries", "2"}, "aigw use --for codex sol --direct --native-retries"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, output, secretStore, _, _ := testApp(t, "")
			target := filepath.Join(t.TempDir(), "config.toml")
			writeFile(t, target, []byte("model = \"gpt-native\"\n"), 0o600)
			cfg := configuration.NewConfig(clients.Admission())
			cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
			cfg.Routes["sol"] = qualifiedRoute("Sol", "gateway", "gpt-sol", configuration.ProtocolOpenAIResponses)
			cfg.SetSelectedRoute(configuration.ClientCodex, "sol", "")
			cfg.SetClientActivation(configuration.ClientCodex, true, executableFixture(t, "codex"), []string{target})
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := secretStore.Set("gateway", "fixture-token"); err != nil {
				t.Fatal(err)
			}
			selection := append([]string{"use", "--for", "codex", "sol", "--forwarding-endpoint", "http://127.0.0.1:8792/v1"}, test.selection...)
			if err := cli.Execute(app, selection); err != nil {
				t.Fatalf("forward the Codex Route: %v\n%s", err, output)
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
				t.Fatalf("the advised direct selection failed: %v\n%s", err, output)
			}
			output.Reset()
			if err := cli.Execute(app, edit); err != nil {
				t.Fatalf("the Account edit after the direct selection: %v\n%s", err, output)
			}
			if projected := string(readFile(t, target)); !strings.Contains(projected, `base_url = "https://moved.test/v1"`) {
				t.Fatalf("the direct connection does not reach the edited Account:\n%s", projected)
			}
		})
	}
}
