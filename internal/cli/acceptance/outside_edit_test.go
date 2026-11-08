package cli_test

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"testing"

	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"

	"go.yaml.in/yaml/v3"
)

// bindCredentialReader gives app a private versioned credential reader, as
// production does. Inspecting a client file first reads which reader it names.
func bindCredentialReader(t *testing.T, app *cli.App) {
	t.Helper()
	root := t.TempDir()
	app.Executable = filepath.Join(root, "bin", executableName("aigw"))
	app.DataDir = filepath.Join(root, "data")
	writeFile(t, app.Executable, []byte("portable program"), 0o755)
	app.UseExecutableIdentity(nil, executableName("aigw"))
}

// afterRecovery restores the projected file and requires synchronization and
// the client's local checks to pass.
func afterRecovery(t *testing.T, app *cli.App, output interface {
	Reset()
	String() string
}, client, target string, projected []byte,
) {
	t.Helper()
	writeFile(t, target, projected, 0o600)
	for _, command := range [][]string{{"sync", "--json"}, {"check", client, "--depth", "local", "--json"}} {
		output.Reset()
		if err := cli.Execute(app, command); err != nil {
			t.Fatalf("%v after the recovery: %v\n%s", command, err, output.String())
		}
	}
}

// Hermes refuses a managed setting changed outside AIGW, and the file's own
// recovery is named wherever the refusal is reported. A model chosen in Hermes
// is not such an edit: synchronization adopts it.
func TestHermesOutsideEditRecoveryDoesNotLoop(t *testing.T) {
	const recovery = "Undo the outside edit to the AIGW-managed Hermes settings, then run `aigw sync`"
	app, output, _, target := shippedHermesApp(t)
	projected := readFile(t, target)
	hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", func(provider *yaml.Node) {
		for index := 0; index+1 < len(provider.Content); index += 2 {
			if provider.Content[index].Value == "base_url" {
				provider.Content[index+1].Value = "https://foreign.test"
			}
		}
	})
	assertRecoveryOnEverySurface(t, app, output, hermes.ID, recovery, filepath.Dir(target), [][]string{
		{"sync", "--dry-run", "--json"},
		{"sync", "--json"},
		{"check", hermes.ID, "--depth", "local", "--json"},
		{"status", "--json"},
	})
	afterRecovery(t, app, output, hermes.ID, target, projected)

	writeFile(t, target, []byte("model: [\n"), 0o600)
	assertRecoveryOnEverySurface(t, app, output, hermes.ID, "Correct the YAML in Hermes' configuration file, then run the same command again.", filepath.Dir(target), [][]string{
		{"sync", "--json"},
		{"check", hermes.ID, "--depth", "local", "--json"},
		{"status", "--json"},
	})
	afterRecovery(t, app, output, hermes.ID, target, projected)

	hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
	output.Reset()
	err := cli.Execute(app, []string{"check", hermes.ID, "--depth", "local", "--json"})
	var checked struct {
		NextAction string `json:"next_action"`
	}
	if decodeErr := json.Unmarshal(output.Bytes(), &checked); decodeErr != nil || checked.NextAction != "aigw sync" {
		t.Fatalf("a model chosen in Hermes: next step %q, %v (command error %v)\n%s", checked.NextAction, decodeErr, err, output)
	}
	if err := cli.Execute(app, []string{"sync", "--json"}); err != nil {
		t.Fatalf("sync of a model chosen in Hermes: %v\n%s", err, output)
	}
}

// Claude Desktop's refusal names each changed setting, and that step, not
// synchronization alone, is the next step wherever the refusal is reported.
func TestClaudeDesktopOutsideEditRecoveryDoesNotLoop(t *testing.T) {
	const recovery = "Restore AIGW's value of gateway base URL in Claude Desktop, then run `aigw sync`"
	app, output, secretStore, _, _ := testApp(t, "")
	library := filepath.Join(t.TempDir(), "nested", "Claude-3p", "configLibrary")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Models["opus"] = qualifiedModel("Opus", "gateway", "claude-test", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(claudedesktop.ID, "opus", "")
	cfg.SetClientActivation(claudedesktop.ID, true, executableFixture(t, "Claude"), []string{library})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("project Claude Desktop: %v\n%s", err, output)
	}
	profile := claudedesktop.PathsForLibrary(library).Profile
	projected := readFile(t, profile)
	var document map[string]any
	if err := json.Unmarshal(projected, &document); err != nil {
		t.Fatal(err)
	}
	document["inferenceGatewayBaseUrl"] = "https://user.example.test"
	edited, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, profile, edited, 0o600)
	assertRecoveryOnEverySurface(t, app, output, claudedesktop.ID, recovery, library, [][]string{
		{"sync", "--dry-run", "--json"},
		{"sync", "--json"},
		{"check", claudedesktop.ID, "--depth", "local", "--json"},
		{"status", "--json"},
	})
	afterRecovery(t, app, output, claudedesktop.ID, profile, projected)

	metadata := claudedesktop.PathsForLibrary(library).Metadata
	projected = readFile(t, metadata)
	writeFile(t, metadata, []byte("[]\n"), 0o600)
	name := filepath.Join(filepath.Base(library), filepath.Base(metadata))
	assertRecoveryOnEverySurface(t, app, output, claudedesktop.ID, "Correct the JSON in Claude Desktop's "+name+", then run the same command again.", library, [][]string{
		{"sync", "--json"},
		{"check", claudedesktop.ID, "--depth", "local", "--json"},
		{"status", "--json"},
	})
	afterRecovery(t, app, output, claudedesktop.ID, metadata, projected)
}

// A credential command that AIGW did not write fails the inspection of the
// credential reader, and is still the outside edit that synchronization refuses.
func TestCodexOutsideCredentialCommandRecoveryDoesNotLoop(t *testing.T) {
	const recovery = "Undo the outside edit to the AIGW-managed Codex settings, then run `aigw sync`"
	app, output, secretStore, _, _ := testApp(t, "")
	bindCredentialReader(t, app)
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
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("project the Codex Route: %v\n%s", err, output)
	}
	projected := readFile(t, target)
	edited := regexp.MustCompile(`(?m)^command = .*$`).ReplaceAll(projected, []byte(`command = "/nonexistent/helper"`))
	if string(edited) == string(projected) {
		t.Fatalf("fixture has no credential command:\n%s", projected)
	}
	writeFile(t, target, edited, 0o600)
	assertRecoveryOnEverySurface(t, app, output, codex.ID, recovery, filepath.Dir(target), [][]string{
		{"sync", "--json"},
		{"check", codex.ID, "--depth", "local", "--json"},
		{"status", "--json"},
	})
	afterRecovery(t, app, output, codex.ID, target, projected)
}
