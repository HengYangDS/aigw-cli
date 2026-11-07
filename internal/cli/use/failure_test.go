package use

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
)

func TestUseSurfacesTokenStoreAndOutputFailures(t *testing.T) {
	runtime, _, _ := configuredRuntime(t)
	runtime.Secrets = failingSecretStore{Store: secret.NewMemoryStore(), setErr: errors.New("store failed")}
	runtime.Interactive = true
	runtime.Prompt = &promptStub{secret: "token"}
	runtime.HTTP = doerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})
	command := NewCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{codex.ID, "gateway/gpt-test"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "store failed") {
		t.Fatalf("store error = %v", err)
	}

	runtime, _, _ = configuredRuntime(t)
	store := secret.NewMemoryStore()
	if err := store.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runtime.Secrets = store
	runtime.Out = failingWriter{err: errors.New("write failed")}
	command = NewCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{codex.ID, "gateway/gpt-test"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("output error = %v", err)
	}
}

type failingSecretStore struct {
	secret.Store
	setErr    error
	deleteErr error
}

func (store failingSecretStore) Set(account, token string) error {
	if store.setErr != nil {
		return store.setErr
	}
	return store.Store.Set(account, token)
}

func (store failingSecretStore) Delete(account string) error {
	if store.deleteErr != nil {
		return store.deleteErr
	}
	return store.Store.Delete(account)
}

type failingWriter struct{ err error }

func (writer failingWriter) Write([]byte) (int, error) { return 0, writer.err }

// changedDuringDiscovery is a discovery that saves a newer configuration, as
// another command would while this one runs, so that the selection's commit
// fails after it stored its Token.
func changedDuringDiscovery(t *testing.T, run invocation.Context, cfg domain.Config, also func()) staticDiscovery {
	t.Helper()
	newer := cfg.Clone()
	provider := newer.Providers["gateway"]
	provider.Label = "Gateway (renamed elsewhere)"
	newer.Providers["gateway"] = provider
	return staticDiscovery{onDiscover: func() {
		if also != nil {
			also()
		}
		if err := run.Config.Save(newer); err != nil {
			t.Fatal(err)
		}
	}}
}

func TestUsePreservesCredentialsWhenCompensationCannotComplete(t *testing.T) {
	deletionError := errors.New("credential store refused deletion")
	for _, test := range []struct {
		name      string
		newer     string
		deleteErr error
		want      string
		retained  string
	}{
		{name: "deletion failure", deleteErr: deletionError, want: deletionError.Error(), retained: "new-token"},
		{name: "newer credential", newer: "newer-token", want: "credential postimage changed", retained: "newer-token"},
	} {
		t.Run(test.name, func(t *testing.T) {
			run, cfg, store, _ := tokenAcquisitionRuntime(t)
			run.Secrets = failingSecretStore{Store: store, deleteErr: test.deleteErr}
			run.Discovery = changedDuringDiscovery(t, run, cfg, func() {
				if test.newer != "" {
					if err := store.Set("gateway", test.newer); err != nil {
						t.Fatal(err)
					}
				}
			})
			// The Token is missing when the selection starts.
			if err := store.Delete("gateway"); err != nil && !errors.Is(err, secret.ErrNotFound) {
				t.Fatal(err)
			}
			err := execute(run, codex.ID, "gateway/gpt-test")
			for _, want := range []string{"preimage changed", "credential rollback also failed", test.want} {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("selection error = %v, want %q", err, want)
				}
			}
			if test.deleteErr != nil && !errors.Is(err, test.deleteErr) {
				t.Fatalf("selection error lost credential store error: %v", err)
			}
			// The failed undo is stated with its cause.
			if impact := terminal.ProblemFor(err).Impact; !strings.Contains(impact, "Removing the Token that this command stored for provider gateway failed") {
				t.Fatalf("failure impact = %q", impact)
			}
			if token, err := store.Get("gateway"); err != nil || token != test.retained {
				t.Fatalf("credential after failed compensation = %q, %v", token, err)
			}
		})
	}
}

func TestUseRollsBackAutomaticBackendSelection(t *testing.T) {
	run, _, _, _ := tokenAcquisitionRuntime(t)
	root := filepath.Join(t.TempDir(), "secrets")
	store, err := secret.Select(secret.Selection{
		GOOS: runtime.GOOS, Root: root,
		KeyringProbe: func(secret.Store) error { return errors.New("isolated file backend") },
	})
	if err != nil {
		t.Fatal(err)
	}
	run.Secrets = store
	cfg, err := run.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	run.Discovery = changedDuringDiscovery(t, run, cfg, nil)
	command := NewCommand(run)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{codex.ID, "gateway/gpt-test"})
	if err := command.ExecuteContext(t.Context()); err == nil || !strings.Contains(err.Error(), "preimage changed") {
		t.Fatalf("selection error = %v", err)
	}
	if secretExists(t, store, "gateway") {
		t.Fatal("failed selection retained its acquired token")
	}
	if _, err := os.Stat(filepath.Join(root, "backend")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed selection retained automatic backend choice: %v", err)
	}
}

func TestUseKeepsCommittedTokenWhenRenderingFails(t *testing.T) {
	run, _, store, _ := tokenAcquisitionRuntime(t)
	outputError := errors.New("output closed")
	run.Out = failingWriter{err: outputError}
	command := NewCommand(run)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{codex.ID, "gateway/gpt-test"})
	if err := command.ExecuteContext(t.Context()); !errors.Is(err, outputError) {
		t.Fatalf("output error = %v", err)
	}
	if token, err := store.Get("gateway"); err != nil || token != "new-token" {
		t.Fatalf("committed token after output failure = %q, %v", token, err)
	}
}
