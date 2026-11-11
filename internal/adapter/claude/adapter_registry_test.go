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
		before.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
		before.Models[domain.NewModelRef("gateway", "claude-test")] = qualifiedModel("Claude", "gateway", "claude-test", domain.ProtocolAnthropic)
		before.SetSelectedModel(ID, domain.NewModelRef("gateway", "claude-test"), "")
		before.SetClientActivation(ID, true, "/opt/claude", nil)
		after := before.Clone()
		delete(before.Models, domain.NewModelRef("gateway", "claude-test"))
		delete(after.Models, domain.NewModelRef("gateway", "claude-test"))
		if changed := claudeRegistry().ChangedClients(before, after); len(changed) != 1 || changed[0] != ID {
			t.Fatal("invalid Claude runtime was treated as unchanged")
		}
	})
}

func TestClaudeConvergesConfiguredExecutablesConservatively(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["gateway"] = domain.Provider{Endpoints: domain.Endpoints{
		Anthropic: "https://gateway.test",
		Responses: "https://gateway.test/v1",
	}}
	cfg.Models[domain.NewModelRef("gateway", "claude-test")] = qualifiedModel("", "gateway", "claude-test", domain.ProtocolAnthropic)
	cfg.Models[domain.NewModelRef("gateway", "gpt-test")] = qualifiedModel("", "gateway", "gpt-test", domain.ProtocolResponses)
	cfg.SetSelectedModel(ID, domain.NewModelRef("gateway", "claude-test"), "")
	cfg.SetSelectedModel(otherClient.ID, domain.NewModelRef("gateway", "gpt-test"), "")
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

func TestClaudeConvergenceIgnoresOnlyAnUnselectedModel(t *testing.T) {
	for _, clientID := range []string{ID} {
		t.Run(clientID+" without a route", func(t *testing.T) {
			if _, err := claudeRegistry().Converge(adapter.Dependencies{}, domain.NewConfig(testAdmission()), discovery.Result{}, clientID); err != nil {
				t.Fatalf("Converge(%q) without a route = %v", clientID, err)
			}
		})

		t.Run(clientID+" with a broken route", func(t *testing.T) {
			cfg := domain.NewConfig(testAdmission())
			cfg.SetSelectedModel(clientID, domain.NewModelRef("gateway", "missing"), "")
			if _, err := claudeRegistry().Converge(adapter.Dependencies{}, cfg, discovery.Result{}, clientID); err == nil || !strings.Contains(err.Error(), `unknown model gateway/missing`) {
				t.Fatalf("Converge(%q) broken route error = %v", clientID, err)
			}
		})
	}
}
