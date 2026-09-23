package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestProfileAddValidation(t *testing.T) {
	t.Run("profile add invalid id", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		err := cli.Execute(app, []string{"profile", "add", "bad id", "--account", "one", "--model", "m"})
		if err == nil || !strings.Contains(err.Error(), "Invalid profile ID") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("profile add load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := cli.Execute(app, []string{"profile", "add", "two", "--account", "one", "--model", "m"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("profile add duplicate", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		err := cli.Execute(app, []string{"profile", "add", "one", "--account", "one", "--model", "m", "--protocol", "anthropic"})
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("profile add unknown account", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		err := cli.Execute(app, []string{"profile", "add", "two", "--account", "missing", "--model", "m", "--protocol", "anthropic"})
		if err == nil || !strings.Contains(err.Error(), "Unknown account") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("profile add default label", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		if err := cli.Execute(app, []string{"profile", "add", "two", "--account", "one", "--model", "m2", "--protocol", "anthropic"}); err != nil {
			t.Fatal(err)
		}
		cfg, _ := app.Config.Load()
		if cfg.Routes["two"].Label != "two" {
			t.Fatalf("profile = %#v", cfg.Routes["two"])
		}
	})
}

func TestProfileRemoveLeavesAccountAndTokenIntact(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Routes["gpt-default"] = qualifiedRoute("GPT Default", "dmx", "gpt-default", configuration.ProtocolOpenAIResponses)
	cfg.Routes["gpt-unused"] = qualifiedRoute("GPT Unused", "dmx", "gpt-unused", configuration.ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(configuration.ClientCodex, "gpt-default")
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
	if _, ok := got.Routes["gpt-unused"]; ok || got.Accounts["dmx"].Label != "DMXAPI" {
		t.Fatalf("remove config = %#v", got)
	}
}

func TestProfileAddReusesAccountTokenAndLeavesRouteUntouched(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "gpt", "dmx", "GPT", configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}, configuration.ClientCodex, "gpt-test")
	cfg.SetSelectedRoute(configuration.ClientCodex, "gpt")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "existing-account-token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"profile", "add", "claude", "--account", "dmx", "--model", "claude-test", "--protocol", "anthropic", "--label", "Claude Test"}); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	profile := got.Routes["claude"]
	if profile.Account != "dmx" || profile.Model != "claude-test" {
		t.Fatalf("added profile = %#v", profile)
	}
	if got.SelectedRoute(configuration.ClientCodex) != "gpt" || !secretExists(t, secretStore, "dmx") || secretExists(t, secretStore, "claude") {
		t.Fatalf("selection or token slots changed: clients=%#v dmx=%v claude=%v", got.Clients, secretExists(t, secretStore, "dmx"), secretExists(t, secretStore, "claude"))
	}
}

func TestProfileAddDoesNotConflateProfileCreationWithClientCompatibility(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "gpt", "openai-only", "OpenAI Only", configuration.Endpoints{OpenAIResponses: "https://openai.test/v1"}, configuration.ClientCodex, "gpt-test")
	cfg.SetSelectedRoute(configuration.ClientCodex, "gpt")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"profile", "add", "claude", "--account", "openai-only", "--model", "claude-test", "--protocol", "openai_responses"}); err != nil {
		t.Fatalf("profile creation should remain client-independent: %v", err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	profile, exists := got.Routes["claude"]
	if !exists || profile.Account != "openai-only" || profile.Model != "claude-test" {
		t.Fatalf("created Profile = %#v, exists=%v", profile, exists)
	}
	if got.SelectedRoute(configuration.ClientClaude) != "" {
		t.Fatalf("profile creation selected a Claude binding: %#v", got.Clients)
	}

	err = cli.Execute(app, []string{"use", "--for", "claude", "claude"})
	if err == nil || !strings.Contains(err.Error(), `does not admit endpoint protocol "anthropic"`) {
		t.Fatalf("client selection error = %v", err)
	}
}

func TestProfileReadsDoNotInspectCredentials(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
	observed := &recordingCredentialStore[string]{backend: secrets.NewMemoryStore(), existsErr: errors.New("credential observation must not run")}
	app.Secrets = observed

	for _, args := range [][]string{{"profile", "list"}, {"profile", "show", "one"}} {
		if err := cli.Execute(app, args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	if len(observed.existsCalls)+len(observed.getCalls) != 0 {
		t.Fatalf("Profile reads inspected credentials: exists=%q get=%q", observed.existsCalls, observed.getCalls)
	}
}

func TestProfileReadsKeepClientAuthenticationInTheBinding(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["native"] = configuration.Account{
		Label: "Native",
		Endpoints: configuration.Endpoints{
			OpenAIResponses: "https://native.test/v1",
		},
	}
	cfg.Routes["native"] = configuration.Route{
		Label:   "Native",
		Account: "native",
		Model:   "native-model",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{
			configuration.ProtocolOpenAIResponses: {},
		},
	}
	cfg.Clients[configuration.ClientCodex] = configuration.ClientBinding{
		Route: "native", ModelProvider: "amazon-bedrock",
		Authentication: configuration.AuthenticationClientNative,
	}
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
	if text := out.String(); strings.Contains(text, "Authentication") || strings.Contains(text, "Token") {
		t.Fatalf("profile list = %q", text)
	}

	out.Reset()
	if err := cli.Execute(app, []string{"profile", "show", "native", "--json"}); err != nil {
		t.Fatal(err)
	}
	if len(observed.existsCalls) != 0 || len(observed.getCalls) != 0 {
		t.Fatalf("profile show accessed client-native credentials: exists=%q get=%q", observed.existsCalls, observed.getCalls)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	for _, clientField := range []string{"authentication", "model_provider", "secret_available", "credential_ownership"} {
		if _, exists := result[clientField]; exists {
			t.Fatalf("Profile JSON contains client-binding field %q: %s", clientField, out.String())
		}
	}

	out.Reset()
	if err := cli.Execute(app, []string{"profile", "show", "native"}); err != nil {
		t.Fatal(err)
	}
	if text := out.String(); strings.Contains(text, "Authentication") || strings.Contains(text, "Token") {
		t.Fatalf("profile show = %q", text)
	}
}

func TestProfileListJSONIsStableAndClientIndependent(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["shared"] = configuration.Account{Label: "Shared", Endpoints: configuration.Endpoints{OpenAIResponses: "https://shared.test/v1", Anthropic: "https://shared.test"}}
	cfg.Routes["zeta"] = qualifiedRoute("Zeta", "shared", "claude-test", configuration.ProtocolAnthropic)
	cfg.Routes["alpha"] = qualifiedRoute("Alpha", "shared", "gpt-test", configuration.ProtocolOpenAIResponses)
	cfg.Clients[configuration.ClientCodex] = configuration.ClientBinding{
		Route: "alpha", ModelProvider: "amazon-bedrock",
		Authentication: configuration.AuthenticationClientNative,
	}
	cfg.SetSelectedRoute(configuration.ClientClaude, "zeta")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("shared", "never-print-this-token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"profile", "list", "--json"}); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Profiles []struct {
			ID              string   `json:"id"`
			SelectedClients []string `json:"selected_clients"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Profiles) != 2 || result.Profiles[0].ID != "alpha" || result.Profiles[1].ID != "zeta" {
		t.Fatalf("profile order = %#v", result.Profiles)
	}
	if !slices.Equal(result.Profiles[0].SelectedClients, []string{configuration.ClientCodex}) {
		t.Fatalf("Codex-selected Profile = %#v", result.Profiles[0])
	}
	if !slices.Equal(result.Profiles[1].SelectedClients, []string{configuration.ClientClaude}) {
		t.Fatalf("Claude-selected Profile = %#v", result.Profiles[1])
	}
	for _, forbidden := range []string{"never-print-this-token", "authentication", "credential_ownership", "secret_available"} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("profile list exposed client or credential field %q: %s", forbidden, out.String())
		}
	}
}

func TestProfileShowRendersEverySecretFreeField(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["shared"] = configuration.Account{Label: "Shared", Endpoints: configuration.Endpoints{OpenAIResponses: "https://shared.test/v1", Anthropic: "https://shared.test"}}
	cfg.Routes["codex"] = qualifiedRoute("Codex Model", "shared", "gpt-test", configuration.ProtocolOpenAIResponses)
	route := cfg.Routes["codex"]
	route.Purpose = "Daily work"
	cfg.Routes["codex"] = route
	cfg.SetSelectedRoute(configuration.ClientCodex, "codex")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("shared", "never-render-this"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"profile", "show", "codex"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Codex Model", "Daily work", "shared", "codex", "gpt-test", "https://shared.test/v1", "https://shared.test"} {
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
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if _, exists := result["secret_available"]; exists || strings.Contains(out.String(), "never-render-this") {
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
		cfg.SetSelectedRoute(configuration.ClientClaude, "one")
		cfg.SetSelectedRoute(configuration.ClientCodex, "two")
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
	cfg.SetSelectedRoute(configuration.ClientCodex, "one")
	cfg.SetClientActivation(configuration.ClientCodex, true, "/opt/codex", []string{target})
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
	cfg.SetSelectedRoute(configuration.ClientClaude, "current")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"profile", "add", "claude-fable-5", "--account", "team", "--model", "claude-fable-5", "--protocol", "anthropic", "--label", "Claude Fable 5", "--purpose", "Default agent"}); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes["claude-fable-5"].Purpose != "Default agent" {
		t.Fatalf("purpose = %q", got.Routes["claude-fable-5"].Purpose)
	}
	if err := secretStore.Set("team", "team-token"); err != nil {
		t.Fatal(err)
	}
	selector := &scriptedPrompt{selections: []string{"claude-fable-5"}}
	app.Interactive = true
	app.Prompt = selector
	if err := cli.Execute(app, []string{"use", "--for", "claude"}); err != nil {
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
	var result struct {
		Purpose string `json:"purpose"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Purpose != "Default agent" {
		t.Fatalf("profile JSON lacks purpose:\n%s", out.String())
	}

	if err := cli.Execute(app, []string{"profile", "edit", "claude-fable-5", "--purpose", "Deep agent"}); err != nil {
		t.Fatal(err)
	}
	got, err = app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes["claude-fable-5"].Purpose != "Deep agent" {
		t.Fatalf("edited purpose = %q", got.Routes["claude-fable-5"].Purpose)
	}
	if err := cli.Execute(app, []string{"profile", "edit", "claude-fable-5", "--purpose", ""}); err != nil {
		t.Fatal(err)
	}
	got, err = app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes["claude-fable-5"].Purpose != "" {
		t.Fatalf("cleared purpose = %q", got.Routes["claude-fable-5"].Purpose)
	}
}

func TestProfileListUsesChineseProductLabelsWithoutRewritingPurpose(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "gpt", "team", "Team Gateway", configuration.Endpoints{OpenAIResponses: "https://team.test/v1"}, configuration.ClientCodex, "gpt-test")
	cfg.Routes["gpt"] = qualifiedRoute("GPT Test", "team", "gpt-test", configuration.ProtocolOpenAIResponses)
	route := cfg.Routes["gpt"]
	route.Purpose = "native Codex picker-aligned daily default"
	cfg.Routes["gpt"] = route
	cfg.SetSelectedRoute(configuration.ClientCodex, "gpt")
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
	for _, want := range []string{"Profiles", "Available profiles", "Configuration  gpt", "GPT Test · native Codex picker-aligned daily default · Selected for codex · Account team · Clients codex"} {
		if !strings.Contains(text, want) {
			t.Fatalf("profile list lacks %q:\n%s", want, text)
		}
	}
	for _, retired := range []string{"Service profiles\n", "Profile  gpt"} {
		if strings.Contains(text, retired) {
			t.Fatalf("profile list retained product label %q:\n%s", retired, text)
		}
	}
}

func TestProfileRemoveRefusesActiveProfile(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "team", "team", "Team", configuration.Endpoints{Anthropic: "https://team.test"}, configuration.ClientClaude, "claude-test")
	cfg.SetSelectedRoute(configuration.ClientClaude, "team")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err := cli.Execute(app, []string{"profile", "remove", "team"})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "selected for claude") {
		t.Fatalf("error = %v", err)
	}
}
