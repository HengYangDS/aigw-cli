package claude

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/client"
	"aigw-cli/internal/client/contract"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestClaudeAdapterKeepsTheSeamContract(t *testing.T) {
	contract.Run(t, func(t *testing.T) contract.Fixture {
		home := t.TempDir()
		executable := filepath.Join(home, "bin", "claude")
		if err := os.MkdirAll(filepath.Dir(executable), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
			t.Fatal(err)
		}
		before := configuration.NewConfig(testAdmission())
		before.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
		before.Routes["claude"] = qualifiedRoute("", "gateway", "claude-test", configuration.ProtocolAnthropic)
		before.SetSelectedRoute(configuration.ClientClaude, "claude", "")
		after := before.Clone()
		after.SetClientActivation(configuration.ClientClaude, true, executable, nil)
		runtime, err := after.ResolveRuntime(configuration.ClientClaude, "")
		if err != nil {
			t.Fatal(err)
		}
		store := secrets.NewMemoryStore()
		if err := store.Set("gateway", "token"); err != nil {
			t.Fatal(err)
		}
		return contract.Fixture{
			Adapter: New(), Home: home, Before: before, After: after, Runtime: runtime,
			Dependencies: client.Dependencies{
				Secrets: store, Runner: &captureAdapterRunner{err: errors.New("contract fixtures run no client")},
				Host: hostAt(home), AIGWExecutable: filepath.Join(home, "bin", "aigw"),
			},
		}
	})
}
