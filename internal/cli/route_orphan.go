package cli

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/platform"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/recovery"
)

func newRouteRecoverOrphanCommand(app *App) *cobra.Command {
	var dryRun, jsonMode, confirmHostIdle, ackUnsetExternalSelection bool
	var caseID string
	cmd := &cobra.Command{
		Use:   "recover-orphan <air>",
		Short: "Quarantine and remove one exact unattributed Air projection",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if args[0] != "air" {
				return errors.New("only `aigw route recover-orphan air` is admitted")
			}
			return runAirRecoverOrphan(app, dryRun, jsonMode, caseID, confirmHostIdle, ackUnsetExternalSelection)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview the exact case without writing recovery state")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the secret-free result as JSON")
	cmd.Flags().StringVar(&caseID, "case-id", "", "Exact case ID returned by the recovery preview")
	cmd.Flags().BoolVar(&confirmHostIdle, "confirm-host-idle", false, "Attest that Air is idle; this command never probes, starts, or stops it")
	cmd.Flags().BoolVar(&ackUnsetExternalSelection, "ack-unset-external-selection", false, "Acknowledge that recovery writes no replacement JetBrains selection")
	return cmd
}

func newRouteSettleCommand(app *App) *cobra.Command {
	var dryRun, jsonMode bool
	var caseID string
	cmd := &cobra.Command{
		Use:   "settle <air>",
		Short: "Settle one Air recovery case after a host roundtrip",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if args[0] != "air" {
				return errors.New("only `aigw route settle air` is admitted")
			}
			return runAirSettle(app, dryRun, jsonMode, caseID)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview settlement without changing AIGW recovery state")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the secret-free result as JSON")
	cmd.Flags().StringVar(&caseID, "case-id", "", "Active Air recovery case ID")
	return cmd
}

func runAirRecoverOrphan(app *App, dryRun, jsonMode bool, caseID string, confirmHostIdle, ackUnsetExternalSelection bool) error {
	air, standalone, err := resolveAirRecoverySurfaces(app)
	if err != nil {
		return err
	}
	cfg, err := app.Config.Load()
	if err != nil {
		return err
	}
	if contains(cfg.Adapters[domain.ClientCodex].Targets, air.ConfigPath) {
		return errors.New("Air remains an AIGW target; resolve target membership before orphan recovery")
	}
	store, err := airRecoveryStore(app)
	if err != nil {
		return err
	}
	options := recovery.AirRecoverOptions{AirPath: air.ConfigPath, StandalonePath: standalone.ConfigPath, CaseID: caseID}
	plan, err := store.PlanAirOrphanRecovery(options)
	if err != nil {
		return err
	}
	if dryRun {
		return renderAirRecoveryPlan(app, jsonMode, plan)
	}
	if caseID == "" {
		return errors.New("Air orphan recovery requires --case-id from the exact preview")
	}
	if !confirmHostIdle {
		return errors.New("Air orphan recovery requires --confirm-host-idle; no process was probed or stopped")
	}
	if !ackUnsetExternalSelection {
		return errors.New("Air orphan recovery requires --ack-unset-external-selection")
	}
	receipt, err := store.RecoverAirOrphan(options)
	if err != nil {
		return err
	}
	return renderAirRecoveryReceipt(app, jsonMode, receipt)
}

func runAirSettle(app *App, dryRun, jsonMode bool, caseID string) error {
	air, standalone, err := resolveAirRecoverySurfaces(app)
	if err != nil {
		return err
	}
	if caseID == "" {
		return errors.New("Air settlement requires --case-id")
	}
	store, err := airRecoveryStore(app)
	if err != nil {
		return err
	}
	options := recovery.AirSettleOptions{AirPath: air.ConfigPath, StandalonePath: standalone.ConfigPath, CaseID: caseID}
	plan, err := store.PlanAirSettlement(options)
	if err != nil {
		return err
	}
	if dryRun {
		return renderAirSettlementPlan(app, jsonMode, plan)
	}
	receipt, err := store.SettleAir(options)
	if err != nil {
		return err
	}
	if err := renderAirSettlementReceipt(app, jsonMode, receipt); err != nil {
		return err
	}
	if receipt.State == recovery.AirRecoveryStateQuarantined {
		err := errors.New("Air recovery remains quarantined after an unexpected host roundtrip")
		if jsonMode {
			return presented(err)
		}
		return err
	}
	return nil
}

