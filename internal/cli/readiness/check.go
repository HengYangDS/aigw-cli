// Package readiness owns read-only status, health, and endpoint checks for
// selected AIGW client bindings.
package readiness

import (
	"fmt"
	"strings"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/diagnostics"
	"aigw-cli/internal/presentation"
	domainreadiness "aigw-cli/internal/readiness"

	"github.com/spf13/cobra"
)

// NewCheckCommand builds the read-only health check for every enabled client.
func NewCheckCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use: "check", Short: "Check client bindings, credentials, projections, and endpoints", Args: cobra.NoArgs,
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
	ConfigPath string                  `json:"config_path"`
	Clients    map[string]clientStatus `json:"clients"`
	OK         bool                    `json:"ok"`
	State      domainreadiness.State   `json:"state,omitempty"`
	NextAction string                  `json:"next_action,omitempty"`
	Error      string                  `json:"error,omitempty"`
}

type evaluatedClient struct {
	client             string
	runtime            configuration.Runtime
	resolveErr         error
	credentialErr      error
	fix                string
	checkPassed        bool
	issue              string
	adapter            bool
	endpointConfigured bool
	diagnostic         diagnostics.Result
}

type checkEvaluation struct {
	configPath string
	clients    []evaluatedClient
}

func evaluateCheck(cmd *cobra.Command, runtime invocation.Context, cfg configuration.Config) checkEvaluation {
	evaluation := checkEvaluation{configPath: runtime.Config.Path()}
	for _, client := range invocation.Synchronizer(runtime).ClientIDs() {
		if !cfg.Clients[client].Enabled {
			continue
		}
		evaluation.clients = append(evaluation.clients, evaluateClient(cmd, runtime, cfg, client))
	}
	return evaluation
}

func evaluateClient(cmd *cobra.Command, runtime invocation.Context, cfg configuration.Config, client string) evaluatedClient {
	result := evaluatedClient{client: client}
	clientRuntime, err := cfg.ResolveRuntime(client, "")
	if err != nil {
		result.resolveErr = err
		result.issue = err.Error()
		return result
	}
	result.runtime = clientRuntime
	result.endpointConfigured = strings.TrimSpace(clientRuntime.Endpoint) != ""
	status := invocation.Synchronizer(runtime).Inspect(cmd.Context(), cfg, client, clientRuntime)
	result.adapter = status.Ready
	result.issue = status.Issue
	result.fix = status.RepairAction
	if !result.adapter {
		return result
	}
	if !clientRuntime.UsesAIGWCredentialStore() {
		result.checkPassed = true
		result.fix = "aigw verify --for " + client
		return result
	}
	token, tokenErr := runtime.Secrets.Get(clientRuntime.AccountID)
	if tokenErr != nil {
		result.credentialErr = tokenErr
		result.issue = "account token is unavailable"
		result.fix = "aigw rotate " + clientRuntime.AccountID
		return result
	}
	result.diagnostic = diagnostics.ProbeStable(cmd.Context(), runtime.HTTP, clientRuntime, token, diagnostics.DefaultStabilityPolicy())
	if result.diagnostic.Kind != diagnostics.Healthy {
		result.issue = result.diagnostic.Summary
		result.fix = result.diagnostic.Fix
	}
	result.checkPassed = result.issue == ""
	return result
}

func (e checkEvaluation) ok() bool {
	for _, client := range e.clients {
		if !client.checkPassed {
			return false
		}
	}
	return true
}

func (e checkEvaluation) client(clientID string) (evaluatedClient, bool) {
	for _, client := range e.clients {
		if client.client == clientID {
			return client, true
		}
	}
	return evaluatedClient{}, false
}

func runJSONCheck(cmd *cobra.Command, runtime invocation.Context) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return writeJSONFailure(runtime, domainreadiness.Invalid, "Cannot read or validate local configuration; run `aigw doctor` to inspect or restore it", "aigw doctor", err)
	}
	if len(cfg.Profiles) == 0 {
		return writeJSONFailure(runtime, domainreadiness.Deferred, "not configured", "aigw setup", fmt.Errorf("not configured"))
	}
	evaluation := evaluateCheck(cmd, runtime, cfg)
	clients := inspectStatusClients(runtime, cfg)
	result := checkJSON{
		ConfigPath: evaluation.configPath,
		Clients:    clients,
		OK:         evaluation.ok(),
	}
	for _, client := range evaluation.clients {
		status := clients[client.client]
		status.Profile = client.runtime.ProfileID
		status.Account = client.runtime.AccountID
		status.Authentication = client.runtime.Authentication
		status.EndpointConfigured = client.endpointConfigured
		status.ProjectionReady = client.adapter
		status.CheckPassed = new(bool)
		*status.CheckPassed = client.checkPassed
		status.DiagnosticKind = string(client.diagnostic.Kind)
		status.Attempts = client.diagnostic.Attempts
		status.Retryable = client.diagnostic.Retryable
		if client.issue != "" {
			status.Detail = client.issue
		}
		if client.fix != "" {
			status.NextAction = client.fix
		}
		if client.diagnostic.Kind != "" && status.State == domainreadiness.Configured {
			status.Client = domainreadiness.WithProbe(status.Client, client.diagnostic)
		}
		clients[client.client] = status
	}
	if err := presentation.WriteJSON(runtime.Out, result); err != nil {
		return err
	}
	if !result.OK {
		// JSON is a protocol, not a prelude to human error rendering. Mark the
		// already-serialized failure as presented so the root command preserves
		// one valid JSON document on stdout while still returning a non-zero
		// result to callers.
		return presentation.Presented(fmt.Errorf("one or more enabled client checks failed"))
	}
	return nil
}

