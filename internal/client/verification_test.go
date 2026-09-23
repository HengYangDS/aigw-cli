package client

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/claude"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
)

type profileVerificationRunner struct {
	plans  []process.Plan
	config []byte
}

func (runner *profileVerificationRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	runner.plans = append(runner.plans, plan)
	if slices.Equal(plan.Args, []string{"--version"}) {
		return []byte("codex-cli 9.9.9\n"), nil
	}
	outputPath := argumentValue(plan.Args, "--output-last-message")
	if outputPath == "" {
		return []byte("AIGW_OK\n"), nil
	}
	home := environmentValue(plan.Env, "CODEX_HOME")
	var err error
	runner.config, err = os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil {
		return nil, err
	}
	return nil, os.WriteFile(outputPath, []byte("AIGW_OK\n"), 0o600)
}

func argumentValue(arguments []string, name string) string {
	for index, argument := range arguments {
		if argument == name && index+1 < len(arguments) {
			return arguments[index+1]
		}
	}
	return ""
}

func environmentValue(environment []string, name string) string {
	prefix := name + "="
	for _, entry := range environment {
		if value, ok := strings.CutPrefix(entry, prefix); ok {
			return value
		}
	}
	return ""
}

func TestCodexVerificationUsesAnIsolatedProjectionForAnUnselectedProfile(t *testing.T) {
	cfg, selected := codexVerificationFixture(t)
	target := cfg.Clients[configuration.ClientCodex].Targets[0]
	if err := codex.DisableConfig(target); err != nil {
		t.Fatal(err)
	}
	base, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	const userCatalog = `model_catalog_json = "/owned/catalog.json"`
	if err := os.WriteFile(target, append([]byte(userCatalog+"\n"), base...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := codex.SyncConfig(target, selected); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes["alternate"] = qualifiedRoute("", "gateway", "gpt-alternate", configuration.ProtocolOpenAIResponses)
	runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "alternate")
	if err != nil {
		t.Fatal(err)
	}
	runner := &profileVerificationRunner{}
	if _, err := (codexAdapter{}).Verify(context.Background(), Dependencies{
		Runner: runner, AIGWExecutable: filepath.Join(t.TempDir(), "aigw"),
	}, cfg, runtime, "alternate"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(before, after) {
		t.Fatal("explicit Profile verification changed the selected Codex projection")
	}
	request := runner.plans[len(runner.plans)-1]
	verificationHome := ""
	for _, entry := range request.Env {
		if value, ok := strings.CutPrefix(entry, "CODEX_HOME="); ok {
			verificationHome = value
			break
		}
	}
	if verificationHome == filepath.Dir(target) || verificationHome == "" {
		t.Fatalf("CODEX_HOME = %q, want an isolated projection", verificationHome)
	}
	if !strings.Contains(string(runner.config), userCatalog) {
		t.Fatalf("isolated Codex projection did not preserve user configuration:\n%s", runner.config)
	}
	if _, err := os.Stat(verificationHome); !os.IsNotExist(err) {
		t.Fatalf("isolated Codex projection survived verification: %s: %v", verificationHome, err)
	}
}

func TestClaudeVerificationUsesAnIsolatedProjectionForAnUnselectedProfile(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "claude")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Routes["selected"] = qualifiedRoute("", "gateway", "claude-selected", configuration.ProtocolAnthropic)
	cfg.Routes["alternate"] = qualifiedRoute("", "gateway", "claude-alternate", configuration.ProtocolAnthropic)
	cfg.SetSelectedRoute(configuration.ClientClaude, "selected")
	cfg.SetClientActivation(configuration.ClientClaude, true, executable, nil)
	selected, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(root, "settings.json")
	aigwExecutable := filepath.Join(root, "aigw")
	if _, err := claude.ReconcileSettings(settings, false, selected, aigwExecutable, selected.Model); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := cfg.ResolveRuntime(configuration.ClientClaude, "alternate")
	if err != nil {
		t.Fatal(err)
	}
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runner := &profileVerificationRunner{}
	if _, err := (claudeAdapter{}).Verify(context.Background(), Dependencies{
		Runner: runner, Secrets: store, ClaudeSettingsPath: settings, AIGWExecutable: aigwExecutable,
	}, cfg, runtime, "alternate"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(before, after) {
		t.Fatal("explicit Profile verification changed the selected Claude projection")
	}
	request := runner.plans[len(runner.plans)-1]
	settingsIndex := slices.Index(request.Args, "--settings")
	if settingsIndex < 0 || settingsIndex+1 >= len(request.Args) {
		t.Fatalf("verification plan has no settings path: %#v", request)
	}
	verificationSettings := request.Args[settingsIndex+1]
	if verificationSettings == settings {
		t.Fatalf("verification settings = %q, want an isolated projection", verificationSettings)
	}
	if _, err := os.Stat(filepath.Dir(verificationSettings)); !os.IsNotExist(err) {
		t.Fatalf("isolated Claude projection survived verification: %s: %v", verificationSettings, err)
	}
}
