package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
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
		Short: "Diagnose configuration, secrets, and adapters",
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
			}
			if jsonMode {
				enc := json.NewEncoder(app.Out)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"checks": checks, "ok": allChecksOK(checks)})
			}
			for _, check := range checks {
				mark := "OK"
				if !check.OK {
					mark = "FAIL"
				}
				fmt.Fprintf(app.Out, "%-4s %-20s %s\n", mark, check.Name, check.Detail)
				if check.Fix != "" {
					fmt.Fprintf(app.Out, "     Fix: %s\n", check.Fix)
				}
			}
			if !allChecksOK(checks) {
				return fmt.Errorf("doctor found problems")
			}
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
