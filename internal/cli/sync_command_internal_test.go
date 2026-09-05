package cli

import (
	"context"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestCodexReconciliationAndAuthenticationInputFailures(t *testing.T) {
	base := configuredCommandState()
	base.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/codex", Targets: []string{"/target"}}
	if _, err := invocation.Synchronizer((&App{}).invocationContext()).Plan(base, base); err == nil {
		t.Fatal("expected discovery error")
	}
	app := &App{Discovery: staticDiscovery{}}
	before := base.Clone()
	before.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Targets: []string{""}}
	if _, err := invocation.Synchronizer(app.invocationContext()).Plan(before, base); err == nil {
		t.Fatal("expected before target error")
	}
	after := base.Clone()
	after.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Targets: []string{""}}
	if _, err := invocation.Synchronizer(app.invocationContext()).Plan(base, after); err == nil {
		t.Fatal("expected after target error")
	}
	if err := invocation.Synchronizer((&App{}).invocationContext()).BindCredential(context.Background(), configuredCommandState(), configuration.ClientCodex, nil); err == nil || !strings.Contains(err.Error(), "enabled adapter") {
		t.Fatalf("error = %v", err)
	}
	missingExecutable := base.Clone()
	missingExecutable.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true}
	if err := invocation.Synchronizer((&App{}).invocationContext()).BindCredential(context.Background(), missingExecutable, configuration.ClientCodex, nil); err == nil || !strings.Contains(err.Error(), "executable") {
		t.Fatalf("error = %v", err)
	}
	app = &App{Runner: commandRunner{}, Secrets: secrets.NewMemoryStore()}
	if err := invocation.Synchronizer(app.invocationContext()).BindCredential(context.Background(), base, configuration.ClientCodex, nil); err == nil || !strings.Contains(err.Error(), "Token") {
		t.Fatalf("error = %v", err)
	}

	invalid := configuration.NewConfig()
	invalid.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true}
	if invocation.Synchronizer(app.invocationContext()).UsesCredentialAccount(invalid, "gateway") {
		t.Fatal("invalid route used a credential account")
	}
	if !invocation.Synchronizer(app.invocationContext()).ProjectionChanged(base, invalid) {
		t.Fatal("invalid projection was not detected as changed")
	}
}
