package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/manifest"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/presentation"
)

func newProfileCommand(app *App) *cobra.Command {
	root := &cobra.Command{Use: "profile", Short: "Manage service profiles"}
	root.AddCommand(newProfileAddCommand(app), newProfileListCommand(app), newProfileShowCommand(app), newProfileEditCommand(app), newProfileRenameCommand(app), newProfileRemoveCommand(app))
	return root
}

func newProfileAddCommand(app *App) *cobra.Command {
	var accountName, client, model, label, purpose string
	cmd := &cobra.Command{
		Use: "add <profile>", Short: "Add a model profile to an existing account", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]
			if !domain.ValidProfileName(profileName) {
				return fmt.Errorf("Invalid profile ID %q; use letters, numbers, dots, hyphens, or underscores", profileName)
			}
			if accountName == "" || client == "" || model == "" {
				return fmt.Errorf("--account, --for, and --model are required; run `aigw profile add --help`")
			}
			if !domain.IsAdmittedClient(client) {
				return fmt.Errorf("--for must be claude or codex; run `aigw profile add --help`")
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			if _, exists := cfg.Profiles[profileName]; exists {
				return fmt.Errorf("Profile %q already exists", profileName)
			}
			account, exists := cfg.Accounts[accountName]
			if !exists {
				return fmt.Errorf("Unknown account %q; first run `aigw add %s ...`", accountName, accountName)
			}
			account.ID = accountName
			if _, err := account.EndpointFor(client); err != nil {
				return err
			}
			if label == "" {
				label = profileName
			}
			models := domain.Models{client: model}
			before := cloneConfig(cfg)
			cfg.Profiles[profileName] = domain.Profile{Label: label, Purpose: strings.TrimSpace(purpose), Account: accountName, Client: client, Models: models}
			if err := commitConfigAndSync(cmd.Context(), app, before, cfg, "profile add"); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "Model profile added")
			r.Row("Configuration", profileName)
			r.Row("Account", accountName)
			r.Row("Model", model)
			if purpose := strings.TrimSpace(purpose); purpose != "" {
				r.Row("Purpose", purpose)
			}
			r.Success("Reused the existing account token; current route was not changed")
			r.Next("aigw use " + profileName + " --for " + client)
			return nil
		},
	}
	cmd.Flags().StringVar(&accountName, "account", "", "Existing account ID")
	cmd.Flags().StringVar(&client, "for", "", "Client: claude or codex")
	cmd.Flags().StringVar(&model, "model", "", "Upstream model ID")
	cmd.Flags().StringVar(&label, "label", "", "Display name")
	cmd.Flags().StringVar(&purpose, "purpose", "", "Purpose note (display only)")
	return cmd
}

func newAccountEditCommand(app *App) *cobra.Command {
	var label, openAIURL, anthropicURL string
	cmd := &cobra.Command{
		Use: "edit <account>", Short: "Update account metadata and protocol endpoints", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if label == "" && openAIURL == "" && anthropicURL == "" {
				return fmt.Errorf("Nothing to update; provide --label, --openai-url, or --anthropic-url")
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			account, exists := cfg.Accounts[args[0]]
			if !exists {
				return fmt.Errorf("Unknown account %q", args[0])
			}
			before := cloneConfig(cfg)
			if label != "" {
				account.Label = label
			}
			if openAIURL != "" {
				account.Endpoints.OpenAIResponses = strings.TrimRight(openAIURL, "/")
			}
			if anthropicURL != "" {
				account.Endpoints.Anthropic = strings.TrimRight(anthropicURL, "/")
			}
			cfg.Accounts[args[0]] = account
			if err := commitConfigAndSync(cmd.Context(), app, before, cfg, "account edit"); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "Account updated")
			r.Row("Account", args[0])
			r.Success("Profiles using this account now use the same endpoints; token was not changed")
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "New display name")
	cmd.Flags().StringVar(&openAIURL, "openai-url", "", "New OpenAI Responses URL")
	cmd.Flags().StringVar(&anthropicURL, "anthropic-url", "", "New Anthropic URL")
	return cmd
}

func newProfileListCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List service profiles", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			names := sortedProfileNames(cfg)
			r := renderer(app)
			r.Title("AIGW", "Service profiles")
			r.Section("Available profiles")
			for _, name := range names {
				state := presentation.Info
				stateText := "Available"
				if name == cfg.Routes.Default {
					state = presentation.OK
					stateText = "Current"
				}
				accountName := cfg.Profiles[name].Account
				if accountName == "" {
					accountName = name
				}
				secret := "Token missing"
				if app.Secrets.Has(accountName) {
					secret = "Token available"
				}
				profile := cfg.Profiles[name]
				detail := []string{}
				if profile.Client != "" {
					detail = append(detail, title(profile.Client))
				}
				detail = append(detail, profileChoiceLabel(profile), stateText, "Account "+accountName, secret)
				r.StatusLine(state, "Configuration", name)
				r.Detail(strings.Join(detail, " · "))
			}
			r.Next("aigw use")
			return nil
		},
	}
}

