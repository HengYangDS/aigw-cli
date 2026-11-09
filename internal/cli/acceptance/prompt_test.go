package cli_test

import (
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
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
	choices     []terminal.Choice
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

func (p *scriptedPrompt) Select(_ string, choices []terminal.Choice) (string, error) {
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
					return prompt.Select("Route", nil)
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

func choiceValues(choices []terminal.Choice) []string {
	values := make([]string, 0, len(choices))
	for _, choice := range choices {
		values = append(values, choice.Value)
	}
	return values
}

func TestUseWithoutNameSelectsModelAndCollectsMissingToken(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	if err := app.Config.Save(twoModelConfig()); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-token")
	prompt := &scriptedPrompt{selections: []string{"two/model-two"}, secrets: []string{"two-token"}}
	app.Interactive = true
	app.Prompt = prompt
	if err := cli.Execute(app, []string{"use", "claude"}); err != nil {
		t.Fatal(err)
	}
	cfg, _ := app.Config.Load()
	if cfg.SelectedModel(claude.ID) != "two" || cfg.SelectedModel(codex.ID) != "one" || !secretExists(t, secretStore, "two") || len(prompt.secretCalls) != 1 {
		t.Fatalf("config=%#v hasSecret=%v prompts=%d", cfg, secretExists(t, secretStore, "two"), len(prompt.secretCalls))
	}
}

func TestUseWithoutModelRequiresInteractiveTerminal(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "one", "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-one")
	cfg.SetSelectedModel(claude.ID, "one", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	err := cli.Execute(app, []string{"use", "claude"})
	if err == nil || !strings.Contains(err.Error(), "aigw use without a terminal needs a <provider/model> argument after the client") {
		t.Fatalf("error = %v", err)
	}
}

func TestUseWithoutModelPromptsInteractively(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "one", "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-one")
	addProviderModel(&cfg, "two", "two", "Two", domain.Endpoints{Anthropic: "https://two.test"}, claude.ID, "claude-two")
	cfg.SetSelectedModel(claude.ID, "one", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	_ = secretStore.Set("two", "two-secret")
	app.Interactive = true
	app.Prompt = &scriptedPrompt{selections: []string{"two/claude-two"}}
	if err := cli.Execute(app, []string{"use", "claude"}); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectedModel(claude.ID) != "two" {
		t.Fatalf("bindings = %#v, want the interactively chosen route", got.Clients)
	}
}

func TestUseSurfacesInteractiveSelectionFailure(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "one", "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-one")
	cfg.SetSelectedModel(claude.ID, "one", "")
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
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "one", "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-one")
	addProviderModel(&cfg, "two", "two", "Two", domain.Endpoints{Anthropic: "https://two.test"}, claude.ID, "claude-two")
	cfg.SetSelectedModel(claude.ID, "one", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err := cli.Execute(app, []string{"use", "claude", "two/claude-two"})
	if err == nil || !strings.Contains(err.Error(), "has no Token, and aigw use asks for one only in a terminal") {
		t.Fatalf("error = %v", err)
	}
}

func TestUseWithMissingTokenSurfacesPromptFailure(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "one", "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-one")
	addProviderModel(&cfg, "two", "two", "Two", domain.Endpoints{Anthropic: "https://two.test"}, claude.ID, "claude-two")
	cfg.SetSelectedModel(claude.ID, "one", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	app.Interactive = true
	app.Prompt = &scriptedPrompt{}
	err := cli.Execute(app, []string{"use", "claude", "two/claude-two"})
	if err == nil || !strings.Contains(err.Error(), "no secret") {
		t.Fatalf("error = %v", err)
	}
}

func TestUseWithMissingTokenRejectsFailedVerification(t *testing.T) {
	app, _, secretStore, _, httpClient := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "one", "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-one")
	addProviderModel(&cfg, "two", "two", "Two", domain.Endpoints{Anthropic: "https://two.test"}, claude.ID, "claude-two")
	cfg.SetSelectedModel(claude.ID, "one", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	app.Interactive = true
	app.Prompt = &scriptedPrompt{secrets: []string{"new-token"}}
	httpClient.status = 401
	err := cli.Execute(app, []string{"use", "claude", "two/claude-two"})
	if err == nil || !strings.Contains(err.Error(), "Token validation failed") {
		t.Fatalf("error = %v", err)
	}
	if secretExists(t, secretStore, "two") {
		t.Fatal("a failed verification must not persist the newly entered token")
	}
}

func TestUseWithMissingTokenSurfacesSecretStoreFailure(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "one", "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-one")
	addProviderModel(&cfg, "two", "two", "Two", domain.Endpoints{Anthropic: "https://two.test"}, claude.ID, "claude-two")
	cfg.SetSelectedModel(claude.ID, "one", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	app.Interactive = true
	app.Prompt = &scriptedPrompt{secrets: []string{"new-token"}}
	want := errors.New("keychain locked")
	app.Secrets = &recordingCredentialStore[string]{backend: secret.NewMemoryStore(), setErr: want}
	err := cli.Execute(app, []string{"use", "claude", "two/claude-two"})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

// TestSetupAtATerminalRunsTheWizard: guided setup asks for a provider, its
// endpoint and Token, then, with Codex the one installed client that speaks
// the endpoint's protocol, for a model ID, which a catalog that lists none
// leaves to the person; it saves them in one transaction and checks Codex.
func TestSetupAtATerminalRunsTheWizard(t *testing.T) {
	app, out, secretStore, runner, _ := testApp(t, "")
	app.Interactive = true
	prompt := &scriptedPrompt{
		selections: []string{string(domain.ProtocolOpenAIResponses)},
		secrets:    []string{"one-paste-token"},
		texts:      []string{"team-gateway", "Team Gateway", "https://gateway.test/v1", "gpt-5.6-terra"},
	}
	app.Prompt = prompt
	codexTarget := guidedCodexTarget(t, app)
	if err := cli.Execute(app, []string{"setup"}); err != nil {
		t.Fatalf("guided setup: %v\n%s", err, out)
	}
	if len(prompt.secretCalls) != 1 || prompt.selectCalls != 1 || !secretExists(t, secretStore, "team-gateway") {
		t.Fatalf("prompts: secret %q, selections %d", prompt.secretCalls, prompt.selectCalls)
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	selected, err := cfg.ResolveRuntime(codex.ID, "")
	if err != nil || selected.ProviderID != "team-gateway" || selected.UpstreamModel != "gpt-5.6-terra" || cfg.Clients[claude.ID].Enabled || !cfg.Clients[codex.ID].Enabled {
		t.Fatalf("configured state = %+v, %v", selected, err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("guided setup ran a client: %#v", runner.plans)
	}
	projected, err := os.ReadFile(codexTarget)
	if err != nil || !strings.Contains(string(projected), selected.CredentialProjectionFingerprint(codex.ID)) {
		t.Fatalf("credential helper projection = %s, %v", projected, err)
	}
	if !strings.Contains(out.String(), "Setup saved") || !strings.Contains(out.String(), "No checked client failed") || strings.Contains(out.String(), "one-paste-token") {
		t.Fatalf("guided setup output = %s", out.String())
	}
}

// guidedCodexTarget installs Claude Code and Codex, whose file the returned
// path is.
func guidedCodexTarget(t *testing.T, app *cli.App) string {
	t.Helper()
	codexTarget := filepath.Join(t.TempDir(), "codex", "configuration.toml")
	writeFile(t, codexTarget, []byte("model_provider = \"native\"\n"), 0o600)
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{claude.ID: "/opt/claude-real", codex.ID: "/opt/codex-real"},
		Surfaces:    []discovery.Surface{{ID: string(codex.HomeDefault), ConfigPath: codexTarget, Present: true, AutoManaged: true}},
	}}
	return codexTarget
}

// With no installed client that speaks the provider's protocol, guided setup
// adds the provider with its Token and stops before a model, naming the
// client list.
func TestGuidedSetupStopsBeforeAModelWithoutAnInstalledClient(t *testing.T) {
	for _, test := range []struct {
		protocol domain.EndpointProtocol
		endpoint string
	}{
		{domain.ProtocolAnthropic, "https://messages.test"},
		{domain.ProtocolOpenAIResponses, "https://responses.test/v1"},
		{domain.ProtocolOpenAIChatCompletions, "https://chat.test/v1"},
	} {
		t.Run(string(test.protocol), func(t *testing.T) {
			app, out, secretStore, runner, _ := testApp(t, "")
			app.Interactive = true
			app.Prompt = &scriptedPrompt{selections: []string{string(test.protocol)}, secrets: []string{"wizard-token"}, texts: []string{"team", "Team", test.endpoint}}
			if err := cli.Execute(app, []string{"setup"}); err != nil {
				t.Fatal(err)
			}
			cfg, err := app.Config.Load()
			if err != nil || cfg.Providers["team"].Endpoints.For(test.protocol) != test.endpoint || len(cfg.Models) != 0 || !secretExists(t, secretStore, "team") {
				t.Fatalf("stopped setup = %+v, %v", cfg, err)
			}
			if len(runner.plans) != 0 || !strings.Contains(out.String(), "aigw client list") || strings.Contains(out.String(), "wizard-token") {
				t.Fatalf("stopped setup output = %s", out)
			}
		})
	}
}

// Guided setup seeds no provider, endpoint or Token of its own.
func TestFirstRunCreatesExplicitGenericProviderWithoutBundledProviderDefault(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &scriptedPrompt{
		selections: []string{string(domain.ProtocolOpenAIResponses)},
		secrets:    []string{"one-paste-token"},
		texts:      []string{"team-gateway", "Team Gateway", "https://gateway.test/v1"},
	}
	if err := cli.Execute(app, []string{"setup"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if secretExists(t, secretStore, "dmx") || !secretExists(t, secretStore, "team-gateway") {
		t.Fatal("guided setup seeded a Token slot or did not store its own")
	}
	if len(cfg.Providers) != 1 || cfg.Providers["team-gateway"].Label != "Team Gateway" {
		t.Fatalf("guided setup providers = %#v", cfg.Providers)
	}
}

// Guided setup on Claude Code checks it as `aigw check claude` does, after
// saving.
func TestSetupWithoutFlagsUsesGenericGuidedFlow(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	app.Interactive = true
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: executableFixture(t, "claude")}}}
	app.Prompt = &scriptedPrompt{
		selections: []string{string(domain.ProtocolAnthropic)},
		secrets:    []string{"one-paste-token"},
		texts:      []string{"team-gateway", "Team Gateway", "https://gateway.test", "claude-sonnet-5-5"},
	}
	if err := cli.Execute(app, []string{"setup"}); err != nil {
		t.Fatalf("guided setup: %v\n%s", err, out)
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	selected, err := cfg.ResolveRuntime(claude.ID, "")
	if !secretExists(t, secretStore, "team-gateway") || err != nil || selected.UpstreamModel != "claude-sonnet-5-5" {
		t.Fatalf("setup state = %+v, %v", selected, err)
	}
	if text := out.String(); !strings.Contains(text, "Setup saved") || !strings.Contains(text, "AIGW  Check") {
		t.Fatalf("setup continuation = %q", text)
	}
}

// Guided setup runs again on a configured home: it offers the provider, keeps
// its stored Token, and leaves the client's selection of the same model.
func TestGuidedSetupRunsAgainOnAConfiguredHome(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	app.Interactive = true
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: executableFixture(t, "claude")}}}
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Models["claude"] = qualifiedModel("Claude", "gateway", "claude-test", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(claude.ID, "claude", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "stored-token"); err != nil {
		t.Fatal(err)
	}
	prompt := &scriptedPrompt{selections: []string{"gateway"}, texts: []string{"claude-test"}}
	app.Prompt = prompt
	if err := cli.Execute(app, []string{"setup"}); err != nil {
		t.Fatalf("guided setup again: %v\n%s", err, out)
	}
	values := choiceValues(prompt.choices)
	if len(prompt.secretCalls) != 0 || len(values) == 0 || values[0] != "gateway" {
		t.Fatalf("prompts: secret %q, provider choices %q", prompt.secretCalls, values)
	}
	after, err := app.Config.Load()
	if err != nil || after.Clients[claude.ID].Model != "claude" || len(after.Models) != 1 {
		t.Fatalf("configured home after guided setup = %+v, %v", after, err)
	}
	if token, err := secretStore.Get("gateway"); err != nil || token != "stored-token" {
		t.Fatalf("the stored Token = %q, %v", token, err)
	}
}

