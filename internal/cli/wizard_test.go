package cli_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/cli"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
)

type fakePrompt struct {
	secret      string
	secretCalls int
	selected    string
	text        string
}

func (p *fakePrompt) Secret(string) (string, error) {
	p.secretCalls++
	if p.secret == "" {
		return "", errors.New("no secret")
	}
	return p.secret, nil
}

func (p *fakePrompt) Text(string) (string, error) {
	if p.text == "" {
		return "", errors.New("no text")
	}
	return p.text, nil
}

func (p *fakePrompt) Select(_ string, _ []cli.Choice) (string, error) {
	return p.selected, nil
}

type fakeDiscovery struct{ result discovery.Result }

func (d fakeDiscovery) Discover() discovery.Result { return d.result }

func TestNoArgsRunsAutomaticFirstUseWizard(t *testing.T) {
	app, out, secretStore, runner := testApp(t, "")
	app.Interactive = true
	prompt := &fakePrompt{secret: "one-paste-token"}
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
		CodexTargets:     []string{codexTarget},
	}}
	if err := execute(t, app); err != nil {
		t.Fatal(err)
	}
	if prompt.secretCalls != 1 {
		t.Fatalf("token prompts = %d, want 1", prompt.secretCalls)
	}
	if !secretStore.Has("dmx") {
		t.Fatal("DMX token was not stored")
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Routes.Default != "dmx" || !cfg.Adapters["claude"].Enabled || !cfg.Adapters["codex"].Enabled {
		t.Fatalf("configured state = %#v", cfg)
	}
	if len(runner.plans) != 1 || runner.plans[0].Executable != "/opt/codex-real" {
		t.Fatalf("Codex login plans = %#v", runner.plans)
	}
	if _, err := os.Stat(filepath.Join(shimDir, "claude")); err != nil {
		t.Fatalf("Claude shim missing: %v", err)
	}
	if !strings.Contains(out.String(), "已就绪") || strings.Contains(out.String(), "one-paste-token") {
		t.Fatalf("wizard output = %s", out.String())
	}
}

func TestWizardFailureLeavesNoProfileSecretOrClientProjection(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &fakePrompt{secret: "bad-sync-token"}
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
		ClaudeExecutable: "/opt/claude-real", CodexExecutable: "/opt/codex-real", CodexTargets: []string{codexTarget},
	}}
	err := execute(t, app)
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("error = %v", err)
	}
	if secretStore.Has("dmx") {
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
	if !strings.Contains(out.String(), "下一步") || !strings.Contains(out.String(), "aigw") {
		t.Fatalf("noninteractive output = %s", out.String())
	}
}
