package configuration

import (
	"reflect"
	"strings"
	"testing"
)

func TestMergeRouteRetirementRejectsDanglingRecommendationWithoutMutatingInput(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["gateway"] = Account{
		Label: "Gateway", Endpoints: Endpoints{OpenAIResponses: "https://gateway.example/v1"},
	}
	cfg.Routes["old"] = testRoute("Old", "gateway", "gpt-old", ProtocolOpenAIResponses)
	cfg.Routes["selected"] = testRoute("Selected", "gateway", "gpt-selected", ProtocolOpenAIResponses)
	cfg.Recommendations[ClientCodex] = ClientRecommendation{Primary: ClientSelection{Route: "old"}}
	cfg.SetSelectedRoute(ClientCodex, "selected")
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	before := cfg.Clone()
	incoming := Manifest{
		Version: currentVersion,
		Routes: map[string]Route{
			"new": testRoute("New", "gateway", "gpt-new", ProtocolOpenAIResponses),
		},
	}
	_, err := MergeWithOptions(cfg, incoming, MergeOptions{RetireRoutes: map[string]bool{"old": true}})
	if err == nil || !strings.Contains(err.Error(), "client recommendation") {
		t.Fatalf("dangling recommendation error = %v", err)
	}
	if !reflect.DeepEqual(cfg, before) {
		t.Fatal("rejected retirement mutated its input")
	}
}

func TestMergeFalseRetirementSelectorDoesNotRetireRoute(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["gateway"] = Account{
		Label: "Gateway", Endpoints: Endpoints{OpenAIResponses: "https://gateway.example/v1"},
	}
	cfg.Routes["old"] = testRoute("Old", "gateway", "gpt-old", ProtocolOpenAIResponses)
	cfg.Normalize()
	incoming := Manifest{
		Version: currentVersion,
		Routes: map[string]Route{
			"new": testRoute("New", "gateway", "gpt-new", ProtocolOpenAIResponses),
		},
	}
	got, err := MergeWithOptions(cfg, incoming, MergeOptions{RetireRoutes: map[string]bool{"old": false}})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := got.Routes["old"]; !exists {
		t.Fatal("false retirement selector removed its Route")
	}
	if _, exists := got.Models["gpt-old"]; !exists {
		t.Fatal("false retirement selector removed its Model")
	}
}
