package store

import (
	"errors"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

// A model may declare a protocol for which its provider has no endpoint: on a
// provider that the configuration keeps, the protocol is unusable, and a
// recommendation of it waits, until the provider has one; a provider that
// the file adds is refused.
func TestParseAdmitsAModelOnAProtocolWithoutAnEndpoint(t *testing.T) {
	manifest, err := ParseTeamFile([]byte(`version = 8

[[providers]]
id = "gateway"
label = "Gateway"
endpoints = {anthropic = "https://gateway.test"}

[[providers.models]]
id = "model"
family = "model"
label = "Codex"
protocols = {responses = []}

[recommendations.codex]
primary = {model = "gateway/model"}
`), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := domain.MergeTeamFile(domain.NewConfig(testAdmission()), manifest, false); err == nil || !strings.Contains(err.Error(), "provider gateway, which the file adds") {
		t.Fatalf("a new provider without the model's endpoint = %v", err)
	}
	local := domain.NewConfig(testAdmission())
	local.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg, report, err := domain.MergeTeamFile(local, manifest, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Unusable) != 1 || report.Unusable[0].Protocol != domain.ProtocolResponses {
		t.Errorf("unusable protocols %+v", report.Unusable)
	}
	var missing *domain.RuntimeMissingEndpointError
	if _, err := cfg.ResolveModelProtocol(ClientCodex, domain.NewModelRef("gateway", "model"), domain.ProtocolResponses); !errors.As(err, &missing) || missing.ProviderID != "gateway" {
		t.Fatalf("a selection on the unusable protocol = %v", err)
	}
}

// A recommendation may name a model that only the configuration defines, so
// the merge, not the parser, refuses one that neither defines.
func TestMergeRejectsRecommendedModelReferencingUnknownModel(t *testing.T) {
	raw := []byte(`version = 8

[[providers]]
id = "team"
label = "Team"
endpoints = {anthropic = "https://team.test"}

[[providers.models]]
id = "claude-team"
family = "claude-team"
label = "Team"
protocols = {anthropic = []}

[recommendations.claude]
primary = {model = "unknown/missing"}
`)
	manifest, err := ParseTeamFile(raw, testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := domain.MergeTeamFile(domain.NewConfig(testAdmission()), manifest, false); err == nil || !strings.Contains(err.Error(), `claude recommendation names unknown/missing`) {
		t.Fatalf("unknown recommended model error = %v", err)
	}
}

// A team file never sets a client's selection, so a [clients] section is
// refused before anything else of it is read.
func TestParseRefusesATeamFileThatSetsClients(t *testing.T) {
	_, err := ParseTeamFile([]byte("version = 8\n[clients.claude]\nmodel = \"team/model\"\n"), testAdmission())
	if !errors.Is(err, ErrTeamFileClients) {
		t.Fatalf("a [clients] section = %v", err)
	}
}
