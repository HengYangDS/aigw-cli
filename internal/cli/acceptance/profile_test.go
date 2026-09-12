package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileAddValidation(t *testing.T) {
	t.Run("profile add invalid id", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		err := cli.Execute(app, []string{"profile", "add", "bad id", "--account", "one", "--for", "claude", "--model", "m"})
		if err == nil || !strings.Contains(err.Error(), "Invalid profile ID") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("profile add load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := cli.Execute(app, []string{"profile", "add", "two", "--account", "one", "--for", "claude", "--model", "m"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("profile add duplicate", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		err := cli.Execute(app, []string{"profile", "add", "one", "--account", "one", "--for", "claude", "--model", "m"})
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("profile add unknown account", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		err := cli.Execute(app, []string{"profile", "add", "two", "--account", "missing", "--for", "claude", "--model", "m"})
		if err == nil || !strings.Contains(err.Error(), "Unknown account") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("profile add default label", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		if err := cli.Execute(app, []string{"profile", "add", "two", "--account", "one", "--for", "claude", "--model", "m2"}); err != nil {
			t.Fatal(err)
		}
		cfg, _ := app.Config.Load()
		if cfg.Profiles["two"].Label != "two" {
			t.Fatalf("profile = %#v", cfg.Profiles["two"])
		}
	})
}

func TestProfileRemoveLeavesAccountAndTokenIntact(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["gpt-default"] = configuration.Profile{Label: "GPT Default", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-default"}
	cfg.Profiles["gpt-unused"] = configuration.Profile{Label: "GPT Unused", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-unused"}
	cfg.Routes[configuration.ClientCodex] = "gpt-default"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "account-token")
	if err := cli.Execute(app, []string{"profile", "remove", "gpt-unused"}); err != nil {
		t.Fatal(err)
	}
	if got, err := secretStore.Get("dmx"); err != nil || got != "account-token" {
		t.Fatalf("account token changed: %q %v", got, err)
	}
	got, _ := app.Config.Load()
	if _, ok := got.Profiles["gpt-unused"]; ok || got.Accounts["dmx"].Label != "DMXAPI" {
		t.Fatalf("remove config = %#v", got)
	}
}

func TestProfileAddReusesAccountTokenAndLeavesRouteUntouched(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "gpt", "dmx", "GPT", configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}, configuration.ClientCodex, "gpt-test")
	cfg.Routes[configuration.ClientCodex] = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "existing-account-token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"profile", "add", "claude", "--account", "dmx", "--for", "claude", "--model", "claude-test", "--label", "Claude Test"}); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	profile := got.Profiles["claude"]
	if profile.Account != "dmx" || profile.Client != configuration.ClientClaude || profile.Model != "claude-test" {
		t.Fatalf("added profile = %#v", profile)
	}
	if got.Routes[configuration.ClientCodex] != "gpt" || !secretExists(t, secretStore, "dmx") || secretExists(t, secretStore, "claude") {
		t.Fatalf("route or token slots changed: routes=%#v dmx=%v claude=%v", got.Routes, secretExists(t, secretStore, "dmx"), secretExists(t, secretStore, "claude"))
	}
}

func TestProfileAddRejectsClientWithoutMatchingAccountEndpoint(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "gpt", "openai-only", "OpenAI Only", configuration.Endpoints{OpenAIResponses: "https://openai.test/v1"}, configuration.ClientCodex, "gpt-test")
	cfg.Routes[configuration.ClientCodex] = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	err := cli.Execute(app, []string{"profile", "add", "claude", "--account", "openai-only", "--for", "claude", "--model", "claude-test"})
	if err == nil || !strings.Contains(err.Error(), "no Anthropic endpoint") {
		t.Fatalf("profile add error = %v", err)
	}
	got, loadErr := app.Config.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if _, exists := got.Profiles["claude"]; exists {
		t.Fatalf("unusable Profile was persisted: %#v", got.Profiles["claude"])
	}
}