func newProfileShowCommand(app *App) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use: "show <profile>", Short: "Show secret-free profile metadata", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			profile, ok := cfg.Profiles[args[0]]
			if !ok {
				return fmt.Errorf("Unknown profile %q", args[0])
			}
			accountName := profile.Account
			if accountName == "" {
				accountName = args[0]
			}
			account := cfg.Accounts[accountName]
			if jsonMode {
				return json.NewEncoder(app.Out).Encode(map[string]any{"id": args[0], "label": profile.Label, "purpose": profile.Purpose, "account": accountName, "models": profile.Models, "endpoints": account.Endpoints, "secret_available": app.Secrets.Has(accountName)})
			}
			r := renderer(app)
			r.Title("AIGW", "Service details")
			r.Section("Service profiles")
			r.Row("Profile ID", args[0])
			r.Row("Name", profile.Label)
			if purpose := strings.TrimSpace(profile.Purpose); purpose != "" {
				r.Row("Purpose", purpose)
			}
			r.Row("Account", accountName)
			if profile.ModelFor(domain.ClientCodex) != "" {
				r.Row("Codex model", profile.ModelFor(domain.ClientCodex))
			}
			if profile.ModelFor(domain.ClientClaude) != "" {
				r.Row("Claude model", profile.ModelFor(domain.ClientClaude))
			}
			if account.Endpoints.OpenAIResponses != "" {
				r.Row("OpenAI", account.Endpoints.OpenAIResponses)
			}
			if account.Endpoints.Anthropic != "" {
				r.Row("Anthropic", account.Endpoints.Anthropic)
			}
			secretState := presentation.Warn
			secretText := "Missing"
			if app.Secrets.Has(accountName) {
				secretState = presentation.OK
				secretText = "Available"
			}
			r.Status(secretState, "System secret", secretText)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	return cmd
}

func newProfileEditCommand(app *App) *cobra.Command {
	var label, purpose string
	cmd := &cobra.Command{
		Use: "edit <profile>", Short: "Update profile display metadata", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if label == "" && !cmd.Flags().Changed("purpose") {
				return fmt.Errorf("Nothing to update; provide --label or --purpose; use `aigw account edit <account>` for endpoints")
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			before := cloneConfig(cfg)
			profile, ok := cfg.Profiles[args[0]]
			if !ok {
				return fmt.Errorf("Unknown profile %q", args[0])
			}
			if label != "" {
				profile.Label = label
			}
			if cmd.Flags().Changed("purpose") {
				profile.Purpose = strings.TrimSpace(purpose)
			}
			cfg.Profiles[args[0]] = profile
			projectionChanged := codexProjectionChanged(before, cfg)
			if err := commitConfigAndSync(context.Background(), app, before, cfg, "profile"); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "Profile updated")
			r.Row("Configuration", args[0])
			if projectionChanged {
				r.Success("Client configuration synchronized")
			} else {
				r.Success("Display metadata saved; client configuration was not changed")
			}
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "New display name")
	cmd.Flags().StringVar(&purpose, "purpose", "", "Purpose note (pass an empty value to clear)")
	return cmd
}

func newProfileRenameCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use: "rename <old> <new>", Short: "Rename a profile", Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			oldName, newName := args[0], args[1]
			if !domain.ValidProfileName(newName) {
				return fmt.Errorf("Invalid new profile ID %q", newName)
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			before := cloneConfig(cfg)
			profile, ok := cfg.Profiles[oldName]
			if !ok {
				return fmt.Errorf("Unknown profile %q", oldName)
			}
			if _, exists := cfg.Profiles[newName]; exists {
				return fmt.Errorf("Profile %q already exists", newName)
			}
			delete(cfg.Profiles, oldName)
			cfg.Profiles[newName] = profile
			if cfg.Routes.Default == oldName {
				cfg.Routes.Default = newName
			}
			for client, name := range cfg.Routes.Overrides {
				if name == oldName {
					cfg.Routes.Overrides[client] = newName
				}
			}
			if err := commitConfigAndSync(context.Background(), app, before, cfg, "profile rename"); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "Profile renamed")
			r.Row("Previous profile", oldName)
			r.Row("New profile", newName)
			r.Row("Account", profile.Account)
			r.Success("The account token remains in place and routes were synchronized")
			return nil
		},
	}
}

func newProfileRemoveCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use: "remove <profile>", Short: "Remove an unused profile", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			before := cloneConfig(cfg)
			name := args[0]
			profile, ok := cfg.Profiles[name]
			if !ok {
				return fmt.Errorf("Unknown profile %q", name)
			}
			if cfg.Routes.Default == name {
				return fmt.Errorf("Profile %q is the default route; first run `aigw use <other>`", name)
			}
			for client, route := range cfg.Routes.Overrides {
				if route == name {
					return fmt.Errorf("Profile %q is used by %s; first run `aigw route reset %s`", name, client, client)
				}
			}
			delete(cfg.Profiles, name)
			if err := commitConfigAndSync(context.Background(), app, before, cfg, "profile remove"); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "Profile removed")
			r.Row("Configuration", name)
			r.Row("Account", profile.Account)
			r.Success("The account and token remain in place")
			return nil
		},
	}
}

func newRouteCommand(app *App) *cobra.Command {
	root := &cobra.Command{Use: "route", Short: "Manage client routes"}
	root.AddCommand(
		&cobra.Command{Use: "list", Short: "Show the default route and client overrides", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error { return runRouteList(app) }},
		&cobra.Command{Use: "reset <claude|codex>", Short: "Restore a client's inheritance of the default route", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			client := args[0]
			if !domain.IsAdmittedClient(client) {
				return fmt.Errorf("Client must be claude or codex; run `aigw route reset --help`")
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			before := cloneConfig(cfg)
			delete(cfg.Routes.Overrides, client)
			if err := commitConfigAndSync(cmd.Context(), app, before, cfg, "route reset"); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "Route reset")
			r.Row("Client", title(client))
			r.Success("Now inherits the default service")
			r.Next("aigw check")
			return nil
		}},
		newRouteFallbackCommand(app),
		newRouteRestoreCommand(app),
		newRouteRecoverCommand(app),
		newRouteDoctorCommand(app),
		newRouteAttestCommand(app),
	)
	return root
}

func newAdapterCommand(app *App) *cobra.Command {
	root := &cobra.Command{Use: "adapter", Short: "Manage Claude/Codex adapters"}
	root.AddCommand(newAdapterListCommand(app), newAdapterDiscoverCommand(app), newAdapterEnableCommand(app), newAdapterAuthCommand(app), newAdapterDisableCommand(app))
	return root
}

func newAdapterListCommand(app *App) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List adapter status", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := app.Config.Load()
		if err != nil {
			return err
		}
		r := renderer(app)
		r.Title("AIGW", "Client adapters")
		r.Section("Adapter")
		for _, name := range domain.AdmittedClientIDs() {
			adapter := cfg.Adapters[name]
			state := presentation.Info
			stateText := "Disabled"
			if adapter.Enabled {
				state = presentation.OK
				stateText = "Enabled"
			}
			r.Status(state, title(name), stateText)
			if adapter.Executable != "" {
				r.Detail(adapter.Executable)
			}
		}
		return nil
	}}
}

func newAdapterDiscoverCommand(app *App) *cobra.Command {
	return &cobra.Command{Use: "discover", Short: "Discover installed Claude and Codex executables", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		r := renderer(app)
		r.Title("AIGW", "Client discovery")
		r.Section("Installed clients")
		for _, name := range domain.AdmittedClientIDs() {
			path, err := exec.LookPath(name)
			if err != nil {
				r.Status(presentation.Info, title(name), "Not found")
				continue
			}
			r.Status(presentation.OK, title(name), path)
		}
		return nil
	}}
}

