package adapter

import (
	"errors"
	"fmt"
	"testing"

	"aigw-cli/internal/domain"
)

func TestARefusedSelectionRestoresTheRecordedSelectionAsItsNextAction(t *testing.T) {
	for _, test := range []struct {
		name            string
		recorded        RecordedSelection
		what, adds      string
		message, action string
	}{
		{
			name:     "a model that the provider has not added",
			recorded: RecordedSelection{Client: "codex", Product: "Codex", Model: domain.NewModelRef("team", "model-a")},
			what:     `selects model "ghost", which provider "team" has not added`, adds: "aigw use codex team/ghost",
			message: "Codex selects model \"ghost\", which provider \"team\" has not added; AIGW's selection for Codex is team/model-a; `aigw use codex team/ghost` adds and selects the model that Codex names",
			action:  "aigw use codex team/model-a",
		},
		{
			name:     "a protocol in the restoring command, and no adding command",
			recorded: RecordedSelection{Client: "hermes", Product: "Hermes", Model: domain.NewModelRef("team", "model-a"), Protocol: "responses"},
			what:     `selects provider "foreign", which AIGW did not write`,
			message:  "Hermes selects provider \"foreign\", which AIGW did not write; AIGW's selection for Hermes is team/model-a",
			action:   "aigw use hermes team/model-a --protocol responses",
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
