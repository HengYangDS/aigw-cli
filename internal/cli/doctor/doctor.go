// Package doctor owns diagnostic collection and its human-safe projection.
package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	clientdomain "aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/presentation"
	domainreadiness "aigw-cli/internal/readiness"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/synchronization"
	"github.com/spf13/cobra"
)

// Dependencies are the capabilities required by the diagnostic command.
type Dependencies struct {
	Config    configuration.Store
	Secrets   secrets.Store
	Clients   synchronization.Synchronizer
	Env       []string
	Inspect   func(configuration.Config) map[string]domainreadiness.Client
	Out       io.Writer
	Color     bool
	Width     int
	RenderOut io.Writer
}

// Check is one machine-readable diagnostic result.
type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

var forbiddenClientTokenEnvironmentNames = []string{
	"ANTHROPIC_AUTH_TOKEN",
	"ANTHROPIC_API_KEY",
	"OPENAI_API_KEY",
}

// NewCommand constructs the diagnostic command.
func NewCommand(deps Dependencies) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Show detailed diagnostics for configuration, secrets, and adapters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			credentialBackend, backendErr := secrets.Inspect(deps.Secrets)
			checks := Collect(cmd.Context(), deps)
			if backendErr != nil {
				checks = append([]Check{{
					Name:   "credential:backend",
					Detail: "credential backend is unavailable: " + backendErr.Error(),
					Fix:    credentialBackend.RecoveryAction,
				}}, checks...)
			}
			clients := inspectClients(deps)
			if jsonMode {
				result := map[string]any{
					"checks":             checks,
					"clients":            clients,
					"credential_backend": credentialBackend,
					"ok":                 AllOK(checks),
				}
				enc := json.NewEncoder(deps.Out)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			r := presentation.NewWithWidth(renderWriter(deps), deps.Color, deps.Width)
			r.ProductTitle("Detailed diagnostics")
			renderClients(r, clients)
			r.Section("Checks")
			for _, check := range checks {
				state := presentation.OK
				if !check.OK {
					state = presentation.Fail
				}
				r.Status(state, Label(check.Name), Detail(check))
				if check.Fix != "" {
					r.Detail("Fix: " + Fix(check))
				}
			}
			if !AllOK(checks) {
				r.Next(NextAction(checks))
				if err := r.Err(); err != nil {
					return err
				}
				return fmt.Errorf("doctor found problems")
			}
			r.Section("Result")
			r.Success("No problems found")
			return r.Err()
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	return cmd
}

func inspectClients(deps Dependencies) map[string]domainreadiness.Client {
	if deps.Inspect == nil {
		return map[string]domainreadiness.Client{}
	}
	cfg, err := deps.Config.Load()
	if err != nil {
		return map[string]domainreadiness.Client{}
	}
	clients := deps.Inspect(cfg)
	return clients
}

func renderClients(renderer *presentation.Renderer, clients map[string]domainreadiness.Client) {
	if len(clients) == 0 {
		return
	}
	renderer.Section("Clients")
	for _, spec := range configuration.AdmittedClientSpecs() {
		client, ok := clients[spec.ID]
		if !ok {
			continue
		}
		state := presentation.Info
		switch client.State {
		case domainreadiness.Ready:
			state = presentation.OK
		case domainreadiness.Degraded, domainreadiness.Invalid, domainreadiness.Unavailable:
			state = presentation.Warn
		}
		renderer.Status(state, spec.Label, client.State.Label())
	}
}

func renderWriter(deps Dependencies) io.Writer {
	if deps.RenderOut != nil {
		return deps.RenderOut
	}
	return deps.Out
}

// Collect evaluates configuration, credentials, client executables, and
// projections.
func Collect(ctx context.Context, deps Dependencies) []Check {
	checks := []Check{}
	if names := ForbiddenClientTokenEnvironment(deps.Env); len(names) > 0 {
		checks = append(checks, Check{
			Name:   "environment:client-token",
			Detail: "global client token environment variables are set: " + strings.Join(names, ", "),
			Fix:    "remove them from the parent environment; AIGW supplies credentials only to explicit client verification processes",
		})
	} else {
		checks = append(checks, Check{
			Name:   "environment:client-token",
			OK:     true,
			Detail: "no global client token environment variables",
		})
	}
	cfg, err := deps.Config.Load()
	if err != nil {
		return append(checks, Check{"config", false, err.Error(), "inspect or restore " + deps.Config.Path()})
	}
	if len(cfg.Profiles) == 0 {
		checks = append(checks, Check{"config", false, "not configured", "run `aigw setup`"})
	} else {
		checks = append(checks, Check{"config", true, "valid", ""})
	}
	for _, name := range cfg.RequiredAccountTokenIDs() {
		ok, observationErr := deps.Secrets.Exists(name)
		if observationErr != nil {
			checks = append(checks, Check{"secret:" + name, false, "credential backend failed: " + observationErr.Error(), "inspect the selected credential backend"})
			continue
		}
		fix := ""
		if !ok {
			fix, _ = credential.TokenRecovery(deps.Secrets, name)
		}
		detail := "missing"
		if ok {
			detail = "available"
		}
		checks = append(checks, Check{"secret:" + name, ok, detail, fix})
	}
	checks = append(checks, adapterChecks(ctx, deps.Clients, cfg)...)
	return checks
}

