package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"
)

// verifiedRecoveryError reports a configuration outcome as a failed commit does.
type verifiedRecoveryError struct{ restored bool }

func (verifiedRecoveryError) Error() string                        { return "client file write failed" }
func (recovery verifiedRecoveryError) ConfigurationRestored() bool { return recovery.restored }

func TestRollbackFailureReportsVerifiedConfigurationRecovery(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{name: "restored", err: fmt.Errorf("rollback %w", verifiedRecoveryError{restored: true}), want: "Configuration was restored"},
		{name: "incomplete", err: verifiedRecoveryError{}, want: "Configuration restoration was incomplete"},
		{name: "unverified", err: errors.New("rollback synchronization preflight failed"), want: "may have changed; inspect them"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := rollbackFailureImpact(test.err); !strings.Contains(got, test.want) {
				t.Fatalf("rollback failure impact = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRollbackCoversLoadSnapshotAndBackupFailures(t *testing.T) {
	run := func(configStore store.Store, args ...string) error {
		command := NewCommand(invocation.Context{Clients: clients.Registry(), Config: configStore})
		command.SetArgs(append([]string{"rollback"}, args...))
		return executeManifestCommand(command)
	}
	t.Run("current config load", func(t *testing.T) {
		if err := run(store.NewStore(t.TempDir(), clients.Admission())); err == nil {
			t.Fatal("current configuration load failure was accepted")
		}
	})
	t.Run("invalid verified snapshot", func(t *testing.T) {
		configStore := rollbackStore(t)
		if err := os.WriteFile(configStore.Path()+".verified.json", []byte("not-json"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := run(configStore); err == nil || !strings.Contains(errors.Unwrap(err).Error(), "parse verified checkpoint") {
			t.Fatalf("rollback error = %v", err)
		}
	})
	t.Run("invalid previous backup", func(t *testing.T) {
		configStore := rollbackStore(t)
		if err := os.WriteFile(configStore.Path()+".bak", []byte("not = [toml"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := run(configStore, "--last-change"); err == nil || !strings.Contains(errors.Unwrap(err).Error(), "parse config") {
			t.Fatalf("rollback error = %v", err)
		}
	})
}

func rollbackStore(t *testing.T) store.Store {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["one"] = domain.Provider{Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://one.test"}}
	cfg.Models["one"] = domain.Model{
		Label: "One", Provider: "one", Family: "claude-test",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	cfg.SetSelectedModel(claude.ID, "one", "")
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return configStore
}

// rollbackRuntime is a home whose configuration's last change renamed the
// provider's label: its backup still holds the old one.
func rollbackRuntime(t *testing.T) (invocation.Context, *bytes.Buffer) {
	t.Helper()
	runtime, _, out := savedRuntime(t, localConfig())
	changed := localConfig()
	provider := changed.Providers["local"]
	provider.Label = "Changed"
	changed.Providers["local"] = provider
	if err := runtime.Config.Save(changed); err != nil {
		t.Fatal(err)
	}
	return runtime, out
}

func runRollback(runtime invocation.Context, args ...string) error {
	command := NewCommand(runtime)
	command.SetArgs(append([]string{"rollback"}, args...))
	return executeManifestCommand(command)
}

// TestRollbackRestoresTheLastChange restores the backup, in JSON and in
// text, after previews in both that name it and write nothing.
func TestRollbackRestoresTheLastChange(t *testing.T) {
	for _, jsonMode := range []bool{true, false} {
		runtime, out := rollbackRuntime(t)
		before, err := os.ReadFile(runtime.Config.Path())
		if err != nil {
			t.Fatal(err)
		}
		preview := []string{"--last-change", "--dry-run"}
		if jsonMode {
			preview = append(preview, "--json")
		}
		if err := runRollback(runtime, preview...); err != nil {
			t.Fatalf("preview: %v\n%s", err, out)
		}
		if jsonMode && !strings.Contains(out.String(), `"source": "previous_backup"`) || !jsonMode && !strings.Contains(out.String(), "Rollback preview") {
			t.Fatalf("preview = %s", out)
		}
		if after, err := os.ReadFile(runtime.Config.Path()); err != nil || !bytes.Equal(after, before) {
			t.Fatalf("a preview changed the configuration: %v", err)
		}
		out.Reset()
		args := []string{"--last-change"}
		if jsonMode {
			args = append(args, "--json")
		}
		if err := runRollback(runtime, args...); err != nil {
			t.Fatalf("rollback: %v\n%s", err, out)
		}
		want := "Rolled back safely"
		if jsonMode {
			want = `"next_action": "aigw check --depth local"`
		}
		cfg, err := runtime.Config.Load()
		if err != nil || cfg.Providers["local"].Label != "Local" || cfg.SelectedModel(codex.ID) != "local" || !strings.Contains(out.String(), want) {
			t.Fatalf("restored label = %q, %v\n%s", cfg.Providers["local"].Label, err, out)
		}
	}
}

// TestRollbackPrefersTheVerifiedSnapshot restores the latest fully verified
// configuration, which a check to the client depth wrote, unless
// --last-change asks for the backup.
func TestRollbackPrefersTheVerifiedSnapshot(t *testing.T) {
	runtime, _ := rollbackRuntime(t)
	current, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Config.SaveVerifiedCheckpoint(t.Context(), current, []store.VerifiedClient{{Client: codex.ID, Depth: "client"}}); err != nil {
		t.Fatal(err)
	}
	for args, want := range map[string]string{"": "verified_checkpoint", "--last-change": "previous_backup"} {
		out := &bytes.Buffer{}
		runtime.Out = out
		command := []string{"--dry-run", "--json"}
		if args != "" {
			command = append(command, args)
		}
		if err := runRollback(runtime, command...); err != nil || !strings.Contains(out.String(), `"source": "`+want+`"`) {
			t.Fatalf("rollback %q = %v\n%s", args, err, out)
		}
	}
}

// TestRollbackWithoutARecoverySourceNamesWhatHelps refuses a home without a
// configuration, naming setup, and a configuration without a backup, naming
// the local check; neither changes anything.
func TestRollbackWithoutARecoverySourceNamesWhatHelps(t *testing.T) {
	empty := invocation.Context{Clients: clients.Registry(), Config: store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission()),
		Secrets: secret.NewMemoryStore(), Out: io.Discard}
	err := runRollback(empty)
	if problem := terminal.ProblemFor(err); problem.Title != "Configuration rollback is unavailable" || problem.Fix != "aigw setup" {
		t.Fatalf("an empty home = %+v", problem)
	}
	single, _, _ := savedRuntime(t, localConfig())
	err = runRollback(single, "--last-change")
	if problem := terminal.ProblemFor(err); problem.Title != "Configuration rollback is unavailable" || problem.Fix != "aigw check --depth local" {
		t.Fatalf("a configuration without a backup = %+v", problem)
	}
}

// TestRollbackThatCannotWriteReportsWhatChanged refuses a rollback whose
// write fails, with the conservative impact when nothing verified it.
func TestRollbackThatCannotWriteReportsWhatChanged(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a read-only folder does not refuse a write on Windows")
	}
	runtime, _ := rollbackRuntime(t)
	folder := filepath.Dir(runtime.Config.Path())
	if err := os.Chmod(folder, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(folder, 0o700) })
	err := runRollback(runtime, "--last-change")
	problem := terminal.ProblemFor(err)
	if problem.Title != "Configuration rollback did not complete" || problem.Fix != "aigw check --depth local" || problem.Impact == "" {
		t.Fatalf("a rollback that cannot write = %+v", problem)
	}
}
