package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/cli/readiness"
	clientdomain "aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
)

func TestCommandBoundaryRouteAndReconciliationErrors(t *testing.T) {
	cfg := configuredCommandState()
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Targets: []string{filepath.Join(t.TempDir(), "configuration.toml")}}
	if _, err := invocation.Synchronizer((&App{}).invocationContext()).Plan(cfg, cfg); err == nil {
		t.Fatal("expected reconciliation planning error")
	}
	if err := invocation.Synchronizer((&App{}).invocationContext()).Reconcile(context.Background(), cfg, cfg); err == nil {
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
	cfg := configuredCommandState()
	runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	app := &App{}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true}
	if status := invocation.Synchronizer(app.invocationContext()).Inspect(context.Background(), cfg, configuration.ClientCodex, runtime, clientdomain.InspectionOptions{}); status.Ready || !strings.Contains(status.Issue, "executable") {
		t.Fatalf("status=%#v", status)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "codex"}
	if status := invocation.Synchronizer(app.invocationContext()).Inspect(context.Background(), cfg, configuration.ClientCodex, runtime, clientdomain.InspectionOptions{}); status.Ready || !strings.Contains(status.Issue, "target") {
		t.Fatalf("status=%#v", status)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "codex", Targets: []string{filepath.Join(t.TempDir(), "missing.toml")}}
	if status := invocation.Synchronizer(app.invocationContext()).Inspect(context.Background(), cfg, configuration.ClientCodex, runtime, clientdomain.InspectionOptions{}); status.Ready || !strings.Contains(status.Issue, "drift") {
		t.Fatalf("status=%#v", status)
	}

	claudeRuntime, _ := cfg.ResolveRuntime(configuration.ClientClaude, "")
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "claude"}
	if status := invocation.Synchronizer(app.invocationContext()).Inspect(context.Background(), cfg, configuration.ClientClaude, claudeRuntime, clientdomain.InspectionOptions{}); status.Ready || !strings.Contains(status.Issue, "executable is unavailable") {
		t.Fatalf("status=%#v", status)
	}

	profiles := configuration.NewConfig()
	profiles.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{Anthropic: "https://one.test"}}
	profiles.Profiles["skip"] = configuration.Profile{Account: "one", Client: configuration.ClientClaude, Model: "claude"}
	profiles.Profiles["generic"] = configuration.Profile{Account: "one"}
	if got := profiles.FirstProfileForClient(configuration.ClientCodex); got != "" {
		t.Fatalf("unexpected Codex profile %q", got)
	}
	if got := profiles.FirstProfileForClient(configuration.ClientClaude); got != "generic" {
		t.Fatalf("Claude profile = %q", got)
	}
}
