// Package readiness owns the canonical operational state vocabulary shared by
// AIGW's setup, synchronization, inspection, and verification surfaces.
package readiness

import "aigw-cli/internal/diagnostics"

// State is the stable classification of one operational capability.
type State string

const (
	Configured  State = "configured"
	Deferred    State = "deferred"
	Ready       State = "ready"
	Degraded    State = "degraded"
	Invalid     State = "invalid"
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
	case Ready:
		return "Ready"
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
	State      State  `json:"state"`
	Profile    string `json:"profile,omitempty"`
	Account    string `json:"account,omitempty"`
	Detail     string `json:"detail,omitempty"`
	NextAction string `json:"next_action,omitempty"`
}

// ClientFacts are the local observations that determine one client's state.
type ClientFacts struct {
	Profile                    string
	Account                    string
	RouteIssue                 string
	RouteAction                string
	CredentialObservationIssue string
	CredentialRequired         bool
	CredentialAvailable        bool
	CredentialAction           string
	AdapterEnabled             bool
	AdapterReady               bool
	AdapterIssue               string
	AdapterAction              string
	SuggestedProfile           string
}

// ClassifyClient classifies local readiness facts without performing probes or
// reading credential values.
func ClassifyClient(facts ClientFacts) Client {
	state := Client{Profile: facts.Profile, Account: facts.Account}
	switch {
	case facts.RouteIssue != "":
		state.State = Invalid
		state.Detail = facts.RouteIssue
		state.NextAction = facts.RouteAction
	case facts.Profile == "":
		state.State = Deferred
		state.Detail = "No profile is selected for this client"
		if facts.SuggestedProfile != "" {
			state.NextAction = "aigw use " + facts.SuggestedProfile
		} else {
			state.NextAction = "aigw profile add"
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
	case !facts.AdapterEnabled:
		state.State = Deferred
		state.Detail = "The client is not installed or enabled"
		state.NextAction = "aigw sync"
	case !facts.AdapterReady:
		state.State = Invalid
		state.Detail = facts.AdapterIssue
		state.NextAction = facts.AdapterAction
		if state.NextAction == "" {
			state.NextAction = "aigw repair"
		}
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
		state.State = Ready
		state.NextAction = ""
	case diagnostics.InvalidToken, diagnostics.TokenDisabled,
		diagnostics.TokenRestricted, diagnostics.EndpointMismatch:
		state.State = Invalid
	case diagnostics.AuthenticationUnstable, diagnostics.QuotaExhausted,
		diagnostics.RateLimited, diagnostics.ModelUnavailable,
		diagnostics.GatewayFailure, diagnostics.NetworkFailure:
		state.State = Degraded
	default:
		state.State = Unavailable
	}
	return state
}
