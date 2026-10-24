// Package readiness owns read-only status, health, and endpoint checks for
// selected AIGW client bindings.
package readiness

import (
	"errors"
	"fmt"
	"strings"

	clientactivation "aigw-cli/internal/activation"
	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/diagnostics"
	"aigw-cli/internal/presentation"
	domainreadiness "aigw-cli/internal/readiness"
	"aigw-cli/internal/secrets"

	"github.com/spf13/cobra"
)

// NewCheckCommand builds the read-only health check for every enabled client.
func NewCheckCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	var client string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check enabled clients; selected-model probes may use quota",
		Long: "Check every enabled client by default, or one with --for.\n" +
			"Inspect its binding, credential, and native projection.\n" +
			"For AIGW-managed Tokens, the default sends one bounded selected-model\n" +
			"inference request per eligible client and may use provider quota.\n" +
			"Use --endpoint-only for a model-free authenticated endpoint request.\n" +
			"This command does not execute a native client or change configuration;\n" +
			"use verify for real-client evidence.",
		Args: cobra.MatchAll(cobra.NoArgs, func(cmd *cobra.Command, _ []string) error {
			if cmd.Flags().Changed("for") && strings.TrimSpace(client) == "" {
				return fmt.Errorf("--for requires a non-empty client; run `aigw check --help`")
			}
			if client != "" && !configuration.IsAdmittedClient(client) {
				return fmt.Errorf("--for must be %s; run `aigw check --help`", configuration.AdmittedClientUsage())
			}
			return nil
		}),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if jsonMode {
				return runJSONCheck(cmd, runtime)
			}
			return RunCheck(cmd, runtime)
		},
	}
	cmd.Flags().StringVar(&client, "for", "", "Check one enabled client: "+configuration.AdmittedClientUsage()+"; omit for all")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	cmd.Flags().Bool("endpoint-only", false, "Use model-free endpoint authentication instead of inference")
	return cmd
}

type checkJSON struct {
	Clients        map[string]clientStatus `json:"clients"`
	EnabledClients int                     `json:"enabled_clients"`
	OK             bool                    `json:"ok"`
	State          domainreadiness.State   `json:"state,omitempty"`
	NextAction     string                  `json:"next_action,omitempty"`
	Error          string                  `json:"error,omitempty"`
}

func activationCheckIssue(activation clientactivation.Activation) (problem, evidence, jsonError string, cause error) {
	switch {
	case activation.State == domainreadiness.Unavailable:
		return "Credential metadata is unavailable",
			"The selected Account credential backend cannot be observed.",
			"Credential metadata is unavailable; no endpoint or model was checked",
			fmt.Errorf("credential metadata unavailable")
	case activation.EnabledClients != 0:
		evidence = "A Client Binding is selected, but its native projection is not available."
		jsonError = "No checked client has a native projection; no endpoint or model was checked"
		if len(activation.ClientCredentialPrerequisites) != 0 {
			evidence = "The selected Account Token is unavailable and its native projection is deferred."
			jsonError = "The selected Account Token and native projection are unavailable; no endpoint or model was checked"
		}
		return "Client projection is deferred", evidence, jsonError,
			fmt.Errorf("client projection is deferred")
	default:
		return "No client is enabled",
			"The imported Routes are available, but no Client Binding is active.",
			"No client is enabled; no endpoint or model was checked",
			fmt.Errorf("no enabled Client Bindings")
	}
}

type evaluatedClient struct {
	client              string
	runtime             configuration.Runtime
	resolveErr          error
	credentialErr       error
	credentialState     domainreadiness.State
	fix                 string
	checkPassed         bool
	issue               string
	adapter             bool
	endpointConfigured  bool
	nativeModelOverride bool
	diagnostic          diagnostics.Result
}

type checkEvaluation struct {
	clients []evaluatedClient
}

func selectedCheckScope(cmd *cobra.Command) diagnostics.Scope {
	endpointOnly, err := cmd.Flags().GetBool("endpoint-only")
	if err == nil && endpointOnly {
		return diagnostics.ScopeEndpoint
	}
	return diagnostics.ScopeInference
}

func selectedCheckClient(cmd *cobra.Command) string {
	flag := cmd.Flags().Lookup("for")
	if flag == nil {
		return ""
	}
	return flag.Value.String()
}

