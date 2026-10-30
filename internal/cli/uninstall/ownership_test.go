package uninstall

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/store"
	"aigw-cli/internal/upgrade"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortableInstallAndUninstallPreserveHomebrewOwnership(t *testing.T) {
	root := t.TempDir()
	version := filepath.Join(root, "Cellar", "aigw", "0.1.0")
	target := filepath.Join(version, "bin", "aigw")
	source := filepath.Join(root, "download")
	for name, data := range map[string]string{target: "managed", source: "download", filepath.Join(version, "INSTALL_RECEIPT.json"): `{"homebrew_version":"7.0.2","source":{"tap":"owner/tap"}}`} {
		if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for name, operation := range map[string]func() error{
		"install":   func() error { return upgrade.Install(source, target) },
		"uninstall": func() error { return upgrade.Uninstall(target) },
		"command": func() error {
			command := NewCommand(invocation.Context{Clients: clients.Registry(), Executable: target, Config: store.NewStore("invalid\x00config", clients.Admission())})
			command.SetArgs(nil)
			return command.Execute()
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := operation()
			if err == nil || !strings.Contains(err.Error(), "Homebrew") {
				t.Fatalf("ownership error = %v", err)
			}
			data, err := os.ReadFile(target)
			if err != nil || string(data) != "managed" {
				t.Fatalf("managed program changed: %q, %v", data, err)
			}
		})
	}
}
