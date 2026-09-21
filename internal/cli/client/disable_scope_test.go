package client

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	claudedesktop "aigw-cli/internal/claude/desktop"
	configuration "aigw-cli/internal/configuration"
)

func TestDisableScopesProjectionToTheSelectedClient(t *testing.T) {
	cfg := adapterConfig()
	cfg.Profiles["desktop"] = configuration.Profile{
		Label: "Desktop", Account: "gateway", Model: "claude-test", Protocols: []configuration.EndpointProtocol{configuration.ProtocolAnthropic},
	}
	cfg.SetSelectedProfile(configuration.ClientClaudeDesktop, "desktop")
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
	if err := executeAdapter(t, runtime, "enable", configuration.ClientClaudeDesktop, "--executable", executable, "--target", library); err != nil {
		t.Fatal(err)
	}
	configured, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	configured.SetSelectedProfile(configuration.ClientHermes, "desktop")
	configured.SetClientActivation(configuration.ClientHermes, true, "/missing/hermes", nil)
	if err := runtime.Config.Save(configured); err != nil {
		t.Fatal(err)
	}

	if err := executeAdapter(t, runtime, "disable", configuration.ClientClaudeDesktop); err != nil {
		t.Fatalf("unrelated Hermes state blocked Claude Desktop disable: %v", err)
	}
	got, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Clients[configuration.ClientClaudeDesktop].Enabled {
		t.Fatal("Claude Desktop remains enabled")
	}
	paths := claudedesktop.PathsForLibrary(library)
	for _, path := range []string{paths.StandardConfig, paths.ThirdPartyConfig, paths.Profile, paths.Metadata, paths.State} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("disable left %s: %v", path, err)
		}
	}
}