func checkConfigurationFor(cfg configuration.Config, client string) (configuration.Config, string, error) {
	if client == "" {
		return cfg, "", nil
	}
	if !cfg.Clients[client].Enabled {
		// Selecting a Route enables its binding; enablement alone needs an executable.
		action := "aigw use --for " + client + " <route>"
		if route := cfg.SelectedRoute(client); route != "" {
			action = "aigw use --for " + client + " " + route
		}
		return cfg, action, fmt.Errorf("%s is not enabled", configuration.ClientLabel(client))
	}
	scoped := cfg.Clone()
	for id, binding := range scoped.Clients {
		if id != client {
			binding.Enabled = false
			scoped.Clients[id] = binding
		}
	}
	return scoped, "", nil
}

func evaluateCheck(cmd *cobra.Command, runtime invocation.Context, cfg configuration.Config) checkEvaluation {
	var evaluation checkEvaluation
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
	result.nativeModelOverride = status.NativeModelOverride
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
		if !errors.Is(tokenErr, secrets.ErrNotFound) {
			result.credentialState = domainreadiness.Unavailable
			result.issue = "account token could not be read"
			result.fix = domainreadiness.CredentialBackendRecovery
			return result
		}
		result.credentialState = domainreadiness.Deferred
		result.issue = "account token is unavailable"
		result.fix = "aigw rotate " + clientRuntime.AccountID
		if instruction, writable := credential.TokenRecovery(runtime.Secrets, clientRuntime.AccountID); !writable {
			result.fix = instruction
		}
		return result
	}
	scope := selectedCheckScope(cmd)
	if result.nativeModelOverride {
		scope = diagnostics.ScopeEndpoint
	}
	result.diagnostic = diagnostics.ProbeBounded(cmd.Context(), runtime.HTTP, clientRuntime, token, scope)
	if result.diagnostic.Kind != diagnostics.Healthy {
		result.issue = result.diagnostic.Summary
		result.fix = result.diagnostic.Fix
	} else if result.nativeModelOverride {
		result.fix = "aigw verify --for claude"
	}
	result.checkPassed = result.issue == ""
	return result
}

func (e checkEvaluation) ok() bool {
	if len(e.clients) == 0 {
		return false
	}
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
	if len(cfg.Routes) == 0 {
		return writeJSONFailure(runtime, domainreadiness.Deferred, "not configured", "aigw setup", fmt.Errorf("not configured"))
	}
	selected := selectedCheckClient(cmd)
	cfg, action, err := checkConfigurationFor(cfg, selected)
	if err != nil {
		return writeJSONFailure(runtime, domainreadiness.Deferred, err.Error(), action, err)
	}
	activation := clientactivation.AssessActivation(cfg, runtime.Secrets)
	if activation.State == domainreadiness.Deferred || activation.State == domainreadiness.Unavailable {
		_, _, issue, cause := activationCheckIssue(activation)
		result := checkJSON{
			Clients:        inspectStatusClients(runtime, cfg, &activation, selected),
			EnabledClients: activation.EnabledClients,
			OK:             false,
			State:          activation.State,
			NextAction:     activation.NextActionFor(nil),
			Error:          issue,
		}
		if err := presentation.WriteJSON(runtime.Out, result); err != nil {
			return err
		}
		return presentation.Presented(cause)
	}
	evaluation := evaluateCheck(cmd, runtime, cfg)
	clients := checkedClientStatuses(runtime, cfg, &activation, evaluation, selected)
	result := checkJSON{
		Clients:        clients,
		EnabledClients: activation.EnabledClients,
		OK:             evaluation.ok(),
	}
	var failure error
	if !result.OK {
		result.NextAction, _ = failedCheckContinuation(&activation, evaluation, clients)
		failure = fmt.Errorf("one or more enabled client checks failed")
		result.Error = failure.Error()
	}
	if err := presentation.WriteJSON(runtime.Out, result); err != nil {
		return err
	}
	if failure != nil {
		// JSON is a protocol, not a prelude to human error rendering. Mark the
		// already-serialized failure as presented so the root command preserves
		// one valid JSON document on stdout while still returning a non-zero
		// result to callers.
		return presentation.Presented(failure)
	}
	return nil
}

