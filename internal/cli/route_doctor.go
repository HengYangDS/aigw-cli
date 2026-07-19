package cli

import (
	"encoding/json"
	"errors"
	"sort"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/recovery"
)

type routeSurfaceStatus struct {
	SurfaceID           string `json:"surface_id"`
	Product             string `json:"product"`
	BaselineAuthority   string `json:"baseline_authority"`
	Fallback            string `json:"fallback"`
	ProjectionMode      string `json:"projection_mode"`
	AttributionState    string `json:"attribution_state"`
	DiskSelection       string `json:"disk_selection"`
	HostAuthentication  string `json:"host_authentication"`
	SessionMetadata     string `json:"session_metadata"`
	ObservedEndpointHop string `json:"observed_endpoint_hop"`
	TerminalOutcome     string `json:"terminal_outcome"`
	BillingEvidence     string `json:"billing_evidence"`
	Present             bool   `json:"present"`
	Management          string `json:"management"`
	State               string `json:"state"`
	RecoveryState       string `json:"recovery_state,omitempty"`
	RecoveryHealth      string `json:"recovery_health,omitempty"`
	RecoveryReasonCode  string `json:"recovery_reason_code,omitempty"`
}

type routeDoctorReport struct {
	Surfaces []routeSurfaceStatus `json:"surfaces"`
	OK       bool                 `json:"ok"`
}

func newRouteDoctorCommand(app *App) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Inspect host routing ownership without probes or writes",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			report, err := buildRouteDoctorReport(app)
			if err != nil {
				return err
			}
			if jsonMode {
				encoder := json.NewEncoder(app.Out)
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(report); err != nil {
					return err
				}
			} else {
				renderRouteDoctorReport(app, report)
			}
			if !report.OK {
				// The JSON report is the complete machine-readable result.  Do
				// not append a terminal problem card to the same stream, or an
				// automation caller cannot decode a detected conflict.
				if jsonMode {
					return presented(errors.New("route ownership conflict detected; review the reported surface"))
				}
				return errors.New("route ownership conflict detected; review the reported surface")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the secret-free report as JSON")
	return cmd
}

