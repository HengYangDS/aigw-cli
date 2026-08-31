// Package readiness owns read-only status, health, and endpoint checks for the
// selected AIGW routes.
package readiness

import (
	"encoding/json"
	"fmt"
	"strings"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/diagnostics"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/providers"
	"github.com/spf13/cobra"
)

// NewCheckCommand builds the read-only health check for every enabled client.
func NewCheckCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use: "check", Short: "Check configuration, tokens, clients, and gateway", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if jsonMode {
				return runJSONCheck(cmd, runtime)
			}
			return RunCheck(cmd, runtime)
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	return cmd
}

type checkJSON struct {
	ConfigPath string                `json:"config_path"`
	Routes     map[string]checkRoute `json:"routes"`
	OK         bool                  `json:"ok"`
	Error      string                `json:"error,omitempty"`
	Fix        string                `json:"fix,omitempty"`
}

type checkRoute struct {
	Client         string `json:"client"`
	Profile        string `json:"profile,omitempty"`
	Account        string `json:"account,omitempty"`
	EndpointReady  bool   `json:"endpoint_ready"`
	AdapterReady   bool   `json:"adapter_ready"`
	Ready          bool   `json:"ready"`
	Issue          string `json:"issue,omitempty"`
	DiagnosticKind string `json:"diagnostic_kind,omitempty"`
	Fix            string `json:"fix,omitempty"`
	Attempts       int    `json:"attempts,omitempty"`
	Retryable      bool   `json:"retryable,omitempty"`
}

type evaluatedRoute struct {
	client         string
	runtime        configuration.Runtime
	resolveErr     error
	credentialErr  error
	tokenAvailable bool
	fix            string
	ready          bool
	issue          string
	adapter        bool
	endpoint       bool
	diagnostic     diagnostics.Result
}

type checkEvaluation struct {
	configPath string
	routes     []evaluatedRoute
}

func evaluateCheck(cmd *cobra.Command, runtime invocation.Context, cfg configuration.Config) checkEvaluation {
	evaluation := checkEvaluation{configPath: runtime.Config.Path()}
	for _, client := range configuration.AdmittedClientIDs() {
		if !cfg.Adapters[client].Enabled {
			continue
		}
		evaluation.routes = append(evaluation.routes, evaluateRoute(cmd, runtime, cfg, client))
	}
	return evaluation
}

func evaluateRoute(cmd *cobra.Command, runtime invocation.Context, cfg configuration.Config, client string) evaluatedRoute {
	route := evaluatedRoute{client: client}
	clientRuntime, err := cfg.ResolveRuntime(client, "")
	if err != nil {
		route.resolveErr = err
		route.issue = err.Error()
		return route
	}
	route.runtime = clientRuntime
	route.endpoint = strings.TrimSpace(clientRuntime.Endpoint) != ""
	route.adapter, route.issue = AdapterRouteReady(runtime, cfg, client, clientRuntime)
	if !route.adapter {
		route.fix = "aigw repair"
		if client == configuration.ClientCodex && strings.Contains(route.issue, "projection") {
			route.fix = "aigw sync"
		}
	}
	if !runtime.Secrets.Has(clientRuntime.AccountID) {
		route.issue = "account token is unavailable"
		route.fix = "aigw rotate " + clientRuntime.AccountID
		return route
	}
	token, tokenErr := runtime.Secrets.Get(clientRuntime.AccountID)
	if tokenErr != nil {
		route.credentialErr = tokenErr
		route.issue = "account token is unavailable"
		route.fix = "aigw rotate " + clientRuntime.AccountID
		return route
	}
	route.tokenAvailable = true
	if !route.adapter {
		return route
	}
	route.diagnostic = diagnostics.ProbeStable(cmd.Context(), runtime.HTTP, clientRuntime, token, diagnostics.DefaultStabilityPolicy())
	if route.diagnostic.Kind != diagnostics.Healthy {
		route.issue = route.diagnostic.Summary
		route.fix = route.diagnostic.Fix
	}
	route.ready = route.issue == ""
	return route
}

func (e checkEvaluation) ok() bool {
	for _, route := range e.routes {
		if !route.ready {
			return false
		}
	}
	return true
}

func (e checkEvaluation) route(client string) (evaluatedRoute, bool) {
	for _, route := range e.routes {
		if route.client == client {
			return route, true
		}
	}
	return evaluatedRoute{}, false
}

func runJSONCheck(cmd *cobra.Command, runtime invocation.Context) error {
	if isLocalProgramBuild(runtime.Version) {
		return writeJSONFailure(runtime, "local program is not an official release", "aigw update", fmt.Errorf("local program build"))
	}
	cfg, err := runtime.Config.Load()
	if err != nil {
		return writeJSONFailure(runtime, "Cannot read or validate local configuration; run `aigw doctor` to inspect or restore it", "aigw doctor", err)
	}
	if len(cfg.Profiles) == 0 {
		return writeJSONFailure(runtime, "not configured", "aigw setup", fmt.Errorf("not configured"))
	}
	evaluation := evaluateCheck(cmd, runtime, cfg)
	result := checkJSON{ConfigPath: evaluation.configPath, Routes: map[string]checkRoute{}, OK: evaluation.ok()}
	for _, route := range evaluation.routes {
		result.Routes[route.client] = checkRoute{
			Client: route.client, Profile: route.runtime.ProfileID, Account: route.runtime.AccountID,
			EndpointReady: route.endpoint, AdapterReady: route.adapter, Ready: route.ready, Issue: route.issue,
			DiagnosticKind: string(route.diagnostic.Kind), Fix: route.fix,
			Attempts: route.diagnostic.Attempts, Retryable: route.diagnostic.Retryable,
		}
	}
	if err := json.NewEncoder(runtime.Out).Encode(result); err != nil {
		return err
	}
	if !result.OK {
		// JSON is a protocol, not a prelude to human error rendering. Mark the
		// already-serialized failure as presented so the root command preserves
		// one valid JSON document on stdout while still returning a non-zero
		// result to callers.
		return presentation.Presented(fmt.Errorf("one or more enabled routes are not ready"))
	}
	return nil
}

