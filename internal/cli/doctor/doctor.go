// Package doctor owns diagnostic collection and its human-safe projection.
package doctor

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"aigw-cli/internal/app"
	clientactivation "aigw-cli/internal/app/activation"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// schemaVersion is the version of `aigw doctor`'s JSON document.
const schemaVersion = 1

// checkResult is one machine-readable diagnostic result. Target names the native file
// the check inspected relative to the home directory; it is omitted for a file
// outside the home directory, so diagnostics never expose a private path.
type checkResult struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
	Target string `json:"target,omitempty"`
	// Stage, Line and Column locate a configuration that could not be loaded.
	Stage  string `json:"stage,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
	// outsideHome records that the inspected file lies outside the home
	// directory, so the human label says so instead of naming the file.
	outsideHome bool
	// adapterAction records that Fix is a client adapter's repair action. The
	// adapter's actions are fixed text without local paths, so a sentence-form
	// one is shown as written.
	adapterAction bool
}

const credentialBackendUnavailable = "credential backend is unavailable"

type commandResult struct {
	CredentialBackend secret.BackendSelection           `json:"credential_backend"`
	Checks            []checkResult                     `json:"checks"`
	Clients           map[string]domainreadiness.Client `json:"clients"`
	OK                bool                              `json:"-"`
	OKScope           string                            `json:"ok_scope"`
	EnabledClients    int                               `json:"enabled_clients"`
	State             domainreadiness.State             `json:"state,omitempty"`
	NextAction        string                            `json:"next_action,omitempty"`
}

var forbiddenClientTokenEnvironmentNames = []string{
	"ANTHROPIC_AUTH_TOKEN",
	"ANTHROPIC_API_KEY",
	"OPENAI_API_KEY",
}

// inspector observes the canonical state of every admitted client.
type inspector func(app.Dependencies, domain.Config) map[string]domainreadiness.Client

// NewCommand constructs the diagnostic command.
func NewCommand(runtime invocation.Context) *cobra.Command {
	return newCommand(runtime, clientStates)
}

func newCommand(runtime invocation.Context, inspect inspector) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Show detailed diagnostics for configuration, secrets, and adapters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			operation := runtime.Dependencies
			operation.Secrets = secret.ObserveAvailability(runtime.Secrets)
			result := collectResult(cmd.Context(), operation, inspect)
			if jsonMode {
				if err := terminal.WriteDocument(runtime.Out, schemaVersion, result.OK, result); err != nil {
					return err
				}
				if !result.OK {
					return terminal.Presented(fmt.Errorf("doctor found problems"))
				}
				return nil
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Detailed diagnostics")
			admission := runtime.Config.Admission()
			renderClients(r, admission, result.Clients)
			r.Section("Checks")
			checks := make([]terminal.Field, 0, len(result.Checks))
			for _, check := range result.Checks {
				field := terminal.Field{State: terminal.OK, Label: checkLabel(admission, check), Value: checkDetail(admission, check)}
				if !check.OK {
					field.State = terminal.Fail
				}
				if check.Fix != "" {
					field.Details = []string{"Fix: " + safeFix(check)}
				}
				checks = append(checks, field)
			}
			r.Rows(checks...)
			if !result.OK {
				r.Next(result.NextAction)
				if err := r.Err(); err != nil {
					return err
				}
				return terminal.Presented(fmt.Errorf("doctor found problems"))
			}
			r.Section("Result")
			if result.NextAction != "" {
				message := "Selected client work remains"
				switch {
				case result.EnabledClients == 0:
					message = "No client is enabled"
				case result.State == domainreadiness.Deferred:
					message = "Selected client projection is deferred"
				}
				r.Rows(terminal.Field{
					State: terminal.Info, Label: "Client activation", Value: message,
					Details: []string{"Local diagnostics passed; no client endpoint or model was checked"},
				})
				r.Next(result.NextAction)
				return r.Err()
			}
			r.Success("No problems found")
			return r.Err()
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	return cmd
}

func collectResult(ctx context.Context, deps app.Dependencies, inspect inspector) commandResult {
	backend, backendErr := secret.Inspect(deps.Secrets)
	checks := collectChecks(ctx, deps)
	if backendErr != nil {
		checks = append([]checkResult{{
			Name:   "credential:backend",
			Detail: credentialBackendUnavailable,
			Fix:    backend.RecoveryAction,
		}}, checks...)
	}
	result := commandResult{
		CredentialBackend: backend,
		Checks:            checks,
		Clients:           inspectClients(deps, inspect),
		OK:                allOK(checks),
		OKScope:           "local_diagnostics",
	}
	if !result.OK {
		result.NextAction = nextAction(checks)
	}
	for _, spec := range deps.Config.Admission().Specs() {
		client, observed := result.Clients[spec.ID]
		if !observed {
			continue
		}
		switch client.State {
		case domainreadiness.Configured, domainreadiness.Deferred, domainreadiness.EndpointChecked, domainreadiness.InferenceChecked:
			continue
		case domainreadiness.Degraded, domainreadiness.Invalid, domainreadiness.Unavailable:
			result.OK = false
		default:
			result.OK = false
			client.NextAction = "aigw doctor --json"
		}
		if result.NextAction == "" {
			result.NextAction = client.NextAction
			if result.NextAction == "" {
				result.NextAction = "aigw doctor --json"
			}
		}
	}
	if cfg, err := deps.Config.Load(); err == nil {
		applyActivation(&result, cfg, deps.Secrets)
	}
	return result
}

func applyActivation(result *commandResult, cfg domain.Config, store secret.Store) {
	activation := clientactivation.AssessActivation(cfg, store)
	result.EnabledClients = activation.EnabledClients
	result.State = activation.State
	if activation.State == domainreadiness.Unavailable {
		result.OK = false
		result.NextAction = activation.NextActionFor(nil)
		return
	}
	if !result.OK {
		return
	}
	ordered := make([]domainreadiness.Client, 0, len(result.Clients))
	for _, spec := range cfg.Admission().Specs() {
		ordered = append(ordered, result.Clients[spec.ID])
	}
	if action := activation.NextActionFor(ordered); action != "aigw check" {
		result.NextAction = action
	}
}

// clientStates returns the canonical, secret-free local state of every
// admitted client without authenticating an endpoint or reading Token values.
func clientStates(deps app.Dependencies, cfg domain.Config) map[string]domainreadiness.Client {
	observed := app.InspectStatusClients(deps, cfg, nil, "")
	clients := make(map[string]domainreadiness.Client, len(observed))
	for client, status := range observed {
		clients[client] = status.Client
	}
	return clients
}

func inspectClients(deps app.Dependencies, inspect inspector) map[string]domainreadiness.Client {
	if inspect == nil {
		return map[string]domainreadiness.Client{}
	}
	cfg, err := deps.Config.Load()
	if err != nil {
		return map[string]domainreadiness.Client{}
	}
	return inspect(deps, cfg)
}

func renderClients(renderer *terminal.Renderer, admission domain.Admission, clients map[string]domainreadiness.Client) {
	if len(clients) == 0 {
		return
	}
	renderer.Section("Clients")
	rows := make([]terminal.Field, 0, len(clients))
	for _, spec := range admission.Specs() {
		client, ok := clients[spec.ID]
		if !ok {
			continue
		}
		state := terminal.Info
		switch client.State {
		case domainreadiness.EndpointChecked, domainreadiness.InferenceChecked:
			state = terminal.OK
		case domainreadiness.Configured, domainreadiness.Deferred:
			state = terminal.Info
		case domainreadiness.Degraded, domainreadiness.Invalid, domainreadiness.Unavailable:
			state = terminal.Warn
		}
		message := client.State.Label()
		if client.ProjectionDeferred {
			message += " · Native projection deferred"
		}
		if client.NativeModelOverride {
			message += " · " + client.Detail
		}
		row := terminal.Field{State: state, Label: spec.Label, Value: message}
		if client.NativeModelOverride && client.NextAction != "" {
			row.Details = []string{"Verify the native model: " + client.NextAction}
		}
		rows = append(rows, row)
	}
	renderer.Rows(rows...)
}

// collectChecks evaluates configuration, credentials, client executables, and
// projections.
func collectChecks(ctx context.Context, deps app.Dependencies) []checkResult {
	checks := []checkResult{}
	if names := forbiddenClientTokenEnvironment(deps.Env); len(names) > 0 {
		checks = append(checks, checkResult{
			Name:   "environment:client-token",
			Detail: "global client token environment variables are set: " + strings.Join(names, ", "),
			Fix:    "remove them from the parent environment; AIGW supplies credentials only to explicit client verification processes",
		})
	} else {
		checks = append(checks, checkResult{
			Name:   "environment:client-token",
			OK:     true,
			Detail: "no global client token environment variables",
		})
	}
	cfg, err := deps.Config.Load()
	if err != nil {
		failure := checkResult{Name: "config", Detail: "cannot read or validate configuration", Fix: "inspect or restore the local configuration file"}
		failure.Stage, failure.Line, failure.Column = terminal.ConfigurationFailure(err)
		return append(checks, failure)
	}
	if len(cfg.Routes) == 0 {
		checks = append(checks, checkResult{Name: "config", Detail: "not configured", Fix: "run `aigw setup`"})
	} else {
		checks = append(checks, checkResult{Name: "config", OK: true, Detail: "valid"})
	}
	for _, name := range cfg.RequiredAccountTokenIDs() {
		ok, observationErr := deps.Secrets.Exists(name)
		if observationErr != nil {
			checks = append(checks, checkResult{Name: "secret:" + name, Detail: credentialBackendUnavailable, Fix: "inspect the selected credential backend"})
			continue
		}
		fix := ""
		if !ok {
			fix, _ = secret.TokenRecovery(deps.Secrets, name)
		}
		detail := "missing"
		if ok {
			detail = "available"
		}
		checks = append(checks, checkResult{Name: "secret:" + name, OK: ok, Detail: detail, Fix: fix})
	}
	checks = append(checks, adapterChecks(ctx, deps, cfg, homeDirectory(deps.Env))...)
	return checks
}

func adapterChecks(ctx context.Context, deps app.Dependencies, cfg domain.Config, home string) []checkResult {
	checks := make([]checkResult, 0)
	projectionPrerequisites := clientactivation.ProjectionPrerequisites(cfg)
	for _, clientID := range deps.Synchronizer().ClientIDs() {
		adapter := cfg.Clients[clientID]
		if !adapter.Enabled {
			checks = append(checks, checkResult{Name: "adapter:" + clientID, OK: true, Detail: "disabled"})
			continue
		}
		runtime, err := cfg.ResolveRuntime(clientID, "")
		if err != nil {
			checks = append(checks, checkResult{Name: "projection:" + clientID, Detail: "selected client route cannot be resolved", Fix: "run `aigw use --for " + clientID + " <route>`"})
			continue
		}
		if projectionPrerequisites[clientID] != "" {
			continue
		}
		status := deps.InspectClient(ctx, cfg, clientID, runtime)
		adapterReady := status.Ready || len(status.Checks) > 0
		detail, fix := "enabled", ""
		if status.NativeModelOverride {
			detail = "enabled; native model preference requires explicit verification"
		}
		if !adapterReady {
			detail, fix = status.Issue, commandFix(status.RepairAction)
		}
		checks = append(checks, checkResult{Name: "adapter:" + clientID, OK: adapterReady, Detail: detail, Fix: fix, adapterAction: fix != ""})
		for _, observation := range status.Checks {
			target, outside := publicTarget(observation.Target, home)
			checks = append(checks, checkResult{
				Name: observation.ID, OK: observation.Ready, Detail: observation.Detail,
				Fix: commandFix(observation.RepairAction), Target: target, outsideHome: outside,
				adapterAction: observation.RepairAction != "",
			})
		}
	}
	return checks
}

// commandFix presents an adapter's repair action: a bare command is one to
// run, and a sentence-form action, which names its own commands, is kept as
// written.
func commandFix(action string) string {
	if !strings.HasPrefix(action, "aigw ") {
		return action
	}
	return "run `" + action + "`"
}

// safeFix returns a safe actionable command without exposing local paths.
func safeFix(check checkResult) string {
	switch check.Fix {
	case "aigw doctor":
		return "aigw doctor"
	case "run `aigw setup`":
		return "aigw setup"
	case "run `aigw repair`":
		return "aigw repair"
	}
	if strings.HasPrefix(check.Fix, "remove them from the parent environment") {
		return "Remove the variables above from the parent environment that launched this terminal"
	}
	if strings.HasPrefix(check.Fix, "run `") && strings.HasSuffix(check.Fix, "`") {
		return strings.TrimSuffix(strings.TrimPrefix(check.Fix, "run `"), "`")
	}
	if strings.HasPrefix(check.Fix, "inspect or restore ") {
		return "Inspect or restore the local configuration file"
	}
	if check.adapterAction {
		return check.Fix
	}
	return "aigw doctor --json"
}

// forbiddenClientTokenEnvironment returns forbidden names without their values.
func forbiddenClientTokenEnvironment(values []string) []string {
	present := map[string]bool{}
	for _, value := range values {
		name, _, ok := strings.Cut(value, "=")
		if !ok {
			continue
		}
		for _, forbidden := range forbiddenClientTokenEnvironmentNames {
			if name == forbidden {
				present[name] = true
			}
		}
	}
	names := make([]string, 0, len(present))
	for name := range present {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// allOK reports whether every diagnostic passed.
func allOK(checks []checkResult) bool {
	for _, check := range checks {
		if !check.OK {
			return false
		}
	}
	return true
}

// nextAction selects the smallest safe action supported by every failure.
func nextAction(checks []checkResult) string {
	for _, check := range checks {
		if check.Name == "config" && !check.OK && check.Detail == "not configured" {
			return "aigw setup"
		}
	}
	actions := map[string]bool{}
	for _, check := range checks {
		if check.OK {
			continue
		}
		action := safeFix(check)
		if action == "" || action == "aigw doctor --json" {
			return "aigw repair"
		}
		actions[action] = true
	}
	if len(actions) == 1 {
		for action := range actions {
			return action
		}
	}
	return "aigw repair"
}