// buildRouteDoctorReport is deliberately local and observational: it does not
// invoke a runner, query HTTP, read credentials, open session stores, or run
// Junie. Host authentication, session state, endpoint hops, terminal outcomes,
// and billing therefore remain explicitly not-probed or unknown.
func buildRouteDoctorReport(app *App) (routeDoctorReport, error) {
	cfg, err := app.Config.Load()
	if err != nil {
		return routeDoctorReport{}, err
	}
	discovered, err := discoveredResult(app)
	if err != nil {
		return routeDoctorReport{}, err
	}
	configured := map[string]bool{}
	for _, target := range cfg.Adapters["codex"].Targets {
		if surface, ok := discovered.SurfaceForConfigPath(target); ok {
			configured[surface.ID] = true
		}
	}
	report := routeDoctorReport{OK: true, Surfaces: make([]routeSurfaceStatus, 0, len(discovered.Surfaces))}
	standalonePath := ""
	if standalone, ok := discovered.Surface(discovery.SurfaceCodexCLIStandalone); ok && standalone.Present {
		standalonePath = standalone.ConfigPath
	}
	for _, surface := range discovered.Surfaces {
		status := routeSurfaceStatus{
			SurfaceID:           surface.ID,
			Product:             surface.Product,
			BaselineAuthority:   surface.Authority,
			Fallback:            "none",
			ProjectionMode:      "none",
			AttributionState:    "not-applicable",
			DiskSelection:       "not-inspected",
			HostAuthentication:  "not-probed",
			SessionMetadata:     "not-probed",
			ObservedEndpointHop: "not-probed",
			TerminalOutcome:     "not-probed",
			BillingEvidence:     "unknown",
			Present:             surface.Present,
			Management:          "external",
			State:               "not-managed",
		}
		if surface.ID == discovery.SurfaceJunieCLI {
			status.Management = "external-jetbrains"
			report.Surfaces = append(report.Surfaces, status)
			continue
		}
		if !surface.Present || surface.ConfigPath == "" {
			if surface.Authority == discovery.AuthorityJetBrainsAI {
				status.Management = "external-jetbrains"
			}
			if configured[surface.ID] {
				status.State = "configured-surface-missing"
				report.OK = false
			}
			report.Surfaces = append(report.Surfaces, status)
			continue
		}
		var inspection adapters.CodexInspection
		if surface.ID == discovery.SurfaceAirCodex {
			inspection, err = adapters.InspectAirCodexConfig(surface.ConfigPath, standalonePath)
		} else {
			inspection, err = adapters.InspectCodexConfig(surface.ConfigPath)
		}
		if err != nil {
			return routeDoctorReport{}, err
		}
		status.State = inspection.State
		status.ProjectionMode = nonEmptyRouteField(inspection.ProjectionMode, "none")
		status.AttributionState = nonEmptyRouteField(inspection.AttributionState, "none")
		status.DiskSelection = inspection.DiskSelection
		switch surface.ID {
		case discovery.SurfaceCodexCLIStandalone:
			status.Management = "auto-managed"
			if configured[surface.ID] {
				if inspection.State != "aigw-managed" && inspection.State != "legacy-full-selection" {
					report.OK = false
				}
			} else if inspection.AIGWManaged || inspection.State == "orphaned-aigw-marker" {
				status.State = "unlisted-aigw-projection"
				report.OK = false
			}
		case discovery.SurfacePyCharmCodex:
			status.Management = "external-jetbrains"
			if configured[surface.ID] {
				status.State = "forbidden-aigw-target-membership"
				report.OK = false
			} else if inspection.AIGWManaged || inspection.State == "orphaned-aigw-marker" {
				report.OK = false
			}
		case discovery.SurfaceAirCodex:
			status.Management = "external-jetbrains"
			if inspection.State == "fallback-staged" {
				status.Fallback = "staged"
				status.Management = "manual-aigw-fallback"
				if !configured[surface.ID] {
					status.State = "stale-unlisted-fallback"
					report.OK = false
				}
			} else if inspection.State == "fallback-selected-conflict" {
				status.Fallback = "selected-conflict"
				report.OK = false
			} else if inspection.AIGWManaged || inspection.State == "orphaned-aigw-marker" {
				report.OK = false
			} else if configured[surface.ID] {
				status.State = "listed-without-valid-fallback"
				report.OK = false
			}
		}
		if surface.ID == discovery.SurfaceAirCodex {
			store, storeErr := airRecoveryStore(app)
			if storeErr != nil {
				return routeDoctorReport{}, storeErr
			}
			lifecycle, lifecycleErr := store.InspectAirLifecycle(surface.ConfigPath, standalonePath)
			if lifecycleErr != nil {
				return routeDoctorReport{}, lifecycleErr
			}
			status.RecoveryState = lifecycle.RecoveryState
			status.RecoveryHealth = lifecycle.RecoveryHealth
			status.RecoveryReasonCode = lifecycle.RecoveryReasonCode
			if lifecycle.DerivedState != "" {
				status.State = lifecycle.DerivedState
			}
			if lifecycle.RecoveryHealth == recovery.AirRecoveryHealthInvalid {
				report.OK = false
			}
			switch lifecycle.RecoveryState {
			case "prepared", "awaiting-host-roundtrip", "quarantined":
				report.OK = false
			}
		}
		if inspection.AttributionState == "foreign-or-incomplete" || inspection.State == "aigw-drift" || inspection.State == "stale-sidecar" || inspection.State == "ownership-conflict" || inspection.State == "partial-or-foreign-residue" || status.State == "orphaned-exact-full-selection" || status.State == "reappeared-after-recovery" {
			report.OK = false
		}
		report.Surfaces = append(report.Surfaces, status)
	}
	sort.Slice(report.Surfaces, func(left, right int) bool {
		return report.Surfaces[left].SurfaceID < report.Surfaces[right].SurfaceID
	})
	return report, nil
}

func renderRouteDoctorReport(app *App, report routeDoctorReport) {
	r := renderer(app)
	r.Title("AIGW", "Host routing ownership")
	for _, surface := range report.Surfaces {
		r.Section(surface.Product)
		r.Row("Surface", surface.SurfaceID)
		r.Row("Authority", surface.BaselineAuthority)
		r.Row("Management", surface.Management)
		r.Row("Disk selection", surface.DiskSelection)
		r.Row("Fallback", surface.Fallback)
		r.Row("State", surface.State)
		if surface.RecoveryState != "" {
			r.Row("Recovery", surface.RecoveryState)
		}
		if surface.RecoveryHealth != "" {
			r.Row("Recovery health", surface.RecoveryHealth)
		}
		if surface.RecoveryReasonCode != "" {
			r.Row("Recovery reason", surface.RecoveryReasonCode)
		}
		r.Row("Authentication", surface.HostAuthentication)
		r.Row("Endpoint", surface.ObservedEndpointHop)
		r.Row("Billing", surface.BillingEvidence)
	}
	if report.OK {
		r.Success("No route ownership conflict was detected")
	} else {
		for _, surface := range report.Surfaces {
			if surface.SurfaceID == discovery.SurfaceAirCodex && surface.State == "recoverable-stale-full-selection" {
				r.Next("aigw route recover air --dry-run")
				return
			}
			if surface.SurfaceID == discovery.SurfaceAirCodex && surface.State == "orphaned-exact-full-selection" {
				r.Next("aigw route recover-orphan air --dry-run --json")
				return
			}
		}
		r.Next("aigw repair --dry-run")
	}
}

func nonEmptyRouteField(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
