package domain

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// localTeamConfig is a configured home: team's Anthropic endpoint points at a
// local proxy, solo has no balance probe, and Claude selects solo's model.
func localTeamConfig(t *testing.T) Config {
	t.Helper()
	cfg := NewConfig(testAdmission())
	cfg.Providers["team"] = Provider{
		Label:        "Team",
		Endpoints:    Endpoints{Anthropic: "http://127.0.0.1:8792/team", Responses: "https://team.example/v1"},
		BalanceProbe: &BalanceProbe{Kind: "dmxapi", BaseURL: "https://team.example"},
	}
	cfg.Providers["solo"] = Provider{Label: "Solo", Endpoints: Endpoints{Anthropic: "https://solo.example"}}
	cfg.Models[NewModelRef("team", "sonnet")] = Model{Provider: "team", Family: "sonnet", ID: "sonnet", Protocols: map[EndpointProtocol][]Capability{ProtocolAnthropic: {CapabilityText}}}
	cfg.Models[NewModelRef("team", "gpt")] = testModel("", "team", "gpt", ProtocolResponses)
	cfg.Models[NewModelRef("team", "spare")] = testModel("", "team", "spare", ProtocolAnthropic)
	cfg.Models[NewModelRef("solo", "solo")] = testModel("", "solo", "solo", ProtocolAnthropic)
	cfg.Families["unused"] = Family{Label: "Unused"}
	cfg.Recommendations[ClientCodex] = ClientRecommendation{Primary: ClientSelection{Model: NewModelRef("team", "gpt")}}
	cfg.SetSelectedModel(ClientClaude, NewModelRef("solo", "solo"), "")
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	return cfg
}

// teamFile is the team's file: team's endpoints point at the provider, and a
// new provider, fresh, comes with one model that Codex is recommended.
func teamFile(t *testing.T) Config {
	t.Helper()
	file := Config{
		Version: ConfigVersion,
		Providers: map[string]Provider{
			"team": {
				Label:        "Team Gateway",
				Endpoints:    Endpoints{Anthropic: "https://team.example", Responses: "https://team.example/v1"},
				BalanceProbe: &BalanceProbe{Kind: "dmxapi", BaseURL: "https://team.example"},
			},
			"fresh": {Label: "Fresh", Endpoints: Endpoints{Responses: "https://fresh.example/v1"}},
		},
		Families: map[string]Family{"gpt": {Label: "GPT"}, "fresh": {Label: "Fresh"}},
		Models: map[ModelRef]Model{
			NewModelRef("team", "gpt"):    testModel("Team GPT", "team", "gpt", ProtocolResponses),
			NewModelRef("fresh", "fresh"): testModel("", "fresh", "fresh", ProtocolResponses),
		},
		Recommendations: map[string]ClientRecommendation{ClientCodex: {Primary: ClientSelection{Model: NewModelRef("fresh", "fresh")}}},
	}
	file.SetAdmission(testAdmission())
	admitted, err := file.AdmitTeamFile()
	if err != nil {
		t.Fatal(err)
	}
	return admitted
}

func mergeTeam(t *testing.T, cfg Config, file Config, prune bool) (Config, TeamMerge) {
	t.Helper()
	before := cfg.Clone()
	merged, report, err := MergeTeamFile(cfg, file, prune)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, before) {
		t.Fatal("the merge changed its input")
	}
	return merged, report
}

