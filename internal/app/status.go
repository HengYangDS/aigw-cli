package app

import (
	clientactivation "aigw-cli/internal/activation"
	"aigw-cli/internal/diagnostics"
	"aigw-cli/internal/domain"
	domainreadiness "aigw-cli/internal/readiness"
	"aigw-cli/internal/secret"
	"context"
	"net/url"
	"strings"
)

// ClientStatus is one client's readiness as status and check report it.
type ClientStatus struct {
	domainreadiness.Client
	Authentication     domain.Authentication `json:"authentication"`
	EndpointConfigured bool                  `json:"endpoint_configured"`
	Transport          EndpointTransportKind `json:"transport,omitempty"`
	ProjectionReady    bool                  `json:"projection_ready"`
	QualifiedModes     []string              `json:"qualified_modes,omitempty"`
	QualifiedPlatforms []string              `json:"qualified_platforms,omitempty"`
	CheckPassed        *bool                 `json:"check_passed,omitempty"`
	DiagnosticKind     string                `json:"diagnostic_kind,omitempty"`
	DiagnosticScope    diagnostics.Scope     `json:"diagnostic_scope,omitempty"`
	Attempts           int                   `json:"attempts,omitempty"`
	Retryable          bool                  `json:"retryable,omitempty"`
}

// EndpointTransportKind names how a client reaches its endpoint.
type EndpointTransportKind string

// EndpointTransportExternalLoopback marks an endpoint on the loopback
// interface, whose runtime AIGW does not manage.
const EndpointTransportExternalLoopback EndpointTransportKind = "external_loopback"

// StatusReport is the local readiness of every admitted client.
type StatusReport struct {
	CredentialBackend secret.BackendSelection `json:"credential_backend"`
	Clients           map[string]ClientStatus `json:"clients"`
	Routes            int                     `json:"routes"`
	EnabledClients    int                     `json:"enabled_clients"`
	State             domainreadiness.State   `json:"state,omitempty"`
	NextAction        string                  `json:"next_action,omitempty"`
}

// InspectStatusClients observes every admitted client without authenticating
// an endpoint or reading Token values.
func InspectStatusClients(runtime Dependencies, cfg domain.Config, activation *clientactivation.Activation, selected string) map[string]ClientStatus {
	if activation == nil {
		assessed := clientactivation.AssessActivation(cfg, runtime.Secrets)
		activation = &assessed
	}
	clientIDs := runtime.Synchronizer().ClientIDs()
	clients := make(map[string]ClientStatus, len(clientIDs))
	for _, clientID := range clientIDs {
		if selected != "" && clientID != selected {
			continue
		}
		spec, _ := runtime.Clients.Admission().Spec(clientID)
		clientRuntime, resolveErr := cfg.ResolveRuntime(clientID, "")
		if resolveErr != nil {
			client := unresolvedClientStatus(&cfg, clientID, resolveErr)
			client.QualifiedModes = spec.QualifiedModes
			client.QualifiedPlatforms = spec.QualifiedPlatforms
			clients[clientID] = client
			continue
		}
		adapterStatus := runtime.InspectClient(context.Background(), cfg, clientID, clientRuntime)
		projectionPrerequisite := activation.ProjectionPrerequisites[clientID]
		projectionAction := adapterStatus.RepairAction
		if projectionPrerequisite != "" {
			projectionAction = projectionPrerequisite
		}
		facts := domainreadiness.ClientFacts{
			Route:              clientRuntime.RouteID,
			Account:            clientRuntime.AccountID,
			CredentialRequired: clientRuntime.UsesAIGWCredentialStore(),
			ProjectionEnabled:  cfg.Clients[clientID].Enabled,
			ProjectionDeferred: cfg.Clients[clientID].Enabled && projectionPrerequisite != "",
			ProjectionReady:    adapterStatus.Ready,
			ProjectionIssue:    adapterStatus.Issue,
			ProjectionAction:   projectionAction,
		}
		applyCredentialFacts(&facts, activation, runtime.Secrets, clientID, clientRuntime.AccountID)
		state := domainreadiness.ClassifyClient(facts)
		if adapterStatus.NativeModelOverride && state.State == domainreadiness.Configured {
			state.NativeModelOverride = true
			state.Detail = runtime.Clients.Admission().ProductName(clientID) + " uses a native model preference; its AIGW endpoint and credential helper remain configured"
			state.NextAction = "aigw verify --for " + clientID
		}
		client := ClientStatus{
			Client:             state,
			Authentication:     clientRuntime.Authentication,
			EndpointConfigured: strings.TrimSpace(clientRuntime.Endpoint) != "",
			Transport:          EndpointTransport(clientRuntime.Endpoint),
			ProjectionReady:    adapterStatus.Ready,
			QualifiedModes:     spec.QualifiedModes,
			QualifiedPlatforms: spec.QualifiedPlatforms,
		}
		if adapterStatus.Ready && !clientRuntime.UsesAIGWCredentialStore() {
			client.State = domainreadiness.Configured
			client.Detail = "Projection ready; client-owned authentication is not proven"
			if clientRuntime.CredentialCommand != "" {
				client.Detail = "Projection ready; external credential helper is not verified"
			}
			client.NextAction = "aigw verify --for " + clientID
		}
		clients[clientID] = client
	}
	return clients
}

