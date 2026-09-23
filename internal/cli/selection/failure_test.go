package selection

import (
	"errors"
	"net/http"
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
