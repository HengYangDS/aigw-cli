package cli_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestConfigImportRefusesAccountConflictUntilExplicitReplacementAndPreservesToken(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Personal Gateway", Endpoints: configuration.Endpoints{Anthropic: "https://personal.example.test"}}
	cfg.Routes["local"] = qualifiedRoute("Local", "team", "local-model", configuration.ProtocolAnthropic)
	cfg.SetSelectedRoute(configuration.ClientClaude, "local", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "personal-token"); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 7
[recommendations.claude.primary]
route = "team-route"
[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"
[models.team-model]
label = "Team Model"
[routes.team-route]
label = "Team Route"
account = "team"
model = "team-model"
upstream_model = "team-model"
interfaces = { anthropic = ["text"] }
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	err := cli.Execute(app, []string{"config", "import", manifestPath})
	if err == nil || !strings.Contains(err.Error(), "--replace-account team") {
		t.Fatalf("default import error = %v", err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Accounts["team"].Endpoints.Anthropic != "https://personal.example.test" || got.SelectedRoute(configuration.ClientClaude) != "local" {
		t.Fatalf("default import mutated local identity: %#v", got)
	}
	if token, err := secretStore.Get("team"); err != nil || token != "personal-token" {
		t.Fatalf("default import altered token: %q, %v", token, err)
	}

	if err := cli.Execute(app, []string{"config", "import", manifestPath, "--replace-account", "team"}); err != nil {
		t.Fatal(err)
	}
	got, err = app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Accounts["team"].Endpoints.Anthropic != "https://team.example.test" || got.SelectedRoute(configuration.ClientClaude) != "local" {
		t.Fatalf("explicit replacement result: %#v", got)
	}
	if token, err := secretStore.Get("team"); err != nil || token != "personal-token" {
		t.Fatalf("explicit replacement altered token: %q, %v", token, err)
	}
}

func TestConfigImportDefersCredentialObservationToStatus(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	store := &recordingCredentialStore[string]{backend: secrets.NewMemoryStore(), existsErr: errors.New("unexpected credential observation")}
	app.Secrets = store
	if err := cli.Execute(app, []string{"config", "import", writeConfigurationManifest(t, configurationManifestFixture)}); err != nil {
		t.Fatal(err)
	}
	if len(store.existsCalls) != 0 || len(store.getCalls) != 0 {
		t.Fatalf("import observed Account credentials: exists=%q get=%q", store.existsCalls, store.getCalls)
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Accounts) != 2 || len(cfg.Routes) != 3 {
		t.Fatalf("imported public catalogue = %#v", cfg)
	}
	output := out.String()
	if !strings.Contains(output, "Configuration manifest imported") || !strings.Contains(output, "aigw status") || strings.Contains(output, "Token") || strings.Contains(output, "credential observation") {
		t.Fatalf("import output mixed configuration and credential state: %q", output)
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
		app.Config = configuration.NewStore(t.TempDir())
		if err := cli.Execute(app, []string{"config", "export"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("export output", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandRoute(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		want := errors.New("output failed")
		app.Out = failingOutput{err: want}
		if err := cli.Execute(app, []string{"config", "export"}); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("import read", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		if err := cli.Execute(app, []string{"config", "import", filepath.Join(t.TempDir(), "missing.toml")}); err == nil {
			t.Fatal("expected manifest read failure")
		}
	})

	t.Run("import parse", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		path := filepath.Join(t.TempDir(), "bad.toml")
		if err := os.WriteFile(path, []byte("not = [valid"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := cli.Execute(app, []string{"config", "import", path}); err == nil {
			t.Fatal("expected manifest parse failure")
		}
	})
}

func TestConfigImportAndExportAreSecretFree(t *testing.T) {
	app, out, secrets, _, _ := testApp(t, "")
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 7
[recommendations.claude.primary]
route = "team"
[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.test"
[models.claude-model]
label = "Claude Model"
[routes.team]
label = "Team Gateway"
purpose = "Default agent"
account = "team"
model = "claude-model"
upstream_model = "claude-model"
interfaces = { anthropic = ["text"] }
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"config", "import", manifestPath}); err != nil {
		t.Fatal(err)
	}
	if secretExists(t, secrets, "team") {
		t.Fatal("manifest import invented a secret")
	}
	out.Reset()
	if err := cli.Execute(app, []string{"config", "export"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(out.String()), "token") || !strings.Contains(out.String(), "Team Gateway") || !strings.Contains(out.String(), "Default agent") {
		t.Fatalf("unsafe export:\n%s", out.String())
	}
}

func TestConfigImportRefusesRouteConflictUntilExplicitReplacement(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Team Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://team.example.test/v1"}}
	cfg.Routes["shared"] = qualifiedRoute("Personal Model", "team", "personal-model", configuration.ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(configuration.ClientCodex, "shared", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 7
[recommendations.codex.primary]
route = "shared"
[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
openai_responses = "https://team.example.test/v1"
[models.team-model]
label = "Team Model"
[routes.shared]
label = "Team Model"
account = "team"
model = "team-model"
upstream_model = "team-model"
interfaces = { openai_responses = ["text"] }
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	err := cli.Execute(app, []string{"config", "import", manifestPath})
	if err == nil || !strings.Contains(err.Error(), "--replace-route shared") {
		t.Fatalf("default import error = %v", err)
	}
	if err := cli.Execute(app, []string{"config", "import", manifestPath, "--replace-route", "shared"}); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes["shared"].Model != "team-model" {
		t.Fatalf("explicit route replacement = %#v", got.Routes["shared"])
	}
}
