package client

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/client/claudedesktop"
	"aigw-cli/internal/client/hermes"
	configuration "aigw-cli/internal/configuration"
)

func TestDisableScopesProjectionToTheSelectedClient(t *testing.T) {
	cfg := adapterConfig()
	cfg.Routes["desktop"] = configuration.Route{
		Label: "Desktop", Account: "gateway", Model: "claude-test",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}},
	}
	cfg.SetSelectedRoute(claudedesktop.ID, "desktop", "")
	runtime, _, secretStore, _ := adapterRuntime(t, cfg)
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
	configured, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	configured.SetSelectedRoute(hermes.ID, "desktop", "")
	configured.SetClientActivation(hermes.ID, true, "/missing/hermes", nil)
	if err := runtime.Config.Save(configured); err != nil {
		t.Fatal(err)
	}

	if err := executeAdapter(t, runtime, "disable", claudedesktop.ID); err != nil {
		t.Fatalf("unrelated Hermes state blocked Claude Desktop disable: %v", err)
	}
	got, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Clients[claudedesktop.ID].Enabled {
		t.Fatal("Claude Desktop remains enabled")
	}
	paths := claudedesktop.PathsForLibrary(library)
	for _, path := range []string{paths.StandardConfig, paths.ThirdPartyConfig, paths.Profile, paths.Metadata, paths.State} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("disable left %s: %v", path, err)
		}
	}
}
