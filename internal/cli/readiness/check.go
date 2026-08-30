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

// NewCheckCommand builds the read-only health check for every enabled client.
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
	renderer := Renderer(runtime)
	renderer.ProductTitle("Health check")
	renderer.Section("Configuration")
	renderer.Status(presentation.OK, "Configuration file", "Healthy")
	renderer.Section("Client")
	clientCount := 0
	diagnosticAccounts := map[string]bool{}
	balanceCommands := []string{}
	for _, client := range configuration.AdmittedClientIDs() {
		adapter := cfg.Adapters[client]
		if !adapter.Enabled {
			renderer.Status(presentation.Info, invocation.Title(client), "Disabled")
			continue
		}
		clientRuntime, resolveErr := cfg.ResolveRuntime(client, "")
		if resolveErr != nil {
			return invocation.Problem(runtime, invocation.Title(client)+" route cannot be resolved", resolveErr.Error(), invocation.Title(client)+" cannot determine which profile to use.", "aigw use <"+client+"-profile>", resolveErr)
		}
		if !runtime.Secrets.Has(clientRuntime.AccountID) {
			instruction, _ := credential.TokenRecovery(runtime.Secrets, clientRuntime.AccountID)
			return invocation.Problem(
				runtime,
				invocation.Title(client)+" account token is unavailable",
				"Account "+clientRuntime.AccountID+" has no available Token.",
				invocation.Title(client)+" cannot authenticate to its selected gateway.",
				instruction,
				fmt.Errorf("%s account token unavailable", client),
			)
		}
		token, tokenErr := runtime.Secrets.Get(clientRuntime.AccountID)
		if tokenErr != nil {
			return tokenErr
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
		result := diagnostics.ProbeStable(cmd.Context(), runtime.HTTP, clientRuntime, token, diagnostics.DefaultStabilityPolicy())
		if result.Kind != diagnostics.Healthy {
			evidence := result.Detail
			if result.HTTPStatus != 0 {
				evidence = fmt.Sprintf("HTTP %d", result.HTTPStatus)
				if result.Detail != "" {
					evidence += " · " + result.Detail
				}
			}
			return invocation.Problem(runtime, result.Summary, evidence, invocation.Title(client)+" is unavailable.", result.Fix, fmt.Errorf("%s diagnostic kind %s", client, result.Kind))
		}
		renderer.Status(presentation.OK, invocation.Title(client), clientRuntime.ProfileLabel+" · Ready")
		if result.RecoveredTransient {
			renderer.Detail(invocation.Title(client) + " authentication recovered after a transient response")
		}
		if transport := TransportStatus(clientRuntime.Endpoint); transport.Kind == "external_loopback" {
			renderer.Detail(invocation.Title(client) + " uses an external loopback compatibility layer that AIGW does not manage")
		}
		if !diagnosticAccounts[clientRuntime.AccountID] {
			diagnosticAccounts[clientRuntime.AccountID] = true
			providerAccount := cfg.Accounts[clientRuntime.AccountID]
			if providerAccount.AccountProbe != nil && providers.Supports(providerAccount.AccountProbe.Kind) {
				balanceCommands = append(balanceCommands, "aigw balance "+clientRuntime.AccountID)
				if runtime.Accounts.Has(clientRuntime.AccountID) {
					renderer.Detail(clientRuntime.AccountLabel + " precise balance enabled")
				} else {
					renderer.Detail("aigw account connect " + clientRuntime.AccountID)
				}
			} else if providerAccount.AccountProbe != nil {
				renderer.Detail(clientRuntime.AccountLabel + " has no diagnostic driver in this version")
			}
		}
		clientCount++
	}
	renderer.Section("Result")
	if clientCount == 0 {
		renderer.Success("Configuration is healthy; no clients are enabled")
		return nil
	}
	renderer.Success("Every enabled client route is healthy")
	for _, command := range balanceCommands {
		renderer.Next(command)
	}
	return nil
}

func isLocalProgramBuild(version string) bool {
	version = strings.TrimSpace(version)
	return version == "" || strings.HasSuffix(version, "-dev") || strings.Contains(version, "+local")
}
