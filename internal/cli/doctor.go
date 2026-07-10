package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/presentation"
)

type doctorCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

func newDoctorCommand(app *App) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "查看配置、密钥与适配器的详细诊断",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			checks := []doctorCheck{}
			cfg, err := app.Config.Load()
			if err != nil {
				checks = append(checks, doctorCheck{"config", false, err.Error(), "inspect or restore " + app.Config.Path()})
			} else {
				if len(cfg.Profiles) == 0 {
					checks = append(checks, doctorCheck{"config", false, "not configured", "run `aigw setup`"})
				} else {
					checks = append(checks, doctorCheck{"config", true, "valid", ""})
				}
				for name := range cfg.Profiles {
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
					if adapter.Enabled {
						detail = "enabled"
					}
					checks = append(checks, doctorCheck{"adapter:" + client, true, detail, ""})
				}
				if adapter := cfg.Adapters[domain.ClientClaude]; adapter.Enabled {
					discovered := app.Discovery.Discover()
					ok := discovered.ClaudeExecutable != ""
					check := doctorCheck{Name: "shim:claude", OK: ok}
					if ok {
						check.Detail = "discoverable on PATH"
					} else {
						check.Detail = "AIGW Claude shim is not discoverable on PATH"
						check.Fix = "run `aigw repair`; then open a new terminal if PATH was updated"
					}
					checks = append(checks, check)
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

func allChecksOK(checks []doctorCheck) bool {
	for _, check := range checks {
		if !check.OK {
			return false
		}
	}
	return true
}
