// Package readiness owns the canonical operational state vocabulary shared by
// AIGW's setup, synchronization, inspection, and verification surfaces.
package readiness

import "aigw-cli/internal/app/diagnostics"

// State is the stable classification of one operational capability.
type State string

// The depth that `aigw check` reaches, not a state, says how much a request
// or a client session proved about a configured client.
const (
	// Configured identifies a client whose local prerequisites pass, whatever
	// a check proved beyond them.
	Configured State = "configured"
	// Deferred identifies an intentionally postponed credential or client projection.
	Deferred State = "deferred"
	// Degraded identifies a client that remains configured but has a recoverable operational problem.
	Degraded State = "degraded"
	// Invalid identifies configuration or owned state that cannot be safely used.
	Invalid State = "invalid"
	// Unavailable identifies a required external capability that is not present.
	Unavailable State = "unavailable"
)

// CredentialBackendRecovery is the safe public action when credential
// metadata cannot be observed without reading a secret value.
const CredentialBackendRecovery = "aigw check --depth local"

// Label returns the stable human projection of a readiness state.
func (state State) Label() string {
	switch state {
	case Configured:
		return "Configured"
	case Deferred:
		return "Deferred"
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
	Model      string `json:"route,omitempty"`
	Provider   string `json:"account,omitempty"`
	Detail     string `json:"detail,omitempty"`
	NextAction string `json:"next_action,omitempty"`
	// NextDetail is a next step that a script cannot run as it stands, which
	// a document writes instead of NextAction.
	NextDetail          string `json:"next_detail,omitempty"`
	ProjectionDeferred  bool   `json:"projection_deferred,omitempty"`
	NativeModelOverride bool   `json:"native_model_override,omitempty"`
}

// ClientFacts are the local observations that determine one client's state.
type ClientFacts struct {
	Model                      string
	Provider                   string
	BindingIssue               string
	BindingAction              string
	CredentialObservationIssue string
	CredentialRequired         bool
	CredentialAvailable        bool
	CredentialAction           string
	ProjectionEnabled          bool
	ProjectionDeferred         bool
	ProjectionReady            bool
	ProjectionIssue            string
	ProjectionAction           string
	SuggestedModel             string
	// ClientInstalled marks a client whose files wait while discovery finds
	// its executable, so that a sync writes them.
	ClientInstalled bool
}

// ClassifyClient classifies local readiness facts without performing probes or
// reading credential values.
func ClassifyClient(facts ClientFacts) Client {
	state := Client{Model: facts.Model, Provider: facts.Provider, ProjectionDeferred: facts.ProjectionDeferred}
	switch {
	case facts.BindingIssue != "":
		state.State = Invalid
		state.Detail = facts.BindingIssue
		state.NextAction = facts.BindingAction
	case facts.Model == "":
		state.State = Deferred
		state.Detail = "No model is selected"
		state.NextAction = facts.BindingAction
	// The Token comes before the client's files: without it, a sync writes
	// no file of the client, and with a credential store that it cannot
	// observe, a sync fails the same way, so neither fixes the files.
	case facts.CredentialRequired && facts.CredentialObservationIssue != "":
		state.State = Unavailable
		state.Detail = facts.CredentialObservationIssue
		state.NextAction = CredentialBackendRecovery
	case facts.CredentialRequired && !facts.CredentialAvailable:
		state.State = Deferred
		state.Detail = "The selected provider has no Token"
		state.NextAction = facts.CredentialAction
		if state.NextAction == "" {
			state.NextAction = "aigw provider rotate " + facts.Provider
		}
	case facts.ProjectionEnabled && !facts.ProjectionReady && !facts.ProjectionDeferred:
		state.State = Invalid
		state.Detail = facts.ProjectionIssue
		state.NextAction = facts.ProjectionAction
		if state.NextAction == "" {
			state.NextAction = "aigw repair"
		}
	case facts.ProjectionDeferred && facts.ClientInstalled:
		state.State = Deferred
		state.Detail = "The client is installed, but AIGW has not written its files yet"
		state.NextAction = "aigw sync"
	case facts.ProjectionDeferred:
		state.State = Deferred
		state.Detail = "The client is not installed"
		state.NextAction = facts.ProjectionAction
		if state.NextAction == "" {
			state.NextAction = "aigw sync"
		}
	case !facts.ProjectionEnabled:
		// A disabled client names no step of its own, so it cannot change
		// the step that the enabled clients need: sync skips it, and only
		// an explicit selection turns it on again, which activation names
		// when no client is enabled.
		state.State = Deferred
		state.Detail = "The client is not installed or enabled"
	default:
		state.State = Configured
	}
	return state
}

// ProbeState is the state of a configured client whose authenticated
// request failed as kind says. The diagnostic kind, rather than transport
// heuristics, owns the distinction between invalid configuration and
// transient degradation.
func ProbeState(kind diagnostics.Kind) State {
	switch kind {
	case diagnostics.Healthy:
		return Configured
	case diagnostics.InvalidToken, diagnostics.TokenDisabled,
		diagnostics.TokenRestricted, diagnostics.EndpointMismatch,
		diagnostics.ModelUnresolved:
		return Invalid
	case diagnostics.QuotaExhausted,
		diagnostics.RateLimited, diagnostics.Timeout, diagnostics.ModelUnavailable,
		diagnostics.UpstreamFailure, diagnostics.NetworkFailure:
		return Degraded
	case diagnostics.Unexpected:
		return Unavailable
	default:
		return Unavailable
	}
}
