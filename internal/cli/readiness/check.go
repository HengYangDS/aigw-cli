// Package readiness owns read-only status, health, and endpoint checks for the
// selected AIGW routes.
package readiness

import (
	"fmt"
	"strings"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/diagnostics"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/providers"
	"github.com/spf13/cobra"
)

// NewCheckCommand builds the read-only health check for the active service.
func NewCheckCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{
		Use: "check", Short: "Check configuration, tokens, clients, and gateway", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return RunCheck(cmd, runtime) },
	}
}

// RunCheck verifies the selected route, configured client projections, and
// gateway authentication without mutating configuration or credentials.
func RunCheck(cmd *cobra.Command, runtime invocation.Context) error {
	if isLocalProgramBuild(runtime.Version) {
		return invocation.Problem(runtime, "Local program is not an official release", "Detected local build marker: "+runtime.Version, "A local development build must not replace a verified team release.", "aigw update", fmt.Errorf("local program build"))
	}
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Profiles) == 0 {
		return invocation.Problem(runtime, "Not configured", "No service profiles have been created.", "Cannot check, synchronize, or repair configuration that does not exist.", "aigw setup", fmt.Errorf("not configured"))
	}
	selected, err := firstCheckRuntime(cfg)
	if err != nil {
		return err
	}
	accountName := selected.AccountID
	providerAccount := cfg.Accounts[accountName]
	token, err := runtime.Secrets.Get(accountName)
	if err != nil {
		instruction, _ := credential.TokenRecovery(runtime.Secrets, accountName)
		return fmt.Errorf("System secret is missing\nFix: %s", instruction)
	}
	renderer := Renderer(runtime)
	renderer.ProductTitle("Health check")
	renderer.Section("Configuration")
	renderer.Status(presentation.OK, "Configuration file", "Healthy")
	renderer.Row("Current service", selected.ProfileLabel)
	renderer.Status(presentation.OK, "System secret", "Available")
	renderer.Section("Client")
	clientCount := 0
	for _, client := range configuration.AdmittedClientIDs() {
		adapter := cfg.Adapters[client]
		if !adapter.Enabled {
			renderer.Status(presentation.Info, invocation.Title(client), "Disabled")
			continue
		}
		clientRuntime, _, resolveErr := cfg.ResolveRuntime(client, "")
		if resolveErr != nil {
			return invocation.Problem(runtime, invocation.Title(client)+" route cannot be resolved", resolveErr.Error(), invocation.Title(client)+" cannot determine which profile to use.", "aigw use <profile> --for "+client, resolveErr)
		}
		ready, issue := AdapterRouteReady(runtime, cfg, client, clientRuntime)
		if !ready {
			fix := "aigw repair"
			impact := invocation.Title(client) + " cannot inherit AIGW routes, tokens, or configuration projections."
			if client == configuration.ClientCodex && strings.Contains(issue, "projection") {
				fix = "aigw sync"
				impact = "Codex may use the wrong model or endpoint."
			}
			return invocation.Problem(runtime, invocation.Title(client)+" adapter is not ready", issue, impact, fix, fmt.Errorf("%s adapter not ready", client))
		}
		renderer.Status(presentation.OK, invocation.Title(client), "Ready")
		clientCount++
	}
	result := diagnostics.ProbeStable(cmd.Context(), runtime.HTTP, selected, token, diagnostics.DefaultStabilityPolicy())
	if result.Kind != diagnostics.Healthy {
		evidence := result.Detail
		if result.HTTPStatus != 0 {
			evidence = fmt.Sprintf("HTTP %d", result.HTTPStatus)
			if result.Detail != "" {
				evidence += " · " + result.Detail
			}
		}
		return invocation.Problem(runtime, result.Summary, evidence, healthImpact(clientCount), result.Fix, fmt.Errorf("diagnostic kind %s", result.Kind))
	}
	renderer.Section("Gateway")
	renderer.Status(presentation.OK, "API Token", "Authentication healthy")
	if result.RecoveredTransient {
		renderer.Detail("Authentication recovered after a transient response")
	}
	if transport := TransportStatus(selected.Endpoint); transport.Kind == "external_loopback" {
		renderer.Section("Transport")
		renderer.Status(presentation.Info, "Codex", "External loopback compatibility layer")
		renderer.Detail("Codex requests use the external listener")
		renderer.Detail("AIGW does not start, stop, or configure it")
	}
	if providerAccount.AccountProbe != nil && providers.Supports(providerAccount.AccountProbe.Kind) && runtime.Accounts.Has(accountName) {
		renderer.Status(presentation.OK, "Precise balance", "Enabled")
	} else if providerAccount.AccountProbe != nil && providers.Supports(providerAccount.AccountProbe.Kind) {
		renderer.Status(presentation.Warn, "Precise balance", "Disabled")
		renderer.Detail("aigw account connect " + accountName)
	} else if providerAccount.AccountProbe != nil {
		renderer.Status(presentation.Info, "Precise balance", "This version does not provide diagnostics for this provider")
	}
	renderer.Section("Result")
	renderer.Success("Everything is healthy")
	if providerAccount.AccountProbe != nil && providers.Supports(providerAccount.AccountProbe.Kind) {
		renderer.Next("aigw balance " + accountName)
	}
	return nil
}

func isLocalProgramBuild(version string) bool {
	version = strings.TrimSpace(version)
	return version == "" || strings.HasSuffix(version, "-dev") || strings.Contains(version, "+local")
}

func healthImpact(configuredClients int) string {
	switch configuredClients {
	case 0:
		return "The current API route is unavailable."
	case 1:
		return "The configured AI client is unavailable."
	default:
		return fmt.Sprintf("%d configured AI clients are unavailable.", configuredClients)
	}
}

func firstCheckRuntime(cfg configuration.Config) (configuration.Runtime, error) {
	profile, ok := cfg.Profiles[cfg.Routes.Default]
	if !ok {
		return configuration.Runtime{}, noTestableEndpointError()
	}
	client := profile.Client
	if client == "" {
		for _, candidate := range configuration.AdmittedClientIDs() {
			if _, _, err := cfg.ResolveRuntime(candidate, cfg.Routes.Default); err == nil {
				client = candidate
				break
			}
		}
	}
	if client == "" {
		return configuration.Runtime{}, noTestableEndpointError()
	}
	resolved, _, err := cfg.ResolveRuntime(client, cfg.Routes.Default)
	if err != nil {
		return configuration.Runtime{}, noTestableEndpointError()
	}
	return resolved, nil
}

func noTestableEndpointError() error {
	return fmt.Errorf("Current default route has no testable endpoint; run `aigw use` to choose a model profile")
}