func checkedClientStatuses(runtime invocation.Context, cfg configuration.Config, activation *clientactivation.Activation, evaluation checkEvaluation, selected string) map[string]clientStatus {
	clients := inspectStatusClients(runtime, cfg, activation, selected)
	for _, client := range evaluation.clients {
		status := clients[client.client]
		status.Route = client.runtime.RouteID
		status.Account = client.runtime.AccountID
		status.Authentication = client.runtime.Authentication
		status.EndpointConfigured = client.endpointConfigured
		status.ProjectionReady = client.adapter
		status.CheckPassed = new(bool)
		*status.CheckPassed = client.checkPassed
		status.DiagnosticKind = string(client.diagnostic.Kind)
		status.DiagnosticScope = client.diagnostic.Scope
		status.NativeModelOverride = client.nativeModelOverride
		status.Attempts = client.diagnostic.Attempts
		status.Retryable = client.diagnostic.Retryable
		if client.credentialErr != nil {
			status.State = client.credentialState
		}
		if !status.ProjectionDeferred {
			if client.issue != "" {
				status.Detail = client.issue
			}
			if client.fix != "" {
				status.NextAction = client.fix
			}
		}
		if client.diagnostic.Kind != "" && status.State == domainreadiness.Configured {
			status.Client = domainreadiness.WithProbe(status.Client, client.diagnostic)
		}
		if client.nativeModelOverride && client.checkPassed {
			status.NativeModelOverride = true
			status.NextAction = "aigw verify --for claude"
		}
		clients[client.client] = status
	}
	return clients
}

func failedCheckContinuation(activation *clientactivation.Activation, evaluation checkEvaluation, clients map[string]clientStatus) (action, clientID string) {
	failed := make([]domainreadiness.Client, 0, len(evaluation.clients))
	ids := make([]string, 0, len(evaluation.clients))
	for _, client := range evaluation.clients {
		if client.checkPassed {
			continue
		}
		failed = append(failed, clients[client.client].Client)
		ids = append(ids, client.client)
	}
	if len(ids) == 0 {
		return "aigw repair", ""
	}
	action = activation.NextActionFor(failed)
	for index, client := range failed {
		if client.NextAction == action {
			return action, ids[index]
		}
	}
	return action, ids[0]
}

func writeJSONFailure(runtime invocation.Context, state domainreadiness.State, message, fix string, cause error) error {
	result := checkJSON{Clients: map[string]clientStatus{}, State: state, NextAction: fix, Error: message}
	if err := presentation.WriteJSON(runtime.Out, result); err != nil {
		return err
	}
	return presentation.Presented(cause)
}

// RunCheck verifies selected client bindings, configured projections, and
// the performed endpoint or inference scope without mutating local state.
func RunCheck(cmd *cobra.Command, runtime invocation.Context) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Routes) == 0 {
		return invocation.Problem(runtime, "Not configured", "No Routes have been created.", "Cannot check, synchronize, or repair configuration that does not exist.", "aigw setup", fmt.Errorf("not configured"))
	}
	selected := selectedCheckClient(cmd)
	cfg, action, err := checkConfigurationFor(cfg, selected)
	if err != nil {
		return invocation.Problem(runtime, "Client is not enabled", err.Error(), "No endpoint or model was checked.", action, err)
	}
	activation := clientactivation.AssessActivation(cfg, runtime.Secrets)
	if activation.State == domainreadiness.Deferred || activation.State == domainreadiness.Unavailable {
		problem, evidence, _, cause := activationCheckIssue(activation)
		return invocation.Problem(runtime, problem, evidence, "No endpoint or model was checked.", activation.NextActionFor(nil), cause)
	}
	evaluation := evaluateCheck(cmd, runtime, cfg)
	if !evaluation.ok() {
		clients := checkedClientStatuses(runtime, cfg, &activation, evaluation, selected)
		action, clientID := failedCheckContinuation(&activation, evaluation, clients)
		if client, found := evaluation.client(clientID); found {
			field, _, _, err := checkedClientField(runtime, client, action)
			if err != nil {
				return err
			}
			invocation.Renderer(runtime).Rows(field)
		}
		return invocation.Problem(runtime, "Enabled client check failed", "At least one enabled client did not pass its applicable check scope.", "All-client readiness was not established.", action, fmt.Errorf("enabled client check failed"))
	}
	rows := []presentation.Field{}
	verificationCommands := []string{}
	inferenceUnverified := false
	for _, client := range invocation.Synchronizer(runtime).ClientIDs() {
		if selected != "" && client != selected {
			continue
		}
		adapter := cfg.Clients[client]
		if !adapter.Enabled {
			rows = append(rows, presentation.Field{State: presentation.Info, Label: configuration.ClientLabel(client), Value: "Disabled"})
			continue
		}
		result, _ := evaluation.client(client)
		field, command, unchecked, err := checkedClientField(runtime, result, "")
		if err != nil {
			return err
		}
		rows = append(rows, field)
		if command != "" {
			verificationCommands = append(verificationCommands, command)
		}
		inferenceUnverified = inferenceUnverified || unchecked
	}
	renderer := invocation.Renderer(runtime)
	renderer.ProductTitle("Health check")
	renderer.Section("Configuration")
	renderer.Rows(presentation.Field{State: presentation.OK, Label: "Configuration file", Value: "Healthy"})
	renderer.Section("Client")
	renderer.Rows(rows...)
	renderer.Section("Result")
	// Passing this command establishes only the scopes reported per client.
	if selected == "" {
		renderer.Success("All enabled client checks passed")
	} else {
		renderer.Success("Selected client check passed")
	}
	if inferenceUnverified {
		renderer.Detail("Model inference was not verified for endpoint-only or client-native checks")
	}
	renderer.Detail("Real-client execution was not verified")
	for _, command := range verificationCommands {
		renderer.Next(command)
	}
	return nil
}

