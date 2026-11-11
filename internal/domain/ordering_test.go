package domain

import (
	"slices"
	"strings"
	"testing"
)

func anthropicModel(provider, id string) Model {
	return Model{Provider: provider, ID: id, Protocols: map[EndpointProtocol][]Capability{ProtocolAnthropic: {}}}
}

// TestAppendRecordsTheFileOrder appends providers, models and families as
// the store reads a file: every list follows that order, the models grouped
// by provider in the providers' order, and an entry that a command adds to a
// map directly comes after the recorded ones, in identity order.
func TestAppendRecordsTheFileOrder(t *testing.T) {
	cfg := NewConfig(testAdmission())
	for _, id := range []string{"zeta", "alpha"} {
		if err := cfg.AppendProvider(id, Provider{Label: strings.ToUpper(id), Endpoints: Endpoints{Anthropic: "https://" + id + ".test"}}); err != nil {
			t.Fatal(err)
		}
	}
	for _, model := range []Model{anthropicModel("alpha", "second"), anthropicModel("zeta", "first"), anthropicModel("alpha", "first")} {
		if err := cfg.AppendModel(model); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"sol", "opus"} {
		if err := cfg.AppendFamily(id, Family{Label: strings.ToUpper(id)}); err != nil {
			t.Fatal(err)
		}
	}
	if got := cfg.ProviderIDs(); !slices.Equal(got, []string{"zeta", "alpha"}) {
		t.Fatalf("providers = %v", got)
	}
	if got := cfg.FamilyIDs(); !slices.Equal(got, []string{"sol", "opus"}) {
		t.Fatalf("families = %v", got)
	}
	recorded := []ModelRef{NewModelRef("zeta", "first"), NewModelRef("alpha", "second"), NewModelRef("alpha", "first")}
	if got := cfg.ModelRefs(); !slices.Equal(got, recorded) {
		t.Fatalf("models = %v", got)
	}
	if got := cfg.ProviderModelRefs("alpha"); !slices.Equal(got, recorded[1:]) {
		t.Fatalf("alpha's models = %v", got)
	}

	cfg.Providers["beta"] = Provider{Label: "Beta", Endpoints: Endpoints{Anthropic: "https://beta.test"}}
	cfg.Models[NewModelRef("beta", "only")] = anthropicModel("beta", "only")
	cfg.Models[NewModelRef("alpha", "added")] = anthropicModel("alpha", "added")
	cfg.Families["haiku"] = Family{Label: "Haiku"}
	if got := cfg.ProviderIDs(); !slices.Equal(got, []string{"zeta", "alpha", "beta"}) {
		t.Fatalf("providers after a direct addition = %v", got)
	}
	if got := cfg.FamilyIDs(); !slices.Equal(got, []string{"sol", "opus", "haiku"}) {
		t.Fatalf("families after a direct addition = %v", got)
	}
	want := []ModelRef{recorded[0], recorded[1], recorded[2], NewModelRef("alpha", "added"), NewModelRef("beta", "only")}
	if got := cfg.ModelRefs(); !slices.Equal(got, want) {
		t.Fatalf("models after a direct addition = %v", got)
	}
	// Normalize records the order, so a clone keeps it.
	cfg.Normalize()
	clone := cfg.Clone()
	if got := clone.ModelRefs(); !slices.Equal(got, want) {
		t.Fatalf("a clone's models = %v", got)
	}
}

// TestAppendRefusesARepeatedEntry refuses what a file names twice, by name,
// and changes nothing.
func TestAppendRefusesARepeatedEntry(t *testing.T) {
	var cfg Config
	if err := cfg.AppendProvider("team", Provider{Label: "Team"}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AppendFamily("opus", Family{Label: "Opus"}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AppendModel(anthropicModel("team", "opus-1")); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		err  error
		want string
	}{
		{cfg.AppendProvider("team", Provider{Label: "Again"}), `provider "team" is defined twice`},
		{cfg.AppendFamily("opus", Family{Label: "Again"}), `family "opus" is defined twice`},
		{cfg.AppendModel(Model{Provider: "team", ID: "opus-1", Label: "Again"}), `provider "team" defines model "opus-1" twice`},
	} {
		if test.err == nil || test.err.Error() != test.want {
			t.Errorf("repeat = %v, want %q", test.err, test.want)
		}
	}
	if cfg.Providers["team"].Label != "Team" || cfg.Families["opus"].Label != "Opus" || cfg.Models[NewModelRef("team", "opus-1")].Label != "" ||
		len(cfg.ProviderIDs()) != 1 || len(cfg.FamilyIDs()) != 1 || len(cfg.ModelRefs()) != 1 {
		t.Fatalf("a refused repeat changed the configuration: %+v", cfg)
	}
}

// TestModelsOfUnknownProvidersComeLast keeps the models of providers that
// the configuration does not hold, which validation refuses, after the
// others, in identity order.
func TestModelsOfUnknownProvidersComeLast(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["known"] = Provider{Label: "Known"}
	for _, ref := range []ModelRef{NewModelRef("ghost-b", "m"), NewModelRef("known", "m"), NewModelRef("ghost-a", "m")} {
		cfg.Models[ref] = anthropicModel(ref.Provider(), ref.ID())
	}
	want := []ModelRef{NewModelRef("known", "m"), NewModelRef("ghost-a", "m"), NewModelRef("ghost-b", "m")}
	if got := cfg.ModelRefs(); !slices.Equal(got, want) {
		t.Fatalf("models = %v, want %v", got, want)
	}
}

func TestCompareProtocolsFollowsOutputOrder(t *testing.T) {
	protocols := []EndpointProtocol{ProtocolChat, ProtocolResponses, ProtocolAnthropic}
	slices.SortFunc(protocols, CompareProtocols)
	if want := []EndpointProtocol{ProtocolAnthropic, ProtocolResponses, ProtocolChat}; !slices.Equal(protocols, want) {
		t.Fatalf("protocols = %v, want %v", protocols, want)
	}
}