func applyCredentialFacts(facts *domainreadiness.ClientFacts, activation *clientactivation.Activation, store secret.Store, clientID, accountID string) {
	if !facts.CredentialRequired {
		return
	}
	available, observationErr, observed := activation.CredentialAvailability(accountID)
	if !observed {
		if !facts.ProjectionEnabled {
			// Activation owns which Accounts a read-only command reads, so a
			// disabled binding outside its scope is classified without them.
			facts.CredentialRequired = false
			return
		}
		available, observationErr = store.Exists(accountID)
	}
	facts.CredentialAvailable = available
	if observationErr != nil {
		facts.CredentialObservationIssue = "Credential metadata is unavailable"
		return
	}
	if available {
		return
	}
	facts.CredentialAction = activation.ClientCredentialPrerequisites[clientID]
	if facts.CredentialAction == "" {
		facts.CredentialAction, _ = secret.TokenRecovery(store, accountID)
	}
}

func unresolvedClientStatus(cfg *domain.Config, clientID string, resolveErr error) ClientStatus {
	facts := domainreadiness.ClientFacts{}
	if route := cfg.SelectedRoute(clientID); route != "" {
		facts.Route = route
		facts.BindingIssue = resolveErr.Error()
		facts.BindingAction = "aigw use --for " + clientID + " <route>"
	} else if suggested := cfg.RecommendedRoute(clientID); suggested != "" {
		facts.SuggestedRoute = suggested
		facts.BindingAction = "aigw use --for " + clientID + " " + suggested
	}
	state := domainreadiness.ClassifyClient(facts)
	if facts.Route == "" && facts.SuggestedRoute == "" {
		state.NextAction = ""
	}
	return ClientStatus{Client: state}
}

// InspectClients returns the canonical, secret-free local state of every
// admitted client without authenticating an endpoint or reading Token values.
func InspectClients(runtime Dependencies, cfg domain.Config) map[string]domainreadiness.Client {
	observed := InspectStatusClients(runtime, cfg, nil, "")
	clients := make(map[string]domainreadiness.Client, len(observed))
	for client, status := range observed {
		clients[client] = status.Client
	}
	return clients
}

func orderedClientStates(runtime Dependencies, clients map[string]ClientStatus) []domainreadiness.Client {
	ordered := make([]domainreadiness.Client, 0, len(clients))
	for _, clientID := range runtime.Synchronizer().ClientIDs() {
		ordered = append(ordered, clients[clientID].Client)
	}
	return ordered
}

// CollectStatus observes the credential backend and every admitted client
// without reading Token values or changing state.
func CollectStatus(runtime Dependencies, cfg domain.Config) StatusReport {
	backend, backendErr := secret.Inspect(runtime.Secrets)
	if backendErr != nil {
		backend.RecoveryAction = domainreadiness.CredentialBackendRecovery
	}
	activation := clientactivation.AssessActivation(cfg, runtime.Secrets)
	clients := InspectStatusClients(runtime, cfg, &activation, "")
	return StatusReport{
		CredentialBackend: backend,
		Clients:           clients,
		Routes:            len(cfg.Routes),
		EnabledClients:    activation.EnabledClients,
		State:             activation.State,
		NextAction:        activation.NextActionFor(orderedClientStates(runtime, clients)),
	}
}

// EndpointTransport classifies how a client reaches endpoint.
func EndpointTransport(endpoint string) EndpointTransportKind {
	parsed, err := url.Parse(endpoint)
	if err == nil && domain.IsLoopbackHost(parsed.Hostname()) {
		return EndpointTransportExternalLoopback
	}
	return ""
}
