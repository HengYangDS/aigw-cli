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
		Order:        3,
		Endpoints:    Endpoints{Anthropic: "http://127.0.0.1:8792/team", Responses: "https://team.example/v1"},
		BalanceProbe: &BalanceProbe{Kind: "dmxapi", BaseURL: "https://team.example"},
	}
	cfg.Providers["solo"] = Provider{Label: "Solo", Endpoints: Endpoints{Anthropic: "https://solo.example"}}
	cfg.Models["sonnet"] = Model{Provider: "team", Family: "sonnet", Protocols: map[EndpointProtocol][]Capability{ProtocolAnthropic: {CapabilityText}}}
	cfg.Models["gpt"] = testModel("", "team", "gpt", ProtocolResponses)
	cfg.Models["spare"] = testModel("", "team", "spare", ProtocolAnthropic)
	cfg.Models["solo"] = testModel("", "solo", "solo", ProtocolAnthropic)
	cfg.Families["unused"] = Family{Label: "Unused"}
	cfg.Recommendations[ClientCodex] = ClientRecommendation{Primary: ClientSelection{Model: "gpt"}}
	cfg.SetSelectedModel(ClientClaude, "solo", "")
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	return cfg
}

// teamFile is the team's file: team's endpoints point at the provider, and a
// new provider, fresh, comes with one model that Codex is recommended.
func teamFile(t *testing.T) Manifest {
	t.Helper()
	file := Manifest{
		Version: ManifestVersion,
		Providers: map[string]Provider{
			"team": {
				Label:        "Team Gateway",
				Endpoints:    Endpoints{Anthropic: "https://team.example", Responses: "https://team.example/v1"},
				BalanceProbe: &BalanceProbe{Kind: "dmxapi", BaseURL: "https://team.example"},
			},
			"fresh": {Label: "Fresh", Endpoints: Endpoints{Responses: "https://fresh.example/v1"}},
		},
		Families: map[string]Family{"gpt": {Label: "GPT"}, "fresh": {Label: "Fresh"}},
		Models: map[string]Model{
			"gpt":   testModel("Team GPT", "team", "gpt", ProtocolResponses),
			"fresh": testModel("", "fresh", "fresh", ProtocolResponses),
		},
		Recommendations: map[string]ClientRecommendation{ClientCodex: {Primary: ClientSelection{Model: "fresh"}}},
	}
	admitted, err := file.Admit(testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	return admitted
}

func mergeTeam(t *testing.T, cfg Config, file Manifest, prune bool) (Config, TeamMerge) {
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
	if got := merged.Models["gpt"].Label; got != "Team GPT" {
		t.Errorf("the file's model label = %q", got)
	}
	if got := merged.Families["gpt"].Label; got != "GPT" {
		t.Errorf("the file's family label = %q", got)
	}
	if got := merged.Recommendations[ClientCodex].Primary.Model; got != "fresh" {
		t.Errorf("the codex recommendation = %q", got)
	}
	for _, key := range []string{"sonnet", "spare", "solo"} {
		if _, kept := merged.Models[key]; !kept {
			t.Errorf("local-only model %s was removed", key)
		}
	}
	if _, kept := merged.Families["unused"]; !kept {
		t.Error("a local-only family was removed without pruning")
	}
	if got := merged.Providers["team"].Label; got != "Team Gateway" || merged.Providers["team"].Order != 0 {
		t.Errorf("the file's provider label = %q, order %d", got, merged.Providers["team"].Order)
	}
	if got := merged.Clients[ClientClaude].Model; got != "solo" {
		t.Errorf("claude's selection = %q", got)
	}
	if !slices.Equal(report.AddedProviders, []string{"fresh"}) || !slices.Equal(report.AddedModels, []ModelRef{NewModelRef("fresh", "fresh")}) ||
		!slices.Equal(report.AddedFamilies, []string{"fresh"}) {
		t.Errorf("added %v %v %v", report.AddedProviders, report.AddedModels, report.AddedFamilies)
	}
	if !slices.Equal(report.ChangedProviders, []string{"team"}) || !slices.Equal(report.ChangedModels, []ModelRef{NewModelRef("team", "gpt")}) ||
		!slices.Equal(report.ChangedFamilies, []string{"gpt"}) || !slices.Equal(report.ChangedRecommendations, []string{ClientCodex}) || len(report.AddedRecommendations) != 0 {
		t.Errorf("changed %v %v %v %v", report.ChangedProviders, report.ChangedModels, report.ChangedFamilies, report.ChangedRecommendations)
	}
	if len(report.PrunedModels)+len(report.PrunedFamilies) != 0 {
		t.Errorf("pruned without --prune: %v %v", report.PrunedModels, report.PrunedFamilies)
	}
}

func TestMergeTeamFilePrunesOnlyUnusedLocalEntries(t *testing.T) {
	cfg := localTeamConfig(t)
	// The local recommendation names spare, so pruning keeps it.
	cfg.Recommendations[ClientHermes] = ClientRecommendation{Primary: ClientSelection{Model: "spare", Protocol: ProtocolAnthropic}}
	merged, report := mergeTeam(t, cfg, teamFile(t), true)
	if !slices.Equal(report.PrunedModels, []ModelRef{NewModelRef("team", "sonnet")}) {
		t.Errorf("pruned models %v", report.PrunedModels)
	}
	if !slices.Equal(report.PrunedFamilies, []string{"sonnet", "unused"}) {
		t.Errorf("pruned families %v", report.PrunedFamilies)
	}
	for _, key := range []string{"solo", "spare", "gpt", "fresh"} {
		if _, kept := merged.Models[key]; !kept {
			t.Errorf("pruning removed %s, which is selected, recommended or in the file", key)
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
	file.Models["sonnet"] = Model{Provider: "team", Family: "sonnet", UpstreamModel: "sonnet",
		Protocols: map[EndpointProtocol][]Capability{ProtocolAnthropic: {CapabilityText, CapabilityToolSearch}}}
	// The Responses endpoint is the same in both, so its capabilities stay.
	gpt := file.Models["gpt"]
	gpt.Protocols = map[EndpointProtocol][]Capability{ProtocolResponses: {CapabilityReasoning}}
	file.Models["gpt"] = gpt
	merged, report := mergeTeam(t, cfg, file, false)
	if got := merged.Models["sonnet"].Protocols[ProtocolAnthropic]; !slices.Equal(got, []Capability{CapabilityText}) {
		t.Errorf("sonnet's Anthropic capabilities = %v", got)
	}
	want := []DroppedCapabilities{{Model: NewModelRef("team", "sonnet"), Protocol: ProtocolAnthropic, Capabilities: []Capability{CapabilityToolSearch}}}
	if !reflect.DeepEqual(report.DroppedCapabilities, want) {
		t.Errorf("dropped %+v", report.DroppedCapabilities)
	}
	if got := merged.Models["gpt"].Protocols[ProtocolResponses]; !slices.Equal(got, []Capability{CapabilityReasoning}) {
		t.Errorf("gpt's Responses capabilities = %v", got)
	}
}

func TestMergeTeamFileKeepsAModelWhoseChangeWouldMoveASelection(t *testing.T) {
	for _, tc := range []struct {
		name string
		file Model
		want KeptModel
	}{
		{"a protocol that the selection uses", testModel("", "solo", "solo"),
			KeptModel{Model: NewModelRef("solo", "solo"), Client: ClientClaude, Protocol: ProtocolAnthropic, File: NewModelRef("solo", "solo")}},
		{"another model ID", testModel("", "solo", "solo-2", ProtocolAnthropic),
			KeptModel{Model: NewModelRef("solo", "solo"), Client: ClientClaude, File: NewModelRef("solo", "solo-2")}},
	} {
		cfg := localTeamConfig(t)
		file := teamFile(t)
		file.Providers["solo"] = cfg.Providers["solo"]
		file.Families[tc.file.Family] = Family{Label: "Solo"}
		file.Models["solo"] = tc.file
		file.Models["solo"] = Model{Provider: "solo", Family: tc.file.Family, UpstreamModel: tc.file.Family, Protocols: tc.file.Protocols}
		merged, report := mergeTeam(t, cfg, file, false)
		if !reflect.DeepEqual(merged.Models["solo"], cfg.Models["solo"]) {
			t.Errorf("%s: the selected model = %+v", tc.name, merged.Models["solo"])
		}
		if !slices.Equal(report.KeptModels, []KeptModel{tc.want}) {
			t.Errorf("%s: kept %+v", tc.name, report.KeptModels)
		}
	}
}

func TestMergeTeamFileAddsAModelOnAProtocolWithoutAnEndpoint(t *testing.T) {
	cfg := localTeamConfig(t)
	file := teamFile(t)
	file.Providers["solo"] = Provider{Label: "Solo", Endpoints: Endpoints{Anthropic: "https://solo.example", Responses: "https://solo.example/v1"}}
	file.Families["solo-gpt"] = Family{Label: "Solo GPT"}
	file.Models["solo-gpt"] = Model{Provider: "solo", Family: "solo-gpt", UpstreamModel: "solo-gpt", Protocols: map[EndpointProtocol][]Capability{ProtocolResponses: {}}}
	file.Recommendations[ClientHermes] = ClientRecommendation{Primary: ClientSelection{Model: "solo-gpt", Protocol: ProtocolResponses}}
	merged, report := mergeTeam(t, cfg, file, false)
	if _, added := merged.Models["solo-gpt"]; !added {
		t.Fatal("the model was not added")
	}
	if want := []UnusableProtocol{{Model: NewModelRef("solo", "solo-gpt"), Protocol: ProtocolResponses}}; !slices.Equal(report.Unusable, want) {
		t.Errorf("unusable %+v", report.Unusable)
	}
	if _, err := merged.ResolveModelProtocol(ClientHermes, "solo-gpt", ProtocolResponses); err == nil {
		t.Error("a client resolves the model on a protocol without an endpoint")
	}
	selected, err := merged.SelectModelsForConnectedProviders([]string{"solo"})
	if err != nil {
		t.Fatal(err)
	}
	if got := selected.Clients[ClientHermes].Model; got != "" {
		t.Errorf("hermes was given %q on a protocol without an endpoint", got)
	}
}

func TestMergeTeamFileRefusesWhatItCannotApply(t *testing.T) {
	cfg := localTeamConfig(t)
	undefined := teamFile(t)
	undefined.Recommendations[ClientHermes] = ClientRecommendation{Primary: ClientSelection{Model: "missing", Protocol: ProtocolAnthropic}}
	if _, _, err := MergeTeamFile(cfg, undefined, false); err == nil || !strings.Contains(err.Error(), `hermes recommendation names "missing", which neither the team file nor the configuration defines`) {
		t.Errorf("an undefined recommendation: %v", err)
	}
	local := teamFile(t)
	local.Recommendations[ClientHermes] = ClientRecommendation{Primary: ClientSelection{Model: "spare", Protocol: ProtocolAnthropic}}
	if merged, _ := mergeTeam(t, cfg, local, true); merged.Recommendations[ClientHermes].Primary.Model != "spare" {
		t.Error("a recommendation of a model that only the configuration defines was not stored")
	}
	version := teamFile(t)
	version.Version = ManifestVersion - 1
	if _, _, err := MergeTeamFile(cfg, version, false); err == nil || !strings.Contains(err.Error(), "unsupported configuration manifest version") {
		t.Errorf("another version: %v", err)
	}
}

func TestTeamFileMayRecommendAModelThatOnlyTheConfigurationDefines(t *testing.T) {
	file := Manifest{Version: ManifestVersion, Recommendations: map[string]ClientRecommendation{ClientCodex: {Primary: ClientSelection{Model: "local"}}}}
	if _, err := file.Admit(testAdmission()); err != nil {
		t.Errorf("a recommendation of a local model: %v", err)
	}
	empty := Manifest{Version: ManifestVersion, Recommendations: map[string]ClientRecommendation{ClientCodex: {Alternatives: []ClientSelection{{Model: "local"}}}}}
	if _, err := empty.Admit(testAdmission()); err == nil || !strings.Contains(err.Error(), "has no primary route") {
		t.Errorf("a recommendation without a primary: %v", err)
	}
}

// A provider that the team file adds has only the file's endpoints, so a
// model protocol that it has none for is refused, naming the model, the
// provider and the protocol, while a kept provider's is reported unusable.
func TestMergeTeamFileRefusesANewProvidersProtocolWithoutAnEndpoint(t *testing.T) {
	file := teamFile(t)
	file.Providers["fresh"] = Provider{Label: "Fresh", Endpoints: Endpoints{Anthropic: "https://fresh.example"}}
	file.Models["fresh"] = testModel("", "fresh", "fresh", ProtocolChat)
	file.Models["fresh"] = Model{Provider: "fresh", Family: "fresh", UpstreamModel: "fresh", Protocols: map[EndpointProtocol][]Capability{ProtocolChat: {}}}
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
	file.Models["solo-chat"] = Model{Provider: "solo", Family: "solo-chat", UpstreamModel: "solo-chat", Protocols: map[EndpointProtocol][]Capability{ProtocolChat: {}}}
	file.Recommendations[ClientHermes] = ClientRecommendation{Primary: ClientSelection{Model: "solo-chat"}}
	merged, report := mergeTeam(t, cfg, file, false)
	if merged.Recommendations[ClientHermes].Primary.Model != "solo-chat" || len(report.Unusable) != 1 {
		t.Fatalf("recommendation %+v, unusable %+v", merged.Recommendations[ClientHermes], report.Unusable)
	}
	selected, err := merged.SelectModelsForConnectedProviders([]string{"solo", "team"})
	if err != nil || selected.Clients[ClientHermes].Model != "" {
		t.Fatalf("hermes was given %q, %v", selected.Clients[ClientHermes].Model, err)
	}
	// A recommendation whose model the client cannot speak at all is still
	// refused.
	wrong := teamFile(t)
	wrong.Recommendations[ClientCodex] = ClientRecommendation{Primary: ClientSelection{Model: "sonnet"}}
	if _, _, err := MergeTeamFile(cfg, wrong, false); err == nil {
		t.Fatal("a recommendation of a model that codex cannot speak was stored")
	}
}