func TestMergeTeamFileTakesTheFilesDefinitionsAndKeepsLocalOnes(t *testing.T) {
	cfg := localTeamConfig(t)
	merged, report := mergeTeam(t, cfg, teamFile(t), false)
	if got := merged.Models[NewModelRef("team", "gpt")].Label; got != "Team GPT" {
		t.Errorf("the file's model label = %q", got)
	}
	if got := merged.Families["gpt"].Label; got != "GPT" {
		t.Errorf("the file's family label = %q", got)
	}
	if got := merged.Recommendations[ClientCodex].Primary.Model; got != NewModelRef("fresh", "fresh") {
		t.Errorf("the codex recommendation = %q", got)
	}
	for _, ref := range []ModelRef{NewModelRef("team", "sonnet"), NewModelRef("team", "spare"), NewModelRef("solo", "solo")} {
		if _, kept := merged.Models[ref]; !kept {
			t.Errorf("local-only model %s was removed", ref)
		}
	}
	if _, kept := merged.Families["unused"]; !kept {
		t.Error("a local-only family was removed without pruning")
	}
	if got := merged.Providers["team"].Label; got != "Team Gateway" {
		t.Errorf("the file's provider label = %q", got)
	}
	// The file's entries come first, in its order, then the local-only ones.
	if got := merged.ProviderIDs(); !slices.Equal(got, []string{"fresh", "team", "solo"}) {
		t.Errorf("providers in display order = %v", got)
	}
	wantModels := []ModelRef{NewModelRef("fresh", "fresh"), NewModelRef("team", "gpt"), NewModelRef("team", "sonnet"), NewModelRef("team", "spare"), NewModelRef("solo", "solo")}
	if got := merged.ModelRefs(); !slices.Equal(got, wantModels) {
		t.Errorf("models in display order = %v", got)
	}
	if got := merged.Clients[ClientClaude].Model; got != NewModelRef("solo", "solo") {
		t.Errorf("claude's selection = %q", got)
	}
	if !slices.Equal(report.AddedProviders, []string{"fresh"}) || !slices.Equal(report.AddedModels, []ModelRef{NewModelRef("fresh", "fresh")}) ||
		!slices.Equal(report.AddedFamilies, []string{"fresh", "gpt"}) {
		t.Errorf("added %v %v %v", report.AddedProviders, report.AddedModels, report.AddedFamilies)
	}
	if !slices.Equal(report.ChangedProviders, []string{"team"}) || !slices.Equal(report.ChangedModels, []ModelRef{NewModelRef("team", "gpt")}) ||
		len(report.ChangedFamilies) != 0 || !slices.Equal(report.ChangedRecommendations, []string{ClientCodex}) || len(report.AddedRecommendations) != 0 {
		t.Errorf("changed %v %v %v %v", report.ChangedProviders, report.ChangedModels, report.ChangedFamilies, report.ChangedRecommendations)
	}
	if len(report.PrunedModels)+len(report.PrunedFamilies) != 0 {
		t.Errorf("pruned without --prune: %v %v", report.PrunedModels, report.PrunedFamilies)
	}
}

// The report names every label that the file replaces, a provider's, a
// model's and a family's, with the local and the file's value, and a second
// merge of the same file replaces none.
func TestMergeTeamFileReportsEveryLabelThatItReplaces(t *testing.T) {
	cfg := localTeamConfig(t)
	cfg.Families["gpt"] = Family{ID: "gpt", Label: "Local GPT"}
	merged, report := mergeTeam(t, cfg, teamFile(t), false)
	want := []ChangedLabel{
		{Provider: "team", Local: "Team", File: "Team Gateway"},
		{Model: NewModelRef("team", "gpt"), File: "Team GPT"},
		{Family: "gpt", Local: "Local GPT", File: "GPT"},
	}
	if !reflect.DeepEqual(report.ChangedLabels, want) {
		t.Fatalf("changed labels = %+v, want %+v", report.ChangedLabels, want)
	}
	if _, again := mergeTeam(t, merged, teamFile(t), false); len(again.ChangedLabels) != 0 {
		t.Fatalf("a second merge replaces labels: %+v", again.ChangedLabels)
	}
}

