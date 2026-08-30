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
	"github.com/spf13/cobra"
)

func TestAccountReferenceRequiresAChoiceOnlyWhenAmbiguous(t *testing.T) {
	tests := []struct {
		name     string
		accounts map[string]configuration.Account
		args     []string
		want     string
		wantErr  string
	}{
		{name: "explicit", accounts: map[string]configuration.Account{"one": {}, "two": {}}, args: []string{"two"}, want: "two"},
		{name: "only account", accounts: map[string]configuration.Account{"one": {}}, want: "one"},
		{name: "none", accounts: map[string]configuration.Account{}, wantErr: "0 accounts"},
		{name: "ambiguous", accounts: map[string]configuration.Account{"one": {}, "two": {}}, wantErr: "2 accounts"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := configuration.NewConfig()
			cfg.Accounts = test.accounts
			got, err := accountReference(cfg, test.args)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("accountReference() error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("accountReference() = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestAccountCommandsRequireAnAccountBeforeProviderWork(t *testing.T) {
	runtime := invocation.Context{
		Config:      configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml")),
		Interactive: true,
		Out:         &bytes.Buffer{},
		RenderOut:   &bytes.Buffer{},
	}

	accountCommand := NewCommand(runtime, &cobra.Command{Use: "rename"})
	accountCommand.SetArgs([]string{"connect"})
	accountCommand.SilenceErrors = true
	accountCommand.SilenceUsage = true
	if err := accountCommand.Execute(); err == nil || !strings.Contains(err.Error(), "0 accounts") {
		t.Fatalf("account connect error = %v", err)
	}

	balanceCommand := NewBalanceCommand(runtime)
	balanceCommand.SilenceErrors = true
	balanceCommand.SilenceUsage = true
	if err := balanceCommand.Execute(); err == nil || !strings.Contains(err.Error(), "0 accounts") {
		t.Fatalf("balance error = %v", err)
	}
}

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
	cfg.Profiles["current"] = configuration.Profile{Label: "Current", Account: "current", Client: configuration.ClientCodex, Model: "gpt-current"}
	cfg.Routes[configuration.ClientCodex] = "current"
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
