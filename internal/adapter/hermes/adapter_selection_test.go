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
			// Following comes first: the transition starts from the configuration
			// that already holds the choice, as the synchronization's working
			// configuration does.
			projectHermes(t, deps, after, after)
			if !(Selection{Provider: windowEntry(), Model: "wire-z"}).selects(projectedFile(t, target)) {
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

	choice := hermesChoiceOf(cfg, Selection{Provider: "aigw-gateway-anthropic", Model: "claude-opus-5-5"})
	if !choice.listed || choice.ref != domain.NewModelRef("gateway", "claude-opus-5-5") || choice.protocol != domain.ProtocolAnthropic {
		t.Fatalf("followed choice = %+v", choice)
	}
	recorded := adapter.RecordedSelection{Client: ID, Product: "Hermes", Model: domain.NewModelRef("gateway", "claude-opus-5-5"), Protocol: "anthropic"}
	for _, test := range []struct {
		name      string
		selection Selection
		want      []string
	}{
		// The model must belong to the selected provider's protocol, not any protocol.
		{"a model of another protocol", Selection{Provider: "aigw-gateway-anthropic", Model: "gpt-6.1-sol"}, []string{
			`Hermes selects model "gpt-6.1-sol", which provider "gateway" has not added on Anthropic Messages`,
			"`aigw use hermes gateway/gpt-6.1-sol --protocol anthropic` adds and selects the model that Hermes names",
		}},
		{"a model that no one added", Selection{Provider: "aigw-gateway-responses", Model: "gpt-unadded"}, []string{
			`Hermes selects model "gpt-unadded", which provider "gateway" has not added on OpenAI Responses`,
			"`aigw use hermes gateway/gpt-unadded --protocol responses` adds and selects the model that Hermes names",
		}},
		{"a provider entry that AIGW does not write", Selection{Provider: "aigw-other-anthropic", Model: "claude-opus-5-5"}, []string{
			`Hermes selects provider "aigw-other-anthropic", which AIGW does not write`,
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			choice := hermesChoiceOf(cfg, test.selection)
			if choice.listed {
				t.Fatalf("followed %+v", choice)
			}
			refusal := choice.refusal(recorded, test.selection)
			if refusal.NextAction() != "aigw use hermes gateway/claude-opus-5-5 --protocol anthropic" {
				t.Fatalf("next action = %q, want the restoring command", refusal.NextAction())
			}
			for _, want := range test.want {
				if !strings.Contains(refusal.Error(), want) {
					t.Fatalf("refusal = %v, want %s", refusal, want)
				}
			}
			if len(test.want) == 1 && strings.Contains(refusal.Error(), "adds and selects") {
				t.Fatalf("refusal names an adding command for an entry that AIGW does not write: %v", refusal)
			}
		})
	}
}
