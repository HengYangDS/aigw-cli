package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/account"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/diagnostics"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/presentation"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/providers"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/selfupdate"
)

func newCheckCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use: "check", Short: "Check configuration, tokens, clients, and gateway", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if isLocalProgramBuild(appVersion(app)) {
				return problem("Local program is not an official release", "Detected local build marker: "+appVersion(app), "A local development build must not replace a verified team release.", "aigw update", fmt.Errorf("local program build"))
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			if len(cfg.Profiles) == 0 {
				return problem("Not configured", "No service profiles have been created.", "Cannot check, synchronize, or repair configuration that does not exist.", "aigw setup", fmt.Errorf("not configured"))
			}
			runtime, err := firstCheckRuntime(cfg)
			if err != nil {
				return err
			}
			accountName := runtime.AccountID
			providerAccount := cfg.Accounts[accountName]
			token, err := app.Secrets.Get(accountName)
			if err != nil {
				return fmt.Errorf("System secret is missing\nFix: aigw rotate %s", accountName)
			}
			r := renderer(app)
			r.Title("AIGW", "Health check")
			r.Section("Configuration")
			r.Status(presentation.OK, "Configuration file", "Healthy")
			r.Row("Current service", runtime.ProfileLabel)
			r.Status(presentation.OK, "System secret", "Available")
			r.Section("Client")
			clientCount := 0
			for _, client := range domain.AdmittedClientIDs() {
				adapter := cfg.Adapters[client]
				if adapter.Enabled {
					clientRuntime, _, resolveErr := cfg.ResolveRuntime(client, "")
					if resolveErr != nil {
						return problem(title(client)+" route cannot be resolved", resolveErr.Error(), title(client)+" cannot determine which profile to use.", "aigw use <profile> --for "+client, resolveErr)
					}
					ready, issue := adapterRouteReady(app, cfg, client, clientRuntime)
					if !ready {
						fix := "aigw repair"
						impact := title(client) + " cannot inherit AIGW routes, tokens, or configuration projections."
						if client == domain.ClientCodex && strings.Contains(issue, "projection") {
							fix = "aigw sync"
							impact = "Codex may use the wrong model or endpoint."
						}
						return problem(title(client)+" adapter is not ready", issue, impact, fix, fmt.Errorf("%s adapter not ready", client))
					}
					r.Status(presentation.OK, title(client), "Ready")
					clientCount++
				} else {
					r.Status(presentation.Info, title(client), "Disabled")
				}
			}
			result := diagnostics.ProbeStable(cmd.Context(), app.HTTP, runtime, token, diagnostics.DefaultStabilityPolicy())
			if result.Kind != diagnostics.Healthy {
				evidence := result.Detail
				if result.HTTPStatus != 0 {
					evidence = fmt.Sprintf("HTTP %d", result.HTTPStatus)
					if result.Detail != "" {
						evidence += " · " + result.Detail
					}
				}
				return problem(result.Summary, evidence, healthImpact(clientCount), result.Fix, fmt.Errorf("diagnostic kind %s", result.Kind))
			}
			r.Section("Gateway")
			r.Status(presentation.OK, "API Token", "Authentication healthy")
			if result.RecoveredTransient {
				r.Detail("Authentication recovered after a transient response")
			}
			if transport := transportStatus(runtime.Endpoint); transport.Kind == "external_loopback" {
				r.Section("Transport")
				r.Status(presentation.Info, "Codex", "External loopback compatibility layer")
				r.Detail("Codex requests use the external listener")
				r.Detail("AIGW does not start, stop, or configure it")
			}
			if providerAccount.AccountProbe != nil && providers.Supports(providerAccount.AccountProbe.Kind) && app.Accounts.Has(accountName) {
				r.Status(presentation.OK, "Precise balance", "Enabled")
			} else if providerAccount.AccountProbe != nil && providers.Supports(providerAccount.AccountProbe.Kind) {
				r.Status(presentation.Warn, "Precise balance", "Disabled")
				r.Detail("aigw account connect " + accountName)
			} else if providerAccount.AccountProbe != nil {
				r.Status(presentation.Info, "Precise balance", "This version does not provide diagnostics for this provider")
			}
			r.Section("Result")
			r.Success("Everything is healthy")
			if providerAccount.AccountProbe != nil && providers.Supports(providerAccount.AccountProbe.Kind) {
				r.Next("aigw balance " + accountName)
			}
			return nil
		},
	}
}

