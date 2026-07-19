package cli

import (
	"encoding/json"
	"errors"
	"runtime"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/attestation"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/platform"
)

func newRouteAttestCommand(app *App) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use:   "attest <air>",
		Short: "Inspect bounded Air router evidence without probes or writes",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if args[0] != "air" {
				return errors.New("only `aigw route attest air` is admitted")
			}
			report, err := buildAirRuntimeAttestation(app)
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
				renderAirRuntimeAttestation(app, report)
			}
			if report.State != "host-mirror-runtime-attested" {
				err := errors.New("Air host-mirror runtime is not attested by fresh JetBrains router evidence")
				if jsonMode {
					return presented(err)
				}
				return err
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the redacted attestation as JSON")
	return cmd
}

func buildAirRuntimeAttestation(app *App) (attestation.AirRuntimeAttestation, error) {
	cfg, err := app.Config.Load()
	if err != nil {
		return attestation.AirRuntimeAttestation{}, errors.New("local AIGW configuration is unavailable")
	}
	configuredRuntime, _, err := cfg.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		return attestation.AirRuntimeAttestation{}, errors.New("configured Codex route is unavailable")
	}
	discovered, err := discoveredResult(app)
	if err != nil {
		return attestation.AirRuntimeAttestation{}, errors.New("Air surface discovery is unavailable")
	}
	airSurface, ok := discovered.Surface(discovery.SurfaceAirCodex)
	if !ok || !airSurface.Present || airSurface.ConfigPath == "" {
		return attestation.AirRuntimeAttestation{}, errors.New("the JetBrains Air Codex surface is not present")
	}
	standaloneSurface, ok := discovered.Surface(discovery.SurfaceCodexCLIStandalone)
	if !ok || standaloneSurface.ConfigPath == "" {
		return attestation.AirRuntimeAttestation{}, errors.New("the standalone Codex reference surface is unavailable")
	}
	inspection, err := adapters.InspectAirCodexConfig(airSurface.ConfigPath, standaloneSurface.ConfigPath)
	if err != nil {
		return attestation.AirRuntimeAttestation{}, errors.New("Air configuration inspection is unavailable")
	}
	goos := app.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	logDir, err := platform.AirLogDirFor(goos, environmentMap(app.Env))
	if err != nil {
		return attestation.AirRuntimeAttestation{}, errors.New("Air router log discovery is unavailable")
	}
	report, err := attestation.InspectAirRuntime(attestation.AirOptions{
		LogDir:             logDir,
		AIGWEndpoint:       configuredRuntime.Endpoint,
		ConfigurationState: inspection.State,
		Now:                appNow(app),
	})
	if err != nil {
		return attestation.AirRuntimeAttestation{}, err
	}
	return report, nil
}

func appNow(app *App) time.Time {
	if app.Now != nil {
		return app.Now()
	}
	return time.Now()
}

func renderAirRuntimeAttestation(app *App, report attestation.AirRuntimeAttestation) {
	r := renderer(app)
	r.Title("AIGW", "Air runtime attestation")
	r.Row("Surface", report.SurfaceID)
	r.Row("Configuration", report.ConfigurationState)
	r.Row("State", report.State)
	r.Row("Runtime authority", report.RuntimeAuthority)
	r.Row("Requests", strconv.Itoa(report.RequestCount))
	r.Row("JetBrains requests", strconv.Itoa(report.JetBrainsRequestCount))
	r.Row("AIGW requests", strconv.Itoa(report.AIGWRequestCount))
	r.Row("Other requests", strconv.Itoa(report.OtherRequestCount))
	if report.WindowStart != "" {
		r.Row("Window start", report.WindowStart)
		r.Row("Window end", report.WindowEnd)
	}
	r.Row("Authentication", report.HostAuthentication)
	r.Row("Billing", report.BillingEvidence)
	r.Row("Evidence", report.EvidenceSource)
	if report.State == "host-mirror-runtime-attested" {
		r.Success("Fresh Air router evidence matches the external JetBrains runtime")
	} else {
		r.Text("Fresh external JetBrains runtime evidence was not established")
	}
}