func TestMergeTeamFilePrunesOnlyUnusedLocalEntries(t *testing.T) {
	cfg := localTeamConfig(t)
	// The local recommendation names spare, so pruning keeps it.
	cfg.Recommendations[ClientHermes] = ClientRecommendation{Primary: ClientSelection{Model: NewModelRef("team", "spare"), Protocol: ProtocolAnthropic}}
	merged, report := mergeTeam(t, cfg, teamFile(t), true)
	if !slices.Equal(report.PrunedModels, []ModelRef{NewModelRef("team", "sonnet")}) {
		t.Errorf("pruned models %v", report.PrunedModels)
	}
	if !slices.Equal(report.PrunedFamilies, []string{"unused"}) {
		t.Errorf("pruned families %v", report.PrunedFamilies)
	}
	for _, ref := range []ModelRef{NewModelRef("solo", "solo"), NewModelRef("team", "spare"), NewModelRef("team", "gpt"), NewModelRef("fresh", "fresh")} {
		if _, kept := merged.Models[ref]; !kept {
			t.Errorf("pruning removed %s, which is selected, recommended or in the file", ref)
		}
	}
	if len(merged.Providers) != 3 {
		t.Errorf("pruning changed the providers: %v", slices.Sorted(func(yield func(string) bool) {
			for id := range merged.Providers {
				yield(id)
			}
		}))
	}
}

func TestMergeTeamFileKeepsAnExistingProvidersEndpointsAndProbe(t *testing.T) {
	cfg := localTeamConfig(t)
	file := teamFile(t)
	team := file.Providers["team"]
	team.Endpoints.Chat = "https://team.example/v1"
	team.BalanceProbe = &BalanceProbe{Kind: "newapi", BaseURL: "https://team.example"}
	file.Providers["team"] = team
	solo := Provider{Label: "Solo", Endpoints: Endpoints{Anthropic: "https://solo.example"}, BalanceProbe: &BalanceProbe{Kind: "newapi", BaseURL: "https://solo.example"}}
	file.Providers["solo"] = solo
	merged, report := mergeTeam(t, cfg, file, false)
	if got := merged.Providers["team"].Endpoints; got != cfg.Providers["team"].Endpoints {
		t.Errorf("team's endpoints = %+v", got)
	}
	wantEndpoints := []KeptEndpoint{
		{Provider: "team", Protocol: ProtocolAnthropic, Local: "http://127.0.0.1:8792/team", File: "https://team.example"},
		{Provider: "team", Protocol: ProtocolChat, File: "https://team.example/v1"},
	}
	if !slices.Equal(report.KeptEndpoints, wantEndpoints) {
		t.Errorf("kept endpoints %+v", report.KeptEndpoints)
	}
	if got := merged.Providers["team"].BalanceProbe; *got != *cfg.Providers["team"].BalanceProbe {
		t.Errorf("team's probe = %+v", got)
	}
	wantProbes := []KeptProbe{{Provider: "team", Local: *cfg.Providers["team"].BalanceProbe, File: *team.BalanceProbe}}
	if !slices.Equal(report.KeptProbes, wantProbes) {
		t.Errorf("kept probes %+v", report.KeptProbes)
	}
	if got := merged.Providers["solo"].BalanceProbe; got == nil || *got != *solo.BalanceProbe {
		t.Errorf("solo did not take the file's probe: %+v", got)
	}
}

func TestMergeTeamFileDropsCapabilitiesQualifiedOnAnotherEndpoint(t *testing.T) {
	cfg := localTeamConfig(t)
	file := teamFile(t)
	file.Models[NewModelRef("team", "sonnet")] = Model{Provider: "team", Family: "sonnet", ID: "sonnet",
		Protocols: map[EndpointProtocol][]Capability{ProtocolAnthropic: {CapabilityText, CapabilityToolSearch}}}
	// The Responses endpoint is the same in both, so its capabilities stay.
	gpt := file.Models[NewModelRef("team", "gpt")]
	gpt.Protocols = map[EndpointProtocol][]Capability{ProtocolResponses: {CapabilityReasoning}}
	file.Models[NewModelRef("team", "gpt")] = gpt
	merged, report := mergeTeam(t, cfg, file, false)
	if got := merged.Models[NewModelRef("team", "sonnet")].Protocols[ProtocolAnthropic]; !slices.Equal(got, []Capability{CapabilityText}) {
		t.Errorf("sonnet's Anthropic capabilities = %v", got)
	}
	want := []DroppedCapabilities{{Model: NewModelRef("team", "sonnet"), Protocol: ProtocolAnthropic, Capabilities: []Capability{CapabilityToolSearch}}}
	if !reflect.DeepEqual(report.DroppedCapabilities, want) {
		t.Errorf("dropped %+v", report.DroppedCapabilities)
	}
	if got := merged.Models[NewModelRef("team", "gpt")].Protocols[ProtocolResponses]; !slices.Equal(got, []Capability{CapabilityReasoning}) {
		t.Errorf("gpt's Responses capabilities = %v", got)
	}
}

