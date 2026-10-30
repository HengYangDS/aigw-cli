package uninstall

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

// hostAt observes home as the user's home directory, so Claude Code's settings
// file is home/.claude/settings.json.
func hostAt(home string) discovery.Host {
	return discovery.Host{GOOS: runtime.GOOS, Home: home, Env: map[string]string{"HOME": home, "USERPROFILE": home}}
}

func TestUninstallCommandDefaultsToRunningExecutableAndReportsFailure(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "aigw")
	if err := os.WriteFile(target, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := new(bytes.Buffer)
	command := NewCommand(invocation.Context{Clients: clients.Registry(), Executable: target, Out: out})
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
	command = NewCommand(invocation.Context{Clients: clients.Registry(), Executable: ""})
	command.SetArgs(nil)
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "target is empty") {
		t.Fatalf("empty target error = %v", err)
	}
}

func TestUninstallCommandHandlesConfigurationAndWithdrawalFailures(t *testing.T) {
	t.Run("configuration load", func(t *testing.T) {
		root := t.TempDir()
		command := NewCommand(invocation.Context{Clients: clients.Registry(), Executable: filepath.Join(root, "aigw"), Config: store.NewStore(root, clients.Admission())})
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
		command := NewCommand(invocation.Context{Clients: clients.Registry(),
			Executable: target,
			Config:     store.NewStore(filepath.Join(root, "invalid")+"\x00configuration.toml", clients.Admission()),
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
		configStore := store.NewStore(filepath.Join(root, "configuration.toml"), clients.Admission())
		cfg := domain.NewConfig(clients.Admission())
		cfg.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
		cfg.Routes["claude"] = domain.Route{
			Label: "Claude", Account: "gateway", Model: "claude-test",
			Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
		}
		cfg.SetSelectedRoute(claude.ID, "claude", "")
		cfg.SetClientActivation(claude.ID, true, "/opt/claude", nil)
		if err := configStore.Save(cfg); err != nil {
			t.Fatal(err)
		}
		blockedHome := filepath.Join(root, "blocked")
		if err := os.MkdirAll(filepath.Join(blockedHome, ".claude", "settings.json"), 0o700); err != nil {
			t.Fatal(err)
		}
		command := NewCommand(invocation.Context{Clients: clients.Registry(),
			Executable: target,
			Config:     configStore,
			Host:       hostAt(blockedHome),
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
	predecessorReader, err := secret.VersionedEntrypointPath(dataDir, target, "aigw")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := secret.EnsureEntrypoint(target, predecessorReader); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("installed"), 0o700); err != nil {
		t.Fatal(err)
	}
	invokerReader, err := secret.VersionedEntrypointPath(dataDir, invoker, "aigw")
	if err != nil {
		t.Fatal(err)
	}
	targetReader, err := secret.VersionedEntrypointPath(dataDir, target, "aigw")
	if err != nil {
		t.Fatal(err)
	}
	for source, reader := range map[string]string{invoker: invokerReader, target: targetReader} {
		if _, err := secret.EnsureEntrypoint(source, reader); err != nil {
			t.Fatal(err)
		}
	}
	command := NewCommand(invocation.Context{Clients: clients.Registry(),
		Executable: invoker, CredentialPath: invokerReader, Config: store.NewStore(filepath.Join(root, "config.toml"), clients.Admission()),
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
	invokerReader, err := secret.VersionedEntrypointPath(dataDir, invoker, "aigw")
	if err != nil {
		t.Fatal(err)
	}
	targetReader, err := secret.VersionedEntrypointPath(dataDir, target, "aigw")
	if err != nil || targetReader != invokerReader {
		t.Fatalf("identical installations did not share one reader: %v", err)
	}
	if _, err := secret.EnsureEntrypoint(invoker, invokerReader); err != nil {
		t.Fatal(err)
	}
	command := NewCommand(invocation.Context{Clients: clients.Registry(),
		Executable: invoker, CredentialPath: invokerReader,
		Config: store.NewStore(filepath.Join(root, "config.toml"), clients.Admission()),
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