func adapterChecks(ctx context.Context, clients synchronization.Synchronizer, cfg configuration.Config) []Check {
	checks := make([]Check, 0)
	for _, clientID := range clients.ClientIDs() {
		adapter := cfg.Adapters[clientID]
		if !adapter.Enabled {
			checks = append(checks, Check{Name: "adapter:" + clientID, OK: true, Detail: "disabled"})
			continue
		}
		runtime, err := cfg.ResolveRuntime(clientID, "")
		if err != nil {
			checks = append(checks, Check{Name: "projection:" + clientID, Detail: err.Error(), Fix: "run `aigw use <" + clientID + "-profile>`"})
			continue
		}
		status := clients.Inspect(ctx, cfg, clientID, runtime, clientdomain.InspectionOptions{})
		adapterReady := status.Ready || len(status.Checks) > 0
		detail, fix := "enabled", ""
		if !adapterReady {
			detail, fix = status.Issue, commandFix(status.RepairAction)
		}
		checks = append(checks, Check{Name: "adapter:" + clientID, OK: adapterReady, Detail: detail, Fix: fix})
		for _, observation := range status.Checks {
			checks = append(checks, Check{
				Name: observation.ID, OK: observation.Ready, Detail: observation.Detail,
				Fix: commandFix(observation.RepairAction),
			})
		}
	}
	return checks
}

func commandFix(action string) string {
	if action == "" {
		return ""
	}
	return "run `" + action + "`"
}

// Label maps a diagnostic identity to a stable human label.
func Label(name string) string {
	switch {
	case name == "environment:client-token":
		return "Client token environment"
	case name == "config":
		return "Local configuration"
	case name == "credential:backend":
		return "Credential backend"
	case strings.HasPrefix(name, "secret:"):
		return "System secret"
	case name == "adapter:claude":
		return "Claude adapter"
	case name == "adapter:codex":
		return "Codex adapter"
	case name == "projection:codex":
		return "Codex route"
	case strings.HasPrefix(name, "codex:target-"):
		return "Codex configuration target " + strings.TrimPrefix(name, "codex:target-")
	default:
		return "Other check"
	}
}

// Detail maps machine diagnostics to non-sensitive human text.
func Detail(check Check) string {
	name, detail := check.Name, check.Detail
	switch {
	case name == "environment:client-token":
		if check.OK {
			return "No global client token environment variables detected"
		}
		if names, ok := strings.CutPrefix(detail, "global client token environment variables are set: "); ok {
			return "Global client token environment variables detected: " + names
		}
		return "Global client token environment variables detected"
	case name == "config":
		if detail == "valid" {
			return "Configuration is valid"
		}
		if detail == "not configured" {
			return "First-time setup is incomplete"
		}
		return "Cannot read or validate configuration"
	case name == "credential:backend":
		return "Credential storage is unavailable"
	case strings.HasPrefix(name, "secret:"):
		account := strings.TrimPrefix(name, "secret:")
		if check.OK {
			return account + " · available"
		}
		return account + " · missing"
	case name == "adapter:claude" || name == "adapter:codex":
		if check.OK {
			return doctorTitle(detail)
		}
		if strings.Contains(detail, "executable is missing") || strings.Contains(detail, "executable is not configured") {
			return "Enabled, but no executable is configured"
		}
		if strings.Contains(detail, "no Codex config target") || strings.Contains(detail, "configuration target is missing") {
			return "Enabled, but no Codex configuration file is configured"
		}
	case name == "projection:codex":
		return "Current Codex route cannot be resolved"
	case strings.HasPrefix(name, "codex:target-"):
		if check.OK {
			return "Matches the current route"
		}
		return "Does not match the current route"
	}
	if check.OK {
		return "Healthy"
	}
	return "Check failed"
}

func doctorTitle(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

// Fix returns a safe actionable command without exposing local paths.
func Fix(check Check) string {
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
	return "aigw doctor --json"
}

// ForbiddenClientTokenEnvironment returns forbidden names without their values.
func ForbiddenClientTokenEnvironment(values []string) []string {
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

// AllOK reports whether every diagnostic passed.
func AllOK(checks []Check) bool {
	for _, check := range checks {
		if !check.OK {
			return false
		}
	}
	return true
}

// NextAction selects the smallest safe action supported by every failure.
func NextAction(checks []Check) string {
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
		action := Fix(check)
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
