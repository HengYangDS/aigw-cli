package codex

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
)

// codexRegistry registers only this adapter, so it admits only Codex.
func codexRegistry() adapter.Registry {
	registry, err := adapter.NewRegistry(New())
	if err != nil {
		panic(err)
	}
	return registry
}

func TestCodexProjectionChangesWhenForwardedUpstreamIdentityChanges(t *testing.T) {
	before := domain.NewConfig(testAdmission())
	before.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://first.test/v1", Anthropic: "https://first.test"}}
	before.Routes["route"] = qualifiedRoute("Route", "gateway", "model", domain.ProtocolOpenAIResponses)
	before.Clients[ID] = domain.ClientBinding{Route: "route", Enabled: true}
	if err := before.SetForwardingEndpoint(ID, "http://127.0.0.1:8792/v1"); err != nil {
		t.Fatal(err)
	}
	after := before.Clone()
	account := after.Accounts["gateway"]
	account.Endpoints = domain.Endpoints{OpenAIResponses: "https://second.test/v1", Anthropic: "https://second.test"}
	after.Accounts["gateway"] = account
	if !(codexAdapter{}).ProjectionChanged(before, after) {
		t.Fatal("unchanged forwarding hid a changed credential projection identity")
	}
}

func TestCodexVerificationBoundsClientProcesses(t *testing.T) {
	cfg, runtime := codexVerificationFixture(t)
	runner := &captureAdapterRunner{err: errors.New("stop after deadline observation")}
	_, _ = (codexAdapter{}).Verify(context.Background(), adapter.Dependencies{Runner: runner}, cfg, runtime, "")
	if len(runner.deadlines) == 0 || !runner.deadlines[0] {
		t.Fatalf("Codex verification deadlines = %#v", runner.deadlines)
	}
}

func TestCodexProjectionTreatsInvalidRuntimeAndTargetChangesAsChanged(t *testing.T) {
	t.Run("invalid enabled Codex runtime", func(t *testing.T) {
		before := codexConfiguration("/target")
		after := before.Clone()
		delete(before.Routes, "codex")
		delete(after.Routes, "codex")
		if changed := codexRegistry().ChangedClients(before, after); len(changed) != 1 || changed[0] != ID {
			t.Fatal("invalid Codex runtime was treated as unchanged")
		}
	})

	t.Run("Codex targets changed", func(t *testing.T) {
		before := codexConfiguration("/first")
		after := before.Clone()
		after.SetClientActivation(ID, true, "/opt/codex", []string{"/second"})
		if changed := codexRegistry().ChangedClients(before, after); len(changed) != 1 || changed[0] != ID {
			t.Fatal("changed Codex target set was treated as unchanged")
		}
	})
}

func TestCodexConvergenceRefusesAnUninspectableConfiguredExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	cfg := codexConfiguration("/target")
	cfg.SetSelectedRoute(ID, "codex", "")
	loop := filepath.Join(t.TempDir(), "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Skipf("symbolic link unavailable: %v", err)
	}
	cfg.SetClientActivation(ID, true, loop, []string{"/target"})
	discovered := discovery.Result{Executables: map[string]string{ID: "/replacement/codex"}}
	if _, err := codexRegistry().Converge(adapter.Dependencies{}, cfg, discovered, ID); err == nil || !strings.Contains(err.Error(), "inspect configured codex executable") {
		t.Fatalf("Converge(codex) error = %v", err)
	}
}

func TestCodexConvergenceIgnoresOnlyAnUnselectedRoute(t *testing.T) {
	t.Run("without a route", func(t *testing.T) {
		if _, err := codexRegistry().Converge(adapter.Dependencies{}, domain.NewConfig(testAdmission()), discovery.Result{}, ID); err != nil {
			t.Fatalf("Converge(codex) without a route = %v", err)
		}
	})

	t.Run("with a broken route", func(t *testing.T) {
		cfg := domain.NewConfig(testAdmission())
		cfg.SetSelectedRoute(ID, "missing-route", "")
		if _, err := codexRegistry().Converge(adapter.Dependencies{}, cfg, discovery.Result{}, ID); err == nil || !strings.Contains(err.Error(), `unknown route "missing-route"`) {
			t.Fatalf("Converge(codex) broken route error = %v", err)
		}
	})
}
