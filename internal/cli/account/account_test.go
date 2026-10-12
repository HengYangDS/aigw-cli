package account

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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
	accountCommand.SetArgs([]string{"diagnostics", "enable"})
	accountCommand.SilenceErrors = true
	accountCommand.SilenceUsage = true
	if err := accountCommand.Execute(); err == nil || !strings.Contains(err.Error(), "0 accounts") {
		t.Fatalf("account diagnostics enable error = %v", err)
	}

	balanceCommand := NewBalanceCommand(runtime)
	balanceCommand.SilenceErrors = true
	balanceCommand.SilenceUsage = true
	if err := balanceCommand.Execute(); err == nil || !strings.Contains(err.Error(), "0 accounts") {
		t.Fatalf("balance error = %v", err)
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

func TestAccountInventoryOrdersEndpointMetadataWithoutChangingConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	store := configuration.NewStore(path)
	cfg := configuration.NewConfig()
	cfg.Accounts["zeta"] = configuration.Account{Label: "Second", Endpoints: configuration.Endpoints{OpenAIResponses: "https://zeta.test/v1"}}
	cfg.Accounts["alpha"] = configuration.Account{Label: "First", Endpoints: configuration.Endpoints{OpenAIResponses: "https://alpha.test/v1"}}
	cfg.Routes["alpha"] = configuration.Route{
		Label: "Alpha", Account: "alpha", Model: "gpt-alpha",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
	}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	runtime := invocation.Context{Config: store, Secrets: secrets.NewMemoryStore(), Out: out, RenderOut: out}
	command := newListCommand(runtime)
	command.SetArgs([]string{"--json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var listed accountListOutput
	if err := json.Unmarshal(out.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	want := []configuration.Account{
		{ID: "alpha", Label: "First", Endpoints: configuration.Endpoints{OpenAIResponses: "https://alpha.test/v1"}},
		{ID: "zeta", Label: "Second", Endpoints: configuration.Endpoints{OpenAIResponses: "https://zeta.test/v1"}},
	}
	if !reflect.DeepEqual(listed.Accounts, want) {
		t.Fatalf("account inventory = %+v, want %+v", listed.Accounts, want)
	}
	out.Reset()
	if err := newListCommand(runtime).Execute(); err != nil {
		t.Fatal(err)
	}
	if first, second := strings.Index(out.String(), "alpha"), strings.Index(out.String(), "zeta"); first < 0 || second <= first {
		t.Fatalf("human inventory lost stable account order: %s", out)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("account inventory changed configuration: %v", err)
	}
}

func blockedConfigurationStore(t *testing.T) (configuration.Store, secrets.Store) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "configuration.toml")
	store := configuration.NewStore(path)
	cfg := configuration.NewConfig()
	cfg.Accounts["current"] = configuration.Account{
		Label:     "Current",
		Endpoints: configuration.Endpoints{OpenAIResponses: "https://current.test/v1"},
	}
	cfg.Routes["current"] = configuration.Route{
		Label: "Current", Account: "current", Model: "gpt-current",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
	}
	cfg.SetSelectedRoute(configuration.ClientCodex, "current")
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