func isLocalProgramBuild(version string) bool {
	version = strings.TrimSpace(version)
	return version == "" || strings.HasSuffix(version, "-dev") || strings.Contains(version, "+local")
}

func newAccountCommand(app *App) *cobra.Command {
	root := &cobra.Command{Use: "account", Short: "Manage account endpoints and optional precise diagnostics"}
	root.AddCommand(
		newAccountEditCommand(app),
		&cobra.Command{Use: "connect [account]", Short: "Bind provider platform credentials to query precise balance", Args: cobra.MaximumNArgs(1), RunE: func(_ *cobra.Command, args []string) error {
			if !app.Interactive {
				return fmt.Errorf("Binding platform credentials requires an interactive terminal")
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			name := cfg.Routes.Default
			if len(args) == 1 {
				name = args[0]
			}
			accountName, providerAccount, err := accountForInput(cfg, name)
			if err != nil {
				return err
			}
			if providerAccount.AccountProbe == nil {
				return fmt.Errorf("Account %q does not support precise account diagnostics", accountName)
			}
			if !providers.Supports(providerAccount.AccountProbe.Kind) {
				return fmt.Errorf("This AIGW version does not include precise diagnostics for provider %q", providerAccount.AccountProbe.Kind)
			}
			systemToken, err := app.Prompt.Secret("Paste the platform system token (not the API token): ")
			if err != nil {
				return err
			}
			userID, err := app.Prompt.Text("User ID: ")
			if err != nil {
				return err
			}
			if err := app.Accounts.Set(accountName, account.Credential{SystemToken: systemToken, UserID: userID}); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "Account diagnostics enabled")
			r.Section("Service")
			r.Row("Name", providerAccount.Label)
			r.Status(presentation.OK, "System credential", "Securely stored")
			r.Next("aigw balance")
			return nil
		}},
		&cobra.Command{Use: "disconnect [account]", Short: "Remove optional provider platform credentials", Args: cobra.MaximumNArgs(1), RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			name := cfg.Routes.Default
			if len(args) == 1 {
				name = args[0]
			}
			accountName, _, err := accountForInput(cfg, name)
			if err != nil {
				return err
			}
			if err := app.Accounts.Delete(accountName); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "Account diagnostics disabled")
			r.Success("Platform system credentials were removed from secure storage")
			return nil
		}},
	)
	return root
}

func newBalanceCommand(app *App) *cobra.Command {
	return &cobra.Command{Use: "balance [account]", Short: "Show account balance and token quota", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := app.Config.Load()
		if err != nil {
			return err
		}
		name := cfg.Routes.Default
		if len(args) == 1 {
			name = args[0]
		}
		accountName, providerAccount, err := accountForInput(cfg, name)
		if err != nil {
			return err
		}
		if providerAccount.AccountProbe == nil {
			return fmt.Errorf("%s does not support precise balance queries", providerAccount.Label)
		}
		if !providers.Supports(providerAccount.AccountProbe.Kind) {
			return fmt.Errorf("Precise diagnostics provider %q is not included in this AIGW version; continue using `aigw check` for general diagnostics", providerAccount.AccountProbe.Kind)
		}
		credential, err := app.Accounts.Get(accountName)
		if err != nil {
			return problem(
				"Precise balance diagnostics are not enabled",
				"Missing "+accountName+" provider platform query credentials; the API token is stored separately in system secret storage.",
				"Cannot distinguish account balance, remaining token quota, disabled token state, and request limits.",
				"aigw account connect "+accountName,
				err,
			)
		}
		apiToken, err := app.Secrets.Get(accountName)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		providerAccount.ID = accountName
		report, err := providers.Probe(ctx, app.HTTP, providerAccount, apiToken, credential)
		if err != nil {
			return err
		}
		status := map[string]string{"enabled": "Enabled", "disabled": "Disabled"}[report.TokenStatus]
		remaining := fmt.Sprintf("$%.4f", report.TokenRemaining)
		if report.TokenUnlimitedQuota {
			remaining = "Unlimited"
		}
		count := fmt.Sprintf("%d requests", report.TokenRemainingCount)
		if report.TokenUnlimitedCount {
			count = "Unlimited requests"
		}
		r := renderer(app)
		r.Title("AIGW", "Account and quota")
		r.Section("Account")
		r.Row("Account", providerAccount.Label)
		r.Row("Account balance", fmt.Sprintf("$%.4f", report.AccountBalance))
		r.Section("Current API token")
		r.Row("Name", report.TokenName)
		state := presentation.OK
		if report.TokenStatus != "enabled" {
			state = presentation.Fail
		}
		r.Status(state, "Token status", status)
		r.Row("Quota used", fmt.Sprintf("$%.4f", report.TokenUsed))
		r.Row("Remaining quota", remaining)
		r.Row("Remaining requests", count)
		r.Next("aigw check")
		return nil
	}}
}

