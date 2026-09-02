package readiness

import (
	"context"
	"encoding/json"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"aigw-cli/internal/claude"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/presentation"
	domainreadiness "aigw-cli/internal/readiness"
	"github.com/spf13/cobra"
)

type routeStatus struct {
	domainreadiness.Client
	SecretAvailable      bool   `json:"secret_available"`
	EndpointReady        bool   `json:"endpoint_ready"`
	Transport            string `json:"transport,omitempty"`
	TransportReady       bool   `json:"transport_ready,omitempty"`
	AdapterReady         bool   `json:"adapter_ready"`
	NativeAuthentication string `json:"native_authentication,omitempty"`
}

type endpointTestResult struct {
	client    string
	profileID string
	status    int
	detail    string
}

type statusOutput struct {
	ConfigPath string                            `json:"config_path"`
	Clients    map[string]domainreadiness.Client `json:"clients"`
	Routes     map[string]routeStatus            `json:"routes"`
	Profiles   int                               `json:"profiles"`
}

var (
	admittedClientIDs        = configuration.AdmittedClientIDs
	validateCodexConfig      = codex.ValidateConfig
	probeAdapterRoute        = AdapterRouteReady
	probeCodexAuthentication = CodexAuthenticationProven
)

const codexAuthenticationInspectionTimeout = 5 * time.Second

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
	routes := make(map[string]routeStatus, len(admittedClientIDs()))
	for _, client := range admittedClientIDs() {
		clientRuntime, resolveErr := cfg.ResolveRuntime(client, "")
		if resolveErr != nil {
			facts := domainreadiness.ClientFacts{}
			if profile := cfg.Routes[client]; profile != "" {
				facts.Profile = profile
				facts.RouteIssue = resolveErr.Error()
				facts.RouteAction = "aigw use <" + client + "-profile>"
			} else {
				facts.SuggestedProfile = cfg.FirstProfileForClient(client)
			}
			state := domainreadiness.ClassifyClient(facts)
			routes[client] = routeStatus{Client: state}
			continue
		}
		secretAvailable, err := runtime.Secrets.Exists(clientRuntime.AccountID)
		if err != nil {
			state := domainreadiness.ClassifyClient(domainreadiness.ClientFacts{
				Profile:                    clientRuntime.ProfileID,
				Account:                    clientRuntime.AccountID,
				CredentialObservationIssue: "Credential metadata is unavailable",
			})
			routes[client] = routeStatus{
				Client:        state,
				EndpointReady: strings.TrimSpace(clientRuntime.Endpoint) != "",
				Transport:     TransportStatus(clientRuntime.Endpoint).Kind,
			}
			continue
		}
		adapterReady, adapterIssue := probeAdapterRoute(runtime, cfg, client, clientRuntime)
		adapterAction := "aigw repair"
		if client == configuration.ClientCodex && strings.Contains(adapterIssue, "projection drift") {
			adapterAction = "aigw sync"
		}
		tokenAction, _ := credential.TokenRecovery(runtime.Secrets, clientRuntime.AccountID)
		state := domainreadiness.ClassifyClient(domainreadiness.ClientFacts{
			Profile:        clientRuntime.ProfileID,
			Account:        clientRuntime.AccountID,
			TokenAvailable: secretAvailable,
			TokenAction:    tokenAction,
			AdapterEnabled: cfg.Adapters[client].Enabled,
			AdapterReady:   adapterReady,
			AdapterIssue:   adapterIssue,
			AdapterAction:  adapterAction,
		})
		route := routeStatus{
			Client:          state,
			SecretAvailable: secretAvailable,
			EndpointReady:   strings.TrimSpace(clientRuntime.Endpoint) != "",
			Transport:       TransportStatus(clientRuntime.Endpoint).Kind,
			AdapterReady:    adapterReady,
		}
		if client == configuration.ClientCodex && adapterReady {
			switch {
			case clientRuntime.ModelProvider != configuration.ModelProviderAIGW:
				route.NativeAuthentication = "not_required"
			case probeCodexAuthentication(runtime, cfg.Adapters[client].Executable, cfg.Adapters[client].Targets):
				route.NativeAuthentication = "present"
			default:
				route.NativeAuthentication = "not_proven"
				route.State = domainreadiness.Configured
				route.Detail = "Projection ready; native authentication is not proven"
				route.NextAction = "aigw adapter auth codex"
			}
		}
		routes[client] = route
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
	routes := inspectStatusClients(runtime, cfg)
	clients := make(map[string]domainreadiness.Client, len(routes))
	for client, route := range routes {
		clients[client] = route.Client
	}
	return statusOutput{ConfigPath: runtime.Config.Path(), Clients: clients, Profiles: len(cfg.Profiles), Routes: routes}
}

// CodexAuthenticationProven returns true only when Codex's public read-only
// status command succeeds for every selected target. It never reads native
// credential files or creates a second authentication-state authority.
func CodexAuthenticationProven(runtime invocation.Context, executable string, targets []string) bool {
	if len(targets) == 0 {
		return false
	}
	for _, target := range targets {
		plan, err := codex.LoginStatusPlan(executable, filepath.Dir(target))
		if err != nil {
			return false
		}
		ctx, cancel := context.WithTimeout(context.Background(), codexAuthenticationInspectionTimeout)
		_, err = invocation.RunCapture(runtime, ctx, plan)
		cancel()
		if err != nil {
			return false
		}
	}
	return true
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

// AdapterRouteReady checks all local conditions that make an enabled adapter
// usable by the selected route. It is deliberately read-only and never starts
// or reloads a client process.
func AdapterRouteReady(runtime invocation.Context, cfg configuration.Config, client string, clientRuntime configuration.Runtime) (bool, string) {
	adapter := cfg.Adapters[client]
	if !adapter.Enabled {
		return false, invocation.Title(client) + " adapter is disabled"
	}
	if adapter.Executable == "" {
		return false, invocation.Title(client) + " executable is not configured"
	}
	switch client {
	case configuration.ClientClaude:
		ready, err := claude.Ready(adapter.Executable)
		if err != nil {
			return false, "Cannot inspect Claude executable"
		}
		if !ready {
			return false, "Claude executable is unavailable"
		}
	case configuration.ClientCodex:
		if len(adapter.Targets) == 0 {
			return false, "Codex configuration target is missing"
		}
		for _, target := range adapter.Targets {
			if err := validateCodexConfig(target, clientRuntime); err != nil {
				return false, "Codex configuration projection drift: " + err.Error()
			}
		}
	}
	return true, ""
}

func CodexModelsEndpoint(endpoint string) string {
	endpoint = strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(endpoint, "/models") {
		return endpoint
	}
	return endpoint + "/models"
}

func Renderer(runtime invocation.Context) *presentation.Renderer {
	out := runtime.RenderOut
	if out == nil {
		out = runtime.Out
	}
	return presentation.NewWithWidth(out, runtime.Color, runtime.Width)
}
