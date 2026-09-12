package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/prompt"
	"aigw-cli/internal/secrets"
	surfaceidentity "aigw-cli/internal/surface"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type scriptedPrompt struct {
	secrets     []string
	texts       []string
	selections  []string
	secretErr   error
	selectErr   error
	secretCalls []string
	textCalls   int
	selectCalls int
	choices     []prompt.Choice
}

func (p *scriptedPrompt) Secret(label string) (string, error) {
	p.secretCalls = append(p.secretCalls, label)
	if p.secretErr != nil {
		return "", p.secretErr
	}
	return consumeAnswer(&p.secrets, "secret")
}

func (p *scriptedPrompt) Text(string) (string, error) {
	p.textCalls++
	return consumeAnswer(&p.texts, "text")
}

func (p *scriptedPrompt) Select(_ string, choices []prompt.Choice) (string, error) {
	p.selectCalls++
	p.choices = slices.Clone(choices)
	if p.selectErr != nil {
		return "", p.selectErr
	}
	return consumeAnswer(&p.selections, "selection")
}

func (p *scriptedPrompt) callCount() int {
	return len(p.secretCalls) + p.textCalls + p.selectCalls
}

func consumeAnswer(answers *[]string, kind string) (string, error) {
	if len(*answers) == 0 {
		return "", fmt.Errorf("no %s answer declared", kind)
	}
	answer := (*answers)[0]
	*answers = (*answers)[1:]
	return answer, nil
}

func TestPromptConsumesOnlyDeclaredAnswers(t *testing.T) {
	for name, prompt := range map[string]*scriptedPrompt{
		"secret":    {secrets: []string{"synthetic"}},
		"text":      {texts: []string{"synthetic"}},
		"selection": {selections: []string{"synthetic"}},
	} {
		t.Run(name, func(t *testing.T) {
			ask := func() (string, error) {
				switch name {
				case "secret":
					return prompt.Secret("Account token")
				case "text":
					return prompt.Text("Account label")
				default:
					return prompt.Select("Profile", nil)
				}
			}
			if answer, err := ask(); answer != "synthetic" || err != nil {
				t.Fatalf("declared answer=%q error=%v", answer, err)
			}
			if answer, err := ask(); answer != "" || err == nil {
				t.Fatalf("exhausted script repeated answer=%q error=%v", answer, err)
			}
			if calls := prompt.callCount(); calls != 2 {
				t.Fatalf("observed %d calls, want both successful and exhausted requests", calls)
			}
		})
	}
}

func choiceValues(choices []prompt.Choice) []string {
	values := make([]string, 0, len(choices))
	for _, choice := range choices {
		values = append(values, choice.Value)
	}
	return values
}

func TestUseWithoutNameSelectsProfileAndCollectsMissingToken(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	if err := app.Config.Save(twoProfileConfig()); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-token")
	prompt := &scriptedPrompt{selections: []string{"two"}, secrets: []string{"two-token"}}
	app.Interactive = true
	app.Prompt = prompt
	if err := cli.Execute(app, []string{"use"}); err != nil {
		t.Fatal(err)
	}
	cfg, _ := app.Config.Load()
	if cfg.Routes[configuration.ClientCodex] != "two" || !secretExists(t, secretStore, "two") || len(prompt.secretCalls) != 1 {
		t.Fatalf("config=%#v hasSecret=%v prompts=%d", cfg, secretExists(t, secretStore, "two"), len(prompt.secretCalls))
	}
}

func TestUseWithoutProfileRequiresInteractiveTerminal(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-one")
	cfg.Routes[configuration.ClientClaude] = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	err := cli.Execute(app, []string{"use"})
	if err == nil || !strings.Contains(err.Error(), "Non-interactive use requires a profile") {
		t.Fatalf("error = %v", err)
	}
}

func TestUseWithoutProfilePromptsInteractively(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-one")
	addAccountProfile(&cfg, "two", "two", "Two", configuration.Endpoints{Anthropic: "https://two.test"}, configuration.ClientClaude, "claude-two")
	cfg.Routes[configuration.ClientClaude] = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	_ = secretStore.Set("two", "two-secret")
	app.Interactive = true
	app.Prompt = &scriptedPrompt{selections: []string{"two"}}
	if err := cli.Execute(app, []string{"use"}); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes[configuration.ClientClaude] != "two" {
		t.Fatalf("routes = %#v, want the interactively chosen profile", got.Routes)
	}
}

