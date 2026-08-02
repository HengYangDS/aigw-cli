package readiness

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/providers"
	"github.com/spf13/cobra"
)

type routeStatus struct {
	Profile          string `json:"profile,omitempty"`
	Inherited        bool   `json:"inherited"`
	SecretAvailable  bool   `json:"secret_available"`
	EndpointReady    bool   `json:"endpoint_ready"`
	Transport        string `json:"transport,omitempty"`
	TransportReady   bool   `json:"transport_ready,omitempty"`
	AdapterReady     bool   `json:"adapter_ready"`
	AdapterIssue     string `json:"adapter_issue,omitempty"`
	NeedsSelection   bool   `json:"needs_selection,omitempty"`
	SuggestedProfile string `json:"suggested_profile,omitempty"`
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
	admittedClientIDs   = configuration.AdmittedClientIDs
	validateCodexConfig = codex.ValidateConfig
	probeAdapterRoute   = AdapterRouteReady
)

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
		result.Routes[client] = routeStatus{
			Profile:         clientRuntime.ProfileID,
			Inherited:       inherited,
			SecretAvailable: runtime.Secrets.Has(clientRuntime.AccountID),
			EndpointReady:   clientRuntime.Endpoint != "",
			Transport:       transport.Kind,
			AdapterReady:    adapterReady,
			AdapterIssue:    adapterIssue,
		}
	}
	if jsonMode {
		enc := json.NewEncoder(runtime.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	if len(cfg.Profiles) == 0 {
		r := Renderer(runtime)
		r.ProductTitle("Not configured")
		r.Section("Get started")
		r.Text("Run the guided setup once to add a service, token, and first model profile.")
		r.Next("aigw setup")
		return nil
	}
	r := Renderer(runtime)
	r.ProductTitle("Ready view")
	r.Text("The active service, client readiness, and the smallest next action.")
	r.Section("Active service")
	current := cfg.Profiles[result.Default]
	accountName := current.Account
	account := cfg.Accounts[accountName]
	r.Row("Current profile", current.Label)
	r.Row("Configuration", result.Default)
	if purpose := strings.TrimSpace(current.Purpose); purpose != "" {
		r.Row("Purpose", purpose)
	}
	r.Row("Account", accountName)
	for _, spec := range configuration.AdmittedClientSpecs() {
		if model := current.ModelFor(spec.ID); model != "" {
			r.Row(spec.Label+" model", model)
		}
	}
	r.Row("Model profiles", fmt.Sprintf("%d", result.Profiles))
	r.Section("Clients")
	attention := false
	selectionCommand := ""
	for _, client := range admittedClientIDs() {
		route := result.Routes[client]
		if route.NeedsSelection {
			state := presentation.Warn
			message := "No " + invocation.Title(client) + " profile selected"
			if route.SuggestedProfile != "" {
				cmd := "aigw use " + route.SuggestedProfile + " --for " + client
				message += " · " + cmd
				if selectionCommand == "" {
					selectionCommand = cmd
				}
			}
			r.Status(state, invocation.Title(client), message)
			attention = true
			continue
		}
		mode := "Explicit override"
		if route.Inherited {
			mode = "Inherits default"
		}
		readiness := route.Profile + " · " + mode + " · Ready"
		state := presentation.OK
		if !route.SecretAvailable || !route.EndpointReady || !route.AdapterReady {
			readiness = route.Profile + " · " + mode + " · Action required"
			if route.AdapterIssue != "" {
				readiness = route.Profile + " · " + mode + " · " + route.AdapterIssue
			}
			state = presentation.Warn
			attention = true
		}
		r.Status(state, invocation.Title(client), readiness)
	}
	for _, client := range admittedClientIDs() {
		route := result.Routes[client]
		if route.Transport != "external_loopback" {
			continue
		}
		r.Section("Transport")
		r.Status(presentation.Info, invocation.Title(client), "External loopback compatibility layer")
		r.Detail(invocation.Title(client) + " requests use the external listener")
		r.Detail("AIGW does not start, stop, or configure it")
		break
	}
	r.Section("Optional diagnostics")
	if account.AccountProbe != nil && providers.Supports(account.AccountProbe.Kind) && runtime.Accounts.Has(accountName) {
		r.Status(presentation.OK, "Precise balance", "Enabled")
	} else if account.AccountProbe != nil && providers.Supports(account.AccountProbe.Kind) {
		r.Status(presentation.Warn, "Precise balance", "Disabled")
		r.Detail("aigw account connect " + accountName)
	} else if account.AccountProbe != nil {
		r.Status(presentation.Info, "Precise balance", "This version does not provide diagnostics for this provider")
	} else {
		r.Status(presentation.Info, "Precise balance", "Provider does not expose a probe")
	}
	if selectionCommand != "" {
		r.Next(selectionCommand)
	} else if attention {
		r.Next("aigw repair")
	} else {
		r.Next("aigw check")
	}
	return nil
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
		ready, err := runtime.ClaudeLauncher.ClaudeLauncherReady()
		if err != nil {
			return false, "Cannot read Claude launcher"
		}
		if !ready {
			return false, "Claude launcher is missing"
		}
		active, err := runtime.ClaudeLauncher.ClaudeActivationReady()
		if err != nil {
			return false, "Cannot read Claude PATH activation"
		}
		if !active {
			return false, "Claude PATH activation is missing"
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
