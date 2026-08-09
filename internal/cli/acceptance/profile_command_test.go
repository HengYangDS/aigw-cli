package cli_test

import (
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileShowRendersEverySecretFreeField(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["shared"] = configuration.Account{Label: "Shared", Endpoints: configuration.Endpoints{OpenAIResponses: "https://shared.test/v1", Anthropic: "https://shared.test"}}
	cfg.Profiles["both"] = configuration.Profile{Label: "Both Models", Purpose: "Daily work", Account: "shared", Models: configuration.Models{configuration.ClientCodex: "gpt-test", configuration.ClientClaude: "claude-test"}}
	cfg.Routes.Default = "both"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("shared", "never-render-this"); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "profile", "show", "both"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Both Models", "Daily work", "shared", "gpt-test", "claude-test", "https://shared.test/v1", "https://shared.test", "Available"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("human output lacks %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "never-render-this") {
		t.Fatalf("human output leaked token: %s", out.String())
	}
	out.Reset()
	if err := execute(t, app, "profile", "show", "both", "--json"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"secret_available":true`) || strings.Contains(out.String(), "never-render-this") {
		t.Fatalf("JSON output = %s", out.String())
	}
}

func TestAdvancedProfileReadEditAndRemoveErrors(t *testing.T) {
	t.Run("list load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := execute(t, app, "profile", "list"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("show load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := execute(t, app, "profile", "show", "one"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("show unknown", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		err := execute(t, app, "profile", "show", "missing")
		if err == nil || !strings.Contains(err.Error(), "Unknown profile") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("edit requires change", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		if err := execute(t, app, "profile", "edit", "one"); err == nil {
			t.Fatal("expected nothing-to-update error")
		}
	})

	t.Run("edit load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := execute(t, app, "profile", "edit", "one", "--label", "New"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("edit unknown", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "m"})
		err := execute(t, app, "profile", "edit", "missing", "--label", "New")
		if err == nil || !strings.Contains(err.Error(), "Unknown profile") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("remove load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := execute(t, app, "profile", "remove", "one"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("remove unknown", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "m"})
		err := execute(t, app, "profile", "remove", "missing")
		if err == nil || !strings.Contains(err.Error(), "Unknown profile") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("remove override", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		cfg := configuration.NewConfig()
		addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{OpenAIResponses: "https://one.test/v1", Anthropic: "https://one.test"}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "m1"})
		addAccountProfile(&cfg, "two", "one", "Two", configuration.Endpoints{}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "m2"})
		cfg.Routes.Default = "one"
		cfg.Routes.Overrides[configuration.ClientCodex] = "two"
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		err := execute(t, app, "profile", "remove", "two")
		if err == nil || !strings.Contains(err.Error(), "used by codex") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestProfileEditSynchronizesActiveCodexProjection(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt"})
	cfg.Routes.Default = "one"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "token")
	if err := execute(t, app, "profile", "edit", "one", "--label", "New Label"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "configuration synchronized") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestConfigImportRefusesProfileConflictUntilExplicitReplacement(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Team Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://team.example.test/v1"}}
	cfg.Profiles["shared"] = configuration.Profile{Label: "Personal Model", Account: "team", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "personal-model"}}
	cfg.Routes.Default = "shared"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 3
recommended_default = "shared"
[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
openai_responses = "https://team.example.test/v1"
[profiles.shared]
label = "Team Model"
account = "team"
client = "codex"
[profiles.shared.models]
codex = "team-model"
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	err := execute(t, app, "config", "import", manifestPath)
	if err == nil || !strings.Contains(err.Error(), "--replace-profile shared") {
		t.Fatalf("default import error = %v", err)
	}
	if err := execute(t, app, "config", "import", manifestPath, "--replace-profile", "shared"); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles["shared"].Models[configuration.ClientCodex] != "team-model" {
		t.Fatalf("explicit profile replacement = %#v", got.Profiles["shared"])
	}
}

func TestProfilePurposeIsOptionalHumanGuidance(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "current", "team", "Team Gateway", configuration.Endpoints{Anthropic: "https://team.test"}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "claude-current"})
	cfg.Routes.Default = "current"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "profile", "add", "claude-fable-5", "--account", "team", "--for", "claude", "--model", "claude-fable-5", "--label", "Claude Fable 5", "--purpose", "Default agent"); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles["claude-fable-5"].Purpose != "Default agent" {
		t.Fatalf("purpose = %q", got.Profiles["claude-fable-5"].Purpose)
	}
	if err := secretStore.Set("team", "team-token"); err != nil {
		t.Fatal(err)
	}
	selector := &fakePrompt{selected: "claude-fable-5"}
	app.Interactive = true
	app.Prompt = selector
	if err := execute(t, app, "use"); err != nil {
		t.Fatal(err)
	}
	if len(selector.choices) != 2 || selector.choices[0].Label != "Claude Fable 5 · Default agent" {
		t.Fatalf("interactive choices = %#v", selector.choices)
	}

	out.Reset()
	if err := execute(t, app, "profile", "list"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Default agent") {
		t.Fatalf("profile list lacks purpose:\n%s", out.String())
	}

	out.Reset()
	if err := execute(t, app, "profile", "show", "claude-fable-5", "--json"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"purpose":"Default agent"`) {
		t.Fatalf("profile JSON lacks purpose:\n%s", out.String())
	}

	if err := execute(t, app, "profile", "edit", "claude-fable-5", "--purpose", "Deep agent"); err != nil {
		t.Fatal(err)
	}
	got, err = app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles["claude-fable-5"].Purpose != "Deep agent" {
		t.Fatalf("edited purpose = %q", got.Profiles["claude-fable-5"].Purpose)
	}
	if err := execute(t, app, "profile", "edit", "claude-fable-5", "--purpose", ""); err != nil {
		t.Fatal(err)
	}
	got, err = app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles["claude-fable-5"].Purpose != "" {
		t.Fatalf("cleared purpose = %q", got.Profiles["claude-fable-5"].Purpose)
	}
}

