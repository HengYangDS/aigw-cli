package check

import (
	"testing"

	"aigw-cli/internal/domain"
)

// Each depth runs the ones before it, model is the default, and any other
// spelling is refused with the four depths.
func TestDepthsClimbInOrder(t *testing.T) {
	for _, test := range []struct {
		text  string
		want  depth
		runs  []depth
		skips []depth
	}{
		{text: "local", want: depthLocal, runs: []depth{depthLocal}, skips: []depth{depthEndpoint, depthModel, depthClient}},
		{text: "endpoint", want: depthEndpoint, runs: []depth{depthLocal, depthEndpoint}, skips: []depth{depthModel, depthClient}},
		{text: "", want: depthModel, runs: []depth{depthLocal, depthEndpoint, depthModel}, skips: []depth{depthClient}},
		{text: "client", want: depthClient, runs: depths},
	} {
		got, err := parseDepth(test.text)
		if err != nil || got != test.want {
			t.Fatalf("parseDepth(%q) = %q, %v, want %q", test.text, got, err, test.want)
		}
		for _, other := range test.runs {
			if !got.runs(other) {
				t.Fatalf("%s does not run %s", got, other)
			}
		}
		for _, other := range test.skips {
			if got.runs(other) {
				t.Fatalf("%s runs %s", got, other)
			}
		}
	}
	if _, err := parseDepth("inference"); err == nil || err.Error() != `--depth must be local, endpoint, model or client, not "inference"; run `+"`aigw check --help`" {
		t.Fatalf("an unknown depth = %v", err)
	}
}

// The exit rule: a check fails when a checked client failed, and when it
// names no client while none is enabled, and so covers none; a skipped depth
// fails nothing, and a named disabled client counts like an enabled one.
func TestCheckExitRule(t *testing.T) {
	passed := result{spec: domain.ClientSpec{ID: "codex"}, enabled: true, reached: depthModel}
	skipped := result{spec: domain.ClientSpec{ID: "claude"}, enabled: true, reached: depthLocal,
		skipped: []skip{{Depth: depthEndpoint, Reason: "own authentication"}, {Depth: depthModel, Reason: "own authentication"}}}
	disabled := result{spec: domain.ClientSpec{ID: "hermes"}, reached: depthModel}
	failed := result{spec: domain.ClientSpec{ID: "hermes"}, enabled: true, reached: depthLocal, failure: &failure{depth: depthEndpoint, problem: "HTTP 401"}}
	for _, test := range []struct {
		name    string
		outcome outcome
		ok      bool
		enabled int
	}{
		{name: "no client named and none enabled", outcome: outcome{}, ok: false},
		{name: "every client passed", outcome: outcome{clients: []result{passed}}, ok: true, enabled: 1},
		{name: "a client skipped depths", outcome: outcome{clients: []result{passed, skipped}}, ok: true, enabled: 2},
		{name: "a named disabled client passed", outcome: outcome{named: true, clients: []result{disabled}}, ok: true},
		{name: "one client failed", outcome: outcome{clients: []result{passed, failed}}, ok: false, enabled: 2},
		{name: "a named client failed", outcome: outcome{named: true, clients: []result{failed}}, ok: false, enabled: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.outcome.ok(); got != test.ok {
				t.Fatalf("ok() = %t, want %t", got, test.ok)
			}
			if got := test.outcome.enabled(); got != test.enabled {
				t.Fatalf("enabled() = %d, want %d", got, test.enabled)
			}
		})
	}
}
