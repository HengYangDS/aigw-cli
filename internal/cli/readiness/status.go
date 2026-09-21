package readiness

import (
	"context"
	"net/url"
	"strings"

	"aigw-cli/internal/cli/invocation"
	clientdomain "aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/presentation"
	domainreadiness "aigw-cli/internal/readiness"
	"aigw-cli/internal/secrets"

	"github.com/spf13/cobra"
)

type clientStatus struct {
	domainreadiness.Client
	Authentication     configuration.Authentication `json:"authentication"`
	EndpointConfigured bool                         `json:"endpoint_configured"`
	Transport          endpointTransportKind        `json:"transport,omitempty"`
	ProjectionReady    bool                         `json:"projection_ready"`
	CheckPassed        *bool                        `json:"check_passed,omitempty"`
	DiagnosticKind     string                       `json:"diagnostic_kind,omitempty"`
	Attempts           int                          `json:"attempts,omitempty"`
	Retryable          bool                         `json:"retryable,omitempty"`
}

type endpointTransportKind string

const endpointTransportExternalLoopback endpointTransportKind = "external_loopback"

type statusOutput struct {
	ConfigPath        string                   `json:"config_path"`
	CredentialBackend secrets.BackendSelection `json:"credential_backend"`
	Clients           map[string]clientStatus  `json:"clients"`
	Profiles          int                      `json:"profiles"`
}

var inspectAdapter = func(ctx context.Context, runtime invocation.Context, cfg configuration.Config, clientID string, clientRuntime configuration.Runtime) clientdomain.Status {
	return invocation.Synchronizer(runtime).Inspect(ctx, cfg, clientID, clientRuntime)
}

// NewStatusCommand constructs the read-only configuration and client readiness command.
func NewStatusCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{Use: "status", Short: "Show each client's selected profile and local readiness", Args: cobra.NoArgs}
	cmd.RunE = func(_ *cobra.Command, _ []string) error { return RunStatus(runtime, jsonMode) }
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	return cmd
}

// RunStatus observes client bindings and projections without reading secret values or mutating state.
func RunStatus(runtime invocation.Context, jsonMode bool) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	result := collectStatus(runtime, cfg)
	if jsonMode {
		return presentation.WriteJSON(runtime.Out, result)
	}
	renderStatus(runtime, cfg, result)
	return nil
}

// inspectStatusClients observes every admitted client without authenticating
// an endpoint or reading Token values.
func inspectStatusClients(runtime invocation.Context, cfg configuration.Config) map[string]clientStatus {
	clientIDs := invocation.Synchronizer(runtime).ClientIDs()
	clients := make(map[string]clientStatus, len(clientIDs))
	for _, clientID := range clientIDs {
		clientRuntime, resolveErr := cfg.ResolveRuntime(clientID, "")
		if resolveErr != nil {
			clients[clientID] = unresolvedClientStatus(&cfg, clientID, resolveErr)
			continue
		}
		adapterStatus := inspectAdapter(context.Background(), runtime, cfg, clientID, clientRuntime)
		facts := domainreadiness.ClientFacts{
			Profile:            clientRuntime.ProfileID,
			Account:            clientRuntime.AccountID,
			CredentialRequired: clientRuntime.UsesAIGWCredentialStore(),
			ProjectionEnabled:  cfg.Clients[clientID].Enabled,
			ProjectionReady:    adapterStatus.Ready,
			ProjectionIssue:    adapterStatus.Issue,
			ProjectionAction:   adapterStatus.RepairAction,
		}
		if facts.CredentialRequired && (!facts.ProjectionEnabled || facts.ProjectionReady) {
			available, observationErr := runtime.Secrets.Exists(clientRuntime.AccountID)
			facts.CredentialAvailable = available
			if observationErr != nil {
				facts.CredentialObservationIssue = "Credential metadata is unavailable"
			} else {
				facts.CredentialAction, _ = credential.TokenRecovery(runtime.Secrets, clientRuntime.AccountID)
			}
		}
		state := domainreadiness.ClassifyClient(facts)
		client := clientStatus{
			Client:             state,
			Authentication:     clientRuntime.Authentication,
			EndpointConfigured: strings.TrimSpace(clientRuntime.Endpoint) != "",
			Transport:          endpointTransport(clientRuntime.Endpoint),
			ProjectionReady:    adapterStatus.Ready,
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

func unresolvedClientStatus(cfg *configuration.Config, clientID string, resolveErr error) clientStatus {
	facts := domainreadiness.ClientFacts{}
	if profile := cfg.SelectedProfile(clientID); profile != "" {
		facts.Profile = profile
		facts.BindingIssue = resolveErr.Error()
		facts.BindingAction = "aigw use --for " + clientID + " <profile>"
	} else if suggested := cfg.RecommendedProfile(clientID); suggested != "" {
		facts.SuggestedProfile = suggested
		facts.BindingAction = "aigw use --for " + clientID + " " + suggested
	}
	state := domainreadiness.ClassifyClient(facts)
	if facts.Profile == "" && facts.SuggestedProfile == "" {
		state.NextAction = ""
	}
	return clientStatus{Client: state}
}

// InspectClients returns the canonical, secret-free local state of every
// admitted client without authenticating an endpoint or reading Token values.
func InspectClients(runtime invocation.Context, cfg configuration.Config) map[string]domainreadiness.Client {
	observed := inspectStatusClients(runtime, cfg)
	clients := make(map[string]domainreadiness.Client, len(observed))
	for client, status := range observed {
		clients[client] = status.Client
	}
	return clients
}

func collectStatus(runtime invocation.Context, cfg configuration.Config) statusOutput {
	backend, backendErr := secrets.Inspect(runtime.Secrets)
	if backendErr != nil {
		backend.RecoveryAction = domainreadiness.CredentialBackendRecovery
	}
	clients := inspectStatusClients(runtime, cfg)
	return statusOutput{
		ConfigPath:        runtime.Config.Path(),
		CredentialBackend: backend,
		Clients:           clients,
		Profiles:          len(cfg.Profiles),
	}
}

func endpointTransport(endpoint string) endpointTransportKind {
	parsed, err := url.Parse(endpoint)
	if err == nil && configuration.IsLoopbackHost(parsed.Hostname()) {
		return endpointTransportExternalLoopback
	}
	return ""
}
