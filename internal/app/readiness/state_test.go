package readiness

import (
	"testing"

	"aigw-cli/internal/app/diagnostics"
)

func TestClassifyClientUsesOneLocalStateVocabulary(t *testing.T) {
	tests := []struct {
		name       string
		facts      ClientFacts
		wantState  State
		wantAction string
	}{
		{
			name:       "route is available but not selected",
			facts:      ClientFacts{SuggestedModel: "claude", BindingAction: "aigw use claude team/claude"},
			wantState:  Deferred,
			wantAction: "aigw use claude team/claude",
		},
		{name: "nothing is selected, and the caller names the selection", facts: ClientFacts{BindingAction: "aigw use claude <provider/model>"}, wantState: Deferred, wantAction: "aigw use claude <provider/model>"},
		{name: "nothing is selected, and the caller names no step", facts: ClientFacts{}, wantState: Deferred},
		{
			name:       "selected account is not connected",
			facts:      ClientFacts{Model: "claude", Provider: "team", CredentialRequired: true},
			wantState:  Deferred,
			wantAction: "aigw provider rotate team",
		},
		{
			name: "credential metadata is unavailable",
			facts: ClientFacts{
				Model:                      "claude",
				Provider:                   "team",
				CredentialRequired:         true,
				CredentialObservationIssue: "Credential metadata is unavailable",
			},
			wantState:  Unavailable,
			wantAction: CredentialBackendRecovery,
		},
		{
			name: "a disabled client names no step of its own",
			facts: ClientFacts{
				Model:               "claude",
				Provider:            "team",
				CredentialRequired:  true,
				CredentialAvailable: true,
			},
			wantState: Deferred,
		},
		{
			name: "a disabled client leaves its activation step to activation",
			facts: ClientFacts{
				Model:               "codex",
				Provider:            "team",
				CredentialRequired:  true,
				CredentialAvailable: true,
				ProjectionAction:    "aigw sync",
			},
			wantState: Deferred,
		},
		{
			name: "enabled projection is invalid",
			facts: ClientFacts{
				Model:               "codex",
				Provider:            "team",
				CredentialRequired:  true,
				CredentialAvailable: true,
				ProjectionEnabled:   true,
				ProjectionIssue:     "projection drift",
				ProjectionAction:    "aigw sync",
			},
			wantState:  Invalid,
			wantAction: "aigw sync",
		},
		{
			name: "invalid projection defaults to repair",
			facts: ClientFacts{
				Model:               "claude",
				Provider:            "team",
				CredentialRequired:  true,
				CredentialAvailable: true,
				ProjectionEnabled:   true,
				ProjectionIssue:     "Claude executable is unavailable",
			},
			wantState:  Invalid,
			wantAction: "aigw sync",
		},
		{
			name: "local prerequisites are configured",
			facts: ClientFacts{
				Model:               "claude",
				Provider:            "team",
				CredentialRequired:  true,
				CredentialAvailable: true,
				ProjectionEnabled:   true,
				ProjectionReady:     true,
			},
			wantState: Configured,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ClassifyClient(test.facts)
			if got.State != test.wantState || got.NextAction != test.wantAction {
				t.Fatalf("ClassifyClient() = %#v, want state %q and action %q", got, test.wantState, test.wantAction)
			}
		})
	}
}

// An enabled client whose files AIGW has not written names installing it,
// unless discovery finds it installed: then a sync writes them.
func TestClassifyClientTellsAnInstalledClientFromOneThatIsNot(t *testing.T) {
	facts := ClientFacts{
		Model: "hermes-route", Provider: "team", ProjectionEnabled: true, ProjectionDeferred: true,
		ProjectionAction: "Install Hermes if needed, then run `aigw sync`",
	}
	if got := ClassifyClient(facts); got.State != Deferred || got.Detail != "The client is not installed" || got.NextAction != facts.ProjectionAction {
		t.Fatalf("a client that is not installed = %#v", got)
	}
	facts.ClientInstalled = true
	if got := ClassifyClient(facts); got.State != Deferred || got.Detail != "The client is installed, but AIGW has not written its files yet" || got.NextAction != "aigw sync" {
		t.Fatalf("an installed client = %#v", got)
	}
}