func newAdapterEnableCommand(app *App) *cobra.Command {
	var executable string
	var targets []string
	cmd := &cobra.Command{Use: "enable <claude|codex>", Short: "Enable a client adapter", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		client := args[0]
		if !domain.IsAdmittedClient(client) {
			return fmt.Errorf("Client must be claude or codex; run `aigw adapter enable --help`")
		}
		if executable == "" {
			return fmt.Errorf("--executable is required; run `aigw adapter discover`")
		}
		if client == domain.ClientCodex && len(targets) == 0 {
			return fmt.Errorf("Codex adapter requires at least one --target config.toml")
		}
		cfg, err := app.Config.Load()
		if err != nil {
			return err
		}
		before := cloneConfig(cfg)
		if current := before.Adapters[client]; current.Enabled {
			return fmt.Errorf("%s adapter is already enabled; disable it before changing the executable or config targets", title(client))
		}
		runtime, _, err := cfg.ResolveRuntime(client, "")
		if err != nil {
			return err
		}
		if runtime.Endpoint == "" {
			return fmt.Errorf("Profile %q has no %s endpoint", runtime.ProfileID, title(client))
		}
		accountName := runtime.AccountID
		if !app.Secrets.Has(accountName) {
			return fmt.Errorf("Account %q is missing a token; run `aigw rotate %s`", accountName, accountName)
		}
		if client == domain.ClientCodex {
			discovered, err := discoveredResult(app)
			if err != nil {
				return err
			}
			for _, target := range targets {
				if err := validateExplicitCodexTarget(discovered, target); err != nil {
					return err
				}
			}
		}
		cfg.Adapters[client] = domain.AdapterConfig{Enabled: true, Executable: executable, Targets: append([]string(nil), targets...)}
		if client == domain.ClientClaude {
			if _, err := app.Shims.EnableClaude(); err != nil {
				return err
			}
		}
		if err := commitConfigAndSync(cmd.Context(), app, before, cfg, "adapter enable"); err != nil {
			if client == domain.ClientClaude {
				_ = app.Shims.DisableClaude()
			}
			return fmt.Errorf("Adapter enablement failed and was rolled back: %w", err)
		}
		r := renderer(app)
		r.Title("AIGW", "Client enabled")
		r.Row("Client", title(client))
		r.Status(presentation.OK, "Adapter", "Configured")
		r.Next("aigw check")
		return nil
	}}
	cmd.Flags().StringVar(&executable, "executable", "", "Path to the real client executable")
	cmd.Flags().StringSliceVar(&targets, "target", nil, "Client configuration path; repeat for multiple Codex homes")
	return cmd
}

func newAdapterAuthCommand(app *App) *cobra.Command {
	return &cobra.Command{Use: "auth codex", Short: "Bind the current account token to Codex", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] != domain.ClientCodex {
			return fmt.Errorf("Native credential binding is available only for codex; run `aigw adapter auth codex`")
		}
		cfg, err := app.Config.Load()
		if err != nil {
			return err
		}
		if !cfg.Adapters[domain.ClientCodex].Enabled {
			return fmt.Errorf("Codex adapter is not enabled; first run `aigw adapter enable codex ...`")
		}
		if err := bindCodexAuthentication(cmd.Context(), app, cfg); err != nil {
			return fmt.Errorf("Failed to bind Codex authentication: %w", err)
		}
		r := renderer(app)
		r.Title("AIGW", "Codex authentication bound")
		r.Success("The current account token was written to Codex native credential storage")
		r.Next("aigw doctor")
		return nil
	}}
}

func newAdapterDisableCommand(app *App) *cobra.Command {
	return &cobra.Command{Use: "disable <claude|codex>", Short: "Disable a client adapter and remove AIGW-owned projections", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		client := args[0]
		if !domain.IsAdmittedClient(client) {
			return fmt.Errorf("Client must be claude or codex; run `aigw adapter disable --help`")
		}
		cfg, err := app.Config.Load()
		if err != nil {
			return err
		}
		before := cloneConfig(cfg)
		adapter, ok := cfg.Adapters[client]
		if !ok || !adapter.Enabled {
			r := renderer(app)
			r.Title("AIGW", "Client adapters")
			r.Status(presentation.Info, title(client), "Already disabled")
			return nil
		}
		if client == domain.ClientClaude {
			if err := app.Shims.DisableClaude(); err != nil {
				return err
			}
		}
		delete(cfg.Adapters, client)
		if err := commitConfigAndSync(context.Background(), app, before, cfg, "adapter disable"); err != nil {
			if client == domain.ClientClaude {
				_, _ = app.Shims.EnableClaude()
			}
			return err
		}
		r := renderer(app)
		r.Title("AIGW", "Client disabled")
		r.Row("Client", title(client))
		r.Success("All AIGW-owned projections were safely removed")
		return nil
	}}
}

func validateExplicitCodexTarget(discovered discovery.Result, path string) error {
	if surface, ok := discovered.SurfaceForExecutablePath(path); ok {
		if surface.ID == discovery.SurfaceJunieCLI {
			return fmt.Errorf("Junie CLI is JetBrains-owned and is not a Codex config target")
		}
		return fmt.Errorf("an executable is not a Codex configuration target")
	}
	surface, ok := discovered.SurfaceForConfigPath(path)
	if !ok {
		return nil
	}
	switch surface.ID {
	case discovery.SurfaceCodexCLIStandalone:
		return nil
	case discovery.SurfaceAirCodex:
		return fmt.Errorf("Air is JetBrains-owned; use an explicit Air fallback command")
	case discovery.SurfacePyCharmCodex:
		return fmt.Errorf("PyCharm is JetBrains-owned and cannot be enabled as an AIGW target")
	default:
		return fmt.Errorf("surface %s is not an AIGW Codex target", surface.ID)
	}
}

