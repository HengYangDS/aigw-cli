package domain

import (
	"reflect"
	"strings"
	"testing"
)

func TestMergeModelRetirementRejectsDanglingRecommendationWithoutMutatingInput(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Accounts["gateway"] = Account{
		Label: "Gateway", Endpoints: Endpoints{OpenAIResponses: "https://gateway.example/v1"},
	}
	cfg.Models["old"] = testModel("Old", "gateway", "gpt-old", ProtocolOpenAIResponses)
	cfg.Models["selected"] = testModel("Selected", "gateway", "gpt-selected", ProtocolOpenAIResponses)
	cfg.Recommendations[ClientCodex] = ClientRecommendation{Primary: ClientSelection{Model: "old"}}
	cfg.SetSelectedModel(ClientCodex, "selected", "")
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	before := cfg.Clone()
	incoming := Manifest{
		Version: ManifestVersion,
		Models: map[string]Model{
			"new": testModel("New", "gateway", "gpt-new", ProtocolOpenAIResponses),
		},
	}
	_, err := MergeWithOptions(cfg, incoming, MergeOptions{RetireModels: map[string]bool{"old": true}})
	if err == nil || !strings.Contains(err.Error(), "client recommendation") {
		t.Fatalf("dangling recommendation error = %v", err)
	}
	if !reflect.DeepEqual(cfg, before) {
		t.Fatal("rejected retirement mutated its input")
	}
}

func TestMergeFalseRetirementSelectorDoesNotRetireModel(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Accounts["gateway"] = Account{
		Label: "Gateway", Endpoints: Endpoints{OpenAIResponses: "https://gateway.example/v1"},
	}
	cfg.Models["old"] = testModel("Old", "gateway", "gpt-old", ProtocolOpenAIResponses)
	cfg.Normalize()
	incoming := Manifest{
		Version: ManifestVersion,
		Models: map[string]Model{
			"new": testModel("New", "gateway", "gpt-new", ProtocolOpenAIResponses),
		},
	}
	got, err := MergeWithOptions(cfg, incoming, MergeOptions{RetireModels: map[string]bool{"old": false}})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := got.Models["old"]; !exists {
		t.Fatal("false retirement selector removed its Route")
	}
	if _, exists := got.Families["gpt-old"]; !exists {
		t.Fatal("false retirement selector removed its Model")
	}
}
