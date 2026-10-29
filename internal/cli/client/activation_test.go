package client

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/claudedesktop"
	"aigw-cli/internal/client/hermes"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/domain"
)

func TestClaudeDesktopLifecycleReportsRequiredRestart(t *testing.T) {
	cfg := adapterConfig()
	cfg.Routes["desktop"] = domain.Route{
		Label: "Desktop", Account: "gateway", Model: "claude-test",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	cfg.SetSelectedRoute(claudedesktop.ID, "desktop", "")
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

	if err := executeAdapter(t, runtime, "enable", claudedesktop.ID, "--executable", executable, "--target", library); err != nil {
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
	if err := executeAdapter(t, runtime, "disable", claudedesktop.ID); err != nil {
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
	cfg.SetClientActivation(claude.ID, true, "", nil)
	runtime, out, secretStore, _ := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	if err := executeAdapter(t, runtime, "enable", claude.ID, "--executable", "/opt/claude"); err != nil {
		t.Fatal(err)
	}
	got, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := got.Clients[claude.ID]
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
	cfg.Clients[hermes.ID] = domain.ClientBinding{
		Route:      "claude",
		Protocol:   domain.ProtocolAnthropic,
		Executable: "/previous/hermes",
		Targets:    []string{target},
	}
	runtime, _, secretStore, _ := adapterRuntime(t, cfg)
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	if err := executeAdapter(t, runtime, "enable", hermes.ID, "--executable", "/opt/hermes"); err != nil {
		t.Fatal(err)
	}
	got, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := got.Clients[hermes.ID]
	if !binding.Enabled || binding.Executable != "/opt/hermes" || !slices.Equal(binding.Targets, []string{target}) {
		t.Fatalf("re-enabled client = %#v", binding)
	}
	if err := executeAdapter(t, runtime, "disable", hermes.ID); err != nil {
		t.Fatal(err)
	}
	disabled, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Clients[hermes.ID].Enabled || !slices.Equal(disabled.Clients[hermes.ID].Targets, []string{target}) {
		t.Fatalf("disabled client = %#v", disabled.Clients[hermes.ID])
	}
	if err := executeAdapter(t, runtime, "enable", hermes.ID, "--executable", "/opt/hermes"); err != nil {
		t.Fatal(err)
	}
}

func TestEnableWithExternalCredentialPreservesUnusedCachedEntrypoint(t *testing.T) {
	cfg := adapterConfig()
	root := t.TempDir()
	external := filepath.Join(root, "external-helper")
	binding := cfg.Clients[claude.ID]
	binding.CredentialCommand = external
	cfg.Clients[claude.ID] = binding
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
	if err := executeAdapter(t, runtime, "enable", claude.ID, "--executable", "/opt/claude"); err != nil {
		t.Fatalf("external credential enablement inspected an unused cached entrypoint: %v", err)
	}
	stored, loadErr := runtime.Config.Load()
	if loadErr != nil || !stored.Clients[claude.ID].Enabled {
		t.Fatalf("committed enablement was not preserved: %#v, %v", stored.Clients, loadErr)
	}
	if _, err := os.Lstat(runtime.CredentialPath); err != nil {
		t.Fatalf("enablement removed a possible cached command: %v", err)
	}
}
