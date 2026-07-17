package cli_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/cli"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

type fakePrompt struct {
	secret          string
	secretCalls     int
	lastSecretLabel string
	selected        string
	choices         []cli.Choice
	text            string
	texts           []string
	textCalls       int
}

func (p *fakePrompt) Secret(label string) (string, error) {
	p.secretCalls++
	p.lastSecretLabel = label
	if p.secret == "" {
		return "", errors.New("no secret")
	}
	return p.secret, nil
}

func (p *fakePrompt) Text(string) (string, error) {
	if p.textCalls < len(p.texts) {
		value := p.texts[p.textCalls]
		p.textCalls++
		return value, nil
	}
	if p.text == "" {
		return "", errors.New("no text")
	}
	return p.text, nil
}

func (p *fakePrompt) Select(_ string, choices []cli.Choice) (string, error) {
	p.choices = append([]cli.Choice(nil), choices...)
	return p.selected, nil
}

type fakeDiscovery struct{ result discovery.Result }

func (d fakeDiscovery) Discover() discovery.Result { return d.result }

func TestNoArgsRunsAutomaticFirstUseWizard(t *testing.T) {
	app, out, secretStore, runner := testApp(t, "")
	app.Interactive = true
	prompt := &fakePrompt{
		selected: "codex",
		secret:   "one-paste-token",
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
	app.Shims.BinDir = shimDir
	app.Shims.AIGWExecutable = filepath.Join(shimDir, "aigw")
	codexTarget := filepath.Join(t.TempDir(), "codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(codexTarget), 0o700); err != nil {
		t.Fatal(err)
	}
	originalCodex := "model_provider = \"native\"\n"
	if err := os.WriteFile(codexTarget, []byte(originalCodex), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		ClaudeExecutable: "/opt/claude-real",
		CodexExecutable:  "/opt/codex-real",
		Surfaces: []discovery.Surface{{
			ID:          discovery.SurfaceCodexCLIStandalone,
			Authority:   discovery.AuthorityAIGW,
			ConfigPath:  codexTarget,
			Present:     true,
			AutoManaged: true,
		}},
	}}
	if err := execute(t, app); err != nil {
		t.Fatal(err)
	}
	if prompt.secretCalls != 1 {
		t.Fatalf("token prompts = %d, want 1", prompt.secretCalls)
	}
	if !secretStore.Has("team-gateway") {
		t.Fatal("generic Account Token was not stored")
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Routes.Default != "gpt-5.6-terra" || cfg.Adapters["claude"].Enabled || !cfg.Adapters["codex"].Enabled {
		t.Fatalf("configured state = %#v", cfg)
	}
	if len(runner.plans) != 1 || runner.plans[0].Executable != "/opt/codex-real" {
		t.Fatalf("Codex login plans = %#v", runner.plans)
	}
	if _, err := os.Stat(filepath.Join(shimDir, "claude")); !os.IsNotExist(err) {
		t.Fatalf("Codex-only first-run wizard created a Claude shim: %v", err)
	}
	if !strings.Contains(out.String(), "Ready") || strings.Contains(out.String(), "one-paste-token") {
		t.Fatalf("wizard output = %s", out.String())
	}
}

func TestFirstRunCreatesExplicitGenericAccountWithoutBundledProviderDefault(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &fakePrompt{
		selected: "codex",
		secret:   "one-paste-token",
		texts: []string{
			"team-gateway",
			"Team Gateway",
			"https://gateway.test/v1",
			"gpt-5.6-terra",
			"gpt-5.6-terra",
		},
	}

	if err := execute(t, app); err != nil {
		t.Fatal(err)
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if secretStore.Has("dmx") {
		t.Fatal("first-run wizard seeded a DMX Token slot")
	}
	if !secretStore.Has("team-gateway") {
		t.Fatal("generic Account Token was not stored")
	}
	profile, ok := cfg.Profiles["gpt-5.6-terra"]
	if !ok || profile.Account != "team-gateway" || profile.Client != "codex" || profile.Models[domain.ClientCodex] != "gpt-5.6-terra" {
		t.Fatalf("generic profile = %#v", profile)
	}
	if _, exists := cfg.Accounts["dmx"]; exists {
		t.Fatalf("first-run wizard seeded a provider Account: %#v", cfg.Accounts)
	}
}

func TestSetupWithoutFlagsUsesGenericGuidedFlow(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &fakePrompt{
		selected: "claude",
		secret:   "one-paste-token",
		texts: []string{
			"team-gateway",
			"Team Gateway",
			"https://gateway.test",
			"claude-sonnet-5",
			"claude-sonnet-5",
		},
	}

	if err := execute(t, app, "setup"); err != nil {
		t.Fatal(err)
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !secretStore.Has("team-gateway") || cfg.Routes.Default != "claude-sonnet-5" {
		t.Fatalf("setup state = %#v", cfg)
	}
}

func TestSetupWithoutFlagsRefusesBeforePromptingWhenAlreadyConfigured(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	app.Interactive = true
	cfg := domain.NewConfig()
	cfg.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Profiles["claude"] = domain.Profile{Label: "Claude", Account: "gateway", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude-test"}}
	cfg.Routes.Default = "claude"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err := execute(t, app, "setup")
	if err == nil || !strings.Contains(err.Error(), "is already configured") {
		t.Fatalf("setup error = %v", err)
	}
}

func TestWizardFailureLeavesNoProfileSecretOrClientProjection(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &fakePrompt{
		selected: "codex",
		secret:   "bad-sync-token",
		texts: []string{
			"team-gateway",
			"Team Gateway",
			"https://gateway.test/v1",
			"gpt-5.6-terra",
			"gpt-5.6-terra",
		},
	}
	app.Runner = &failingRunner{err: errors.New("Codex login failed"), remaining: 1}
	shimDir := t.TempDir()
	app.Shims.BinDir = shimDir
	app.Shims.AIGWExecutable = filepath.Join(shimDir, "aigw")
	codexTarget := filepath.Join(t.TempDir(), "config.toml")
	original := "model_provider = \"native\"\n"
	if err := os.WriteFile(codexTarget, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		ClaudeExecutable: "/opt/claude-real", CodexExecutable: "/opt/codex-real", Surfaces: []discovery.Surface{{
			ID:          discovery.SurfaceCodexCLIStandalone,
			Authority:   discovery.AuthorityAIGW,
			ConfigPath:  codexTarget,
			Present:     true,
			AutoManaged: true,
		}},
	}}
	err := execute(t, app)
	if err == nil || !strings.Contains(err.Error(), "was rolled back") {
		t.Fatalf("error = %v", err)
	}
	if secretStore.Has("team-gateway") {
		t.Fatal("failed wizard left secret")
	}
	if _, err := os.Stat(app.Config.Path()); !os.IsNotExist(err) {
		t.Fatalf("failed wizard left config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(shimDir, "claude")); !os.IsNotExist(err) {
		t.Fatalf("failed wizard left Claude shim: %v", err)
	}
	data, _ := os.ReadFile(codexTarget)
	if string(data) != original {
		t.Fatalf("failed wizard changed Codex config:\n%s", data)
	}
}

func TestNoArgsStaysNonInteractiveInPipelines(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	if err := execute(t, app); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Next") || !strings.Contains(out.String(), "aigw") {
		t.Fatalf("noninteractive output = %s", out.String())
	}
}
