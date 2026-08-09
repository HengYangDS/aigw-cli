package cli

import (
	"aigw-cli/internal/cli/readiness"
	configuration "aigw-cli/internal/configuration"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandBoundaryRouteAndReconciliationErrors(t *testing.T) {
	cfg := configuredCommandState()
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Targets: []string{filepath.Join(t.TempDir(), "configuration.toml")}}
	if _, err := (&App{}).synchronizer().Plan(cfg, cfg); err == nil {
		t.Fatal("expected reconciliation planning error")
	}
	if err := (&App{}).synchronizer().Reconcile(context.Background(), cfg, cfg); err == nil {
		t.Fatal("expected reconciliation error")
	}

	broken := configuredCommandState()
	profile := broken.Profiles["one"]
	profile.Account = "missing"
	broken.Profiles["alias"] = profile
	if _, _, err := broken.ResolveAccount("alias"); err == nil || !strings.Contains(err.Error(), "unknown account") {
		t.Fatalf("account error = %v", err)
	}
}

func TestRouteAndAdapterReadinessHelpers(t *testing.T) {
	if readiness.TransportStatus("%").Kind != "" {
		t.Fatal("invalid URL has transport state")
	}
	if got := readiness.CodexModelsEndpoint("https://one.test/models/"); got != "https://one.test/models" {
		t.Fatalf("models endpoint = %q", got)
	}

	cfg := configuredCommandState()
	runtime, _, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	app := &App{}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true}
	if ready, issue := readiness.AdapterRouteReady(app.invocationContext(), cfg, configuration.ClientCodex, runtime); ready || !strings.Contains(issue, "executable") {
		t.Fatalf("ready=%v issue=%q", ready, issue)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "codex"}
	if ready, issue := readiness.AdapterRouteReady(app.invocationContext(), cfg, configuration.ClientCodex, runtime); ready || !strings.Contains(issue, "target") {
		t.Fatalf("ready=%v issue=%q", ready, issue)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "codex", Targets: []string{filepath.Join(t.TempDir(), "missing.toml")}}
	if ready, issue := readiness.AdapterRouteReady(app.invocationContext(), cfg, configuration.ClientCodex, runtime); ready || !strings.Contains(issue, "drift") {
		t.Fatalf("ready=%v issue=%q", ready, issue)
	}

	claudeRuntime, _, _ := cfg.ResolveRuntime(configuration.ClientClaude, "")
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "claude"}
	if ready, issue := readiness.AdapterRouteReady(app.invocationContext(), cfg, configuration.ClientClaude, claudeRuntime); ready || !strings.Contains(issue, "executable is unavailable") {
		t.Fatalf("ready=%v issue=%q", ready, issue)
	}

	profiles := configuration.NewConfig()
	profiles.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{Anthropic: "https://one.test"}}
	profiles.Profiles["skip"] = configuration.Profile{Account: "one", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "claude"}}
	profiles.Profiles["generic"] = configuration.Profile{Account: "one"}
	if got := profiles.FirstProfileForClient(configuration.ClientCodex); got != "" {
		t.Fatalf("unexpected Codex profile %q", got)
	}
	if got := profiles.FirstProfileForClient(configuration.ClientClaude); got != "generic" {
		t.Fatalf("Claude profile = %q", got)
	}
}
