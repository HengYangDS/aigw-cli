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
	cfg.Profiles["local"] = configuration.Profile{Label: "Local", Account: "team", Client: configuration.ClientClaude, Model: "local-model"}
	cfg.Routes[configuration.ClientClaude] = "local"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "personal-token"); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 4
[recommended_routes]
claude = "team-profile"
[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"
[profiles.team-profile]
label = "Team Profile"
account = "team"
client = "claude"
model = "team-model"
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
	if got.Accounts["team"].Endpoints.Anthropic != "https://personal.example.test" || got.Routes[configuration.ClientClaude] != "local" {
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
	if got.Accounts["team"].Endpoints.Anthropic != "https://team.example.test" || got.Routes[configuration.ClientClaude] != "local" {
		t.Fatalf("explicit replacement result: %#v", got)
	}
	if token, err := secretStore.Get("team"); err != nil || token != "personal-token" {
		t.Fatalf("explicit replacement altered token: %q, %v", token, err)
	}
}

func TestConfigImportReportsMissingAccountTokensNotProfileTokens(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 4
[recommended_routes]
codex = "gpt-long-model"
claude = "claude-long-model"
[accounts.dmx]
label = "DMXAPI"
[accounts.dmx.endpoints]
openai_responses = "https://dmx.test/v1"
anthropic = "https://dmx.test"
[profiles."gpt-long-model"]
label = "GPT Long Model"
account = "dmx"
client = "codex"
model = "gpt-long-model"
[profiles."claude-long-model"]
label = "Claude Long Model"
account = "dmx"
client = "claude"
model = "claude-long-model"
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "existing-token")
	if err := cli.Execute(app, []string{"config", "import", manifestPath}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Contains(text, "Token required") || strings.Contains(text, "gpt-long-model  ") || strings.Contains(text, "claude-long-model  ") {
		t.Fatalf("import reported profile-level missing tokens despite account token:\n%s", text)
	}
	for _, want := range []string{"Accounts", "System secret", "dmx", "Token available", "aigw models"} {
		if !strings.Contains(text, want) {
			t.Fatalf("import output lacks %q:\n%s", want, text)
		}
	}
}

func TestConfigImportReportsOnlyMissingAccounts(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 4
[recommended_routes]
codex = "gpt-long-model"
claude = "claude-long-model"
[accounts.dmx]
label = "DMXAPI"
[accounts.dmx.endpoints]
openai_responses = "https://dmx.test/v1"
anthropic = "https://dmx.test"
[profiles."gpt-long-model"]
label = "GPT Long Model"
account = "dmx"
client = "codex"
model = "gpt-long-model"
[profiles."claude-long-model"]
label = "Claude Long Model"
account = "dmx"
client = "claude"
model = "claude-long-model"
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"config", "import", manifestPath}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "dmx") || !strings.Contains(text, "Token required") || !strings.Contains(text, "aigw rotate dmx") {
		t.Fatalf("import did not point to missing account token:\n%s", text)
	}
	if strings.Contains(text, "gpt-long-model") || strings.Contains(text, "claude-long-model") {
		t.Fatalf("import should not report profile names as missing token slots:\n%s", text)
	}
}

func TestConfigImportReportsCredentialObservationFailureWithoutInventingAbsence(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	want := errors.New("credential observation failed")
	app.Secrets = &recordingCredentialStore[string]{backend: secrets.NewMemoryStore(), existsErr: want}

	if err := cli.Execute(app, []string{"config", "import", writeConfigurationManifest(t, configurationManifestFixture)}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "Credential status unavailable") || !strings.Contains(text, want.Error()) {
		t.Fatalf("import output = %q", text)
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
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
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
	manifest := `version = 4
[recommended_routes]
claude = "team"
[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.test"
[profiles.team]
label = "Team Gateway"
purpose = "Default agent"
account = "team"
client = "claude"
model = "claude-model"
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

func TestConfigImportRefusesProfileConflictUntilExplicitReplacement(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Team Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://team.example.test/v1"}}
	cfg.Profiles["shared"] = configuration.Profile{Label: "Personal Model", Account: "team", Client: configuration.ClientCodex, Model: "personal-model"}
	cfg.Routes[configuration.ClientCodex] = "shared"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 4
[recommended_routes]
codex = "shared"
[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
openai_responses = "https://team.example.test/v1"
[profiles.shared]
label = "Team Model"
account = "team"
client = "codex"
model = "team-model"
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	err := cli.Execute(app, []string{"config", "import", manifestPath})
	if err == nil || !strings.Contains(err.Error(), "--replace-profile shared") {
		t.Fatalf("default import error = %v", err)
	}
	if err := cli.Execute(app, []string{"config", "import", manifestPath, "--replace-profile", "shared"}); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles["shared"].Model != "team-model" {
		t.Fatalf("explicit profile replacement = %#v", got.Profiles["shared"])
	}
}
