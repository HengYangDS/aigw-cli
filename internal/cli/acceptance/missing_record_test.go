package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// projectedClient is a client whose file AIGW projected, with the ownership
// record that AIGW keeps beside it.
type projectedClient struct {
	id     string
	app    *cli.App
	output *bytes.Buffer
	file   string
	record string
}

// surfaces are the commands that report a refusal of the client's files.
func (client projectedClient) surfaces() [][]string {
	return [][]string{
		{"sync", "--dry-run", "--json"},
		{"sync", "--json"},
		{"check", client.id, "--depth", "local", "--json"},
		{"status", "--json"},
	}
}

// directory is the folder of the client's file.
func (client projectedClient) directory() string { return filepath.Dir(client.file) }

// saveAndSync stores cfg and the Token, and projects the client.
func saveAndSync(t *testing.T, app *cli.App, output *bytes.Buffer, store secret.Store, cfg domain.Config) {
	t.Helper()
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("project the client: %v\n%s", err, output)
	}
}

func anthropicConfig() domain.Config {
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Models["opus"] = qualifiedModel("Opus", "gateway", "claude-test", domain.ProtocolAnthropic)
	return cfg
}

// projectedClients builds each client, as it runs in production: Codex in its
// default home, which a sync creates, and each with a versioned credential
// reader.
var projectedClients = map[string]func(*testing.T) projectedClient{
	claude.ID: func(t *testing.T) projectedClient {
		t.Helper()
		app, output, store, _, _ := testApp(t, "")
		bindCredentialReader(t, app)
		cfg := anthropicConfig()
		cfg.SetSelectedModel(claude.ID, "opus", "")
		cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
		saveAndSync(t, app, output, store, cfg)
		settings := claudeSettingsPath(t, app.Host)
		return projectedClient{claude.ID, app, output, settings, settings + adapter.StateSuffix}
	},
	claudedesktop.ID: func(t *testing.T) projectedClient {
		t.Helper()
		app, output, store, _, _ := testApp(t, "")
		bindCredentialReader(t, app)
		library := filepath.Join(t.TempDir(), "nested", "Claude-3p", "configLibrary")
		cfg := anthropicConfig()
		cfg.SetSelectedModel(claudedesktop.ID, "opus", "")
		cfg.SetClientActivation(claudedesktop.ID, true, executableFixture(t, "Claude"), []string{library})
		saveAndSync(t, app, output, store, cfg)
		paths := claudedesktop.PathsForLibrary(library)
		return projectedClient{claudedesktop.ID, app, output, paths.Profile, paths.State}
	},
	codex.ID: func(t *testing.T) projectedClient {
		t.Helper()
		app, output, store, _, _ := testApp(t, "")
		bindCredentialReader(t, app)
		target := filepath.Join(t.TempDir(), "config.toml")
		writeFile(t, target, []byte("model = \"gpt-native\"\n"), 0o600)
		app.Discovery = fakeDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{ID: string(codex.HomeDefault), ConfigPath: target, Present: true, AutoManaged: true}}}}
		cfg := domain.NewConfig(clients.Admission())
		cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
		cfg.Models["sol"] = qualifiedModel("Sol", "gateway", "gpt-sol", domain.ProtocolOpenAIResponses)
		cfg.SetSelectedModel(codex.ID, "sol", "")
		cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), []string{target})
		saveAndSync(t, app, output, store, cfg)
		return projectedClient{codex.ID, app, output, target, target + adapter.StateSuffix}
	},
	hermes.ID: func(t *testing.T) projectedClient {
		t.Helper()
		app, output, _, target := shippedHermesAppWith(t, true)
		return projectedClient{hermes.ID, app, output, target, target + adapter.StateSuffix}
	},
}

// dropClaudeDesktopEntry removes what AIGW added to the file list, as the
// refusal of a deleted profile asks.
func dropClaudeDesktopEntry(t *testing.T, client projectedClient) {
	t.Helper()
	path := filepath.Join(filepath.Dir(client.file), "_meta.json")
	var metadata map[string]any
	if err := json.Unmarshal(readFile(t, path), &metadata); err != nil {
		t.Fatal(err)
	}
	delete(metadata, "appliedId")
	delete(metadata, "entries")
	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, data, 0o600)
}

// A client file deleted while AIGW's record of it remains is refused on every
// surface with the one step that lets synchronization pass: the file restored,
// or the record deleted when the file is gone for good.
func TestADeletedClientFileNamesItsRecord(t *testing.T) {
	for id, build := range projectedClients {
		t.Run(id, func(t *testing.T) {
			client := build(t)
			fileName, recordName := filepath.Base(client.file), filepath.Base(client.record)
			also := ""
			if id == claudedesktop.ID {
				also = " and remove AIGW's entry and appliedId from _meta.json there"
			}
			recovery := "Restore " + fileName + ", or delete " + recordName + " in its folder" + also + " if " + fileName + " is gone for good, then run `aigw sync`"
			projected := readFile(t, client.file)

			if err := os.Remove(client.file); err != nil {
				t.Fatal(err)
			}
			assertRecoveryOnEverySurface(t, client.app, client.output, id, recovery, client.directory(), client.surfaces())
			afterRecovery(t, client.app, client.output, id, client.file, projected)

			if err := os.Remove(client.file); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(client.record); err != nil {
				t.Fatal(err)
			}
			if id == claudedesktop.ID {
				dropClaudeDesktopEntry(t, client)
			}
			syncAndCheckPass(t, client.app, client.output, id)
			if _, err := os.Stat(client.record); err != nil {
				t.Fatalf("the sync kept no new record: %v", err)
			}
		})
	}
}