func newConfigCommand(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:   "config",
		Short: "Import and export configuration",
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("Choose a config subcommand; run `aigw config --help`")
			}
			return fmt.Errorf("Unknown config subcommand %q; run `aigw config --help`", args[0])
		},
	}
	root.AddCommand(
		&cobra.Command{Use: "path", Short: "Print the local configuration path", Args: cobra.NoArgs, Run: func(_ *cobra.Command, _ []string) { fmt.Fprintln(app.Out, app.Config.Path()) }},
		&cobra.Command{Use: "export", Short: "Export a secret-free team manifest", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			data, err := manifest.Export(cfg)
			if err != nil {
				return err
			}
			_, err = app.Out.Write(data)
			return err
		}},
		func() *cobra.Command {
			var replaceAccounts, replaceProfiles []string
			cmd := &cobra.Command{Use: "import <team-profiles.toml>", Short: "Merge a secret-free team manifest", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
				data, err := os.ReadFile(args[0])
				if err != nil {
					return fmt.Errorf("Failed to read team manifest: %w", err)
				}
				team, err := manifest.Parse(data)
				if err != nil {
					return err
				}
				cfg, err := app.Config.Load()
				if err != nil {
					return err
				}
				before := cloneConfig(cfg)
				cfg, err = manifest.MergeWithOptions(cfg, team, manifest.MergeOptions{ReplaceAccounts: namedReplacementSet(replaceAccounts), ReplaceProfiles: namedReplacementSet(replaceProfiles)})
				if err != nil {
					return err
				}
				if err := commitConfigAndSync(context.Background(), app, before, cfg, "team manifest"); err != nil {
					return err
				}
				accountNames := importedAccountNames(team)
				missing := []string{}
				r := renderer(app)
				r.Title("AIGW", "Team configuration imported")
				r.Row("Profiles", fmt.Sprintf("%d", len(team.Profiles)))
				r.Row("Accounts", fmt.Sprintf("%d", len(accountNames)))
				for _, name := range accountNames {
					if app.Secrets.Has(name) {
						r.Status(presentation.OK, "System secret", name+" Token available")
						continue
					}
					missing = append(missing, name)
					r.Status(presentation.Warn, name, "Token required")
				}
				if len(missing) > 0 {
					if len(missing) == 1 {
						r.Next("aigw rotate " + missing[0])
					} else {
						r.Next("aigw rotate <account>")
					}
				} else {
					r.Next("aigw models")
				}
				return nil
			}}
			cmd.Flags().StringSliceVar(&replaceAccounts, "replace-account", nil, "Explicitly replace conflicting account metadata; system tokens remain unchanged")
			cmd.Flags().StringSliceVar(&replaceProfiles, "replace-profile", nil, "Explicitly replace conflicting model profiles")
			return cmd
		}(),
	)
	return root
}

func namedReplacementSet(names []string) map[string]bool {
	result := make(map[string]bool, len(names))
	for _, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			result[name] = true
		}
	}
	return result
}

func importedAccountNames(team manifest.Manifest) []string {
	seen := map[string]bool{}
	for name := range team.Accounts {
		seen[name] = true
	}
	for name, profile := range team.Profiles {
		accountName := profile.Account
		if accountName == "" {
			accountName = name
		}
		seen[accountName] = true
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func RunClaude(app *App, args []string) error {
	cfg, err := app.Config.Load()
	if err != nil {
		return err
	}
	adapter := cfg.Adapters[domain.ClientClaude]
	if !adapter.Enabled || adapter.Executable == "" {
		return fmt.Errorf("Claude adapter is not enabled; run `aigw adapter enable claude --executable <real-claude>`")
	}
	runtime, _, err := cfg.ResolveRuntime(domain.ClientClaude, "")
	if err != nil {
		return err
	}
	accountName := runtime.AccountID
	token, err := app.Secrets.Get(accountName)
	if err != nil {
		return fmt.Errorf("Token for the Claude route is unavailable: %w; run `aigw rotate %s`", err, accountName)
	}
	plan, err := adapters.ClaudePlan(adapter.Executable, args, os.Environ(), runtime, token)
	if err != nil {
		return err
	}
	return app.Runner.Run(context.Background(), plan)
}

func sortedProfileNames(cfg domain.Config) []string {
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedAccountNames(cfg domain.Config) []string {
	cfg.Normalize()
	names := make([]string, 0, len(cfg.Accounts))
	for name := range cfg.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func title(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
