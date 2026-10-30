package claude

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/contract"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
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
		before := domain.NewConfig(testAdmission())
		before.Accounts["gateway"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
		before.Routes["claude"] = qualifiedRoute("", "gateway", "claude-test", domain.ProtocolAnthropic)
		before.SetSelectedRoute(ID, "claude", "")
		after := before.Clone()
		after.SetClientActivation(ID, true, executable, nil)
		runtime, err := after.ResolveRuntime(ID, "")
		if err != nil {
			t.Fatal(err)
		}
		store := secret.NewMemoryStore()
		if err := store.Set("gateway", "token"); err != nil {
			t.Fatal(err)
		}
		return contract.Fixture{
			Adapter: New(), Home: home, Before: before, After: after, Runtime: runtime,
			Dependencies: adapter.Dependencies{
				Secrets: store, Runner: &captureAdapterRunner{err: errors.New("contract fixtures run no client")},
				Host: hostAt(home), AIGWExecutable: filepath.Join(home, "bin", "aigw"),
			},
		}
	})
}
