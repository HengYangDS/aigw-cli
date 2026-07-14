package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/presentation"
)

type doctorCheck struct {
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

func newDoctorCommand(app *App) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Show detailed diagnostics for configuration, secrets, and adapters",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			checks := []doctorCheck{}
			if names := forbiddenClientTokenEnvironment(app.Env); len(names) > 0 {
				checks = append(checks, doctorCheck{
					Name:   "environment:client-token",
					OK:     false,
					Detail: "global client token environment variables are set: " + strings.Join(names, ", "),
					Fix:    "remove them from the parent environment; AIGW injects Claude credentials only through its shim",
				})
			} else {
				checks = append(checks, doctorCheck{
					Name:   "environment:client-token",
					OK:     true,
					Detail: "no global client token environment variables",
				})
			}
			cfg, err := app.Config.Load()
			if err != nil {
				checks = append(checks, doctorCheck{"config", false, err.Error(), "inspect or restore " + app.Config.Path()})
			} else {
				if len(cfg.Profiles) == 0 {
					checks = append(checks, doctorCheck{"config", false, "not configured", "run `aigw setup`"})
				} else {
					checks = append(checks, doctorCheck{"config", true, "valid", ""})
				}
				for _, name := range sortedAccountNames(cfg) {
					ok := app.Secrets.Has(name)
					fix := ""
					if !ok {
						fix = "run `aigw rotate " + name + "`"
					}
					checks = append(checks, doctorCheck{"secret:" + name, ok, map[bool]string{true: "available", false: "missing"}[ok], fix})
				}
				for _, client := range domain.AdmittedClientIDs() {
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
						} else if client == domain.ClientClaude {
							ready, shimErr := app.Shims.ClaudeShimReady()
							if shimErr != nil {
								ok = false
								detail = shimErr.Error()
								fix = "run `aigw repair`"
							} else if !ready {
								ok = false
								detail = "enabled but AIGW-managed Claude shim is missing"
								fix = "run `aigw repair`"
							}
						} else if len(adapter.Targets) == 0 {
							ok = false
							detail = "enabled but no Codex config target is configured"
							fix = "run `aigw repair`"
						}
					}
					checks = append(checks, doctorCheck{"adapter:" + client, ok, detail, fix})
				}
				if adapter := cfg.Adapters[domain.ClientCodex]; adapter.Enabled {
					runtime, _, resolveErr := cfg.ResolveRuntime(domain.ClientCodex, "")
					if resolveErr != nil {
						checks = append(checks, doctorCheck{"projection:codex", false, resolveErr.Error(), "run `aigw use <codex-profile> --for codex`"})
					} else {
						for index, target := range adapter.Targets {
							name := fmt.Sprintf("codex:target-%d", index+1)
							err := adapters.ValidateCodexConfig(target, runtime)
							check := doctorCheck{Name: name, OK: err == nil, Detail: "profile " + runtime.ProfileID}
							if err != nil {
								check.Detail = err.Error()
								check.Fix = "run `aigw sync` to reconcile this target"
							}
							checks = append(checks, check)
						}
					}
				}
				if adapter := cfg.Adapters[domain.ClientClaude]; adapter.Enabled {
					ok, shimErr := app.Shims.ClaudeShimReady()
					check := doctorCheck{Name: "shim:claude", OK: ok}
					if shimErr != nil {
						check.Detail = shimErr.Error()
						check.Fix = "run `aigw repair`"
					} else if ok {
						check.Detail = "AIGW managed shim"
					} else {
						check.Detail = "AIGW managed Claude shim is missing"
						check.Fix = "run `aigw repair`"
					}
					checks = append(checks, check)
					if ok {
						active, activationErr := app.Shims.ClaudeActivationReady()
						activation := doctorCheck{Name: "path:claude", OK: active}
						if activationErr != nil {
							activation.Detail = activationErr.Error()
							activation.Fix = "run `aigw repair`"
						} else if active {
							activation.Detail = "AIGW-managed shell PATH activation"
						} else {
							activation.Detail = "AIGW-managed Claude PATH activation is missing"
							activation.Fix = "run `aigw repair`"
						}
						checks = append(checks, activation)
					}
				}
			}
			if jsonMode {
				enc := json.NewEncoder(app.Out)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"checks": checks, "ok": allChecksOK(checks)})
			}
			r := renderer(app)
			r.Title("AIGW", "Detailed diagnostics")
			r.Section("Checks")
			for _, check := range checks {
				state := presentation.OK
				if !check.OK {
					state = presentation.Fail
				}
				r.Status(state, doctorCheckLabel(check.Name), doctorCheckDetail(check))
				if check.Fix != "" {
					r.Detail("Fix: " + doctorCheckFix(check))
				}
			}
			if !allChecksOK(checks) {
				r.Next(doctorNextAction(checks))
				return presented(fmt.Errorf("doctor found problems"))
			}
			r.Section("Result")
			r.Success("No problems found")
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	return cmd
}

func doctorCheckLabel(name string) string {
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
	case name == "shim:claude":
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

func doctorCheckDetail(check doctorCheck) string {
	name := check.Name
	detail := check.Detail
	switch {
	case name == "environment:client-token":
		if check.OK {
			return "No global client token environment variables detected"
		}
		const prefix = "global client token environment variables are set: "
		if names, ok := strings.CutPrefix(detail, prefix); ok {
			return "Global client token environment variables detected: " + names
		}
		return "Global client token environment variables detected"
	case name == "config":
		switch detail {
		case "valid":
			return "Configuration is valid"
		case "not configured":
			return "First-time setup is incomplete"
		default:
			return "Cannot read or validate configuration"
		}
	case strings.HasPrefix(name, "secret:"):
		account := strings.TrimPrefix(name, "secret:")
		if check.OK {
			return account + " · available"
		}
		return account + " · missing"
	case name == "adapter:claude" || name == "adapter:codex":
		if check.OK && detail == "enabled" {
			return "Enabled"
		}
		if check.OK && detail == "disabled" {
			return "Disabled"
		}
		if strings.Contains(detail, "executable is missing") {
			return "Enabled, but no executable is configured"
		}
		if strings.Contains(detail, "no Codex config target") {
			return "Enabled, but no Codex configuration file is configured"
		}
	case name == "shim:claude":
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

func doctorCheckFix(check doctorCheck) string {
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

func allChecksOK(checks []doctorCheck) bool {
	for _, check := range checks {
		if !check.OK {
			return false
		}
	}
	return true
}

func configNeedsSetup(checks []doctorCheck) bool {
	for _, check := range checks {
		if check.Name == "config" && !check.OK && check.Detail == "not configured" {
			return true
		}
	}
	return false
}

// doctorNextAction selects the smallest safe action supported by all failed
// checks. A single actionable drift should not be escalated into broad repair;
// mixed or unclassified failures deliberately fall back to repair.
func doctorNextAction(checks []doctorCheck) string {
	if configNeedsSetup(checks) {
		return "aigw setup"
	}
	actions := map[string]bool{}
	for _, check := range checks {
		if check.OK {
			continue
		}
		action := doctorCheckFix(check)
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
