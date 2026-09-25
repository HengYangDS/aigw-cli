// Package readiness owns the canonical operational state vocabulary shared by
// AIGW's setup, synchronization, inspection, and verification surfaces.
package readiness

import "aigw-cli/internal/diagnostics"

// State is the stable classification of one operational capability.
type State string

const (
	// Configured identifies a valid route whose local prerequisites pass without an endpoint observation.
	Configured State = "configured"
	// Deferred identifies an intentionally postponed credential or client projection.
	Deferred State = "deferred"
	// EndpointChecked identifies a configured route with a successful endpoint diagnostic.
	// It does not prove model inference or real-client execution.
	EndpointChecked State = "endpoint_checked"
	// InferenceChecked identifies a successful request carrying the Route's exact wire model.
	// It does not prove native-client behavior or future availability.
	InferenceChecked State = "inference_checked"
	// Degraded identifies a route that remains configured but has a recoverable operational problem.
	Degraded State = "degraded"
	// Invalid identifies configuration or owned state that cannot be safely used.
	Invalid State = "invalid"
	// Unavailable identifies a required external capability that is not present.
	Unavailable State = "unavailable"
)

// CredentialBackendRecovery is the safe public action when credential
// metadata cannot be observed without reading a secret value.
const CredentialBackendRecovery = "aigw doctor"

// Label returns the stable human projection of a readiness state.
func (state State) Label() string {
	switch state {
	case Configured:
		return "Configured"
	case Deferred:
		return "Deferred"
	case EndpointChecked:
		return "Endpoint checked"
	case InferenceChecked:
		return "Inference checked"
	case Degraded:
		return "Degraded"
	case Invalid:
		return "Invalid"
	case Unavailable:
		return "Unavailable"
	default:
		return "Unavailable"
	}
}

// Client is the canonical, secret-free readiness projection for one admitted
// client. Detail explains the observation; NextAction is empty only when no
// operator action is required.
type Client struct {
	State               State  `json:"state"`
	Route               string `json:"route,omitempty"`
	Account             string `json:"account,omitempty"`
	Detail              string `json:"detail,omitempty"`
	NextAction          string `json:"next_action,omitempty"`
	NativeModelOverride bool   `json:"native_model_override,omitempty"`
}

// ClientFacts are the local observations that determine one client's state.
type ClientFacts struct {
	Route                      string
	Account                    string
	BindingIssue               string
	BindingAction              string
	CredentialObservationIssue string
	CredentialRequired         bool
	CredentialAvailable        bool
	CredentialAction           string
	ProjectionEnabled          bool
	ProjectionReady            bool
	ProjectionIssue            string
	ProjectionAction           string
	SuggestedRoute             string
}

// ClassifyClient classifies local readiness facts without performing probes or
// reading credential values.
func ClassifyClient(facts ClientFacts) Client {
	state := Client{Route: facts.Route, Account: facts.Account}
	switch {
	case facts.BindingIssue != "":
		state.State = Invalid
		state.Detail = facts.BindingIssue
		state.NextAction = facts.BindingAction
	case facts.Route == "":
		state.State = Deferred
		state.Detail = "No route is selected for this client"
		if facts.BindingAction != "" {
			state.NextAction = facts.BindingAction
		} else {
			state.NextAction = "aigw route add"
		}
	case facts.ProjectionEnabled && !facts.ProjectionReady:
		state.State = Invalid
		state.Detail = facts.ProjectionIssue
		state.NextAction = facts.ProjectionAction
		if state.NextAction == "" {
			state.NextAction = "aigw repair"
		}
	case facts.CredentialRequired && facts.CredentialObservationIssue != "":
		state.State = Unavailable
		state.Detail = facts.CredentialObservationIssue
		state.NextAction = CredentialBackendRecovery
	case facts.CredentialRequired && !facts.CredentialAvailable:
		state.State = Deferred
		state.Detail = "The selected Account has no available Token"
		state.NextAction = facts.CredentialAction
		if state.NextAction == "" {
			state.NextAction = "aigw rotate " + facts.Account
		}
	case !facts.ProjectionEnabled:
		state.State = Deferred
		state.Detail = "The client is not installed or enabled"
		state.NextAction = "aigw sync"
	default:
		state.State = Configured
	}
	return state
}

// WithProbe refines a configured client with a typed authenticated endpoint
// observation. The diagnostic kind, rather than transport heuristics, owns the
// distinction between invalid configuration and transient degradation.
func WithProbe(state Client, result diagnostics.Result) Client {
	if state.State != Configured {
		return state
	}
	state.Detail = result.Summary
	state.NextAction = result.Fix
	switch result.Kind {
	case diagnostics.Healthy:
		switch result.Scope {
		case diagnostics.ScopeEndpoint:
			state.State = EndpointChecked
			state.NextAction = ""
		case diagnostics.ScopeInference:
			state.State = InferenceChecked
			state.NextAction = ""
		default:
			state.State = Unavailable
			state.Detail = "Diagnostic scope is unavailable"
			state.NextAction = "aigw doctor"
		}
	case diagnostics.InvalidToken, diagnostics.TokenDisabled,
		diagnostics.TokenRestricted, diagnostics.EndpointMismatch,
		diagnostics.ModelUnresolved:
		state.State = Invalid
	case diagnostics.QuotaExhausted,
		diagnostics.RateLimited, diagnostics.ModelUnavailable,
		diagnostics.UpstreamFailure, diagnostics.NetworkFailure:
		state.State = Degraded
	case diagnostics.Unexpected:
		state.State = Unavailable
	default:
		state.State = Unavailable
	}
	return state
}