func TestUseSurfacesInteractiveSelectionFailure(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-one")
	cfg.Routes[configuration.ClientClaude] = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	app.Interactive = true
	want := errors.New("selection cancelled")
	app.Prompt = &scriptedPrompt{selectErr: want}
	err := cli.Execute(app, []string{"use"})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestUseWithMissingTokenRequiresInteractiveTerminal(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-one")
	addAccountProfile(&cfg, "two", "two", "Two", configuration.Endpoints{Anthropic: "https://two.test"}, configuration.ClientClaude, "claude-two")
	cfg.Routes[configuration.ClientClaude] = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err := cli.Execute(app, []string{"use", "two"})
	if err == nil || !strings.Contains(err.Error(), "is missing a token") {
		t.Fatalf("error = %v", err)
	}
}

func TestUseWithMissingTokenSurfacesPromptFailure(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-one")
	addAccountProfile(&cfg, "two", "two", "Two", configuration.Endpoints{Anthropic: "https://two.test"}, configuration.ClientClaude, "claude-two")
	cfg.Routes[configuration.ClientClaude] = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	app.Interactive = true
	app.Prompt = &scriptedPrompt{}
	err := cli.Execute(app, []string{"use", "two"})
	if err == nil || !strings.Contains(err.Error(), "no secret") {
		t.Fatalf("error = %v", err)
	}
}

func TestUseWithMissingTokenRejectsFailedVerification(t *testing.T) {
	app, _, secretStore, _, httpClient := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-one")
	addAccountProfile(&cfg, "two", "two", "Two", configuration.Endpoints{Anthropic: "https://two.test"}, configuration.ClientClaude, "claude-two")
	cfg.Routes[configuration.ClientClaude] = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	app.Interactive = true
	app.Prompt = &scriptedPrompt{secrets: []string{"new-token"}}
	httpClient.status = 401
	err := cli.Execute(app, []string{"use", "two"})
	if err == nil || !strings.Contains(err.Error(), "Token validation failed") {
		t.Fatalf("error = %v", err)
	}
	if secretExists(t, secretStore, "two") {
		t.Fatal("a failed verification must not persist the newly entered token")
	}
}

func TestUseWithMissingTokenSurfacesSecretStoreFailure(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-one")
	addAccountProfile(&cfg, "two", "two", "Two", configuration.Endpoints{Anthropic: "https://two.test"}, configuration.ClientClaude, "claude-two")
	cfg.Routes[configuration.ClientClaude] = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	app.Interactive = true
	app.Prompt = &scriptedPrompt{secrets: []string{"new-token"}}
	want := errors.New("keychain locked")
	app.Secrets = &recordingCredentialStore[string]{backend: secrets.NewMemoryStore(), setErr: want}
	err := cli.Execute(app, []string{"use", "two"})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestNoArgsRunsAutomaticFirstUseWizard(t *testing.T) {
	app, out, secretStore, runner, _ := testApp(t, "")
	app.Interactive = true
	prompt := &scriptedPrompt{
		selections: []string{"codex"},
		secrets:    []string{"one-paste-token"},
		texts: []string{
			"team-gateway",
			"Team Gateway",
			"https://gateway.test/v1",
			"gpt-5.6-terra",
			"gpt-5.6-terra",
		},
	}
	app.Prompt = prompt
	shimDir := t.TempDir()
	codexTarget := filepath.Join(t.TempDir(), "codex", "configuration.toml")
	if err := os.MkdirAll(filepath.Dir(codexTarget), 0o700); err != nil {
		t.Fatal(err)
	}
	originalCodex := "model_provider = \"native\"\n"
	if err := os.WriteFile(codexTarget, []byte(originalCodex), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientClaude: "/opt/claude-real", configuration.ClientCodex: "/opt/codex-real"},
		Surfaces: []discovery.Surface{{
			ID:          string(surfaceidentity.CodexHomeDefault),
			Authority:   string(surfaceidentity.AuthorityAIGW),
			ConfigPath:  codexTarget,
			Present:     true,
			AutoManaged: true,
		}},
	}}
	if err := cli.Execute(app, []string{}); err != nil {
		t.Fatal(err)
	}
	if len(prompt.secretCalls) != 1 {
		t.Fatalf("token prompts = %d, want 1", len(prompt.secretCalls))
	}
	if !secretExists(t, secretStore, "team-gateway") {
		t.Fatal("generic Account Token was not stored")
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Routes[configuration.ClientCodex] != "gpt-5.6-terra" || cfg.Adapters["claude"].Enabled || !cfg.Adapters["codex"].Enabled {
		t.Fatalf("configured state = %#v", cfg)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("wizard invoked a client: %#v", runner.plans)
	}
	projection, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(codexTarget)
	if err != nil || !strings.Contains(string(projected), projection.CredentialProjectionFingerprint(configuration.ClientCodex)) {
		t.Fatalf("credential helper projection = %s, %v", projected, err)
	}
	if _, err := os.Stat(filepath.Join(shimDir, "claude")); !os.IsNotExist(err) {
		t.Fatalf("Codex-only first-run wizard mutated a Claude command path: %v", err)
	}
	if !strings.Contains(out.String(), "Ready") || strings.Contains(out.String(), "one-paste-token") {
		t.Fatalf("wizard output = %s", out.String())
	}
}

func TestFirstRunCreatesExplicitGenericAccountWithoutBundledProviderDefault(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &scriptedPrompt{
		selections: []string{"codex"},
		secrets:    []string{"one-paste-token"},
		texts: []string{
			"team-gateway",
			"Team Gateway",
			"https://gateway.test/v1",
			"gpt-5.6-terra",
			"gpt-5.6-terra",
		},
	}

	if err := cli.Execute(app, []string{}); err != nil {
		t.Fatal(err)
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if secretExists(t, secretStore, "dmx") {
		t.Fatal("first-run wizard seeded a DMX Token slot")
	}
	if !secretExists(t, secretStore, "team-gateway") {
		t.Fatal("generic Account Token was not stored")
	}
	profile, ok := cfg.Profiles["gpt-5.6-terra"]
	if !ok || profile.Account != "team-gateway" || profile.Client != "codex" || profile.Model != "gpt-5.6-terra" {
		t.Fatalf("generic profile = %#v", profile)
	}
	if _, exists := cfg.Accounts["dmx"]; exists {
		t.Fatalf("first-run wizard seeded a provider Account: %#v", cfg.Accounts)
	}
}

func TestSetupWithoutFlagsUsesGenericGuidedFlow(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &scriptedPrompt{
		selections: []string{"claude"},
		secrets:    []string{"one-paste-token"},
		texts: []string{
			"team-gateway",
			"Team Gateway",
			"https://gateway.test",
			"claude-sonnet-5",
			"claude-sonnet-5",
		},
	}

	if err := cli.Execute(app, []string{"setup"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !secretExists(t, secretStore, "team-gateway") || cfg.Routes[configuration.ClientClaude] != "claude-sonnet-5" {
		t.Fatalf("setup state = %#v", cfg)
	}
	text := out.String()
	if !strings.Contains(text, "aigw sync") || strings.Contains(text, "aigw check") {
		t.Fatalf("setup continuation = %q", text)
	}
}

func TestSetupWithoutFlagsRefusesBeforePromptingWhenAlreadyConfigured(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	app.Interactive = true
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "gateway", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientClaude] = "claude"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err := cli.Execute(app, []string{"setup"})
	if err == nil || !strings.Contains(err.Error(), "is already configured") {
		t.Fatalf("setup error = %v", err)
	}
}

func TestWizardFailureLeavesNoProfileSecretOrClientProjection(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &scriptedPrompt{
		selections: []string{"codex"},
		secrets:    []string{"bad-sync-token"},
		texts: []string{
			"team-gateway",
			"Team Gateway",
			"https://gateway.test/v1",
			"gpt-5.6-terra",
			"gpt-5.6-terra",
		},
	}
	app.Executable = "relative-helper"
	shimDir := t.TempDir()
	codexTarget := filepath.Join(t.TempDir(), "configuration.toml")
	original := "model_provider = \"native\"\n"
	if err := os.WriteFile(codexTarget, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientClaude: "/opt/claude-real", configuration.ClientCodex: "/opt/codex-real"}, Surfaces: []discovery.Surface{{
			ID:          string(surfaceidentity.CodexHomeDefault),
			Authority:   string(surfaceidentity.AuthorityAIGW),
			ConfigPath:  codexTarget,
			Present:     true,
			AutoManaged: true,
		}},
	}}
	err := cli.Execute(app, []string{})
	if err == nil || !strings.Contains(err.Error(), "preflight failed") || strings.Contains(err.Error(), "configuration was rolled back") {
		t.Fatalf("error = %v", err)
	}
	if secretExists(t, secretStore, "team-gateway") {
		t.Fatal("failed wizard left secret")
	}
	if _, err := os.Stat(app.Config.Path()); !os.IsNotExist(err) {
		t.Fatalf("failed wizard left config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(shimDir, "claude")); !os.IsNotExist(err) {
		t.Fatalf("failed wizard left a Claude command-path artifact: %v", err)
	}
	data, _ := os.ReadFile(codexTarget)
	if string(data) != original {
		t.Fatalf("failed wizard changed Codex config:\n%s", data)
	}
}

func TestNoArgsStaysNonInteractiveInPipelines(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	if err := cli.Execute(app, []string{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Next") || !strings.Contains(out.String(), "aigw") {
		t.Fatalf("noninteractive output = %s", out.String())
	}
}