func resolveAirRecoverySurfaces(app *App) (discovery.Surface, discovery.Surface, error) {
	air, discovered, err := resolveAirSurface(app)
	if err != nil {
		return discovery.Surface{}, discovery.Surface{}, err
	}
	standalone, ok := discovered.Surface(discovery.SurfaceCodexCLIStandalone)
	if !ok || !standalone.Present || standalone.ConfigPath == "" {
		return discovery.Surface{}, discovery.Surface{}, errors.New("the standalone Codex reference surface is unavailable")
	}
	return air, standalone, nil
}

func airRecoveryStore(app *App) (recovery.Store, error) {
	dataDir := app.DataDir
	if dataDir == "" {
		goos := app.GOOS
		if goos == "" {
			goos = runtime.GOOS
		}
		resolved, err := platform.DataDirFor(goos, environmentMap(app.Env))
		if err == nil {
			dataDir = resolved
		} else if app.Config.Path() != "" {
			dataDir = filepath.Dir(app.Config.Path())
		} else {
			return recovery.Store{}, errors.New("AIGW recovery data directory is unavailable")
		}
	}
	return recovery.NewStore(filepath.Join(dataDir, "recovery")), nil
}

func renderAirRecoveryPlan(app *App, jsonMode bool, plan recovery.AirRecoveryPlan) error {
	if jsonMode {
		return encodeAirRouteJSON(app, plan)
	}
	r := renderer(app)
	r.Title("AIGW", "Air orphan recovery preview")
	r.Row("Surface", plan.SurfaceID)
	r.Row("State", plan.State)
	r.Row("Action", plan.Action)
	r.Row("Case", plan.CaseID)
	r.Success("Preview made no persistent changes")
	r.Next("aigw route recover-orphan air --case-id " + plan.CaseID + " --confirm-host-idle --ack-unset-external-selection")
	return nil
}

func renderAirRecoveryReceipt(app *App, jsonMode bool, receipt recovery.AirRecoveryReceipt) error {
	if jsonMode {
		return encodeAirRouteJSON(app, receipt)
	}
	r := renderer(app)
	r.Title("AIGW", "Air orphan recovery")
	r.Row("Surface", receipt.SurfaceID)
	r.Row("State", receipt.State)
	r.Row("Case", receipt.CaseID)
	r.Success("The exact orphan was quarantined and removed without writing an external selection")
	r.Next("aigw route settle air --case-id " + receipt.CaseID + " --dry-run --json")
	return nil
}

func renderAirSettlementPlan(app *App, jsonMode bool, plan recovery.AirSettlementPlan) error {
	if jsonMode {
		return encodeAirRouteJSON(app, plan)
	}
	r := renderer(app)
	r.Title("AIGW", "Air recovery settlement preview")
	r.Row("Surface", plan.SurfaceID)
	r.Row("State", plan.State)
	r.Row("Action", plan.Action)
	r.Row("Case", plan.CaseID)
	r.Success("Preview made no persistent changes")
	return nil
}

func renderAirSettlementReceipt(app *App, jsonMode bool, receipt recovery.AirSettlementReceipt) error {
	if jsonMode {
		return encodeAirRouteJSON(app, receipt)
	}
	r := renderer(app)
	r.Title("AIGW", "Air recovery settlement")
	r.Row("Surface", receipt.SurfaceID)
	r.Row("State", receipt.State)
	r.Row("Case", receipt.CaseID)
	if receipt.State == recovery.AirRecoveryStateSettled {
		r.Success("The host roundtrip was accepted and the private quarantine payload was removed")
	} else {
		r.Text("The recovery remains quarantined; Air configuration was not changed")
	}
	return nil
}

func encodeAirRouteJSON(app *App, value any) error {
	encoder := json.NewEncoder(app.Out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
