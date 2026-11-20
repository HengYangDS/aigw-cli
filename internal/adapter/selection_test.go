package adapter

import (
	"errors"
	"fmt"
	"testing"

	"aigw-cli/internal/domain"
)

func TestARefusedSelectionRestoresTheRecordedSelectionAsItsNextAction(t *testing.T) {
	codex := domain.ClientSpec{ID: "codex", Label: "Codex", EndpointProtocols: []domain.EndpointProtocol{domain.ProtocolResponses}}
	hermes := domain.ClientSpec{ID: "hermes", Label: "Hermes", EndpointProtocols: []domain.EndpointProtocol{domain.ProtocolAnthropic, domain.ProtocolResponses}}
	runtime := func(provider, model string, protocol domain.EndpointProtocol) domain.Runtime {
		return domain.Runtime{Model: domain.NewModelRef(provider, model), Protocol: protocol}
	}
	for _, test := range []struct {
		name            string
		recorded        RecordedSelection
		what, adds      string
		message, action string
	}{
		{
			name:     "a client with one protocol names none",
			recorded: Recorded(codex, runtime("team", "model-a", domain.ProtocolResponses)),
			what:     `selects model "ghost", which provider "team" has not added on OpenAI Responses`,
			adds:     SelectCommand("codex", domain.NewModelRef("team", "ghost"), ""),
			message:  "Codex selects model \"ghost\", which provider \"team\" has not added on OpenAI Responses; AIGW's selection for Codex is team/model-a; `aigw use codex team/ghost` adds and selects the model that Codex names",
			action:   "aigw use codex team/model-a",
		},
		{
			name:     "a client with several protocols names its own, and no adding command",
			recorded: Recorded(hermes, runtime("team", "model-a", domain.ProtocolResponses)),
			what:     `selects provider "foreign", which AIGW does not write`,
			message:  "Hermes selects provider \"foreign\", which AIGW does not write; AIGW's selection for Hermes is team/model-a",
			action:   "aigw use hermes team/model-a --protocol responses",
		},
		{
			name:     "a model that a shell would not pass through is restored in prose and adds no command",
			recorded: Recorded(codex, runtime("team", "sol [1m]", domain.ProtocolResponses)),
			what:     `selects model "my model", which provider "team" has not added on OpenAI Responses`,
			adds:     SelectCommand("codex", domain.NewModelRef("team", "my model"), ""),
			message:  "Codex selects model \"my model\", which provider \"team\" has not added on OpenAI Responses; AIGW's selection for Codex is team/sol [1m]",
			action:   "Select team/sol [1m] for Codex again with `aigw use`, quoting the model for your shell.",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			refusal := RefusedSelection(test.recorded, test.what, test.adds)
			if refusal.Error() != test.message || refusal.NextAction() != test.action {
				t.Fatalf("refusal = %q, next %q; want %q, next %q", refusal.Error(), refusal.NextAction(), test.message, test.action)
			}
			wrapped := fmt.Errorf("sync synchronization preflight failed: %w", refusal)
			if found, ok := errors.AsType[*SelectionError](wrapped); !ok || found.NextAction() != test.action {
				t.Fatalf("a wrapped refusal lost its next action: %v", wrapped)
			}
		})
	}
}