func TestProfileListUsesChineseProductLabelsWithoutRewritingPurpose(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "gpt", "team", "Team Gateway", configuration.Endpoints{OpenAIResponses: "https://team.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt-test"})
	cfg.Profiles["gpt"] = configuration.Profile{
		Label:   "GPT Test",
		Purpose: "native Codex picker-aligned daily default",
		Account: "team",
		Client:  configuration.ClientCodex,
		Models:  configuration.Models{configuration.ClientCodex: "gpt-test"},
	}
	cfg.Routes.Default = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "team-token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "profile", "list"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"Service profiles", "Available profiles", "Configuration  gpt", "Codex · GPT Test · native Codex picker-aligned daily default · Current · Account team · Token available"} {
		if !strings.Contains(text, want) {
			t.Fatalf("profile list lacks %q:\n%s", want, text)
		}
	}
	for _, retired := range []string{"Profiles\n", "Profile  gpt"} {
		if strings.Contains(text, retired) {
			t.Fatalf("profile list retained product label %q:\n%s", retired, text)
		}
	}
}

func TestProfileRemoveRefusesActiveProfile(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", configuration.Endpoints{Anthropic: "https://team.test"}, "", configuration.Models{})
	cfg.Routes.Default = "team"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err := execute(t, app, "profile", "remove", "team")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "default route") {
		t.Fatalf("error = %v", err)
	}
}

func TestUseWithoutNameSelectsProfileAndCollectsMissingToken(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	if err := app.Config.Save(twoProfileConfig()); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-token")
	prompt := &fakePrompt{selected: "two", secret: "two-token"}
	app.Interactive = true
	app.Prompt = prompt
	if err := execute(t, app, "use"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := app.Config.Load()
	if cfg.Routes.Default != "two" || !secretStore.Has("two") || prompt.secretCalls != 1 {
		t.Fatalf("config=%#v hasSecret=%v prompts=%d", cfg, secretStore.Has("two"), prompt.secretCalls)
	}
}

func TestRotateWithoutNameUsesCurrentProfileAndOnePaste(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := twoProfileConfig()
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "old-token")
	prompt := &fakePrompt{secret: "new-token"}
	app.Interactive = true
	app.Prompt = prompt
	if err := execute(t, app, "rotate"); err != nil {
		t.Fatal(err)
	}
	got, _ := secretStore.Get("one")
	if got != "new-token" || prompt.secretCalls != 1 {
		t.Fatalf("token=%q prompts=%d", got, prompt.secretCalls)
	}
}

func TestRepairCanRestoreClaudeWithoutAnyCodexProfile(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	claudeExecutable := executableFixture(t, "claude")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "claude", "Claude", configuration.Endpoints{Anthropic: "https://example.test"}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "claude-test"})
	cfg.Routes.Default = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: claudeExecutable}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude", "token"); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{}}

	if err := execute(t, app, "repair"); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if adapter := got.Adapters[configuration.ClientClaude]; !adapter.Enabled || adapter.Executable != claudeExecutable {
		t.Fatalf("Claude adapter changed during repair: %#v", adapter)
	}
}

func TestTerminalErrorLocalizesResolvedProfileClientMismatch(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Team", Endpoints: configuration.Endpoints{OpenAIResponses: "https://team.test/v1", Anthropic: "https://team.test"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "team", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	err := execute(t, app, "test", "--for", "claude")
	if err == nil {
		t.Fatal("test command unexpectedly succeeded")
	}
	text := out.String()
	for _, want := range []string{"profile \"gpt\" is for codex, not claude", "Recommended action", "aigw check"} {
		if !strings.Contains(text, want) {
			t.Fatalf("localized terminal error lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Connectivity test") {
		t.Fatalf("failed test command emitted partial success view:\n%s", text)
	}
}
