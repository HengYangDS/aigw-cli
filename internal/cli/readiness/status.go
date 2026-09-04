package readiness

import (
	"context"
	"encoding/json"
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

type routeStatus struct {
	domainreadiness.Client
	Authentication       configuration.Authentication `json:"authentication"`
	EndpointReady        bool                         `json:"endpoint_ready"`
	Transport            string                       `json:"transport,omitempty"`
	TransportReady       bool                         `json:"transport_ready,omitempty"`
	AdapterReady         bool                         `json:"adapter_ready"`
	NativeAuthentication string                       `json:"native_authentication,omitempty"`
}

type endpointTestResult struct {
	client    string
	profileID string
	status    int
	detail    string
}

type statusOutput struct {
	ConfigPath        string                            `json:"config_path"`
	CredentialBackend secrets.BackendSelection          `json:"credential_backend"`
	Clients           map[string]domainreadiness.Client `json:"clients"`
	Routes            map[string]routeStatus            `json:"routes"`
	Profiles          int                               `json:"profiles"`
}

var inspectAdapter = func(ctx context.Context, runtime invocation.Context, cfg configuration.Config, clientID string, clientRuntime configuration.Runtime, options clientdomain.InspectionOptions) clientdomain.Status {
	return invocation.Synchronizer(runtime).Inspect(ctx, cfg, clientID, clientRuntime, options)
}

func NewStatusCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{Use: "status", Short: "Show the active service and the next useful action", Args: cobra.NoArgs}
	cmd.RunE = func(_ *cobra.Command, _ []string) error { return RunStatus(runtime, jsonMode) }
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	return cmd
}

func RunStatus(runtime invocation.Context, jsonMode bool) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	result := collectStatus(runtime, cfg)
	if jsonMode {
		enc := json.NewEncoder(runtime.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	renderStatus(runtime, cfg, result)
	return nil
}

// inspectStatusClients observes every admitted client without authenticating
// an endpoint or reading Token values.
func inspectStatusClients(runtime invocation.Context, cfg configuration.Config) map[string]routeStatus {
	clientIDs := invocation.Synchronizer(runtime).ClientIDs()
	routes := make(map[string]routeStatus, len(clientIDs))
	for _, clientID := range clientIDs {
		clientRuntime, resolveErr := cfg.ResolveRuntime(clientID, "")
		if resolveErr != nil {
			facts := domainreadiness.ClientFacts{}
			if profile := cfg.Routes[clientID]; profile != "" {
				facts.Profile = profile
				facts.RouteIssue = resolveErr.Error()
				facts.RouteAction = "aigw use <" + clientID + "-profile>"
			} else {
				facts.SuggestedProfile = cfg.FirstProfileForClient(clientID)
			}
			state := domainreadiness.ClassifyClient(facts)
			routes[clientID] = routeStatus{Client: state}
			continue
		}
		credentialAvailable := true
		credentialAction := ""
		if clientRuntime.RequiresAccountToken() {
			available, observationErr := runtime.Secrets.Exists(clientRuntime.AccountID)
			if observationErr != nil {
				state := domainreadiness.ClassifyClient(domainreadiness.ClientFacts{
					Profile:                    clientRuntime.ProfileID,
					Account:                    clientRuntime.AccountID,
					CredentialRequired:         true,
					CredentialObservationIssue: "Credential metadata is unavailable",
				})
				routes[clientID] = routeStatus{
					Client:         state,
					Authentication: clientRuntime.Authentication,
					EndpointReady:  strings.TrimSpace(clientRuntime.Endpoint) != "",
					Transport:      TransportStatus(clientRuntime.Endpoint).Kind,
				}
				continue
			}
			credentialAvailable = available
			credentialAction, _ = credential.TokenRecovery(runtime.Secrets, clientRuntime.AccountID)
		}
		adapterStatus := inspectAdapter(context.Background(), runtime, cfg, clientID, clientRuntime, clientdomain.InspectionOptions{NativeAuthentication: true})
		state := domainreadiness.ClassifyClient(domainreadiness.ClientFacts{
			Profile:             clientRuntime.ProfileID,
			Account:             clientRuntime.AccountID,
			CredentialRequired:  clientRuntime.RequiresAccountToken(),
			CredentialAvailable: credentialAvailable,
			CredentialAction:    credentialAction,
			AdapterEnabled:      cfg.Adapters[clientID].Enabled,
			AdapterReady:        adapterStatus.Ready,
			AdapterIssue:        adapterStatus.Issue,
			AdapterAction:       adapterStatus.RepairAction,
		})
		route := routeStatus{
			Client:               state,
			Authentication:       clientRuntime.Authentication,
			EndpointReady:        strings.TrimSpace(clientRuntime.Endpoint) != "",
			Transport:            TransportStatus(clientRuntime.Endpoint).Kind,
			AdapterReady:         adapterStatus.Ready,
			NativeAuthentication: adapterStatus.NativeAuthentication,
		}
		if adapterStatus.Ready && !clientRuntime.RequiresAccountToken() {
			route.State = domainreadiness.Configured
			route.Detail = "Projection ready; client-owned authentication is not proven"
			route.NextAction = "aigw verify --for " + clientID
		} else if adapterStatus.Ready && adapterStatus.NativeAuthentication == "not_proven" {
			route.State = domainreadiness.Configured
			route.Detail = "Projection ready; native authentication is not proven"
			route.NextAction = "aigw adapter auth " + clientID
		}
		routes[clientID] = route
	}
	return routes
}

// InspectClients returns the canonical, secret-free local state of every
// admitted client without authenticating an endpoint or reading Token values.
func InspectClients(runtime invocation.Context, cfg configuration.Config) map[string]domainreadiness.Client {
	routes := inspectStatusClients(runtime, cfg)
	clients := make(map[string]domainreadiness.Client, len(routes))
	for client, route := range routes {
		clients[client] = route.Client
	}
	return clients
}

func collectStatus(runtime invocation.Context, cfg configuration.Config) statusOutput {
	backend, backendErr := secrets.Inspect(runtime.Secrets)
	if backendErr != nil {
		backend.RecoveryAction = domainreadiness.CredentialBackendRecovery
	}
	routes := inspectStatusClients(runtime, cfg)
	clients := make(map[string]domainreadiness.Client, len(routes))
	for client, route := range routes {
		clients[client] = route.Client
	}
	return statusOutput{
		ConfigPath:        runtime.Config.Path(),
		CredentialBackend: backend,
		Clients:           clients,
		Profiles:          len(cfg.Profiles),
		Routes:            routes,
	}
}

type transportState struct {
	Kind string
}

func TransportStatus(endpoint string) transportState {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return transportState{}
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "127.0.0.1", "::1", "localhost":
		return transportState{Kind: "external_loopback"}
	default:
		return transportState{}
	}
}

func Renderer(runtime invocation.Context) *presentation.Renderer {
	out := runtime.RenderOut
	if out == nil {
		out = runtime.Out
	}
	return presentation.NewWithWidth(out, runtime.Color, runtime.Width)
}
