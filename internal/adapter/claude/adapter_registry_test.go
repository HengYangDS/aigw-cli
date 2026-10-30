package claude

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
)

// claudeRegistry registers only this adapter, so it admits only Claude Code.
func claudeRegistry() adapter.Registry {
	registry, err := adapter.NewRegistry(New())
	if err != nil {
		panic(err)
	}
	return registry
}

func TestClaudeProjectionTreatsAnInvalidRuntimeAsChanged(t *testing.T) {
	t.Run("invalid enabled Claude runtime", func(t *testing.T) {
		before := domain.NewConfig(testAdmission())
		before.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
		before.Routes["claude"] = qualifiedRoute("Claude", "gateway", "claude-test", domain.ProtocolAnthropic)
		before.SetSelectedRoute(ID, "claude", "")
		before.SetClientActivation(ID, true, "/opt/claude", nil)
		after := before.Clone()
		delete(before.Routes, "claude")
		delete(after.Routes, "claude")
		if changed := claudeRegistry().ChangedClients(before, after); len(changed) != 1 || changed[0] != ID {
			t.Fatal("invalid Claude runtime was treated as unchanged")
		}
	})
}

func TestClaudeConvergesConfiguredExecutablesConservatively(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	cfg := domain.NewConfig(testAdmission())
	cfg.Accounts["gateway"] = domain.Account{Endpoints: domain.Endpoints{
		Anthropic:       "https://gateway.test",
		OpenAIResponses: "https://gateway.test/v1",
	}}
	cfg.Routes["claude"] = qualifiedRoute("", "gateway", "claude-test", domain.ProtocolAnthropic)
	cfg.Routes["codex"] = qualifiedRoute("", "gateway", "gpt-test", domain.ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(ID, "claude", "")
	cfg.SetSelectedRoute(otherClient.ID, "codex", "")
	cfg.SetClientActivation(ID, true, missing, nil)

	after, err := claudeRegistry().Converge(adapter.Dependencies{}, cfg, discovery.Result{}, ID)
	if err != nil || after.Clients[ID].Executable != missing {
		t.Fatalf("configured missing executable = %q, %v", after.Clients[ID].Executable, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	loop := filepath.Join(t.TempDir(), "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Skipf("symbolic link unavailable: %v", err)
	}
	cfg.SetClientActivation(ID, true, loop, nil)
	discovered := discovery.Result{Executables: map[string]string{ID: "/replacement/claude"}}
	for _, clientID := range []string{ID} {
		if _, err := claudeRegistry().Converge(adapter.Dependencies{}, cfg, discovered, clientID); err == nil || !strings.Contains(err.Error(), "inspect configured "+clientID+" executable") {
			t.Fatalf("Converge(%q) error = %v", clientID, err)
		}
	}
}

func TestClaudeConvergenceIgnoresOnlyAnUnselectedRoute(t *testing.T) {
	for _, clientID := range []string{ID} {
		t.Run(clientID+" without a route", func(t *testing.T) {
			if _, err := claudeRegistry().Converge(adapter.Dependencies{}, domain.NewConfig(testAdmission()), discovery.Result{}, clientID); err != nil {
				t.Fatalf("Converge(%q) without a route = %v", clientID, err)
			}
		})

		t.Run(clientID+" with a broken route", func(t *testing.T) {
			cfg := domain.NewConfig(testAdmission())
			cfg.SetSelectedRoute(clientID, "missing-route", "")
			if _, err := claudeRegistry().Converge(adapter.Dependencies{}, cfg, discovery.Result{}, clientID); err == nil || !strings.Contains(err.Error(), `unknown route "missing-route"`) {
				t.Fatalf("Converge(%q) broken route error = %v", clientID, err)
			}
		})
	}
}
