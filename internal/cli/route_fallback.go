package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

type routeChangePreview struct {
	DryRun              bool   `json:"dry_run"`
	SurfaceID           string `json:"surface_id"`
	Authority           string `json:"authority"`
	ProjectionMode      string `json:"projection_mode"`
	Action              string `json:"action"`
	ConfigurationAction string `json:"configuration_action,omitempty"`
}

var reconcileCodexConfigs = adapters.ReconcileCodexConfigs

func newRouteFallbackCommand(app *App) *cobra.Command {
	var dryRun, jsonMode, confirmHostIdle bool
	cmd := &cobra.Command{
		Use:   "fallback <air>",
		Short: "Stage an explicit non-default host fallback",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "air" {
				return errors.New("only `aigw route fallback air` is admitted")
			}
			return runAirFallback(cmd.Context(), app, dryRun, jsonMode, confirmHostIdle)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without writing configuration or authentication")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the secret-free preview as JSON")
	cmd.Flags().BoolVar(&confirmHostIdle, "confirm-host-idle", false, "Attest that Air is idle; this command never probes, starts, or stops it")
	return cmd
}

func newRouteRestoreCommand(app *App) *cobra.Command {
	var dryRun, jsonMode, confirmHostIdle bool
	cmd := &cobra.Command{
		Use:   "restore <air>",
		Short: "Remove an explicitly staged Air fallback",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if args[0] != "air" {
				return errors.New("only `aigw route restore air` is admitted")
			}
			return runAirRestore(app, dryRun, jsonMode, confirmHostIdle)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without writing configuration")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the secret-free preview as JSON")
	cmd.Flags().BoolVar(&confirmHostIdle, "confirm-host-idle", false, "Attest that Air is idle; this command never probes, starts, or stops it")
	return cmd
}

func newRouteRecoverCommand(app *App) *cobra.Command {
	var dryRun, jsonMode, confirmHostIdle bool
	cmd := &cobra.Command{
		Use:   "recover <air>",
		Short: "Recover Air from a verified stale AIGW full selection",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if args[0] != "air" {
				return errors.New("only `aigw route recover air` is admitted")
			}
			return runAirRecover(app, dryRun, jsonMode, confirmHostIdle)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without writing configuration")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the secret-free preview as JSON")
	cmd.Flags().BoolVar(&confirmHostIdle, "confirm-host-idle", false, "Attest that Air is idle; this command never probes, starts, or stops it")
	return cmd
}

func runAirFallback(ctx context.Context, app *App, dryRun, jsonMode, confirmHostIdle bool) error {
	surface, discovered, err := resolveAirSurface(app)
	if err != nil {
		return err
	}
	before, err := app.Config.Load()
	if err != nil {
		return err
	}
	runtime, err := validateAirFallbackPreconditions(app, before)
	if err != nil {
		return err
	}
	selected, err := airTopLevelSelectsAIGW(surface.ConfigPath)
	if err != nil {
		return err
	}
	if selected {
		return errors.New("Air currently selects AIGW; switch Air back to JetBrains AI before staging fallback")
	}
	identity, err := adapters.ReadCodexProjectionIdentity(surface.ConfigPath)
	if err != nil {
		return err
	}
	if identity.Present && identity.AttributionState == "recognized" && identity.ProjectionMode != adapters.CodexProjectionNamespacedFallback {
		return errors.New("Air has a full-selection AIGW projection; restore it before staging the namespaced fallback")
	}
	after := cloneConfig(before)
	adapter := after.Adapters[domain.ClientCodex]
	adapter.Targets = addSortedTarget(adapter.Targets, surface.ConfigPath)
	after.Adapters[domain.ClientCodex] = adapter
	beforeRefs, afterRefs, err := airFallbackRefs(discovered, before, after)
	if err != nil {
		return err
	}
	plans, err := adapters.PlanCodexReconciliation(beforeRefs, afterRefs, runtime)
	if err != nil {
		return err
	}
	plan, err := planForAirSurface(discovered, plans)
	if err != nil {
		return err
	}
	preview := routeChangePreview{
		DryRun:         true,
		SurfaceID:      discovery.SurfaceAirCodex,
		Authority:      discovery.AuthorityJetBrainsAI,
		ProjectionMode: adapters.CodexProjectionNamespacedFallback,
		Action:         plan.Action,
	}
	if dryRun {
		return renderRoutePreview(app, jsonMode, preview)
	}
	if !confirmHostIdle {
		return errors.New("Air fallback requires --confirm-host-idle; no process was probed or stopped")
	}
	if !identity.Present || identity.AttributionState != "recognized" || identity.ProjectionMode != adapters.CodexProjectionNamespacedFallback {
		if err := bindCodexAuthenticationTargets(ctx, app, after, []string{surface.ConfigPath}); err != nil {
			return fmt.Errorf("Air authentication failed before fallback staging: %w", err)
		}
	}
	return commitAirFallback(app, before, after, beforeRefs, afterRefs, runtime)
}

