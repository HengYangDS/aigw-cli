package client

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

func TestClaudeDesktopLifecycleReportsRequiredRestart(t *testing.T) {
	cfg := adapterConfig()
	cfg.Models[domain.NewModelRef("gateway", "claude-test")] = domain.Model{
		Label: "Desktop", Provider: "gateway", Family: "claude-test", ID: "claude-test",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	cfg.SetSelectedModel(claudedesktop.ID, domain.NewModelRef("gateway", "claude-test"), "")
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
	if want, _ := filepath.Abs("/opt/claude"); !binding.Enabled || binding.Executable != want {
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
		Model:      domain.NewModelRef("gateway", "claude-test"),
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
	if want, _ := filepath.Abs("/opt/hermes"); !binding.Enabled || binding.Executable != want || !slices.Equal(binding.Targets, []string{target}) {
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
	reader := filepath.Join(root, "data", "credential", "aigw")
	runtime.ResolveCredentialPath = func() (string, error) { return reader, nil }
	if _, err := ensureReader(runtime.Executable, reader); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reader+".sha256", []byte("changed receipt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := executeAdapter(t, runtime, "enable", claude.ID, "--executable", "/opt/claude"); err != nil {
		t.Fatalf("external credential enablement inspected an unused cached entrypoint: %v", err)
	}
	stored, loadErr := runtime.Config.Load()
	if loadErr != nil || !stored.Clients[claude.ID].Enabled {
		t.Fatalf("committed enablement was not preserved: %#v, %v", stored.Clients, loadErr)
	}
	if _, err := os.Lstat(reader); err != nil {
		t.Fatalf("enablement removed a possible cached command: %v", err)
	}
}

// ensureReader installs the program at source as the credential reader at
// target, or verifies the one there, as a synchronization does, and returns
// the compensation that removes only what it created.
func ensureReader(source, target string) (func() error, error) {
	program, err := secret.ReadProgram(nil, source)
	if err != nil {
		return nil, err
	}
	_, undo, err := program.EnsureEntrypoint(target)
	return undo, err
}
