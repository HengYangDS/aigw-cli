package use

import (
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// A disabled client that selects a model stays disabled when the selection
// moves, which is what the fix of a refused edit or removal runs; only
// `aigw client enable` enables it. A client without a selection is enabled by
// its first one.
func TestMovingTheSelectionOfADisabledClientKeepsItDisabled(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	credentials := secret.NewMemoryStore()
	if err := credentials.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runtime.Secrets = credentials
	cfg.Models[domain.NewModelRef("gateway", "gpt-next")] = domain.Model{Label: "Next", Provider: "gateway", Family: "gpt-next", ID: "gpt-next", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}}
	cfg.SetClientActivation(codex.ID, false, "", nil)
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--dry-run"}, {}} {
		result := useDocument(t, runtime, append([]string{codex.ID, "gateway/gpt-next"}, args...)...)
		if result.Enabled || result.WasDisabled || !result.StaysDisabled {
			t.Fatalf("a selection of a disabled client %v = %#v", args, result)
		}
	}
	stored, err := runtime.Config.Load()
	if err != nil || stored.Clients[codex.ID].Enabled || stored.SelectedModel(codex.ID).String() != "gateway/gpt-next" {
		t.Fatalf("the client is not disabled on its new model: %#v, %v", stored.Clients[codex.ID], err)
	}
	out.Reset()
	if err := execute(runtime, codex.ID, "gateway/gpt-test"); err != nil {
		t.Fatal(err)
	}
	if text := out.String(); !strings.Contains(text, "Stays disabled; `aigw client enable codex` enables it") || strings.Contains(text, "Now enabled") {
		t.Fatalf("human result does not say that Codex stays disabled:\n%s", text)
	}
	// Without a selection, a binding that was disabled is enabled by one.
	stored, err = runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	stored.Clients[claude.ID] = domain.ClientBinding{}
	if err := runtime.Config.Save(stored); err != nil {
		t.Fatal(err)
	}
	cfg.Models[domain.NewModelRef("gateway", "claude-next")] = domain.Model{Label: "Claude", Provider: "gateway", Family: "claude-next", ID: "claude-next", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	stored.Models = cfg.Models
	if err := runtime.Config.Save(stored); err != nil {
		t.Fatal(err)
	}
	if first := useDocument(t, runtime, claude.ID, "gateway/claude-next"); !first.Enabled || !first.WasDisabled || first.StaysDisabled {
		t.Fatalf("a first selection of a disabled binding = %#v", first)
	}
}
