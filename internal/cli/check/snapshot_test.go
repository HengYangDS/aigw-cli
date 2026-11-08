package check

import (
	"testing"

	"aigw-cli/internal/domain"
)

// A check writes the verified snapshot only to the client depth, without
// named clients, when no client failed and every client that can run a
// session ran one; Claude Desktop, which needs acceptance inside the app,
// counts at the depth that it reached.
func TestCheckWritesTheSnapshotOnlyForACompleteCheck(t *testing.T) {
	session := result{spec: domain.ClientSpec{ID: "codex"}, enabled: true, reached: depthClient}
	desktop := result{spec: domain.ClientSpec{ID: "claude-desktop", InAppAcceptance: true}, enabled: true, reached: depthModel,
		skipped: []skip{{Depth: depthClient, Reason: "Claude Desktop needs acceptance inside the app"}}}
	stopped := result{spec: domain.ClientSpec{ID: "hermes"}, enabled: true, reached: depthModel, failure: &failure{depth: depthClient}}
	short := result{spec: domain.ClientSpec{ID: "claude"}, enabled: true, reached: depthModel}
	for _, test := range []struct {
		name    string
		outcome outcome
		writes  bool
	}{
		{name: "sessions and Claude Desktop", outcome: outcome{requested: depthClient, clients: []result{session, desktop}}, writes: true},
		{name: "the model depth", outcome: outcome{requested: depthModel, clients: []result{session}}},
		{name: "named clients", outcome: outcome{requested: depthClient, named: true, clients: []result{session}}},
		{name: "no client", outcome: outcome{requested: depthClient}},
		{name: "a client that stopped", outcome: outcome{requested: depthClient, clients: []result{session, stopped}}},
		{name: "a client that ran no session", outcome: outcome{requested: depthClient, clients: []result{session, short}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.outcome.writesSnapshot(); got != test.writes {
				t.Fatalf("writesSnapshot() = %t, want %t", got, test.writes)
			}
		})
	}
	refused := outcome{requested: depthClient, clients: []result{session}, snapshot: &snapshot{problem: "The verified snapshot was not written", fix: "aigw check --depth client"}}
	if refused.ok() || refused.next() != "aigw check --depth client" || refused.problem() != "The verified snapshot was not written" {
		t.Fatalf("a refused snapshot = ok %t, next %q, problem %q", refused.ok(), refused.next(), refused.problem())
	}
}