func newRepairCommand(app *App) *cobra.Command {
	var dryRun, jsonMode bool
	cmd := &cobra.Command{
		Use: "repair", Short: "Discover and repair client configuration", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return runRepair(cmd.Context(), app, dryRun, jsonMode) },
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview repair without writing configuration or authentication")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write a secret-free repair preview as JSON")
	return cmd
}

type repairPreview struct {
	DryRun              bool                      `json:"dry_run"`
	ConfigurationAction string                    `json:"configuration_action"`
	Codex               []repairProjectionPreview `json:"codex"`
}

type repairProjectionPreview struct {
	SurfaceID string `json:"surface_id"`
	Action    string `json:"action"`
}

func runRepair(ctx context.Context, app *App, dryRun, jsonMode bool) error {
	before, err := app.Config.Load()
	if err != nil {
		return err
	}
	if len(before.Profiles) == 0 {
		return problem("Not configured", "No service profiles have been created.", "Cannot check, synchronize, or repair configuration that does not exist.", "aigw setup", fmt.Errorf("not configured"))
	}
	after, discovered, enableClaude, newClaude, err := repairDesiredConfig(app, before)
	if err != nil {
		return err
	}
	if dryRun {
		plans, err := planCodexReconciliation(app, before, after)
		if err != nil {
			return err
		}
		return renderRepairPreview(app, jsonMode, before, after, discovered, plans)
	}
	if enableClaude {
		if _, err := app.Shims.EnableClaude(); err != nil {
			return err
		}
	}
	if err := commitConfigAndSync(ctx, app, before, after, "repair"); err != nil {
		if newClaude {
			_ = app.Shims.DisableClaude()
		}
		return err
	}
	if after.Adapters[domain.ClientCodex].Enabled && !codexProjectionChanged(before, after) {
		if err := syncCodexProjection(ctx, app, after); err != nil {
			return fmt.Errorf("Failed to repair Codex configuration projection: %w", err)
		}
	}
	r := renderer(app)
	r.Title("AIGW", "Repair completed")
	r.Section("Results")
	r.Status(presentation.OK, "Client", "Rediscovered")
	r.Status(presentation.OK, "Configuration", "Synchronized")
	authentication := "Unchanged"
	if codexAuthenticationChanged(before, after) {
		authentication = "Bound"
	}
	r.Status(presentation.OK, "Authentication", authentication)
	r.Next("aigw check")
	return nil
}

func repairDesiredConfig(app *App, before domain.Config) (domain.Config, discovery.Result, bool, bool, error) {
	after := cloneConfig(before)
	discovered, err := discoveredResult(app)
	if err != nil {
		return domain.Config{}, discovery.Result{}, false, false, err
	}
	claudeRuntime, _, claudeRouteErr := after.ResolveRuntime(domain.ClientClaude, "")
	codexRuntime, _, codexRouteErr := after.ResolveRuntime(domain.ClientCodex, "")
	enableClaude := false
	newClaude := false
	claudeAdapter := after.Adapters[domain.ClientClaude]
	claudeExecutable := claudeAdapter.Executable
	if claudeExecutable == "" {
		claudeExecutable = discovered.ClaudeExecutable
	}
	if claudeRouteErr == nil && claudeExecutable != "" && claudeRuntime.Endpoint != "" {
		enableClaude = true
		newClaude = !claudeAdapter.Enabled
		after.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: claudeExecutable}
	}
	if codexRouteErr == nil && codexRuntime.Endpoint != "" {
		currentCodex := after.Adapters[domain.ClientCodex]
		targets := repairCodexTargets(discovered, currentCodex.Targets)
		executable := currentCodex.Executable
		if discovered.CodexExecutable != "" {
			executable = discovered.CodexExecutable
		}
		if executable != "" && len(targets) > 0 {
			after.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: executable, Targets: targets}
		} else if currentCodex.Enabled && len(targets) == 0 {
			delete(after.Adapters, domain.ClientCodex)
		}
	}
	return after, discovered, enableClaude, newClaude, nil
}

