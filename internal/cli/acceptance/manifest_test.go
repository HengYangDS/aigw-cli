package cli_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

// A team file never redirects an existing provider: its endpoint and Token
// stay, its label is the file's, and the client keeps its selection.
func TestSetupFromKeepsAnExistingProvidersEndpointAndToken(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["team"] = domain.Provider{Label: "Personal Gateway", Endpoints: domain.Endpoints{Anthropic: "https://personal.example.test"}}
	cfg.Models[domain.NewModelRef("team", "local-model")] = qualifiedModel("Local", "team", "local-model", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("team", "local-model"), "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "personal-token"); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 8

[[providers]]
id = "team"
label = "Team Gateway"
endpoints = {anthropic = "https://team.example.test"}

[[providers.models]]
id = "team-model"
family = "team-model"
label = "Team Route"
protocols = {anthropic = ["text"]}

[[families]]
id = "team-model"
label = "Team Model"

[recommendations.claude]
primary = {model = "team/team-model"}
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"setup", "--from", manifestPath}); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Providers["team"].Endpoints.Anthropic != "https://personal.example.test" || got.Providers["team"].Label != "Team Gateway" || got.SelectedModel(claude.ID) != domain.NewModelRef("team", "local-model") {
		t.Fatalf("setup redirected a local provider: %#v", got)
	}
	if token, err := secretStore.Get("team"); err != nil || token != "personal-token" {
		t.Fatalf("setup altered the Token: %q, %v", token, err)
	}
}

// Team setup observes the presence of only the Tokens that a recommendation
// needs, and reads none.
func TestSetupFromObservesOnlyTheTokensThatARecommendationNeeds(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	store := &recordingCredentialStore[string]{backend: secret.NewMemoryStore()}
	app.Secrets = store
	if err := cli.Execute(app, []string{"setup", "--from", writeConfigurationManifest(t, configurationManifestFixture)}); err != nil {
		t.Fatal(err)
	}
	if len(store.getCalls) != 0 || slices.ContainsFunc(store.existsCalls, func(id string) bool { return id != "aihubmix" && id != "dmxapi" }) {
		t.Fatalf("setup read or observed other credentials: exists=%q get=%q", store.existsCalls, store.getCalls)
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Providers) != 2 || len(cfg.Models) != 3 {
		t.Fatalf("applied team file = %#v", cfg)
	}
}

func TestConfigCommandIOFailures(t *testing.T) {
	t.Run("path output", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		want := errors.New("output failed")
		app.Out = failingOutput{err: want}
		if err := cli.Execute(app, []string{"config", "path"}); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("export load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = store.NewStore(t.TempDir(), clients.Admission())
		if err := cli.Execute(app, []string{"config", "export"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("export output", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "m")
		want := errors.New("output failed")
		app.Out = failingOutput{err: want}
		if err := cli.Execute(app, []string{"config", "export"}); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("team file read", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		if err := cli.Execute(app, []string{"setup", "--from", filepath.Join(t.TempDir(), "missing.toml")}); err == nil {
			t.Fatal("expected a team file read failure")
		}
	})

	t.Run("team file parse", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		path := filepath.Join(t.TempDir(), "bad.toml")
		if err := os.WriteFile(path, []byte("not = [valid"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := cli.Execute(app, []string{"setup", "--from", path}); err == nil {
			t.Fatal("expected a team file parse failure")
		}
	})
}

func TestSetupFromAndExportAreSecretFree(t *testing.T) {
	app, out, secrets, _, _ := testApp(t, "")
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 8

[[providers]]
id = "team"
label = "Team Gateway"
endpoints = {anthropic = "https://team.test"}

[[providers.models]]
id = "claude-model"
family = "claude-model"
label = "Team Gateway"
protocols = {anthropic = ["text"]}

[[families]]
id = "claude-model"
label = "Claude Model"

[recommendations.claude]
primary = {model = "team/claude-model"}
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"setup", "--from", manifestPath}); err != nil {
		t.Fatal(err)
	}
	if secretExists(t, secrets, "team") {
		t.Fatal("setup invented a Token")
	}
	out.Reset()
	if err := cli.Execute(app, []string{"config", "export"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(out.String()), "token") || !strings.Contains(out.String(), "Team Gateway") || strings.Contains(out.String(), "[clients") {
		t.Fatalf("unsafe export:\n%s", out.String())
	}
}

// A team file whose model would take the protocol that a client selects it on
// keeps the local model, and the result says so.
func TestSetupFromKeepsAModelWhoseChangeWouldMoveASelection(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["team"] = domain.Provider{Label: "Team Gateway", Endpoints: domain.Endpoints{Responses: "https://team.example.test/v1"}}
	cfg.Models[domain.NewModelRef("team", "personal-model")] = qualifiedModel("Personal Model", "team", "personal-model", domain.ProtocolResponses)
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("team", "personal-model"), "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 8

[[providers]]
id = "team"
label = "Team Gateway"
endpoints = {anthropic = "https://team.example.test", responses = "https://team.example.test/v1"}

[[providers.models]]
id = "personal-model"
family = "team-model"
label = "Team Model"
protocols = {anthropic = ["text"]}

[[families]]
id = "team-model"
label = "Team Model"
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"setup", "--from", manifestPath, "--json"}); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	kept := got.Models[domain.NewModelRef("team", "personal-model")]
	_, responses := kept.Protocols[domain.ProtocolResponses]
	if kept.Family != "personal-model" || !responses || !strings.Contains(out.String(), `"kept_models"`) ||
		!strings.Contains(out.String(), `"model": "team/personal-model"`) || !strings.Contains(out.String(), `"protocol": "responses"`) {
		t.Fatalf("the selected model = %#v\n%s", kept, out)
	}
}
