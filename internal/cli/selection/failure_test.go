package selection

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestUseSurfacesTokenStoreAndOutputFailures(t *testing.T) {
	runtime, _, _ := configuredRuntime(t)
	runtime.Secrets = failingSecretStore{Store: secrets.NewMemoryStore(), setErr: errors.New("store failed")}
	runtime.Interactive = true
	runtime.Prompt = &promptStub{secret: "token"}
	runtime.HTTP = doerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})
	command := NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", configuration.ClientCodex, "codex"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "store failed") {
		t.Fatalf("store error = %v", err)
	}

	runtime, _, _ = configuredRuntime(t)
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runtime.Secrets = store
	runtime.RenderOut = failingWriter{err: errors.New("write failed")}
	command = NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", configuration.ClientCodex, "codex"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("output error = %v", err)
	}
}

type failingSecretStore struct {
	secrets.Store
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
			run.Discovery = staticDiscovery{onDiscover: func() {
				if test.newer != "" {
					if err := store.Set("gateway", test.newer); err != nil {
						t.Fatal(err)
					}
				}
			}}
			cfg.Routes["next"] = configuration.Route{
				Label: "Next", Account: "gateway", Model: "gpt-next",
				Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
			}
			cfg.SetClientActivation(configuration.ClientCodex, true, "", []string{filepath.Join(t.TempDir(), "missing.toml")})
			if err := run.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			command := NewUseCommand(run)
			command.SilenceErrors = true
			command.SilenceUsage = true
			command.SetArgs([]string{"--for", configuration.ClientCodex, "next"})
			err := command.ExecuteContext(t.Context())
			for _, want := range []string{"synchronization preflight failed", "credential rollback also failed", test.want} {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("selection error = %v, want %q", err, want)
				}
			}
			if test.deleteErr != nil && !errors.Is(err, test.deleteErr) {
				t.Fatalf("selection error lost credential store error: %v", err)
			}
			if token, err := store.Get("gateway"); err != nil || token != test.retained {
				t.Fatalf("credential after failed compensation = %q, %v", token, err)
			}
			current, err := run.Config.Load()
			if err != nil || current.SelectedRoute(configuration.ClientCodex) != "codex" {
				t.Fatalf("failed selection changed binding: %#v, %v", current.Clients, err)
			}
		})
	}
}

func TestUseRollsBackAutomaticBackendSelection(t *testing.T) {
	run, _, _, _ := tokenAcquisitionRuntime(t)
	root := filepath.Join(t.TempDir(), "secrets")
	store, err := secrets.Select(secrets.Selection{
		GOOS: runtime.GOOS, Root: root,
		KeyringProbe: func(secrets.Store) error { return errors.New("isolated file backend") },
	})
	if err != nil {
		t.Fatal(err)
	}
	run.Secrets = store
	run.Discovery = nil
	command := NewUseCommand(run)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", configuration.ClientCodex, "codex"})
	if err := command.ExecuteContext(t.Context()); err == nil || !strings.Contains(err.Error(), "client discovery is unavailable") {
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
	run.RenderOut = failingWriter{err: outputError}
	command := NewUseCommand(run)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", configuration.ClientCodex, "codex"})
	if err := command.ExecuteContext(t.Context()); !errors.Is(err, outputError) {
		t.Fatalf("output error = %v", err)
	}
	if token, err := store.Get("gateway"); err != nil || token != "new-token" {
		t.Fatalf("committed token after output failure = %q, %v", token, err)
	}
}
