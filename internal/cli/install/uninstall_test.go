package install

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
)

func TestUninstallCommandDefaultsToRunningExecutableAndReportsFailure(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "aigw")
	if err := os.WriteFile(target, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := new(bytes.Buffer)
	command := NewUninstallCommand(invocation.Context{Executable: target, Out: out})
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target remains: %v", err)
	}
	if !strings.Contains(out.String(), "credential-store secrets were preserved") ||
		!strings.Contains(out.String(), "credential readers were retained") {
		t.Fatalf("output = %q", out.String())
	}
	command = NewUninstallCommand(invocation.Context{Executable: ""})
	command.SetArgs(nil)
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "target is empty") {
		t.Fatalf("empty target error = %v", err)
	}
}

func TestUninstallCommandHandlesConfigurationAndWithdrawalFailures(t *testing.T) {
	t.Run("configuration load", func(t *testing.T) {
		root := t.TempDir()
		command := NewUninstallCommand(invocation.Context{Executable: filepath.Join(root, "aigw"), Config: configuration.NewStore(root)})
		command.SetArgs(nil)
		if err := command.Execute(); err == nil {
			t.Fatal("uninstall succeeded despite an unreadable configuration path")
		}
	})

	t.Run("configuration inspection", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "aigw")
		if err := os.WriteFile(target, []byte("current"), 0o755); err != nil {
			t.Fatal(err)
		}
		command := NewUninstallCommand(invocation.Context{
			Executable: target,
			Config:     configuration.NewStore(filepath.Join(root, "invalid") + "\x00configuration.toml"),
		})
		command.SetArgs(nil)
		if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "inspect AIGW configuration") {
			t.Fatalf("configuration inspection error = %v", err)
		}
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("program was removed after failed configuration inspection: %v", err)
		}
	})

	t.Run("client withdrawal", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "aigw")
		if err := os.WriteFile(target, []byte("current"), 0o755); err != nil {
			t.Fatal(err)
		}
		store := configuration.NewStore(filepath.Join(root, "configuration.toml"))
		cfg := configuration.NewConfig()
		cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
		cfg.Routes["claude"] = configuration.Route{
			Label: "Claude", Account: "gateway", Model: "claude-test",
			Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}},
		}
		cfg.SetSelectedRoute(configuration.ClientClaude, "claude", "")
		cfg.SetClientActivation(configuration.ClientClaude, true, "/opt/claude", nil)
		if err := store.Save(cfg); err != nil {
			t.Fatal(err)
		}
		blockedSettings := filepath.Join(root, "blocked-settings")
		if err := os.Mkdir(blockedSettings, 0o700); err != nil {
			t.Fatal(err)
		}
		command := NewUninstallCommand(invocation.Context{
			Executable:         target,
			Config:             store,
			ClaudeSettingsPath: blockedSettings,
		})
		command.SetArgs(nil)
		if err := command.Execute(); err == nil {
			t.Fatal("uninstall succeeded despite failed client withdrawal")
		}
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("program was removed after failed client withdrawal: %v", err)
		}
	})
}

func TestUninstallPreservesEveryVersionedReader(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	invoker := filepath.Join(root, "invoker", "aigw")
	target := filepath.Join(root, "installed", "aigw")
	for path, content := range map[string]string{invoker: "invoker", target: "installed"} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(target, []byte("previous target version"), 0o700); err != nil {
		t.Fatal(err)
	}
	predecessorReader, err := credential.VersionedEntrypointPath(dataDir, target, "aigw")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := credential.EnsureEntrypoint(target, predecessorReader); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("installed"), 0o700); err != nil {
		t.Fatal(err)
	}
	invokerReader, err := credential.VersionedEntrypointPath(dataDir, invoker, "aigw")
	if err != nil {
		t.Fatal(err)
	}
	targetReader, err := credential.VersionedEntrypointPath(dataDir, target, "aigw")
	if err != nil {
		t.Fatal(err)
	}
	for source, reader := range map[string]string{invoker: invokerReader, target: targetReader} {
		if _, err := credential.EnsureEntrypoint(source, reader); err != nil {
			t.Fatal(err)
		}
	}
	command := NewUninstallCommand(invocation.Context{
		Executable: invoker, DataDir: dataDir, CredentialPath: invokerReader, Config: configuration.NewStore(filepath.Join(root, "config.toml")),
	})
	command.SetArgs([]string{"--target", target})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{target} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("uninstall retained target-owned path %s: %v", path, err)
		}
	}
	for _, path := range []string{invoker, invokerReader, invokerReader + ".sha256", targetReader, targetReader + ".sha256", predecessorReader, predecessorReader + ".sha256"} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("uninstall removed a retained reader or installation %s: %v", path, err)
		}
	}
}

func TestUninstallPreservesReaderSharedByIdenticalInstallations(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	invoker := filepath.Join(root, "invoker", "aigw")
	target := filepath.Join(root, "installed", "aigw")
	for _, path := range []string{invoker, target} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("identical AIGW bytes"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	invokerReader, err := credential.VersionedEntrypointPath(dataDir, invoker, "aigw")
	if err != nil {
		t.Fatal(err)
	}
	targetReader, err := credential.VersionedEntrypointPath(dataDir, target, "aigw")
	if err != nil || targetReader != invokerReader {
		t.Fatalf("identical installations did not share one reader: %v", err)
	}
	if _, err := credential.EnsureEntrypoint(invoker, invokerReader); err != nil {
		t.Fatal(err)
	}
	command := NewUninstallCommand(invocation.Context{
		Executable: invoker, DataDir: dataDir, CredentialPath: invokerReader,
		Config: configuration.NewStore(filepath.Join(root, "config.toml")),
	})
	command.SetArgs([]string{"--target", target})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("uninstall retained selected installation: %v", err)
	}
	for _, path := range []string{invoker, invokerReader, invokerReader + ".sha256"} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("uninstall removed a reader still shared by another installation: %v", err)
		}
	}
}
