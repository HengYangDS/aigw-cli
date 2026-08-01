package cli

import (
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/synchronization"
	"context"
	"strings"
	"testing"
)

func TestCodexReconciliationAndAuthenticationInputFailures(t *testing.T) {
	base := configuredCommandState()
	base.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/codex", Targets: []string{"/target"}}
	if _, err := (&App{}).synchronizer().Plan(base, base); err == nil {
		t.Fatal("expected discovery error")
	}
	app := &App{Discovery: staticDiscovery{}}
	before := base.Clone()
	before.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Targets: []string{""}}
	if _, err := app.synchronizer().Plan(before, base); err == nil {
		t.Fatal("expected before target error")
	}
	after := base.Clone()
	after.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Targets: []string{""}}
	if _, err := app.synchronizer().Plan(base, after); err == nil {
		t.Fatal("expected after target error")
	}
	if err := (&App{}).synchronizer().BindAuthenticationTargets(context.Background(), configuredCommandState(), nil); err == nil || !strings.Contains(err.Error(), "enabled adapter") {
		t.Fatalf("error = %v", err)
	}
	missingExecutable := base.Clone()
	missingExecutable.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true}
	if err := (&App{}).synchronizer().BindAuthenticationTargets(context.Background(), missingExecutable, nil); err == nil || !strings.Contains(err.Error(), "executable") {
		t.Fatalf("error = %v", err)
	}
	app = &App{Runner: commandRunner{}, Secrets: secrets.NewMemoryStore()}
	if err := app.synchronizer().BindAuthenticationTargets(context.Background(), base, nil); err == nil || !strings.Contains(err.Error(), "Token") {
		t.Fatalf("error = %v", err)
	}

	invalid := configuration.NewConfig()
	invalid.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true}
	if _, ok := synchronization.RouteAccount(invalid); ok {
		t.Fatal("invalid route returned an account")
	}
	if !synchronization.ProjectionChanged(base, invalid) {
		t.Fatal("invalid projection was not detected as changed")
	}
}
