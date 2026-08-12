package account

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestAddRollsBackTokenWhenConfigurationCannotBeSaved(t *testing.T) {
	store, secretStore := blockedConfigurationStore(t)
	command := NewAddCommand(invocation.Context{
		Config:  store,
		Secrets: secretStore,
		In:      strings.NewReader("new-token\n"),
		Out:     &bytes.Buffer{},
	})
	command.SetArgs([]string{"new", "--openai-url", "https://new.test/v1", "--token-stdin"})
	command.SilenceErrors = true
	command.SilenceUsage = true

	if err := command.Execute(); err == nil {
		t.Fatal("configuration save failure was accepted")
	}
	if secretStore.Has("new") {
		t.Fatal("token remained after configuration save failure")
	}
}

func TestEditReturnsTransactionPreparationFailure(t *testing.T) {
	store, secretStore := blockedConfigurationStore(t)
	command := newEditCommand(invocation.Context{Config: store, Secrets: secretStore, Out: &bytes.Buffer{}})
	command.SetArgs([]string{"current", "--label", "Renamed"})
	command.SilenceErrors = true
	command.SilenceUsage = true

	if err := command.Execute(); err == nil {
		t.Fatal("transaction preparation failure was accepted")
	}
}

func blockedConfigurationStore(t *testing.T) (configuration.Store, *secrets.MemoryStore) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "configuration.toml")
	store := configuration.NewStore(path)
	cfg := configuration.NewConfig()
	cfg.Accounts["current"] = configuration.Account{
		Label:     "Current",
		Endpoints: configuration.Endpoints{OpenAIResponses: "https://current.test/v1"},
	}
	cfg.Profiles["current"] = configuration.Profile{Label: "Current", Account: "current"}
	cfg.Routes.Default = "current"
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	backup := path + ".bak"
	if err := os.Mkdir(backup, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "blocker"), []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	return store, secrets.NewMemoryStore()
}
