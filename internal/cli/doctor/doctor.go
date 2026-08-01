// Package doctor owns diagnostic collection and its human-safe projection.
package doctor

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"aigw-cli/internal/claude"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/secrets"
	"github.com/spf13/cobra"
)

// Dependencies are the capabilities required by the diagnostic command.
type Dependencies struct {
	Config         configuration.Store
	Secrets        secrets.Store
	Env            []string
	Out            io.Writer
	Color          bool
	Width          int
	ClaudeLauncher claude.Launcher
	RenderOut      io.Writer
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
		RunE: func(_ *cobra.Command, _ []string) error {
			checks := Collect(deps)
			if jsonMode {
				enc := json.NewEncoder(deps.Out)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"checks": checks, "ok": AllOK(checks)})
			}
			r := presentation.NewWithWidth(renderWriter(deps), deps.Color, deps.Width)
			r.ProductTitle("Detailed diagnostics")
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

func renderWriter(deps Dependencies) io.Writer {
	if deps.RenderOut != nil {
		return deps.RenderOut
	}
	return deps.Out
}

// Collect evaluates configuration, credentials, launchers, and projections.
func Collect(deps Dependencies) []Check {
	checks := []Check{}
	if names := ForbiddenClientTokenEnvironment(deps.Env); len(names) > 0 {
		checks = append(checks, Check{
			Name:   "environment:client-token",
			Detail: "global client token environment variables are set: " + strings.Join(names, ", "),
			Fix:    "remove them from the parent environment; AIGW injects Claude credentials only through its launcher",
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
	for _, name := range sortedDoctorAccountNames(cfg) {
		ok := deps.Secrets.Has(name)
		fix := ""
		if !ok {
			fix = "run `aigw rotate " + name + "`"
		}
		detail := "missing"
		if ok {
			detail = "available"
		}
		checks = append(checks, Check{"secret:" + name, ok, detail, fix})
	}
	for _, client := range configuration.AdmittedClientIDs() {
		adapter := cfg.Adapters[client]
		detail := "disabled"
		ok := true
		fix := ""
		if adapter.Enabled {
			detail = "enabled"
			if adapter.Executable == "" {
				ok = false
				detail = "enabled but executable is missing"
				fix = "run `aigw repair`"
			} else if client == configuration.ClientClaude {
				ready, launcherErr := deps.ClaudeLauncher.ClaudeLauncherReady()
				if launcherErr != nil {
					ok = false
					detail = launcherErr.Error()
					fix = "run `aigw repair`"
				} else if !ready {
					ok = false
					detail = "enabled but AIGW-managed Claude launcher is missing"
					fix = "run `aigw repair`"
				}
			} else if len(adapter.Targets) == 0 {
				ok = false
				detail = "enabled but no Codex config target is configured"
				fix = "run `aigw repair`"
			}
		}
		checks = append(checks, Check{"adapter:" + client, ok, detail, fix})
	}
	checks = append(checks, codexProjectionChecks(cfg)...)
	checks = append(checks, claudeLauncherChecks(deps, cfg)...)
	return checks
}

func codexProjectionChecks(cfg configuration.Config) []Check {
	adapter := cfg.Adapters[configuration.ClientCodex]
	if !adapter.Enabled {
		return nil
	}
	runtime, _, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		return []Check{{"projection:codex", false, err.Error(), "run `aigw use <codex-profile> --for codex`"}}
	}
	checks := make([]Check, 0, len(adapter.Targets))
	for index, target := range adapter.Targets {
		check := Check{Name: fmt.Sprintf("codex:target-%d", index+1), OK: true, Detail: "profile " + runtime.ProfileID}
		if err := codex.ValidateConfig(target, runtime); err != nil {
			check.OK = false
			check.Detail = err.Error()
			check.Fix = "run `aigw sync` to reconcile this target"
		}
		checks = append(checks, check)
	}
	return checks
}

func claudeLauncherChecks(deps Dependencies, cfg configuration.Config) []Check {
	if !cfg.Adapters[configuration.ClientClaude].Enabled {
		return nil
	}
	ok, err := deps.ClaudeLauncher.ClaudeLauncherReady()
	check := Check{Name: "launcher:claude", OK: ok}
	if err != nil {
		check.Detail = err.Error()
		check.Fix = "run `aigw repair`"
	} else if ok {
		check.Detail = "AIGW managed launcher"
	} else {
		check.Detail = "AIGW managed Claude launcher is missing"
		check.Fix = "run `aigw repair`"
	}
	checks := []Check{check}
	if !ok {
		return checks
	}
	active, activationErr := deps.ClaudeLauncher.ClaudeActivationReady()
	activation := Check{Name: "path:claude", OK: active}
	if activationErr != nil {
		activation.Detail = activationErr.Error()
		activation.Fix = "run `aigw repair`"
	} else if active {
		activation.Detail = "AIGW-managed shell PATH activation"
	} else {
		activation.Detail = "AIGW-managed Claude PATH activation is missing"
		activation.Fix = "run `aigw repair`"
	}
	return append(checks, activation)
}

func sortedDoctorAccountNames(cfg configuration.Config) []string {
	names := make([]string, 0, len(cfg.Accounts))
	for name := range cfg.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Label maps a diagnostic identity to a stable human label.
func Label(name string) string {
	switch {
	case name == "environment:client-token":
		return "Client token environment"
	case name == "config":
		return "Local configuration"
	case strings.HasPrefix(name, "secret:"):
		return "System secret"
	case name == "adapter:claude":
		return "Claude adapter"
	case name == "adapter:codex":
		return "Codex adapter"
	case name == "launcher:claude":
		return "Claude launcher"
	case name == "path:claude":
		return "Claude PATH activation"
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
		if strings.Contains(detail, "executable is missing") {
			return "Enabled, but no executable is configured"
		}
		if strings.Contains(detail, "no Codex config target") {
			return "Enabled, but no Codex configuration file is configured"
		}
	case name == "launcher:claude":
		if check.OK {
			return "AIGW-managed Claude launcher is ready"
		}
		if strings.Contains(detail, "is missing") {
			return "AIGW-managed Claude launcher is missing"
		}
	case name == "path:claude":
		if check.OK {
			return "AIGW-managed Claude PATH activation is ready"
		}
		if strings.Contains(detail, "is missing") {
			return "Claude PATH activation is missing"
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
	case "run `aigw setup`":
		return "aigw setup"
	case "run `aigw repair`":
		return "aigw repair"
	case "run `aigw sync` to reconcile this target":
		return "aigw sync"
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
