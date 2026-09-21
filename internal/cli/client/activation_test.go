package client

import (
	"path/filepath"
	"slices"
	"testing"

	configuration "aigw-cli/internal/configuration"
)

func TestEnableMaterializesDeferredClientIntent(t *testing.T) {
	cfg := adapterConfig()
	cfg.SetClientActivation(configuration.ClientClaude, true, "", nil)
	runtime, _, secretStore, _ := adapterRuntime(t, cfg)
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
}

func TestEnablePreservesExistingHermesTarget(t *testing.T) {
	cfg := adapterConfig()
	target := filepath.Join(t.TempDir(), "config.yaml")
	cfg.Clients[configuration.ClientHermes] = configuration.ClientBinding{
		Profile:    "claude",
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
