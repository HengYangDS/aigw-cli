package hermes

import (
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
)

func TestAdoptHermesSelectionLeavesUnprojectedBindingsUnchanged(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Models[domain.NewModelRef("gateway", "claude-opus-5-5")] = qualifiedModel("Opus", "gateway", "claude-opus-5-5", domain.ProtocolAnthropic)
	cfg.Normalize()
	for name, prepare := range map[string]func(*domain.Config){
		"disabled": func(*domain.Config) {},
		"unselected": func(cfg *domain.Config) {
			cfg.SetClientActivation(ID, true, "/opt/hermes", []string{"/missing/config.yaml"})
		},
		"two homes": func(cfg *domain.Config) {
			cfg.SetSelectedModel(ID, domain.NewModelRef("gateway", "claude-opus-5-5"), "")
			cfg.SetClientActivation(ID, true, "/opt/hermes", []string{"/a/config.yaml", "/b/config.yaml"})
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := cfg.Clone()
			prepare(&candidate)
			before := candidate.Clone()
			routeID, err := (hermesAdapter{}).AdoptSelection(adapter.Dependencies{}, &candidate)
			if err != nil || !routeID.IsZero() || candidate.SelectedModel(ID) != before.SelectedModel(ID) {
				t.Fatalf("adoption = %q, %v; binding %#v", routeID, err, candidate.Clients[ID])
			}
		})
	}
}

// TestAdoptSelectionFollowsAModelChosenInHermes holds the scenario "A model
// chosen in Hermes among mapped models" over each shape in which a file lists
// an entry's models: the binding follows the choice, and the synchronization
// that follows leaves the choice in Hermes' file.
func TestAdoptSelectionFollowsAModelChosenInHermes(t *testing.T) {
	for _, shape := range modelShapes {
		t.Run(shape.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "config.yaml")
			before, deps := windowConfig(t, target), windowDependencies(t)
			projectHermes(t, deps, domain.NewConfig(testAdmission()), before)
			shape.rewrite(t, target)
			selectInHermes(t, target, windowEntry(), "wire-z")
			after, chosen := before.Clone(), domain.NewModelRef("gateway", "wire-z")
			if ref, err := (hermesAdapter{}).AdoptSelection(deps, &after); err != nil || ref != chosen || after.SelectedModel(ID) != chosen {
				t.Fatalf("adoption = %q, %v; binding %#v", ref, err, after.Clients[ID])
			}
			projectHermes(t, deps, before, after)
			if !(Desired{SelectedProvider: windowEntry(), SelectedModel: "wire-z"}).selects(projectedFile(t, target)) {
				t.Fatal("synchronization replaced the model chosen in Hermes")
			}
			if action := plannedAction(t, deps, after, after); action != "unchanged" {
				t.Fatalf("the adopted choice plans %s on the next synchronization", action)
			}
		})
	}
}

func TestHermesModelForAdoptsOnlyOneExactModel(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{
		Anthropic: "https://gateway.test", Responses: "https://gateway.test/v1",
	}}
	cfg.Models[domain.NewModelRef("gateway", "claude-opus-5-5")] = qualifiedModel("Opus", "gateway", "claude-opus-5-5", domain.ProtocolAnthropic)
	cfg.Models[domain.NewModelRef("gateway", "gpt-6.1-sol")] = qualifiedModel("Sol", "gateway", "gpt-6.1-sol", domain.ProtocolResponses)
	cfg.Normalize()

	routeID, protocol, err := hermesModelFor(cfg, Selection{Provider: "aigw-gateway-anthropic", Model: "claude-opus-5-5"})
	if err != nil || routeID != domain.NewModelRef("gateway", "claude-opus-5-5") || protocol != domain.ProtocolAnthropic {
		t.Fatalf("adopted Route = %q %q, %v", routeID, protocol, err)
	}
	// The model must belong to the selected provider's protocol, not any protocol.
	if _, _, err := hermesModelFor(cfg, Selection{Provider: "aigw-gateway-anthropic", Model: "gpt-6.1-sol"}); err == nil {
		t.Fatal("a model of another protocol was adopted")
	}
	if _, _, err := hermesModelFor(cfg, Selection{Provider: "aigw-other-anthropic", Model: "claude-opus-5-5"}); err == nil || !strings.Contains(err.Error(), "aigw use hermes <provider/model>") {
		t.Fatalf("an unprojected provider was not refused with a selection command: %v", err)
	}

	// A choice that no added model carries names the selection with a
	// placeholder for the model.
	_, _, err = hermesModelFor(cfg, Selection{Provider: "aigw-gateway-responses", Model: "gpt-unadded"})
	if err == nil || !strings.Contains(err.Error(), "which provider \"gateway\" has not added; run `aigw use hermes gateway/gpt-unadded --protocol responses`") {
		t.Fatalf("an unadded choice = %v", err)
	}
}
