package codex

import (
	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdoptCodexSelectionRecordsTheOneMatchingModel(t *testing.T) {
	cfg, targets := codexAdoptionFixture(t, 1)
	native := chooseCodexModel(t, targets[0], "gpt-terra")
	routeID, err := (codexAdapter{}).AdoptSelection(adapter.Dependencies{}, &cfg)
	if err != nil || routeID != domain.NewModelRef("gateway", "gpt-terra") {
		t.Fatalf("adopted Route = %q, %v", routeID, err)
	}
	if runtime, err := cfg.ResolveRuntime(ID, domain.ModelRef{}); err != nil || runtime.Model != domain.NewModelRef("gateway", "gpt-terra") || runtime.Protocol != domain.ProtocolResponses {
		t.Fatalf("Codex binding = %#v, %v", cfg.Clients[ID], err)
	}
	if after, err := os.ReadFile(targets[0]); err != nil || string(after) != native {
		t.Fatalf("adoption wrote the Codex configuration: %v", err)
	}
}

// A model that Codex's provider has not added, on Codex's protocol, is not
// followed: the refusal's next action restores the recorded selection, and
// its message names the command that adds the model that Codex names.
func TestAdoptCodexSelectionRefusesAModelThatWasNotAdded(t *testing.T) {
	for _, test := range []struct {
		name, model string
		copy        bool
	}{
		{name: "none", model: "gpt-luna"},
		{name: "on another protocol", model: "gpt-chat", copy: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, targets := codexAdoptionFixture(t, 1)
			if test.copy {
				// The model exists, but not on the protocol that Codex uses.
				cfg.Models[domain.NewModelRef("gateway", "gpt-chat")] = qualifiedModel("Chat", "gateway", "gpt-chat", domain.ProtocolChat)
				cfg.Normalize()
			}
			chooseCodexModel(t, targets[0], test.model)
			_, err := (codexAdapter{}).AdoptSelection(adapter.Dependencies{}, &cfg)
			refusal, typed := errors.AsType[*adapter.SelectionError](err)
			if !typed || refusal.NextAction() != "aigw use codex gateway/gpt-sol" {
				t.Fatalf("adoption error = %v, want a refusal that restores gateway/gpt-sol", err)
			}
			for _, want := range []string{
				`Codex selects model "` + test.model + `", which provider "gateway" has not added`,
				"AIGW's selection for Codex is gateway/gpt-sol",
				"`aigw use codex gateway/" + test.model + "` adds and selects the model that Codex names",
			} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("adoption error = %v, want %s", err, want)
				}
			}
			if cfg.SelectedModel(ID) != domain.NewModelRef("gateway", "gpt-sol") {
				t.Fatal("a refused adoption changed the Codex binding")
			}
		})
	}
}

func TestAdoptCodexSelectionRefusesDisagreeingTargets(t *testing.T) {
	cfg, targets := codexAdoptionFixture(t, 2)
	chooseCodexModel(t, targets[0], "gpt-terra")
	chooseCodexModel(t, targets[1], "gpt-luna")
	_, err := (codexAdapter{}).AdoptSelection(adapter.Dependencies{}, &cfg)
	refusal, typed := errors.AsType[*adapter.SelectionError](err)
	if !typed || refusal.NextAction() != "aigw use codex gateway/gpt-sol" || !strings.Contains(err.Error(), `Codex has files that select different models, "gpt-terra" and "gpt-luna"`) {
		t.Fatalf("disagreeing targets were adopted: %v", err)
	}
	if cfg.SelectedModel(ID) != domain.NewModelRef("gateway", "gpt-sol") {
		t.Fatal("a refused adoption changed the Codex binding")
	}
}

func TestAdoptCodexSelectionLeavesUnchangedBindings(t *testing.T) {
	for name, prepare := range map[string]func(*domain.Config, string){
		"unchanged": func(*domain.Config, string) {},
		"disabled": func(cfg *domain.Config, target string) {
			chooseCodexModel(t, target, "gpt-terra")
			binding := cfg.Clients[ID]
			binding.Enabled = false
			cfg.Clients[ID] = binding
		},
		"provider drift": func(_ *domain.Config, target string) {
			current := chooseCodexModel(t, target, "gpt-terra")
			current = strings.Replace(current, `base_url = "https://gateway.test/v1"`, `base_url = "https://drift.test/v1"`, 1)
			if err := os.WriteFile(target, []byte(current), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			cfg, targets := codexAdoptionFixture(t, 1)
			prepare(&cfg, targets[0])
			before := cfg.Clients[ID]
			routeID, err := (codexAdapter{}).AdoptSelection(adapter.Dependencies{}, &cfg)
			if err != nil || !routeID.IsZero() || cfg.Clients[ID].Model != before.Model {
				t.Fatalf("adoption = %q, %v; binding %#v", routeID, err, cfg.Clients[ID])
			}
		})
	}
}

// codexAdoptionFixture projects Route sol into count Codex targets. The selected
// Account also offers terra, and another Account offers the same model.
func codexAdoptionFixture(t *testing.T, count int) (domain.Config, []string) {
	t.Helper()
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Responses: "https://gateway.test/v1"}}
	cfg.Providers["other"] = domain.Provider{Label: "Other", Endpoints: domain.Endpoints{Responses: "https://other.test/v1"}}
	cfg.Models[domain.NewModelRef("gateway", "gpt-sol")] = qualifiedModel("Sol", "gateway", "gpt-sol", domain.ProtocolResponses)
	cfg.Models[domain.NewModelRef("gateway", "gpt-terra")] = qualifiedModel("Terra", "gateway", "gpt-terra", domain.ProtocolResponses)
	cfg.Models[domain.NewModelRef("other", "gpt-terra")] = qualifiedModel("Other terra", "other", "gpt-terra", domain.ProtocolResponses)
	targets := make([]string, 0, count)
	for range count {
		target := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(target, []byte("model = \"gpt-native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		targets = append(targets, target)
	}
	cfg.SetSelectedModel(ID, domain.NewModelRef("gateway", "gpt-sol"), "")
	cfg.SetClientActivation(ID, true, "/opt/codex", targets)
	cfg.Normalize()
	runtime, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.CredentialCommand = filepath.Join(t.TempDir(), "aigw")
	for _, target := range targets {
		if _, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(target)}, runtime, runtime, false); err != nil {
			t.Fatal(err)
		}
	}
	return cfg, targets
}

// chooseCodexModel rewrites the root model the way Codex's picker does: the
// value changes and AIGW's ownership comment disappears.
func chooseCodexModel(t *testing.T, target, model string) string {
	t.Helper()
	projected, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	native := strings.Replace(string(projected), `model = "gpt-sol" # managed by AIGW`, `model = "`+model+`"`, 1)
	if native == string(projected) {
		t.Fatalf("projection has no managed model:\n%s", projected)
	}
	if err := os.WriteFile(target, []byte(native), 0o600); err != nil {
		t.Fatal(err)
	}
	return native
}