// TestMergeTeamFileKeepsAModelWhoseChangeWouldMoveASelection keeps the local
// definition of a model whose file definition omits the protocol of a
// client's selection of it.
func TestMergeTeamFileKeepsAModelWhoseChangeWouldMoveASelection(t *testing.T) {
	cfg := localTeamConfig(t)
	file := teamFile(t)
	file.Providers["solo"] = cfg.Providers["solo"]
	file.Families["solo"] = Family{Label: "Solo"}
	file.Models[NewModelRef("solo", "solo")] = Model{Provider: "solo", Family: "solo", ID: "solo", Protocols: map[EndpointProtocol][]Capability{}}
	merged, report := mergeTeam(t, cfg, file, false)
	if !reflect.DeepEqual(merged.Models[NewModelRef("solo", "solo")], cfg.Models[NewModelRef("solo", "solo")]) {
		t.Errorf("the selected model = %+v", merged.Models[NewModelRef("solo", "solo")])
	}
	if want := []KeptModel{{Model: NewModelRef("solo", "solo"), Client: ClientClaude, Protocol: ProtocolAnthropic}}; !slices.Equal(report.KeptModels, want) {
		t.Errorf("kept %+v", report.KeptModels)
	}
}

func TestMergeTeamFileAddsAModelOnAProtocolWithoutAnEndpoint(t *testing.T) {
	cfg := localTeamConfig(t)
	file := teamFile(t)
	file.Providers["solo"] = Provider{Label: "Solo", Endpoints: Endpoints{Anthropic: "https://solo.example", Responses: "https://solo.example/v1"}}
	file.Families["solo-gpt"] = Family{Label: "Solo GPT"}
	file.Models[NewModelRef("solo", "solo-gpt")] = Model{Provider: "solo", Family: "solo-gpt", ID: "solo-gpt", Protocols: map[EndpointProtocol][]Capability{ProtocolResponses: {}}}
	file.Recommendations[ClientHermes] = ClientRecommendation{Primary: ClientSelection{Model: NewModelRef("solo", "solo-gpt"), Protocol: ProtocolResponses}}
	merged, report := mergeTeam(t, cfg, file, false)
	if _, added := merged.Models[NewModelRef("solo", "solo-gpt")]; !added {
		t.Fatal("the model was not added")
	}
	if want := []UnusableProtocol{{Model: NewModelRef("solo", "solo-gpt"), Protocol: ProtocolResponses}}; !slices.Equal(report.Unusable, want) {
		t.Errorf("unusable %+v", report.Unusable)
	}
	if _, err := merged.ResolveModelProtocol(ClientHermes, NewModelRef("solo", "solo-gpt"), ProtocolResponses); err == nil {
		t.Error("a client resolves the model on a protocol without an endpoint")
	}
	selected, err := merged.SelectModelsForConnectedProviders([]string{"solo"})
	if err != nil {
		t.Fatal(err)
	}
	if got := selected.Clients[ClientHermes].Model; !got.IsZero() {
		t.Errorf("hermes was given %q on a protocol without an endpoint", got)
	}
}