// A guided setup that cannot write a client's files stores no Token, writes
// no configuration and leaves the client's file as it was.
func TestWizardFailureLeavesNoModelSecretOrClientProjection(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &scriptedPrompt{
		selections: []string{string(domain.ProtocolOpenAIResponses)},
		secrets:    []string{"bad-sync-token"},
		texts:      []string{"team-gateway", "Team Gateway", "https://gateway.test/v1", "gpt-5.6-terra"},
	}
	app.Executable = "relative-helper"
	codexTarget := guidedCodexTarget(t, app)
	original := string(readFile(t, codexTarget))
	err := cli.Execute(app, []string{"setup"})
	if err == nil || !strings.Contains(err.Error(), "preflight failed") || strings.Contains(err.Error(), "configuration was rolled back") {
		t.Fatalf("error = %v", err)
	}
	if secretExists(t, secretStore, "team-gateway") {
		t.Fatal("failed guided setup left a Token")
	}
	if _, err := os.Stat(app.Config.Path()); !os.IsNotExist(err) {
		t.Fatalf("failed guided setup left configuration: %v", err)
	}
	if data := string(readFile(t, codexTarget)); data != original {
		t.Fatalf("failed guided setup changed Codex's file:\n%s", data)
	}
}

// TestBareAigwAtATerminalNamesSetup: bare aigw at a terminal in an empty
// home is aigw status, which names `aigw setup` and asks nothing.
func TestBareAigwAtATerminalNamesSetup(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	app.Interactive, app.Prompt = true, &scriptedPrompt{}
	if err := cli.Execute(app, nil); err != nil || !strings.Contains(out.String(), "Not configured") || !strings.Contains(out.String(), "aigw setup") {
		t.Fatalf("bare aigw = %v\n%s", err, out)
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