func runAirRestore(app *App, dryRun, jsonMode, confirmHostIdle bool) error {
	surface, discovered, err := resolveAirSurface(app)
	if err != nil {
		return err
	}
	before, err := app.Config.Load()
	if err != nil {
		return err
	}
	selected, err := airTopLevelSelectsAIGW(surface.ConfigPath)
	if err != nil {
		return err
	}
	if selected {
		return errors.New("Air currently selects AIGW; switch Air back to JetBrains AI before removing fallback")
	}
	airRefs, err := codexTargetRefs(discovered, []string{surface.ConfigPath}, true)
	if err != nil {
		return err
	}
	plans, err := adapters.PlanCodexReconciliation(airRefs, nil, domain.Runtime{})
	if err != nil {
		return err
	}
	plan, err := planForAirSurface(discovered, plans)
	if err != nil {
		return err
	}
	preview := routeChangePreview{
		DryRun:         true,
		SurfaceID:      discovery.SurfaceAirCodex,
		Authority:      discovery.AuthorityJetBrainsAI,
		ProjectionMode: adapters.CodexProjectionNamespacedFallback,
		Action:         plan.Action,
	}
	if dryRun {
		return renderRoutePreview(app, jsonMode, preview)
	}
	if !confirmHostIdle {
		return errors.New("Air restore requires --confirm-host-idle; no process was probed or stopped")
	}
	after := cloneConfig(before)
	adapter := after.Adapters[domain.ClientCodex]
	adapter.Targets = removeTarget(adapter.Targets, surface.ConfigPath)
	after.Adapters[domain.ClientCodex] = adapter
	membershipChanged := !slicesEqual(before.Adapters[domain.ClientCodex].Targets, adapter.Targets)
	if !membershipChanged {
		_, err := adapters.ReconcileCodexConfigs(airRefs, nil, domain.Runtime{})
		return err
	}
	configBefore, err := app.Config.CaptureSnapshot()
	if err != nil {
		return err
	}
	if err := app.Config.Save(after); err != nil {
		return err
	}
	configAfter, err := app.Config.CaptureSnapshot()
	if err != nil {
		return err
	}
	if _, err := adapters.ReconcileCodexConfigs(airRefs, nil, domain.Runtime{}); err != nil {
		if restoreErr := app.Config.RestoreSnapshot(configBefore, configAfter); restoreErr != nil {
			return fmt.Errorf("Air fallback restore failed: %w; config rollback failed: %v", err, restoreErr)
		}
		return err
	}
	r := renderer(app)
	r.Title("AIGW", "Air fallback removed")
	r.Success("Air fallback block was removed; JetBrains remains the active selection and native credentials were retained")
	r.Next("aigw sync --dry-run")
	return nil
}

func runAirRecover(app *App, dryRun, jsonMode, confirmHostIdle bool) error {
	surface, _, err := resolveAirSurface(app)
	if err != nil {
		return err
	}
	before, err := app.Config.Load()
	if err != nil {
		return err
	}
	target := adapters.CodexTargetRef{
		SurfaceID:      discovery.SurfaceAirCodex,
		Authority:      discovery.AuthorityJetBrainsAI,
		ProjectionMode: adapters.CodexProjectionStaleAirFullSelectionRecovery,
		Path:           surface.ConfigPath,
	}
	plans, err := adapters.PlanCodexReconciliation(nil, []adapters.CodexTargetRef{target}, domain.Runtime{})
	if err != nil {
		return errors.New("Air recovery is not admitted for the current configuration")
	}
	if len(plans) != 1 {
		return errors.New("Air stale full-selection recovery plan is missing")
	}
	after := cloneConfig(before)
	adapter := after.Adapters[domain.ClientCodex]
	adapter.Targets = removeTarget(adapter.Targets, surface.ConfigPath)
	after.Adapters[domain.ClientCodex] = adapter
	configurationAction := "already-without-air-fallback-target"
	if !slicesEqual(before.Adapters[domain.ClientCodex].Targets, adapter.Targets) {
		configurationAction = "remove-air-fallback-target"
	}
	preview := routeChangePreview{
		DryRun:              true,
		SurfaceID:           discovery.SurfaceAirCodex,
		Authority:           discovery.AuthorityJetBrainsAI,
		ProjectionMode:      "none",
		Action:              plans[0].Action,
		ConfigurationAction: configurationAction,
	}
	if dryRun {
		return renderRoutePreview(app, jsonMode, preview)
	}
	if !confirmHostIdle {
		return errors.New("Air recovery requires --confirm-host-idle; no process was probed or stopped")
	}
	if configurationAction == "already-without-air-fallback-target" {
		if _, err := reconcileCodexConfigs(nil, []adapters.CodexTargetRef{target}, domain.Runtime{}); err != nil {
			return err
		}
	} else {
		configBefore, err := app.Config.CaptureSnapshot()
		if err != nil {
			return err
		}
		if err := app.Config.Save(after); err != nil {
			return err
		}
		configAfter, err := app.Config.CaptureSnapshot()
		if err != nil {
			return err
		}
		if _, err := reconcileCodexConfigs(nil, []adapters.CodexTargetRef{target}, domain.Runtime{}); err != nil {
			if restoreErr := app.Config.RestoreSnapshot(configBefore, configAfter); restoreErr != nil {
				return fmt.Errorf("Air stale full-selection recovery failed: %w; config rollback failed: %v", err, restoreErr)
			}
			return err
		}
	}
	r := renderer(app)
	r.Title("AIGW", "Air stale full selection recovered")
	r.Success("AIGW-owned full-selection residue was removed; Air remains under JetBrains ownership")
	r.Next("aigw route doctor --json")
	return nil
}

