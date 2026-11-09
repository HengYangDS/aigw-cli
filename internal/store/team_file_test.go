package store

import (
	"errors"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

// A model may declare a protocol for which its provider has no endpoint: the
// protocol is unusable, and a recommendation of it waits, until the provider
// has one.
func TestParseAdmitsAModelOnAProtocolWithoutAnEndpoint(t *testing.T) {
	manifest, err := Parse([]byte(`version = 7

[recommendations.codex.primary]
route = "codex"

[accounts.gateway]
label = "Gateway"
[accounts.gateway.endpoints]
anthropic = "https://gateway.test"

[routes.codex]
label = "Codex"
account = "gateway"
model = "model"
interfaces = { openai_responses = [] }
`), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	cfg, report, err := domain.MergeTeamFile(domain.NewConfig(testAdmission()), manifest, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Unusable) != 1 || report.Unusable[0].Protocol != domain.ProtocolOpenAIResponses {
		t.Errorf("unusable protocols %+v", report.Unusable)
	}
	var missing *domain.RuntimeMissingEndpointError
	if _, err := cfg.ResolveModelProtocol(ClientCodex, "codex", domain.ProtocolOpenAIResponses); !errors.As(err, &missing) || missing.ProviderID != "gateway" {
		t.Fatalf("a selection on the unusable protocol = %v", err)
	}
}

// A recommendation may name a model that only the configuration defines, so
// the merge, not the parser, refuses one that neither defines.
func TestMergeRejectsRecommendedModelReferencingUnknownModel(t *testing.T) {
	raw := []byte(`version = 7
[recommendations.claude.primary]
route = "missing"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://team.test"
[routes.team]
label = "Team"
account = "team"
model = "claude-team"
`)
	manifest, err := Parse(raw, testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := domain.MergeTeamFile(domain.NewConfig(testAdmission()), manifest, false); err == nil || !strings.Contains(err.Error(), `claude recommendation names "missing"`) {
		t.Fatalf("unknown recommended model error = %v", err)
	}
}

// A team file never sets a client's selection, so a [clients] section is
// refused before anything else of it is read.
func TestParseRefusesATeamFileThatSetsClients(t *testing.T) {
	_, err := Parse([]byte("version = 7\n[clients.claude]\nroute = \"team\"\n"), testAdmission())
	if !errors.Is(err, ErrTeamFileClients) {
		t.Fatalf("a [clients] section = %v", err)
	}
}