func TestMergeTeamFileRefusesWhatItCannotApply(t *testing.T) {
	cfg := localTeamConfig(t)
	undefined := teamFile(t)
	undefined.Recommendations[ClientHermes] = ClientRecommendation{Primary: ClientSelection{Model: NewModelRef("team", "missing"), Protocol: ProtocolAnthropic}}
	if _, _, err := MergeTeamFile(cfg, undefined, false); err == nil || !strings.Contains(err.Error(), `hermes recommendation names team/missing, which neither the team file nor the configuration defines`) {
		t.Errorf("an undefined recommendation: %v", err)
	}
	local := teamFile(t)
	local.Recommendations[ClientHermes] = ClientRecommendation{Primary: ClientSelection{Model: NewModelRef("team", "spare"), Protocol: ProtocolAnthropic}}
	if merged, _ := mergeTeam(t, cfg, local, true); merged.Recommendations[ClientHermes].Primary.Model != NewModelRef("team", "spare") {
		t.Error("a recommendation of a model that only the configuration defines was not stored")
	}
	version := teamFile(t)
	version.Version = ConfigVersion - 1
	if _, _, err := MergeTeamFile(cfg, version, false); err == nil || !strings.Contains(err.Error(), "the team file has version 7, and this AIGW reads version 8") {
		t.Errorf("another version: %v", err)
	}
}

func TestTeamFileMayRecommendAModelThatOnlyTheConfigurationDefines(t *testing.T) {
	file := Config{Version: ConfigVersion, Recommendations: map[string]ClientRecommendation{ClientCodex: {Primary: ClientSelection{Model: NewModelRef("team", "local")}}}, admission: testAdmission()}
	if _, err := file.AdmitTeamFile(); err != nil {
		t.Errorf("a recommendation of a local model: %v", err)
	}
	empty := Config{Version: ConfigVersion, Recommendations: map[string]ClientRecommendation{ClientCodex: {Alternatives: []ClientSelection{{Model: NewModelRef("team", "local")}}}}, admission: testAdmission()}
	if _, err := empty.AdmitTeamFile(); err == nil || !strings.Contains(err.Error(), "has no primary model") {
		t.Errorf("a recommendation without a primary: %v", err)
	}
	unadmitted := Config{Version: ConfigVersion, Recommendations: map[string]ClientRecommendation{"gemini": {Primary: ClientSelection{Model: NewModelRef("team", "local")}}}, admission: testAdmission()}
	if _, err := unadmitted.AdmitTeamFile(); err == nil || !strings.Contains(err.Error(), `unsupported client "gemini"`) {
		t.Errorf("a recommendation for an unadmitted client: %v", err)
	}
}

// A provider that the team file adds has only the file's endpoints, so a
// model protocol that it has none for is refused, naming the model, the
// provider and the protocol, while a kept provider's is reported unusable.
func TestMergeTeamFileRefusesANewProvidersProtocolWithoutAnEndpoint(t *testing.T) {
	file := teamFile(t)
	file.Providers["fresh"] = Provider{Label: "Fresh", Endpoints: Endpoints{Anthropic: "https://fresh.example"}}
	file.Models[NewModelRef("fresh", "fresh")] = Model{Provider: "fresh", Family: "fresh", ID: "fresh", Protocols: map[EndpointProtocol][]Capability{ProtocolChat: {}}}
	delete(file.Recommendations, ClientCodex)
	_, _, err := MergeTeamFile(NewConfig(testAdmission()), file, false)
	if err == nil || !strings.Contains(err.Error(), "fresh/fresh") || !strings.Contains(err.Error(), "provider fresh") || !strings.Contains(err.Error(), "OpenAI Chat Completions") {
		t.Fatalf("a new provider without the model's endpoint = %v", err)
	}
}