func writeJSONFailure(runtime invocation.Context, message, fix string, cause error) error {
	result := checkJSON{Routes: map[string]checkRoute{}, Error: message, Fix: fix}
	if err := json.NewEncoder(runtime.Out).Encode(result); err != nil {
		return err
	}
	return presentation.Presented(cause)
}

// RunCheck verifies the selected route, configured client projections, and
// gateway authentication without mutating configuration or credentials.
func RunCheck(cmd *cobra.Command, runtime invocation.Context) error {
	if isLocalProgramBuild(runtime.Version) {
		return invocation.Problem(runtime, "Local program is not an official release", "Detected local build marker: "+runtime.Version, "A local development build must not replace a verified team release.", "aigw update", fmt.Errorf("local program build"))
	}
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Profiles) == 0 {
		return invocation.Problem(runtime, "Not configured", "No service profiles have been created.", "Cannot check, synchronize, or repair configuration that does not exist.", "aigw setup", fmt.Errorf("not configured"))
	}
	evaluation := evaluateCheck(cmd, runtime, cfg)
	renderer := Renderer(runtime)
	renderer.ProductTitle("Health check")
	renderer.Section("Configuration")
	renderer.Status(presentation.OK, "Configuration file", "Healthy")
	renderer.Section("Client")
	clientCount := 0
	diagnosticAccounts := map[string]bool{}
	balanceCommands := []string{}
	for _, client := range configuration.AdmittedClientIDs() {
		adapter := cfg.Adapters[client]
		if !adapter.Enabled {
			renderer.Status(presentation.Info, invocation.Title(client), "Disabled")
			continue
		}
		route, _ := evaluation.route(client)
		if route.resolveErr != nil {
			return invocation.Problem(runtime, invocation.Title(client)+" route cannot be resolved", route.resolveErr.Error(), invocation.Title(client)+" cannot determine which profile to use.", "aigw use <"+client+"-profile>", route.resolveErr)
		}
		if !route.tokenAvailable {
			if route.credentialErr != nil {
				return route.credentialErr
			}
			instruction, _ := credential.TokenRecovery(runtime.Secrets, route.runtime.AccountID)
			return invocation.Problem(
				runtime,
				invocation.Title(client)+" account token is unavailable",
				"Account "+route.runtime.AccountID+" has no available Token.",
				invocation.Title(client)+" cannot authenticate to its selected gateway.",
				instruction,
				fmt.Errorf("%s account token unavailable", client),
			)
		}
		if !route.adapter {
			issue := route.issue
			fix := "aigw repair"
			impact := invocation.Title(client) + " cannot inherit AIGW routes, tokens, or configuration projections."
			if client == configuration.ClientCodex && strings.Contains(issue, "projection") {
				fix = "aigw sync"
				impact = "Codex may use the wrong model or endpoint."
			}
			return invocation.Problem(runtime, invocation.Title(client)+" adapter is not ready", issue, impact, fix, fmt.Errorf("%s adapter not ready", client))
		}
		result := route.diagnostic
		if result.Kind != diagnostics.Healthy {
			evidence := result.Detail
			if result.HTTPStatus != 0 {
				evidence = fmt.Sprintf("HTTP %d", result.HTTPStatus)
				if result.Detail != "" {
					evidence += " · " + result.Detail
				}
			}
			return invocation.Problem(runtime, result.Summary, evidence, invocation.Title(client)+" is unavailable.", result.Fix, fmt.Errorf("%s diagnostic kind %s", client, result.Kind))
		}
		renderer.Status(presentation.OK, invocation.Title(client), route.runtime.ProfileLabel+" · Ready")
		if result.RecoveredTransient {
			renderer.Detail(invocation.Title(client) + " authentication recovered after a transient response")
		}
		if transport := TransportStatus(route.runtime.Endpoint); transport.Kind == "external_loopback" {
			renderer.Detail(invocation.Title(client) + " uses an external loopback compatibility layer that AIGW does not manage")
		}
		if !diagnosticAccounts[route.runtime.AccountID] {
			diagnosticAccounts[route.runtime.AccountID] = true
			providerAccount := cfg.Accounts[route.runtime.AccountID]
			if providerAccount.AccountProbe != nil && providers.Supports(providerAccount.AccountProbe.Kind) {
				balanceCommands = append(balanceCommands, "aigw balance "+route.runtime.AccountID)
				if runtime.Accounts.Has(route.runtime.AccountID) {
					renderer.Detail(route.runtime.AccountLabel + " precise balance enabled")
				} else {
					renderer.Detail("aigw account connect " + route.runtime.AccountID)
				}
			} else if providerAccount.AccountProbe != nil {
				renderer.Detail(route.runtime.AccountLabel + " has no diagnostic driver in this version")
			}
		}
		clientCount++
	}
	renderer.Section("Result")
	if clientCount == 0 {
		renderer.Success("Configuration is healthy; no clients are enabled")
		return nil
	}
	renderer.Success("Every enabled client route is healthy")
	for _, command := range balanceCommands {
		renderer.Next(command)
	}
	return nil
}

func isLocalProgramBuild(version string) bool {
	version = strings.TrimSpace(version)
	return version == "" || strings.HasSuffix(version, "-dev") || strings.Contains(version, "+local")
}