func TestProfileReadsSurfaceCredentialObservationFailure(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
	want := errors.New("credential observation failed")
	app.Secrets = &recordingCredentialStore[string]{backend: secrets.NewMemoryStore(), existsErr: want}

	for _, args := range [][]string{{"profile", "list"}, {"profile", "show", "one"}} {
		if err := cli.Execute(app, args); !errors.Is(err, want) {
			t.Fatalf("%v error = %v, want %v", args, err, want)
		}
	}
}

func TestProfileReadsHonorClientNativeAuthenticationOwnership(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["native"] = configuration.Account{
		Label: "Native",
		Endpoints: configuration.Endpoints{
			OpenAIResponses: "https://native.test/v1",
		},
	}
	cfg.Profiles["native"] = configuration.Profile{
		Label:          "Native",
		Account:        "native",
		Client:         configuration.ClientCodex,
		Model:          "native-model",
		ModelProvider:  "amazon-bedrock",
		Authentication: configuration.AuthenticationClientNative,
	}
	cfg.Routes[configuration.ClientCodex] = "native"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	observed := &recordingCredentialStore[string]{backend: secretStore}
	app.Secrets = observed

	if err := cli.Execute(app, []string{"profile", "list"}); err != nil {
		t.Fatal(err)
	}
	if len(observed.existsCalls) != 0 || len(observed.getCalls) != 0 {
		t.Fatalf("profile list accessed client-native credentials: exists=%q get=%q", observed.existsCalls, observed.getCalls)
	}
	if text := out.String(); !strings.Contains(text, "Client-owned authentication") || strings.Contains(text, "Token missing") {
		t.Fatalf("profile list = %q", text)
	}

	out.Reset()
	if err := cli.Execute(app, []string{"profile", "show", "native", "--json"}); err != nil {
		t.Fatal(err)
	}
	if len(observed.existsCalls) != 0 || len(observed.getCalls) != 0 {
		t.Fatalf("profile show accessed client-native credentials: exists=%q get=%q", observed.existsCalls, observed.getCalls)
	}
	for _, want := range []string{`"authentication":"client-native"`, `"model_provider":"amazon-bedrock"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("profile JSON lacks %q: %s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "secret_available") {
		t.Fatalf("profile JSON projected an AIGW Token fact for client-native authentication: %s", out.String())
	}

	out.Reset()
	if err := cli.Execute(app, []string{"profile", "show", "native"}); err != nil {
		t.Fatal(err)
	}
	if text := out.String(); !strings.Contains(text, "Authentication") || !strings.Contains(text, "Client-owned") {
		t.Fatalf("profile show = %q", text)
	}
}

func TestProfileShowRendersEverySecretFreeField(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["shared"] = configuration.Account{Label: "Shared", Endpoints: configuration.Endpoints{OpenAIResponses: "https://shared.test/v1", Anthropic: "https://shared.test"}}
	cfg.Profiles["codex"] = configuration.Profile{Label: "Codex Model", Purpose: "Daily work", Account: "shared", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientCodex] = "codex"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("shared", "never-render-this"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"profile", "show", "codex"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Codex Model", "Daily work", "shared", "Codex", "gpt-test", "https://shared.test/v1", "https://shared.test", "Available"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("human output lacks %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "never-render-this") {
		t.Fatalf("human output leaked token: %s", out.String())
	}
	out.Reset()
	if err := cli.Execute(app, []string{"profile", "show", "codex", "--json"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"secret_available":true`) || strings.Contains(out.String(), "never-render-this") {
		t.Fatalf("JSON output = %s", out.String())
	}
}

