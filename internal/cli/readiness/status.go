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
	"aigw-cli/internal/presentation"
	"github.com/spf13/cobra"
)

type routeStatus struct {
	Profile              string `json:"profile,omitempty"`
	Inherited            bool   `json:"inherited"`
	SecretAvailable      bool   `json:"secret_available"`
	EndpointReady        bool   `json:"endpoint_ready"`
	Transport            string `json:"transport,omitempty"`
	TransportReady       bool   `json:"transport_ready,omitempty"`
	AdapterReady         bool   `json:"adapter_ready"`
	AdapterIssue         string `json:"adapter_issue,omitempty"`
	NativeAuthentication string `json:"native_authentication,omitempty"`
	NeedsSelection       bool   `json:"needs_selection,omitempty"`
	SuggestedProfile     string `json:"suggested_profile,omitempty"`
}

type endpointTestResult struct {
	client    string
	profileID string
	status    int
	detail    string
}

type statusOutput struct {
	ConfigPath string                 `json:"config_path"`
	Default    string                 `json:"default,omitempty"`
	Routes     map[string]routeStatus `json:"routes"`
	Profiles   int                    `json:"profiles"`
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

func collectStatus(runtime invocation.Context, cfg configuration.Config) statusOutput {
	result := statusOutput{ConfigPath: runtime.Config.Path(), Default: cfg.Routes.Default, Profiles: len(cfg.Profiles), Routes: map[string]routeStatus{}}
	for _, client := range admittedClientIDs() {
		clientRuntime, inherited, resolveErr := cfg.ResolveRuntime(client, "")
		if resolveErr != nil {
			suggested := cfg.FirstProfileForClient(client)
			result.Routes[client] = routeStatus{Inherited: true, NeedsSelection: suggested != "", SuggestedProfile: suggested}
			continue
		}
		adapterReady, adapterIssue := probeAdapterRoute(runtime, cfg, client, clientRuntime)
		transport := TransportStatus(clientRuntime.Endpoint)
		nativeAuthentication := ""
		if client == configuration.ClientCodex && adapterReady {
			switch {
			case clientRuntime.ModelProvider != configuration.ModelProviderAIGW:
				nativeAuthentication = "not_required"
			case probeCodexAuthentication(runtime, cfg.Adapters[client].Executable, cfg.Adapters[client].Targets):
				nativeAuthentication = "present"
			default:
				nativeAuthentication = "not_proven"
			}
		}
		result.Routes[client] = routeStatus{
			Profile:              clientRuntime.ProfileID,
			Inherited:            inherited,
			SecretAvailable:      runtime.Secrets.Has(clientRuntime.AccountID),
			EndpointReady:        clientRuntime.Endpoint != "",
			Transport:            transport.Kind,
			AdapterReady:         adapterReady,
			AdapterIssue:         adapterIssue,
			NativeAuthentication: nativeAuthentication,
		}
	}
	return result
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
