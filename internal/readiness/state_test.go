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
			name:       "route is available but not selected",
			facts:      ClientFacts{SuggestedRoute: "claude", BindingAction: "aigw use --for claude claude"},
			wantState:  Deferred,
			wantAction: "aigw use --for claude claude",
		},
		{
			name:       "no compatible route exists",
			facts:      ClientFacts{},
			wantState:  Deferred,
			wantAction: "aigw route add",
		},
		{
			name:       "selected account is not connected",
			facts:      ClientFacts{Route: "claude", Account: "team", CredentialRequired: true},
			wantState:  Deferred,
			wantAction: "aigw rotate team",
		},
		{
			name: "credential metadata is unavailable",
			facts: ClientFacts{
				Route:                      "claude",
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
				Route:               "claude",
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
				Route:               "codex",
				Account:             "team",
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
				Route:               "claude",
				Account:             "team",
				CredentialRequired:  true,
				CredentialAvailable: true,
				ProjectionEnabled:   true,
				ProjectionIssue:     "Claude executable is unavailable",
			},
			wantState:  Invalid,
			wantAction: "aigw repair",
		},
		{
			name: "local prerequisites are configured",
			facts: ClientFacts{
				Route:               "claude",
				Account:             "team",
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

func TestStateLabelUsesTheCanonicalVocabulary(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{state: Configured, want: "Configured"},
		{state: Deferred, want: "Deferred"},
		{state: EndpointChecked, want: "Endpoint checked"},
		{state: InferenceChecked, want: "Inference checked"},
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
		Route:         "missing",
		BindingIssue:  "unknown route \"missing\"",
		BindingAction: "aigw use <claude-route>",
	})
	if got.State != Invalid || got.Detail != `unknown route "missing"` || got.NextAction != "aigw use <claude-route>" {
		t.Fatalf("ClassifyClient() = %#v", got)
	}
}

func TestClassifyClientPrioritizesObservedProjectionFailure(t *testing.T) {
	for _, metadataIssue := range []string{"", "Credential metadata is unavailable"} {
		got := ClassifyClient(ClientFacts{
			Route: "codex", Account: "team", CredentialRequired: true,
			CredentialObservationIssue: metadataIssue,
			ProjectionEnabled:          true, ProjectionIssue: "projection drift", ProjectionAction: "aigw sync",
		})
		if got.State != Invalid || got.Detail != "projection drift" || got.NextAction != "aigw sync" {
			t.Fatalf("projection failure lost to credential state: %+v", got)
		}
	}
}

func TestWithProbeMapsDiagnosticSemantics(t *testing.T) {
	configured := Client{State: Configured, Route: "codex", Account: "team"}
	tests := []struct {
		name string
		kind diagnostics.Kind
		want State
	}{
		{name: "healthy", kind: diagnostics.Healthy, want: EndpointChecked},
		{name: "invalid token", kind: diagnostics.InvalidToken, want: Invalid},
		{name: "disabled token", kind: diagnostics.TokenDisabled, want: Invalid},
		{name: "restricted token", kind: diagnostics.TokenRestricted, want: Invalid},
		{name: "endpoint mismatch", kind: diagnostics.EndpointMismatch, want: Invalid},
		{name: "unstable authentication", kind: diagnostics.AuthenticationUnstable, want: Degraded},
		{name: "quota exhausted", kind: diagnostics.QuotaExhausted, want: Degraded},
		{name: "rate limited", kind: diagnostics.RateLimited, want: Degraded},
		{name: "model unavailable", kind: diagnostics.ModelUnavailable, want: Degraded},
		{name: "model unresolved", kind: diagnostics.ModelUnresolved, want: Invalid},
		{name: "upstream failure", kind: diagnostics.UpstreamFailure, want: Degraded},
		{name: "network failure", kind: diagnostics.NetworkFailure, want: Degraded},
		{name: "unknown result", kind: diagnostics.Unexpected, want: Unavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := WithProbe(configured, diagnostics.Result{
				Kind:    test.kind,
				Scope:   diagnostics.ScopeEndpoint,
				Summary: "observed result",
				Fix:     "aigw check",
			})
			if got.State != test.want || got.Detail != "observed result" {
				t.Fatalf("WithProbe() = %#v, want state %q", got, test.want)
			}
			if test.want == EndpointChecked && got.NextAction != "" {
				t.Fatalf("endpoint-check next action = %q", got.NextAction)
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

func TestWithProbeDistinguishesSuccessfulInferenceFromEndpointEvidence(t *testing.T) {
	configured := Client{State: Configured, Route: "codex", Account: "team"}

	inference := WithProbe(configured, diagnostics.Result{
		Kind: diagnostics.Healthy, Scope: diagnostics.ScopeInference, Summary: "Inference diagnostic returned a successful response",
	})
	if inference.State != InferenceChecked || inference.NextAction != "" {
		t.Fatalf("inference result = %#v", inference)
	}

	endpoint := WithProbe(configured, diagnostics.Result{
		Kind: diagnostics.Healthy, Scope: diagnostics.ScopeEndpoint, Summary: "Endpoint diagnostic returned a successful response",
	})
	if endpoint.State != EndpointChecked || endpoint.NextAction != "" {
		t.Fatalf("endpoint result = %#v", endpoint)
	}

	unscoped := WithProbe(configured, diagnostics.Result{Kind: diagnostics.Healthy, Summary: "ambiguous success"})
	if unscoped.State != Unavailable {
		t.Fatalf("unscoped success = %#v, want unavailable", unscoped)
	}
}
