package domain

import (
	"strings"
	"testing"
)

// TestVariantLabelsNameTheirSuffix derives a variant's label from its
// family's with the rest of its ID as written, before or after the family's
// ID, and
// leaves a model whose ID neither begins nor ends with its family's alone.
func TestVariantLabelsNameTheirSuffix(t *testing.T) {
	cfg := teamFileConfig(t)
	cfg.Families["gpt-6.2-sol"] = Family{Label: "GPT-6.2 Sol"}
	for id, want := range map[string]string{
		"gpt-6.2-sol":          "Team · GPT-6.2 Sol",
		"gpt-6.2-sol-cdx":      "Team · GPT-6.2 Sol · cdx",
		"gpt-6.2-sol-ssvip":    "Team · GPT-6.2 Sol · ssvip",
		"codex-gpt-6.2-sol":    "Team · GPT-6.2 Sol · codex",
		"GPT-6.2-Sol-CDX":      "Team · GPT-6.2 Sol · CDX",
		"gpt-6.2-sol-2026-11":  "Team · GPT-6.2 Sol · 2026-11",
		"sol-latest-for-teams": "Team · GPT-6.2 Sol",
	} {
		model := anthropicModel("team", id)
		model.Family = "gpt-6.2-sol"
		cfg.Models[model.Ref()] = model
		if got := cfg.ModelLabel(model.Ref()); got != want {
			t.Errorf("label of %s = %q, want %q", id, got, want)
		}
		delete(cfg.Models, model.Ref())
	}
}

// TestTwoModelsOfOneProviderNeedTwoLabels refuses one label, derived or
// explicit, in any case, for two models of one provider, and accepts it on
// two providers.
func TestTwoModelsOfOneProviderNeedTwoLabels(t *testing.T) {
	cfg := teamFileConfig(t)
	// cdx-opus derives "Team · Opus · cdx", which opus-cdx declares in
	// another case.
	clash := anthropicModel("team", "cdx-opus")
	clash.Family = "opus"
	cfg.Models[clash.Ref()] = clash
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "team/opus-cdx") || !strings.Contains(err.Error(), "team/cdx-opus") {
		t.Fatalf("a derived label shared with an explicit one in another case = %v", err)
	}
	delete(cfg.Models, clash.Ref())
	// Two IDs unrelated to their family's derive the family's own label.
	for _, id := range []string{"alias-model", "other-alias"} {
		alias := anthropicModel("team", id)
		alias.Family = "opus"
		cfg.Models[alias.Ref()] = alias
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "team/alias-model") || !strings.Contains(err.Error(), "team/other-alias") {
		t.Fatalf("two derived labels shared = %v", err)
	}
	delete(cfg.Models, NewModelRef("team", "other-alias"))
	clash = cfg.Models[NewModelRef("team", "alias-model")]
	clash.Label = "team · opus · cdx"
	cfg.Models[clash.Ref()] = clash
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "team/opus-cdx") {
		t.Fatalf("an explicit label shared in another case = %v", err)
	}
	clash.Label = "Team · Opus · Two"
	cfg.Models[clash.Ref()] = clash
	if err := cfg.Validate(); err != nil {
		t.Fatalf("two labels = %v", err)
	}
	if err := cfg.AppendProvider("other", Provider{Label: "Team", Endpoints: Endpoints{Anthropic: "https://other.test"}}); err != nil {
		t.Fatal(err)
	}
	elsewhere := anthropicModel("other", "alias-model")
	elsewhere.Family = "opus"
	cfg.Models[elsewhere.Ref()] = elsewhere
	if err := cfg.Validate(); err != nil {
		t.Fatalf("one label on two providers = %v", err)
	}
}