func resolveAirSurface(app *App) (discovery.Surface, discovery.Result, error) {
	discovered, err := discoveredResult(app)
	if err != nil {
		return discovery.Surface{}, discovery.Result{}, err
	}
	surface, ok := discovered.Surface(discovery.SurfaceAirCodex)
	if !ok || !surface.Present || surface.ConfigPath == "" {
		return discovery.Surface{}, discovered, errors.New("JetBrains Air Codex configuration was not found")
	}
	if !surface.ManualFallbackAllowed {
		return discovery.Surface{}, discovered, errors.New("JetBrains Air does not admit an AIGW fallback on this host")
	}
	return surface, discovered, nil
}

func validateAirFallbackPreconditions(app *App, cfg domain.Config) (domain.Runtime, error) {
	adapter := cfg.Adapters[domain.ClientCodex]
	if !adapter.Enabled || adapter.Executable == "" || len(adapter.Targets) == 0 {
		return domain.Runtime{}, errors.New("enable the standalone Codex AIGW adapter before staging Air fallback")
	}
	runtime, _, err := cfg.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		return domain.Runtime{}, err
	}
	if !app.Secrets.Has(runtime.AccountID) {
		return domain.Runtime{}, fmt.Errorf("account %q has no token for Air fallback binding", runtime.AccountID)
	}
	return runtime, nil
}

func airFallbackRefs(discovered discovery.Result, before, after domain.Config) ([]adapters.CodexTargetRef, []adapters.CodexTargetRef, error) {
	beforeRefs, err := codexTargetRefs(discovered, before.Adapters[domain.ClientCodex].Targets, false)
	if err != nil {
		return nil, nil, err
	}
	afterRefs, err := codexTargetRefs(discovered, after.Adapters[domain.ClientCodex].Targets, true)
	if err != nil {
		return nil, nil, err
	}
	return beforeRefs, afterRefs, nil
}

func commitAirFallback(app *App, before, after domain.Config, beforeRefs, afterRefs []adapters.CodexTargetRef, runtime domain.Runtime) error {
	configBefore, err := app.Config.CaptureSnapshot()
	if err != nil {
		return err
	}
	if err := app.Config.Save(after); err != nil {
		return err
	}
	configAfter, err := app.Config.CaptureSnapshot()
	if err != nil {
		return err
	}
	if _, err := adapters.ReconcileCodexConfigs(beforeRefs, afterRefs, runtime); err != nil {
		if restoreErr := app.Config.RestoreSnapshot(configBefore, configAfter); restoreErr != nil {
			return fmt.Errorf("Air fallback staging failed: %w; config rollback failed: %v", err, restoreErr)
		}
		return err
	}
	r := renderer(app)
	r.Title("AIGW", "Air fallback staged")
	r.Success("A namespaced AIGW fallback block was staged; JetBrains remains the active Air selection")
	r.Next("aigw route restore air --dry-run")
	return nil
}

func airTopLevelSelectsAIGW(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			break
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok || strings.TrimSpace(key) != "model_provider" {
			continue
		}
		value, _, _ = strings.Cut(value, "#")
		value = strings.Trim(strings.TrimSpace(value), "\"")
		return value == "aigw" || value == "aigw_fallback", nil
	}
	return false, nil
}

func addSortedTarget(values []string, target string) []string {
	set := make(map[string]struct{}, len(values)+1)
	for _, value := range values {
		set[value] = struct{}{}
	}
	set[target] = struct{}{}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func removeTarget(values []string, target string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func planForAirSurface(discovered discovery.Result, plans []adapters.CodexProjectionPlan) (adapters.CodexProjectionPlan, error) {
	for _, plan := range plans {
		if surface, ok := discovered.SurfaceForConfigPath(plan.Target); ok && surface.ID == discovery.SurfaceAirCodex {
			return plan, nil
		}
	}
	return adapters.CodexProjectionPlan{}, errors.New("Air fallback plan is missing")
}

func renderRoutePreview(app *App, jsonMode bool, preview routeChangePreview) error {
	if jsonMode {
		encoder := json.NewEncoder(app.Out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(preview)
	}
	r := renderer(app)
	r.Title("AIGW", "Air route preview")
	r.Row("Surface", preview.SurfaceID)
	r.Row("Authority", preview.Authority)
	r.Row("Projection", preview.ProjectionMode)
	r.Row("Action", preview.Action)
	if preview.ConfigurationAction != "" {
		r.Row("Configuration", preview.ConfigurationAction)
	}
	r.Success("Preview made no persistent changes")
	if preview.Action == "recover-stale-full-selection" {
		r.Next("aigw route recover air --confirm-host-idle")
	} else {
		r.Next("aigw route fallback air --confirm-host-idle")
	}
	return nil
}

func slicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
