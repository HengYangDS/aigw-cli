package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

// clientSteps decodes the next steps of every client and of the whole
// document that status or check wrote.
type clientSteps struct {
	OK         bool   `json:"ok"`
	NextAction string `json:"next_action"`
	NextDetail string `json:"next_detail"`
	Clients    map[string]struct {
		State      string `json:"state"`
		Detail     string `json:"detail"`
		NextAction string `json:"next_action"`
		NextDetail string `json:"next_detail"`
	} `json:"clients"`
}

// providerWithoutModels starts from an empty home and adds a provider, which
// needs no model, through the CLI.
func providerWithoutModels(t *testing.T) (*cli.App, *bytes.Buffer, secret.Store) {
	t.Helper()
	app, out, secretStore, _, _ := testApp(t, "sk-team-token-0001\n")
	if err := cli.Execute(app, []string{"provider", "add", "team", "--endpoint", "anthropic=https://team.test", "--token-stdin"}); err != nil {
		t.Fatalf("provider add from an empty home: %v\n%s", err, out)
	}
	if token, err := secretStore.Get("team"); err != nil || token != "sk-team-token-0001" {
		t.Fatalf("stored Token = %q, %v", token, err)
	}
	out.Reset()
	return app, out, secretStore
}

// Every client of providers without models selects nothing and names how to
// select a model, and so does the status.
func TestStatusOfProvidersWithoutModels(t *testing.T) {
	app, out, _ := providerWithoutModels(t)
	if err := cli.Execute(app, []string{"status", "--json"}); err != nil {
		t.Fatalf("status: %v\n%s", err, out)
	}
	var status clientSteps
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.NextDetail != "aigw use <client> <provider/model>" || status.NextAction != "" {
		t.Fatalf("status next steps = %+v", status)
	}
	for _, client := range clients.Admission().IDs() {
		got := status.Clients[client]
		if got.State != "deferred" || got.Detail != "No model is selected" || got.NextDetail != "aigw use "+client+" <provider/model>" || got.NextAction != "" {
			t.Fatalf("%s = %+v", client, got)
		}
	}
	out.Reset()
	if err := cli.Execute(app, []string{"status"}); err != nil || strings.Contains(out.String(), "Not configured") || !strings.Contains(out.String(), "No model selected") {
		t.Fatalf("human status = %v\n%s", err, out)
	}
	// Bare aigw at a terminal shows this status instead of starting setup.
	app.Interactive, app.Prompt = true, &scriptedPrompt{}
	out.Reset()
	if err := cli.Execute(app, nil); err != nil || !strings.Contains(out.String(), "Configuration status") {
		t.Fatalf("bare aigw = %v\n%s", err, out)
	}
}

// check has no enabled client, says why, and names the selection.
func TestCheckOfProvidersWithoutModels(t *testing.T) {
	app, out, _ := providerWithoutModels(t)
	if err := cli.Execute(app, []string{"check"}); err == nil ||
		!strings.Contains(out.String(), "No model is added yet, so no client can select one.") || !strings.Contains(out.String(), "aigw use <client> <provider/model>") {
		t.Fatalf("check = %v\n%s", err, out)
	}
	out.Reset()
	if err := cli.Execute(app, []string{"check", "--json"}); err == nil {
		t.Fatalf("check --json succeeded:\n%s", out)
	}
	var check clientSteps
	if err := json.Unmarshal(out.Bytes(), &check); err != nil || check.OK || check.NextDetail != "aigw use <client> <provider/model>" {
		t.Fatalf("check document = %+v, %v\n%s", check, err, out)
	}
}

