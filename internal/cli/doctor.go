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
			r.Title("AIGW", "详细诊断")
			r.Section("检查项")
			for _, check := range checks {
				state := presentation.OK
				if !check.OK {
					state = presentation.Fail
				}
				r.Status(state, doctorCheckLabel(check.Name), doctorCheckDetail(check))
				if check.Fix != "" {
					r.Detail("修复：" + doctorCheckFix(check))
				}
			}
			if !allChecksOK(checks) {
				r.Next(doctorNextAction(checks))
				return presented(fmt.Errorf("doctor found problems"))
			}
			r.Section("结果")
			r.Success("未发现问题")
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "输出机器可读 JSON")
	return cmd
}

func doctorCheckLabel(name string) string {
	switch {
	case name == "environment:client-token":
		return "客户端令牌环境"
	case name == "config":
		return "本机配置"
	case strings.HasPrefix(name, "secret:"):
		return "系统密钥"
	case name == "adapter:claude":
		return "Claude 适配器"
	case name == "adapter:codex":
		return "Codex 适配器"
	case name == "shim:claude":
		return "Claude 启动器"
	case name == "path:claude":
		return "Claude PATH 激活"
	case name == "projection:codex":
		return "Codex 路由"
	case strings.HasPrefix(name, "codex:target-"):
		return "Codex 配置目标 " + strings.TrimPrefix(name, "codex:target-")
	default:
		return "其他检查"
	}
}

func doctorCheckDetail(check doctorCheck) string {
	name := check.Name
	detail := check.Detail
	switch {
	case name == "environment:client-token":
		if check.OK {
			return "未检测到全局客户端令牌环境变量"
		}
		const prefix = "global client token environment variables are set: "
		if names, ok := strings.CutPrefix(detail, prefix); ok {
			return "检测到全局客户端令牌环境变量：" + names
		}
		return "检测到全局客户端令牌环境变量"
	case name == "config":
		switch detail {
		case "valid":
			return "配置有效"
		case "not configured":
			return "尚未完成首次配置"
		default:
			return "无法读取或校验配置"
		}
	case strings.HasPrefix(name, "secret:"):
		account := strings.TrimPrefix(name, "secret:")
		if check.OK {
			return account + " · 可用"
		}
		return account + " · 缺失"
	case name == "adapter:claude" || name == "adapter:codex":
		if check.OK && detail == "enabled" {
			return "已启用"
		}
		if check.OK && detail == "disabled" {
			return "未启用"
		}
		if strings.Contains(detail, "executable is missing") {
			return "已启用，但未配置可执行文件"
		}
		if strings.Contains(detail, "no Codex config target") {
			return "已启用，但未配置 Codex 配置文件"
		}
	case name == "shim:claude":
		if check.OK {
			return "AIGW 管理的 Claude 启动器已就绪"
		}
		if strings.Contains(detail, "is missing") {
			return "AIGW 管理的 Claude 启动器缺失"
		}
	case name == "path:claude":
		if check.OK {
			return "AIGW 管理的 Claude PATH 激活已就绪"
		}
		if strings.Contains(detail, "is missing") {
			return "Claude PATH 激活缺失"
		}
	case name == "projection:codex":
		return "当前 Codex 路由无法解析"
	case strings.HasPrefix(name, "codex:target-"):
		if check.OK {
			return "与当前路由一致"
		}
		return "与当前路由不一致"
	}
	if check.OK {
		return "正常"
	}
	return "检查未通过"
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
		return "从启动当前终端的父环境中移除上述变量"
	}
	if strings.HasPrefix(check.Fix, "run `") && strings.HasSuffix(check.Fix, "`") {
		return strings.TrimSuffix(strings.TrimPrefix(check.Fix, "run `"), "`")
	}
	if strings.HasPrefix(check.Fix, "inspect or restore ") {
		return "检查或恢复本机配置文件"
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
