package hermes

import (
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
)

func TestAdoptHermesSelectionLeavesUnprojectedBindingsUnchanged(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Models["opus"] = qualifiedModel("Opus", "gateway", "claude-opus-5-5", domain.ProtocolAnthropic)
	cfg.Normalize()
	for name, prepare := range map[string]func(*domain.Config){
		"disabled": func(*domain.Config) {},
		"unselected": func(cfg *domain.Config) {
			cfg.SetClientActivation(ID, true, "/opt/hermes", []string{"/missing/config.yaml"})
		},
		"two homes": func(cfg *domain.Config) {
			cfg.SetSelectedModel(ID, "opus", "")
			cfg.SetClientActivation(ID, true, "/opt/hermes", []string{"/a/config.yaml", "/b/config.yaml"})
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := cfg.Clone()
			prepare(&candidate)
			before := candidate.Clone()
			routeID, err := (hermesAdapter{}).AdoptSelection(adapter.Dependencies{}, &candidate)
			if err != nil || routeID != "" || candidate.SelectedModel(ID) != before.SelectedModel(ID) {
				t.Fatalf("adoption = %q, %v; binding %#v", routeID, err, candidate.Clients[ID])
			}
		})
	}
}

func TestHermesModelForAdoptsOnlyOneExactModel(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{
		Anthropic: "https://gateway.test", OpenAIResponses: "https://gateway.test/v1",
	}}
	cfg.Models["opus"] = qualifiedModel("Opus", "gateway", "claude-opus-5-5", domain.ProtocolAnthropic)
	cfg.Models["sol"] = qualifiedModel("Sol", "gateway", "gpt-6.1-sol", domain.ProtocolOpenAIResponses)
	cfg.Normalize()

	routeID, protocol, err := hermesModelFor(cfg, Selection{Provider: "aigw-gateway-anthropic", Model: "claude-opus-5-5"})
	if err != nil || routeID != "opus" || protocol != domain.ProtocolAnthropic {
		t.Fatalf("adopted Route = %q %q, %v", routeID, protocol, err)
	}
	// The model must belong to the selected provider's protocol, not any protocol.
	if _, _, err := hermesModelFor(cfg, Selection{Provider: "aigw-gateway-anthropic", Model: "gpt-6.1-sol"}); err == nil {
		t.Fatal("a model of another protocol was adopted")
	}
	if _, _, err := hermesModelFor(cfg, Selection{Provider: "aigw-other-anthropic", Model: "claude-opus-5-5"}); err == nil || !strings.Contains(err.Error(), "aigw use hermes <provider/model>") {
		t.Fatalf("an unprojected provider was not refused with a selection command: %v", err)
	}

	cfg.Models["opus-copy"] = qualifiedModel("Opus copy", "gateway", "claude-opus-5-5", domain.ProtocolAnthropic)
	cfg.Normalize()
	_, _, err = hermesModelFor(cfg, Selection{Provider: "aigw-gateway-anthropic", Model: "claude-opus-5-5"})
	if err == nil {
		t.Fatal("an ambiguous native choice was adopted")
	}
	for _, want := range []string{"which 2 models of provider \"gateway\" carry", "`aigw config path`"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ambiguity refusal %q does not name %s", err, want)
		}
	}
	// A choice that no added model carries names the selection with a
	// placeholder for the model.
	_, _, err = hermesModelFor(cfg, Selection{Provider: "aigw-gateway-openai-responses", Model: "gpt-unadded"})
	if err == nil || !strings.Contains(err.Error(), "which provider \"gateway\" has not added; run `aigw use hermes <provider/model>`") {
		t.Fatalf("an unadded choice = %v", err)
	}
}