// sync writes no client file, and the credential reader refuses a client
// that nothing enables.
func TestSyncAndTheCredentialReaderOfProvidersWithoutModels(t *testing.T) {
	app, out, _ := providerWithoutModels(t)
	before, err := os.ReadFile(app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	if after, err := os.ReadFile(app.Config.Path()); err != nil || string(after) != string(before) {
		t.Fatalf("sync changed the configuration: %v", err)
	}
	if entries, err := os.ReadDir(app.Host.Home); err != nil || len(entries) != 0 {
		t.Fatalf("sync wrote client files: %v %v", entries, err)
	}
	out.Reset()
	if err := cli.Execute(app, []string{"credential", "claude", "fingerprint"}); err == nil {
		t.Fatalf("the credential reader returned a Token:\n%s", out)
	}
}

// The configuration exports a team file that an empty home imports.
func TestTeamFileOfProvidersWithoutModels(t *testing.T) {
	app, out, _ := providerWithoutModels(t)
	if err := cli.Execute(app, []string{"config", "export"}); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(t.TempDir(), "team.toml")
	if err := os.WriteFile(manifest, out.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	other, otherOut, _, _, _ := testApp(t, "")
	if err := cli.Execute(other, []string{"config", "import", manifest}); err != nil {
		t.Fatalf("import of a team file without models: %v\n%s", err, otherOut)
	}
	if cfg, err := other.Config.Load(); err != nil || len(cfg.Providers) != 1 || len(cfg.Models) != 0 {
		t.Fatalf("imported configuration = %+v, %v", cfg, err)
	}
}

// TestRemovingTheLastModelLeavesClientsThatSelectNothing removes the only
// model and then the only provider: each step leaves a valid configuration.
func TestRemovingTheLastModelLeavesClientsThatSelectNothing(t *testing.T) {
	app, out, secretStore := providerWithoutModels(t)
	if err := cli.Execute(app, []string{"model", "add", "team/team-model", "--unlisted"}); err != nil {
		t.Fatalf("model add: %v\n%s", err, out)
	}
	if err := cli.Execute(app, []string{"model", "remove", "team/team-model"}); err != nil {
		t.Fatalf("model remove of the last model: %v\n%s", err, out)
	}
	if err := cli.Execute(app, []string{"provider", "remove", "team"}); err != nil {
		t.Fatalf("provider remove of the last provider: %v\n%s", err, out)
	}
	if present, _ := secretStore.Exists("team"); present {
		t.Fatal("the removal kept the Token")
	}
	if cfg, err := store.NewStore(app.Config.Path(), clients.Admission()).Load(); err != nil || len(cfg.Providers) != 0 || len(cfg.Models) != 0 {
		t.Fatalf("configuration after removing everything = %+v, %v", cfg, err)
	}
	out.Reset()
	if err := cli.Execute(app, []string{"status"}); err != nil || !strings.Contains(out.String(), "Not configured") || !strings.Contains(out.String(), "aigw setup") {
		t.Fatalf("status after removing everything = %v\n%s", err, out)
	}
}

// Setup refuses an installation whose provider has no models, rather than
// replacing the provider's label, endpoint, balance probe and Token.
func TestSetupRefusesAProviderWithoutModels(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "sk-replacement-0001\n")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["team"] = domain.Provider{
		Label: "Team Gateway", Endpoints: domain.Endpoints{Anthropic: "https://team.test"},
		BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://balance.test"},
	}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "sk-team-token-0001"); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	err = cli.Execute(app, []string{"setup", "--account", "team", "--route", "r", "--for", "claude",
		"--anthropic-url", "https://other.test", "--upstream-model", "m", "--token-stdin"})
	if err == nil || !strings.Contains(out.String(), "AIGW is already configured") || !strings.Contains(out.String(), "Fix\n  aigw status\n") {
		t.Fatalf("setup over a provider without models = %v\n%s", err, out)
	}
	if after, err := os.ReadFile(app.Config.Path()); err != nil || string(after) != string(before) {
		t.Fatalf("a refused setup changed the configuration: %v", err)
	}
	if token, err := secretStore.Get("team"); err != nil || token != "sk-team-token-0001" {
		t.Fatalf("a refused setup changed the Token: %q, %v", token, err)
	}
}