func writeJSONFailure(runtime invocation.Context, state domainreadiness.State, message, fix string, cause error) error {
	result := checkJSON{Clients: map[string]clientStatus{}, State: state, NextAction: fix, Error: message}
	if err := presentation.WriteJSON(runtime.Out, result); err != nil {
		return err
	}
	return presentation.Presented(cause)
}

// RunCheck verifies selected client bindings, configured projections, and
// endpoint authentication without mutating configuration or credentials.
func RunCheck(cmd *cobra.Command, runtime invocation.Context) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Profiles) == 0 {
		return invocation.Problem(runtime, "Not configured", "No Profiles have been created.", "Cannot check, synchronize, or repair configuration that does not exist.", "aigw setup", fmt.Errorf("not configured"))
	}
	evaluation := evaluateCheck(cmd, runtime, cfg)
	renderer := invocation.Renderer(runtime)
	renderer.ProductTitle("Health check")
	renderer.Section("Configuration")
	renderer.Status(presentation.OK, "Configuration file", "Healthy")
	renderer.Section("Client")
	verificationCommands := []string{}
	for _, client := range invocation.Synchronizer(runtime).ClientIDs() {
		adapter := cfg.Clients[client]
		if !adapter.Enabled {
			renderer.Status(presentation.Info, invocation.Title(client), "Disabled")
			continue
		}
		result, _ := evaluation.client(client)
		if result.resolveErr != nil {
			return invocation.Problem(runtime, invocation.Title(client)+" binding cannot be resolved", result.resolveErr.Error(), invocation.Title(client)+" cannot determine which Profile to use.", "aigw use --for "+client+" <profile>", result.resolveErr)
		}
		if result.credentialErr != nil {
			instruction, _ := credential.TokenRecovery(runtime.Secrets, result.runtime.AccountID)
			return invocation.Problem(
				runtime,
				invocation.Title(client)+" account token is unavailable",
				"Account "+result.runtime.AccountID+" has no available Token.",
				invocation.Title(client)+" cannot authenticate to its selected endpoint.",
				instruction,
				fmt.Errorf("%s account token unavailable: %w", client, result.credentialErr),
			)
		}
		if !result.adapter {
			impact := invocation.Title(client) + " cannot receive its AIGW Profile, Token, or configuration projection."
			return invocation.Problem(runtime, invocation.Title(client)+" projection is not ready", result.issue, impact, result.fix, fmt.Errorf("%s projection not ready", client))
		}
		if !result.runtime.UsesAIGWCredentialStore() {
			renderer.Status(presentation.OK, invocation.Title(client), result.runtime.ProfileLabel+" · Local projection checked")
			detail := "Client-owned authentication requires an explicit live verification"
			if result.runtime.CredentialCommand != "" {
				detail = "External credential helper requires an explicit live verification"
			}
			renderer.Detail(detail)
			verificationCommands = append(verificationCommands, result.fix)
			continue
		}
		diagnostic := result.diagnostic
		if diagnostic.Kind != diagnostics.Healthy {
			evidence := diagnostic.Detail
			if diagnostic.HTTPStatus != 0 {
				evidence = fmt.Sprintf("HTTP %d", diagnostic.HTTPStatus)
				if diagnostic.Detail != "" {
					evidence += " · " + diagnostic.Detail
				}
			}
			return invocation.Problem(runtime, diagnostic.Summary, evidence, invocation.Title(client)+" is unavailable.", diagnostic.Fix, fmt.Errorf("%s diagnostic kind %s", client, diagnostic.Kind))
		}
		renderer.Status(presentation.OK, invocation.Title(client), result.runtime.ProfileLabel+" · Endpoint checked")
		if diagnostic.RecoveredTransient {
			renderer.Detail(invocation.Title(client) + " authentication recovered after a transient response")
		}
		if endpointTransport(result.runtime.Endpoint) == endpointTransportExternalLoopback {
			renderer.Detail(invocation.Title(client) + " uses a loopback endpoint; AIGW does not manage the endpoint runtime")
		}
	}
	renderer.Section("Result")
	if len(evaluation.clients) == 0 {
		renderer.Success("Configuration is healthy; no clients are enabled")
		return nil
	}
	// Passing this command establishes only the checks it actually performed.
	// Endpoint diagnostics do not execute a model or a real client.
	renderer.Success("All enabled client checks passed")
	renderer.Detail("Model inference and real-client execution were not verified")
	for _, command := range verificationCommands {
		renderer.Next(command)
	}
	return nil
}