// A recommendation that names no protocol waits, as one that names it does,
// when its client could use the model only on a protocol for which the kept
// provider has no endpoint.
func TestARecommendationWithoutAProtocolWaitsForAnEndpoint(t *testing.T) {
	cfg := localTeamConfig(t)
	file := teamFile(t)
	file.Providers["solo"] = Provider{Label: "Solo", Endpoints: Endpoints{Anthropic: "https://solo.example", Chat: "https://solo.example/v1"}}
	file.Families["solo-chat"] = Family{Label: "Solo Chat"}
	file.Models[NewModelRef("solo", "solo-chat")] = Model{Provider: "solo", Family: "solo-chat", ID: "solo-chat", Protocols: map[EndpointProtocol][]Capability{ProtocolChat: {}}}
	file.Recommendations[ClientHermes] = ClientRecommendation{Primary: ClientSelection{Model: NewModelRef("solo", "solo-chat")}}
	merged, report := mergeTeam(t, cfg, file, false)
	if merged.Recommendations[ClientHermes].Primary.Model != NewModelRef("solo", "solo-chat") || len(report.Unusable) != 1 {
		t.Fatalf("recommendation %+v, unusable %+v", merged.Recommendations[ClientHermes], report.Unusable)
	}
	selected, err := merged.SelectModelsForConnectedProviders([]string{"solo", "team"})
	if err != nil || !selected.Clients[ClientHermes].Model.IsZero() {
		t.Fatalf("hermes was given %q, %v", selected.Clients[ClientHermes].Model, err)
	}
	// A recommendation whose model the client cannot speak at all is still
	// refused.
	wrong := teamFile(t)
	wrong.Recommendations[ClientCodex] = ClientRecommendation{Primary: ClientSelection{Model: NewModelRef("team", "sonnet")}}
	if _, _, err := MergeTeamFile(cfg, wrong, false); err == nil {
		t.Fatal("a recommendation of a model that codex cannot speak was stored")
	}
}

// A file that lists providers, and one provider's models, in another order
// than the local configuration sets their display order: what the file names
// comes first, in the file's order, and local-only entries follow in their
// local order. Only the display order changes; no endpoint, Token or client
// selection does.
func TestMergeTeamFileOrdersByTheFileThenTheLocalEntries(t *testing.T) {
	cfg := NewConfig(testAdmission())
	for _, provider := range []struct {
		id     string
		models []string
	}{{"b", []string{"m3", "m2", "m1"}}, {"c", []string{"c1"}}, {"a", []string{"x"}}} {
		cfg.Providers[provider.id] = Provider{Label: strings.ToUpper(provider.id), Endpoints: Endpoints{Responses: "https://" + provider.id + ".example/v1"}}
		for _, model := range provider.models {
			cfg.Models[NewModelRef(provider.id, model)] = testModel("", provider.id, model, ProtocolResponses)
		}
	}
	cfg.SetSelectedModel(ClientCodex, NewModelRef("b", "m3"), "")
	cfg.Normalize()
	// Local order: providers b, c, a; b's models m3, m2, m1.
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	file := Config{
		Version:   ConfigVersion,
		Providers: map[string]Provider{"a": cfg.Providers["a"], "b": cfg.Providers["b"]},
		Families:  map[string]Family{},
		Models: map[ModelRef]Model{
			NewModelRef("a", "y"):  testModel("", "a", "y", ProtocolResponses),
			NewModelRef("b", "m1"): cfg.Models[NewModelRef("b", "m1")],
			NewModelRef("b", "m2"): cfg.Models[NewModelRef("b", "m2")],
		},
	}
	file.SetAdmission(testAdmission())
	file.order = displayOrder{providers: []string{"a", "b"}, models: []ModelRef{NewModelRef("a", "y"), NewModelRef("b", "m1"), NewModelRef("b", "m2")}}
	merged, _ := mergeTeam(t, cfg, file, false)
	if got := merged.ProviderIDs(); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Errorf("providers = %v, want the file's a, b, then the local-only c", got)
	}
	want := []ModelRef{NewModelRef("a", "y"), NewModelRef("a", "x"), NewModelRef("b", "m1"), NewModelRef("b", "m2"), NewModelRef("b", "m3"), NewModelRef("c", "c1")}
	if got := merged.ModelRefs(); !slices.Equal(got, want) {
		t.Errorf("models = %v, want each provider's file models, then its local-only ones: %v", got, want)
	}
	if merged.Providers["b"].Endpoints != cfg.Providers["b"].Endpoints || merged.Clients[ClientCodex].Model != NewModelRef("b", "m3") {
		t.Errorf("the order changed an endpoint or a selection: %+v", merged.Clients[ClientCodex])
	}
}
