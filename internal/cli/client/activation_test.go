package client

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
)

func TestClaudeDesktopLifecycleReportsRequiredRestart(t *testing.T) {
	cfg := adapterConfig()
	cfg.Routes["desktop"] = configuration.Route{
		Label: "Desktop", Account: "gateway", Model: "claude-test",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}},
	}
	cfg.SetSelectedRoute(configuration.ClientClaudeDesktop, "desktop")
	runtime, out, secretStore, _ := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	executable := filepath.Join(root, "bin", "Claude")
	if err := os.MkdirAll(filepath.Dir(executable), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	library := filepath.Join(root, "Claude-3p", "configLibrary")

	if err := executeAdapter(t, runtime, "enable", configuration.ClientClaudeDesktop, "--executable", executable, "--target", library); err != nil {
		t.Fatal(err)
	}
	enabled := out.String()
	for _, want := range []string{"Client configured", "Restart required", "Restart Claude Desktop, then run `aigw check`"} {
		if !strings.Contains(enabled, want) {
			t.Fatalf("enable output = %q, want %q", enabled, want)
		}
	}
	if strings.Contains(enabled, "Client enabled") {
		t.Fatalf("enable output claims activation before restart: %q", enabled)
	}

	out.Reset()
	if err := executeAdapter(t, runtime, "disable", configuration.ClientClaudeDesktop); err != nil {
		t.Fatal(err)
	}
	disabled := out.String()
	for _, want := range []string{"Restart required", "Restart Claude Desktop to finish deactivation"} {
		if !strings.Contains(disabled, want) {
			t.Fatalf("disable output = %q, want %q", disabled, want)
		}
	}
}

func TestEnableMaterializesDeferredClientIntent(t *testing.T) {
	cfg := adapterConfig()
	cfg.SetClientActivation(configuration.ClientClaude, true, "", nil)
	runtime, out, secretStore, _ := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	if err := executeAdapter(t, runtime, "enable", configuration.ClientClaude, "--executable", "/opt/claude"); err != nil {
		t.Fatal(err)
	}
	got, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := got.Clients[configuration.ClientClaude]
	if !binding.Enabled || binding.Executable != "/opt/claude" {
		t.Fatalf("materialized client = %#v", binding)
	}
	if strings.Contains(out.String(), "Restart required") {
		t.Fatalf("Claude Code enablement reported a Desktop restart: %q", out.String())
	}
}

func TestEnablePreservesExistingHermesTarget(t *testing.T) {
	cfg := adapterConfig()
	target := filepath.Join(t.TempDir(), "config.yaml")
	cfg.Clients[configuration.ClientHermes] = configuration.ClientBinding{
		Route:      "claude",
		Protocol:   configuration.ProtocolAnthropic,
		Executable: "/previous/hermes",
		Targets:    []string{target},
	}
	runtime, _, secretStore, _ := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	if err := executeAdapter(t, runtime, "enable", configuration.ClientHermes, "--executable", "/opt/hermes"); err != nil {
		t.Fatal(err)
	}
	got, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := got.Clients[configuration.ClientHermes]
	if !binding.Enabled || binding.Executable != "/opt/hermes" || !slices.Equal(binding.Targets, []string{target}) {
		t.Fatalf("re-enabled client = %#v", binding)
	}
	if err := executeAdapter(t, runtime, "disable", configuration.ClientHermes); err != nil {
		t.Fatal(err)
	}
	disabled, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Clients[configuration.ClientHermes].Enabled || !slices.Equal(disabled.Clients[configuration.ClientHermes].Targets, []string{target}) {
		t.Fatalf("disabled client = %#v", disabled.Clients[configuration.ClientHermes])
	}
	if err := executeAdapter(t, runtime, "enable", configuration.ClientHermes, "--executable", "/opt/hermes"); err != nil {
		t.Fatal(err)
	}
}

func TestEnableDoesNotClaimRollbackAfterCommittedEntrypointCleanupFailure(t *testing.T) {
	cfg := adapterConfig()
	root := t.TempDir()
	external := filepath.Join(root, "external-helper")
	binding := cfg.Clients[configuration.ClientClaude]
	binding.CredentialCommand = external
	cfg.Clients[configuration.ClientClaude] = binding
	runtime, _, _, _ := adapterRuntime(t, cfg)
	if err := os.WriteFile(runtime.Executable, []byte("AIGW fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	runtime.CredentialPath = filepath.Join(root, "data", "credential", "aigw")
	if _, err := credential.EnsureEntrypoint(runtime.Executable, runtime.CredentialPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtime.CredentialPath+".sha256", []byte("changed receipt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := executeAdapter(t, runtime, "enable", configuration.ClientClaude, "--executable", "/opt/claude")
	if err == nil || !strings.Contains(err.Error(), "configuration and client projections completed") || strings.Contains(err.Error(), "was rolled back") {
		t.Fatalf("enablement reported a false rollback: %v", err)
	}
	stored, loadErr := runtime.Config.Load()
	if loadErr != nil || !stored.Clients[configuration.ClientClaude].Enabled {
		t.Fatalf("committed enablement was not preserved: %#v, %v", stored.Clients, loadErr)
	}
}