func renderRepairPreview(app *App, jsonMode bool, before, after domain.Config, discovered discovery.Result, plans []adapters.CodexProjectionPlan) error {
	preview := repairPreview{DryRun: true, ConfigurationAction: "already-converged", Codex: make([]repairProjectionPreview, 0, len(plans))}
	if !reflect.DeepEqual(before, after) {
		preview.ConfigurationAction = "update"
	}
	for _, plan := range plans {
		surfaceID := "codex-cli-explicit"
		if surface, ok := discovered.SurfaceForConfigPath(plan.Target); ok {
			surfaceID = surface.ID
		}
		preview.Codex = append(preview.Codex, repairProjectionPreview{SurfaceID: surfaceID, Action: plan.Action})
	}
	if jsonMode {
		encoder := json.NewEncoder(app.Out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(preview)
	}
	r := renderer(app)
	r.Title("AIGW", "Repair preview")
	r.Row("Configuration", preview.ConfigurationAction)
	for _, plan := range preview.Codex {
		r.Row(plan.SurfaceID, plan.Action)
	}
	r.Success("Preview did not write configuration, state files, authentication, shims, or conversations")
	r.Next("aigw repair")
	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func repairCodexTargets(discovered discovery.Result, current []string) []string {
	seen := map[string]bool{}
	targets := make([]string, 0, len(current)+len(discovered.Surfaces))
	appendTarget := func(path string) {
		if path != "" && !seen[path] {
			seen[path] = true
			targets = append(targets, path)
		}
	}
	for _, path := range discovered.AutoManagedCodexTargets() {
		appendTarget(path)
	}
	for _, path := range current {
		if surface, ok := discovered.SurfaceForConfigPath(path); ok {
			if surface.ID == discovery.SurfaceCodexCLIStandalone {
				appendTarget(path)
			}
			continue
		}
		// An unknown existing target was explicitly configured by the user. It
		// remains an AIGW-owned standalone candidate; only known JetBrains
		// surfaces are removed by generic repair.
		appendTarget(path)
	}
	return targets
}

func newUpdateCommand(app *App) *cobra.Command {
	var rollback bool
	var candidateArchive string
	var candidateChecksums string
	cmd := &cobra.Command{
		Use: "update", Short: "Install a verified release, a local candidate, or restore the previous portable program", Args: cobra.NoArgs,
		RunE: func(ctx *cobra.Command, _ []string) error {
			if app.Updater == nil {
				return fmt.Errorf("Automatic update is unavailable; install a verified release from GitLab or GitHub")
			}
			var (
				result string
				err    error
			)
			if rollback {
				result, err = app.Updater.Rollback(ctx.Context())
			} else if candidateArchive != "" {
				result, err = app.Updater.UpdateCandidate(ctx.Context(), appVersion(app), selfupdate.CandidateArchive{
					ArchivePath:   candidateArchive,
					ChecksumsPath: candidateChecksums,
				})
			} else {
				result, err = app.Updater.Update(ctx.Context(), appVersion(app))
			}
			if err != nil {
				return err
			}
			r := renderer(app)
			title := "Update"
			if rollback {
				title = "Program rollback"
			} else if candidateArchive != "" {
				title = "Verified local candidate"
			}
			r.Title("AIGW", title)
			r.Success(result)
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().BoolVar(&rollback, "rollback", false, "Roll back the portable AIGW program to the previous version offline")
	cmd.Flags().StringVar(&candidateArchive, "candidate", "", "Install one local portable archive without network access")
	cmd.Flags().StringVar(&candidateChecksums, "checksums", "", "Checksum manifest for --candidate")
	cmd.MarkFlagsRequiredTogether("candidate", "checksums")
	cmd.MarkFlagsMutuallyExclusive("rollback", "candidate")
	return cmd
}

func firstCheckRuntime(cfg domain.Config) (domain.Runtime, error) {
	profile, ok := cfg.Profiles[cfg.Routes.Default]
	if !ok {
		return domain.Runtime{}, fmt.Errorf("Current default route has no testable endpoint; run `aigw use` to choose a model profile")
	}
	client := profile.Client
	if client == "" {
		for _, candidate := range domain.AdmittedClientIDs() {
			if _, _, err := cfg.ResolveRuntime(candidate, cfg.Routes.Default); err == nil {
				client = candidate
				break
			}
		}
	}
	if client == "" {
		return domain.Runtime{}, fmt.Errorf("Current default route has no testable endpoint; run `aigw use` to choose a model profile")
	}
	runtime, _, err := cfg.ResolveRuntime(client, cfg.Routes.Default)
	if err != nil {
		return domain.Runtime{}, fmt.Errorf("Current default route has no testable endpoint; run `aigw use` to choose a model profile")
	}
	return runtime, nil
}