// checkedClientField presents one enabled client's check as a row of the Client
// group, returning its explicit verification command and whether model
// inference remains unverified, or the Problem that stops the check.
func checkedClientField(runtime invocation.Context, result evaluatedClient, overrideAction string) (presentation.Field, string, bool, error) {
	client := result.client
	recommendedAction := func(fallback string) string {
		if overrideAction != "" {
			return overrideAction
		}
		return fallback
	}
	if result.resolveErr != nil {
		return presentation.Field{}, "", false, invocation.Problem(runtime, configuration.ClientLabel(client)+" binding cannot be resolved", result.resolveErr.Error(), configuration.ClientLabel(client)+" cannot determine which Route to use.", recommendedAction("aigw use --for "+client+" <route>"), result.resolveErr)
	}
	if result.credentialErr != nil {
		evidence := "Account " + result.runtime.AccountID + " has no available Token."
		if result.credentialState == domainreadiness.Unavailable {
			evidence = "The selected credential backend could not return Account " + result.runtime.AccountID + "'s Token; its presence cannot be inferred."
		}
		return presentation.Field{}, "", false, invocation.Problem(
			runtime,
			configuration.ClientLabel(client)+" "+result.issue,
			evidence,
			configuration.ClientLabel(client)+" cannot authenticate to its selected endpoint.",
			recommendedAction(result.fix),
			fmt.Errorf("%s account token unavailable: %w", client, result.credentialErr),
		)
	}
	if !result.adapter {
		impact := configuration.ClientLabel(client) + " cannot receive its AIGW Route, Token, or configuration projection."
		return presentation.Field{}, "", false, invocation.Problem(runtime, configuration.ClientLabel(client)+" projection is not ready", result.issue, impact, recommendedAction(result.fix), fmt.Errorf("%s projection not ready", client))
	}
	if !result.runtime.UsesAIGWCredentialStore() {
		detail := "Client-owned authentication requires an explicit live verification"
		if result.runtime.CredentialCommand != "" {
			detail = "External credential helper requires an explicit live verification"
		}
		return presentation.Field{
			State: presentation.OK, Label: configuration.ClientLabel(client), Value: result.runtime.RouteLabel + " · Local projection checked",
			Details: []string{detail},
		}, result.fix, true, nil
	}
	diagnostic := result.diagnostic
	if diagnostic.Kind != diagnostics.Healthy {
		evidence := "No HTTP response was received"
		if diagnostic.HTTPStatus != 0 {
			evidence = fmt.Sprintf("HTTP %d", diagnostic.HTTPStatus)
		} else if diagnostic.Attempts == 0 {
			evidence = "No request was sent"
		}
		impact := configuration.ClientLabel(client) + " is unavailable."
		if diagnostic.Kind == diagnostics.NetworkFailure {
			// AIGW selects the endpoint; it neither runs nor restarts it.
			impact += " AIGW owns this configuration, not the endpoint's lifecycle."
		}
		return presentation.Field{}, "", false, invocation.Problem(runtime, diagnostic.Summary, evidence, impact, recommendedAction(diagnostic.Fix), fmt.Errorf("%s diagnostic kind %s", client, diagnostic.Kind))
	}
	label := "Endpoint checked"
	inferenceUnverified := true
	if diagnostic.Scope == diagnostics.ScopeInference {
		label = "Inference checked"
		inferenceUnverified = false
	}
	field := presentation.Field{State: presentation.OK, Label: configuration.ClientLabel(client), Value: result.runtime.RouteLabel + " · " + label}
	command := ""
	if result.nativeModelOverride {
		field.Details = append(field.Details, "Claude Code uses a native model preference; this endpoint check does not verify that model")
		command = "aigw verify --for claude"
	}
	if endpointTransport(result.runtime.Endpoint) == endpointTransportExternalLoopback {
		field.Details = append(field.Details, configuration.ClientLabel(client)+" uses a loopback endpoint; AIGW does not manage the endpoint runtime")
	}
	return field, command, inferenceUnverified, nil
}
