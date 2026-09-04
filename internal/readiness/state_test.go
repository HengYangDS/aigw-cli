package readiness

import (
	"testing"

	"aigw-cli/internal/diagnostics"
)

func TestClassifyClientUsesOneLocalStateVocabulary(t *testing.T) {
	tests := []struct {
		name       string
		facts      ClientFacts
		wantState  State
		wantAction string
	}{
		{
			name:       "profile is available but not selected",
			facts:      ClientFacts{SuggestedProfile: "claude"},
			wantState:  Deferred,
			wantAction: "aigw use claude",
		},
		{
			name:       "no compatible profile exists",
			facts:      ClientFacts{},
			wantState:  Deferred,
			wantAction: "aigw profile add",
		},
		{
			name:       "selected account is not connected",
			facts:      ClientFacts{Profile: "claude", Account: "team", CredentialRequired: true},
			wantState:  Deferred,
			wantAction: "aigw rotate team",
		},
		{
			name: "credential metadata is unavailable",
			facts: ClientFacts{
				Profile:                    "claude",
				Account:                    "team",
				CredentialRequired:         true,
				CredentialObservationIssue: "Credential metadata is unavailable",
			},
			wantState:  Unavailable,
			wantAction: CredentialBackendRecovery,
		},
		{
			name: "client is intentionally absent",
			facts: ClientFacts{
				Profile:             "claude",
				Account:             "team",
				CredentialRequired:  true,
				CredentialAvailable: true,
			},
			wantState:  Deferred,
			wantAction: "aigw sync",
		},
		{
			name: "enabled projection is invalid",
			facts: ClientFacts{
				Profile:             "codex",
				Account:             "team",
				CredentialRequired:  true,
				CredentialAvailable: true,
				AdapterEnabled:      true,
				AdapterIssue:        "projection drift",
				AdapterAction:       "aigw sync",
			},
			wantState:  Invalid,
			wantAction: "aigw sync",
		},
		{
			name: "invalid projection defaults to repair",
			facts: ClientFacts{
				Profile:             "claude",
				Account:             "team",
				CredentialRequired:  true,
				CredentialAvailable: true,
				AdapterEnabled:      true,
				AdapterIssue:        "Claude executable is unavailable",
			},
			wantState:  Invalid,
			wantAction: "aigw repair",
		},
		{
			name: "local prerequisites are configured",
			facts: ClientFacts{
				Profile:             "claude",
				Account:             "team",
				CredentialRequired:  true,
				CredentialAvailable: true,
				AdapterEnabled:      true,
				AdapterReady:        true,
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

func TestStateLabelUsesTheCanonicalVocabulary(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{state: Configured, want: "Configured"},
		{state: Deferred, want: "Deferred"},
		{state: Ready, want: "Ready"},
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

func TestClassifyClientIncludesRouteFailures(t *testing.T) {
	got := ClassifyClient(ClientFacts{
		Profile:     "missing",
		RouteIssue:  "unknown profile \"missing\"",
		RouteAction: "aigw use <claude-profile>",
	})
	if got.State != Invalid || got.Detail != `unknown profile "missing"` || got.NextAction != "aigw use <claude-profile>" {
		t.Fatalf("ClassifyClient() = %#v", got)
	}
}

func TestWithProbeMapsDiagnosticSemantics(t *testing.T) {
	configured := Client{State: Configured, Profile: "codex", Account: "team"}
	tests := []struct {
		name string
		kind diagnostics.Kind
		want State
	}{
		{name: "healthy", kind: diagnostics.Healthy, want: Ready},
		{name: "invalid token", kind: diagnostics.InvalidToken, want: Invalid},
		{name: "disabled token", kind: diagnostics.TokenDisabled, want: Invalid},
		{name: "restricted token", kind: diagnostics.TokenRestricted, want: Invalid},
		{name: "endpoint mismatch", kind: diagnostics.EndpointMismatch, want: Invalid},
		{name: "unstable authentication", kind: diagnostics.AuthenticationUnstable, want: Degraded},
		{name: "quota exhausted", kind: diagnostics.QuotaExhausted, want: Degraded},
		{name: "rate limited", kind: diagnostics.RateLimited, want: Degraded},
		{name: "model unavailable", kind: diagnostics.ModelUnavailable, want: Degraded},
		{name: "gateway failure", kind: diagnostics.GatewayFailure, want: Degraded},
		{name: "network failure", kind: diagnostics.NetworkFailure, want: Degraded},
		{name: "unknown result", kind: diagnostics.Unexpected, want: Unavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := WithProbe(configured, diagnostics.Result{
				Kind:    test.kind,
				Summary: "observed result",
				Fix:     "aigw check",
			})
			if got.State != test.want || got.Detail != "observed result" {
				t.Fatalf("WithProbe() = %#v, want state %q", got, test.want)
			}
			if test.want == Ready && got.NextAction != "" {
				t.Fatalf("ready next action = %q", got.NextAction)
			}
		})
	}
}

func TestWithProbeDoesNotOverrideDeferredState(t *testing.T) {
	deferred := Client{State: Deferred, NextAction: "aigw sync"}
	got := WithProbe(deferred, diagnostics.Result{Kind: diagnostics.Healthy})
	if got != deferred {
		t.Fatalf("WithProbe() = %#v, want %#v", got, deferred)
	}
}
