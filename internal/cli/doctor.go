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
	"DMXAPI_TOKEN",
	"DMX_API_TOKEN",
	"OPENAI_API_KEY",
}

func newDoctorCommand(app *App) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "查看配置、密钥与适配器的详细诊断",
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
				for _, client := range []string{domain.ClientClaude, domain.ClientCodex} {
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
					profile, _, resolveErr := cfg.Resolve(domain.ClientCodex, "")
					if resolveErr != nil {
						checks = append(checks, doctorCheck{"projection:codex", false, resolveErr.Error(), "run `aigw use <codex-profile> --for codex`"})
					} else {
						for index, target := range adapter.Targets {
							name := fmt.Sprintf("codex:target-%d", index+1)
							err := adapters.ValidateCodexConfig(target, profile)
							check := doctorCheck{Name: name, OK: err == nil, Detail: "profile " + profile.ID}
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
			r.Title("AIGW", "详细诊断")
			r.Section("检查项")
			for _, check := range checks {
				state := presentation.OK
				if !check.OK {
					state = presentation.Fail
				}
				r.Status(state, check.Name, check.Detail)
				if check.Fix != "" {
					r.Detail("修复：" + check.Fix)
				}
			}
			if !allChecksOK(checks) {
				r.Next("aigw repair")
				return presented(fmt.Errorf("doctor found problems"))
			}
			r.Section("结果")
			r.Success("未发现问题")
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "emit machine-readable JSON")
	return cmd
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