func TestStateLabelUsesTheCanonicalVocabulary(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{state: Configured, want: "Configured"},
		{state: Deferred, want: "Deferred"},
		{state: Degraded, want: "Degraded"},
		{state: Invalid, want: "Invalid"},
		{state: Unavailable, want: "Unavailable"},
		{state: State("future"), want: "Unavailable"},
	}
	for _, test := range tests {
		if got := test.state.Label(); got != test.want {
			t.Fatalf("State(%q).Label() = %q, want %q", test.state, got, test.want)
		}
	}
}

func TestClassifyClientIncludesModelFailures(t *testing.T) {
	got := ClassifyClient(ClientFacts{
		Model:         "missing",
		BindingIssue:  "unknown route \"missing\"",
		BindingAction: "aigw use <claude-route>",
	})
	if got.State != Invalid || got.Detail != `unknown route "missing"` || got.NextAction != "aigw use <claude-route>" {
		t.Fatalf("ClassifyClient() = %#v", got)
	}
}

// TestClassifyClientPutsTheTokenBeforeTheClientFiles names the Token's step
// before the files': without the Token a sync writes no file of the client,
// and with a credential store that it cannot observe a sync fails the same
// way. With the Token, the files' own step comes first.
func TestClassifyClientPutsTheTokenBeforeTheClientFiles(t *testing.T) {
	drifted := ClientFacts{
		Model: "codex", Provider: "team", CredentialRequired: true,
		ProjectionEnabled: true, ProjectionIssue: "projection drift", ProjectionAction: "aigw sync",
	}
	missing := ClassifyClient(drifted)
	if missing.State != Deferred || missing.Detail != "The selected provider has no Token" || missing.NextAction != "aigw provider rotate team" {
		t.Fatalf("a missing Token lost to the files: %+v", missing)
	}
	unobservable := drifted
	unobservable.CredentialObservationIssue = "Credential metadata is unavailable"
	if got := ClassifyClient(unobservable); got.State != Unavailable || got.NextAction != CredentialBackendRecovery {
		t.Fatalf("an unobservable credential store lost to the files: %+v", got)
	}
	present := drifted
	present.CredentialAvailable = true
	if got := ClassifyClient(present); got.State != Invalid || got.Detail != "projection drift" || got.NextAction != "aigw sync" {
		t.Fatalf("the files' step lost with the Token present: %+v", got)
	}
}

// A failed request's kind decides the state of the configured client that
// sent it, and a healthy one leaves it configured: the depth that check
// reached says what the request proved.
func TestProbeStateMapsDiagnosticSemantics(t *testing.T) {
	for _, test := range []struct {
		kind diagnostics.Kind
		want State
	}{
		{kind: diagnostics.Healthy, want: Configured},
		{kind: diagnostics.InvalidToken, want: Invalid},
		{kind: diagnostics.TokenDisabled, want: Invalid},
		{kind: diagnostics.TokenRestricted, want: Invalid},
		{kind: diagnostics.EndpointMismatch, want: Invalid},
		{kind: diagnostics.ModelUnresolved, want: Invalid},
		{kind: diagnostics.QuotaExhausted, want: Degraded},
		{kind: diagnostics.RateLimited, want: Degraded},
		{kind: diagnostics.Timeout, want: Degraded},
		{kind: diagnostics.ModelUnavailable, want: Degraded},
		{kind: diagnostics.UpstreamFailure, want: Degraded},
		{kind: diagnostics.NetworkFailure, want: Degraded},
		{kind: diagnostics.Unexpected, want: Unavailable},
		{kind: diagnostics.Kind("future"), want: Unavailable},
	} {
		if got := ProbeState(test.kind); got != test.want {
			t.Fatalf("ProbeState(%q) = %q, want %q", test.kind, got, test.want)
		}
	}
}