func TestAdvancedProfileReadEditAndRemoveErrors(t *testing.T) {
	t.Run("list load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := cli.Execute(app, []string{"profile", "list"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("show load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := cli.Execute(app, []string{"profile", "show", "one"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("show unknown", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		err := cli.Execute(app, []string{"profile", "show", "missing"})
		if err == nil || !strings.Contains(err.Error(), "Unknown profile") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("edit requires change", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		if err := cli.Execute(app, []string{"profile", "edit", "one"}); err == nil {
			t.Fatal("expected nothing-to-update error")
		}
	})

	t.Run("edit load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := cli.Execute(app, []string{"profile", "edit", "one", "--label", "New"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("edit unknown", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		err := cli.Execute(app, []string{"profile", "edit", "missing", "--label", "New"})
		if err == nil || !strings.Contains(err.Error(), "Unknown profile") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("remove load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := cli.Execute(app, []string{"profile", "remove", "one"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("remove unknown", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		err := cli.Execute(app, []string{"profile", "remove", "missing"})
		if err == nil || !strings.Contains(err.Error(), "Unknown profile") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("remove override", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		cfg := configuration.NewConfig()
		addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{OpenAIResponses: "https://one.test/v1", Anthropic: "https://one.test"}, configuration.ClientClaude, "m1")
		addAccountProfile(&cfg, "two", "one", "Two", configuration.Endpoints{}, configuration.ClientCodex, "m2")
		cfg.Routes[configuration.ClientClaude] = "one"
		cfg.Routes[configuration.ClientCodex] = "two"
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		err := cli.Execute(app, []string{"profile", "remove", "two"})
		if err == nil || !strings.Contains(err.Error(), "selected for codex") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestProfileEditSynchronizesActiveCodexProjection(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
	cfg.Routes[configuration.ClientCodex] = "one"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "token")
	if err := cli.Execute(app, []string{"profile", "edit", "one", "--label", "New Label"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Profile metadata saved") {
		t.Fatalf("output = %q", out.String())
	}
	projected, err := os.ReadFile(target)
	if err != nil || !strings.Contains(string(projected), "AIGW: New Label") {
		t.Fatalf("profile label was not projected: %s, %v", projected, err)
	}
}

func TestProfilePurposeIsOptionalHumanGuidance(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "current", "team", "Team Gateway", configuration.Endpoints{Anthropic: "https://team.test"}, configuration.ClientClaude, "claude-current")
	cfg.Routes[configuration.ClientClaude] = "current"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"profile", "add", "claude-fable-5", "--account", "team", "--for", "claude", "--model", "claude-fable-5", "--label", "Claude Fable 5", "--purpose", "Default agent"}); err != nil {
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
	selector := &scriptedPrompt{selections: []string{"claude-fable-5"}}
	app.Interactive = true
	app.Prompt = selector
	if err := cli.Execute(app, []string{"use"}); err != nil {
		t.Fatal(err)
	}
	if len(selector.choices) != 2 || selector.choices[0].Label != "Claude Fable 5 · Default agent" {
		t.Fatalf("interactive choices = %#v", selector.choices)
	}

	out.Reset()
	if err := cli.Execute(app, []string{"profile", "list"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Default agent") {
		t.Fatalf("profile list lacks purpose:\n%s", out.String())
	}

	out.Reset()
	if err := cli.Execute(app, []string{"profile", "show", "claude-fable-5", "--json"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"purpose":"Default agent"`) {
		t.Fatalf("profile JSON lacks purpose:\n%s", out.String())
	}

	if err := cli.Execute(app, []string{"profile", "edit", "claude-fable-5", "--purpose", "Deep agent"}); err != nil {
		t.Fatal(err)
	}
	got, err = app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles["claude-fable-5"].Purpose != "Deep agent" {
		t.Fatalf("edited purpose = %q", got.Profiles["claude-fable-5"].Purpose)
	}
	if err := cli.Execute(app, []string{"profile", "edit", "claude-fable-5", "--purpose", ""}); err != nil {
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
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "gpt", "team", "Team Gateway", configuration.Endpoints{OpenAIResponses: "https://team.test/v1"}, configuration.ClientCodex, "gpt-test")
	cfg.Profiles["gpt"] = configuration.Profile{
		Label:   "GPT Test",
		Purpose: "native Codex picker-aligned daily default",
		Account: "team",
		Client:  configuration.ClientCodex,
		Model:   "gpt-test",
	}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "team-token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"profile", "list"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"Service profiles", "Available profiles", "Configuration  gpt", "Codex · GPT Test · native Codex picker-aligned daily default · Selected for Codex · Account team · Token available"} {
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
	app, _, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", configuration.Endpoints{Anthropic: "https://team.test"}, configuration.ClientClaude, "claude-test")
	cfg.Routes[configuration.ClientClaude] = "team"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err := cli.Execute(app, []string{"profile", "remove", "team"})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "selected for claude") {
		t.Fatalf("error = %v", err)
	}
}
