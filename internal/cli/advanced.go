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
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/manifest"
)

func newProfileCommand(app *App) *cobra.Command {
	root := &cobra.Command{Use: "profile", Short: "Inspect and maintain provider profiles"}
	root.AddCommand(newProfileListCommand(app), newProfileShowCommand(app), newProfileEditCommand(app), newProfileRenameCommand(app), newProfileRemoveCommand(app))
	return root
}

func newProfileListCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List provider profiles", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			names := sortedProfileNames(cfg)
			for _, name := range names {
				mark := " "
				if name == cfg.Routes.Default {
					mark = "*"
				}
				secret := "missing token"
				if app.Secrets.Has(name) {
					secret = "token available"
				}
				fmt.Fprintf(app.Out, "%s %-20s %s · %s\n", mark, name, cfg.Profiles[name].Label, secret)
			}
			return nil
		},
	}
}

func newProfileShowCommand(app *App) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use: "show <profile>", Short: "Show non-secret profile metadata", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			profile, ok := cfg.Profiles[args[0]]
			if !ok {
				return fmt.Errorf("unknown profile %q", args[0])
			}
			if jsonMode {
				return json.NewEncoder(app.Out).Encode(map[string]any{"id": args[0], "label": profile.Label, "endpoints": profile.Endpoints, "secret_available": app.Secrets.Has(args[0])})
			}
			fmt.Fprintf(app.Out, "Profile     %s\nLabel       %s\n", args[0], profile.Label)
			if profile.Endpoints.OpenAIResponses != "" {
				fmt.Fprintf(app.Out, "OpenAI      %s\n", profile.Endpoints.OpenAIResponses)
			}
			if profile.Endpoints.Anthropic != "" {
				fmt.Fprintf(app.Out, "Anthropic   %s\n", profile.Endpoints.Anthropic)
			}
			fmt.Fprintf(app.Out, "Token       %s\n", map[bool]string{true: "available", false: "missing"}[app.Secrets.Has(args[0])])
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "emit machine-readable JSON")
	return cmd
}

func newProfileEditCommand(app *App) *cobra.Command {
	var label, openAIURL, anthropicURL string
	cmd := &cobra.Command{
		Use: "edit <profile>", Short: "Edit profile label or endpoints", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if label == "" && openAIURL == "" && anthropicURL == "" {
				return fmt.Errorf("nothing to edit; provide --label, --openai-url, or --anthropic-url")
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			profile, ok := cfg.Profiles[args[0]]
			if !ok {
				return fmt.Errorf("unknown profile %q", args[0])
			}
			if label != "" {
				profile.Label = label
			}
			if openAIURL != "" {
				profile.Endpoints.OpenAIResponses = strings.TrimRight(openAIURL, "/")
			}
			if anthropicURL != "" {
				profile.Endpoints.Anthropic = strings.TrimRight(anthropicURL, "/")
			}
			cfg.Profiles[args[0]] = profile
			if err := app.Config.Save(cfg); err != nil {
				return err
			}
			if err := syncAdapters(context.Background(), app, cfg); err != nil {
				return fmt.Errorf("profile saved, but adapter sync failed: %w; repair with `aigw sync`", err)
			}
			fmt.Fprintf(app.Out, "Updated %s.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "new display label")
	cmd.Flags().StringVar(&openAIURL, "openai-url", "", "new OpenAI Responses URL")
	cmd.Flags().StringVar(&anthropicURL, "anthropic-url", "", "new Anthropic URL")
	return cmd
}

func newProfileRenameCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use: "rename <old> <new>", Short: "Rename a profile and its secret slot", Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			oldName, newName := args[0], args[1]
			if !domain.ValidProfileName(newName) {
				return fmt.Errorf("invalid new profile name %q", newName)
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			profile, ok := cfg.Profiles[oldName]
			if !ok {
				return fmt.Errorf("unknown profile %q", oldName)
			}
			if _, exists := cfg.Profiles[newName]; exists {
				return fmt.Errorf("profile %q already exists", newName)
			}
			token, err := app.Secrets.Get(oldName)
			if err != nil {
				return fmt.Errorf("cannot rename profile without its secret: %w", err)
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
			if err := app.Secrets.Set(newName, token); err != nil {
				return err
			}
			if err := app.Config.Save(cfg); err != nil {
				_ = app.Secrets.Delete(newName)
				return err
			}
			if err := app.Secrets.Delete(oldName); err != nil {
				return fmt.Errorf("profile renamed, but old secret slot remains: %w", err)
			}
			if err := syncAdapters(context.Background(), app, cfg); err != nil {
				return fmt.Errorf("profile renamed, but adapter sync failed: %w; repair with `aigw sync`", err)
			}
			fmt.Fprintf(app.Out, "Renamed %s to %s.\n", oldName, newName)
			return nil
		},
	}
}

func newProfileRemoveCommand(app *App) *cobra.Command {
	var keepSecret bool
	cmd := &cobra.Command{
		Use: "remove <profile>", Short: "Remove an unused profile", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			name := args[0]
			if _, ok := cfg.Profiles[name]; !ok {
				return fmt.Errorf("unknown profile %q", name)
			}
			if cfg.Routes.Default == name {
				return fmt.Errorf("profile %q is active as the default route; switch with `aigw use <other>` first", name)
			}
			for client, route := range cfg.Routes.Overrides {
				if route == name {
					return fmt.Errorf("profile %q is active for %s; reset with `aigw route reset %s` first", name, client, client)
				}
			}
			delete(cfg.Profiles, name)
			if err := app.Config.Save(cfg); err != nil {
				return err
			}
			if !keepSecret {
				if err := app.Secrets.Delete(name); err != nil {
					return fmt.Errorf("profile removed, but secret deletion failed: %w", err)
				}
			}
			fmt.Fprintf(app.Out, "Removed %s.\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&keepSecret, "keep-secret", false, "leave the system-keyring entry in place")
	return cmd
}

func newRouteCommand(app *App) *cobra.Command {
	root := &cobra.Command{Use: "route", Short: "Inspect or reset inherited client routes"}
	root.AddCommand(
		&cobra.Command{Use: "list", Short: "List resolved routes", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return runStatus(cmd, app, false) }},
		&cobra.Command{Use: "reset <claude|codex>", Short: "Restore one client to default inheritance", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
			client := args[0]
			if client != domain.ClientClaude && client != domain.ClientCodex {
				return fmt.Errorf("route must be claude or codex")
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			delete(cfg.Routes.Overrides, client)
			if err := app.Config.Save(cfg); err != nil {
				return err
			}
			if err := syncAdapters(context.Background(), app, cfg); err != nil {
				return fmt.Errorf("route reset, but adapter sync failed: %w; repair with `aigw sync`", err)
			}
			fmt.Fprintf(app.Out, "%s now inherits the default route.\n", title(client))
			return nil
		}},
	)
	return root
}

func newAdapterCommand(app *App) *cobra.Command {
	root := &cobra.Command{Use: "adapter", Short: "Discover and maintain client adapters"}
	root.AddCommand(newAdapterListCommand(app), newAdapterDiscoverCommand(app), newAdapterEnableCommand(app), newAdapterDisableCommand(app))
	return root
}

func newAdapterListCommand(app *App) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List adapter state", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := app.Config.Load()
		if err != nil {
			return err
		}
		for _, name := range []string{domain.ClientClaude, domain.ClientCodex} {
			adapter := cfg.Adapters[name]
			state := "disabled"
			if adapter.Enabled {
				state = "enabled"
			}
			fmt.Fprintf(app.Out, "%-8s %-8s %s\n", title(name), state, adapter.Executable)
		}
		return nil
	}}
}

func newAdapterDiscoverCommand(app *App) *cobra.Command {
	return &cobra.Command{Use: "discover", Short: "Find installed Claude and Codex executables", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		for _, name := range []string{domain.ClientClaude, domain.ClientCodex} {
			path, err := exec.LookPath(name)
			if err != nil {
				fmt.Fprintf(app.Out, "%-8s not found\n", title(name))
				continue
			}
			fmt.Fprintf(app.Out, "%-8s %s\n", title(name), path)
		}
		return nil
	}}
}

func newAdapterEnableCommand(app *App) *cobra.Command {
	var executable string
	var targets []string
	cmd := &cobra.Command{Use: "enable <claude|codex>", Short: "Enable a client adapter", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		client := args[0]
		if client != domain.ClientClaude && client != domain.ClientCodex {
			return fmt.Errorf("adapter must be claude or codex")
		}
		if executable == "" {
			return fmt.Errorf("--executable is required; run `aigw adapter discover`")
		}
		if client == domain.ClientCodex && len(targets) == 0 {
			return fmt.Errorf("Codex adapter needs at least one --target config.toml")
		}
		cfg, err := app.Config.Load()
		if err != nil {
			return err
		}
		profile, _, err := cfg.Resolve(client, "")
		if err != nil {
			return err
		}
		if _, err := profile.EndpointFor(client); err != nil {
			return err
		}
		if !app.Secrets.Has(profile.ID) {
			return fmt.Errorf("profile %q has no token; repair with `aigw rotate %s`", profile.ID, profile.ID)
		}
		cfg.Adapters[client] = domain.AdapterConfig{Enabled: true, Executable: executable, Targets: append([]string(nil), targets...)}
		if client == domain.ClientClaude {
			if _, err := app.Shims.EnableClaude(); err != nil {
				return err
			}
		}
		if err := app.Config.Save(cfg); err != nil {
			if client == domain.ClientClaude {
				_ = app.Shims.DisableClaude()
			}
			return err
		}
		if err := syncAdapters(cmd.Context(), app, cfg); err != nil {
			delete(cfg.Adapters, client)
			_ = app.Config.Save(cfg)
			for _, target := range targets {
				_ = adapters.DisableCodexConfig(target)
			}
			return fmt.Errorf("adapter enable failed and was rolled back: %w", err)
		}
		fmt.Fprintf(app.Out, "Enabled %s adapter.\n", title(client))
		return nil
	}}
	cmd.Flags().StringVar(&executable, "executable", "", "real client executable path")
	cmd.Flags().StringSliceVar(&targets, "target", nil, "client config path; repeat for multiple Codex homes")
	return cmd
}

func newAdapterDisableCommand(app *App) *cobra.Command {
	return &cobra.Command{Use: "disable <claude|codex>", Short: "Disable a client adapter and remove owned projections", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		client := args[0]
		if client != domain.ClientClaude && client != domain.ClientCodex {
			return fmt.Errorf("adapter must be claude or codex")
		}
		cfg, err := app.Config.Load()
		if err != nil {
			return err
		}
		adapter, ok := cfg.Adapters[client]
		if !ok || !adapter.Enabled {
			fmt.Fprintf(app.Out, "%s adapter is already disabled.\n", title(client))
			return nil
		}
		if client == domain.ClientCodex {
			for _, target := range adapter.Targets {
				if err := adapters.DisableCodexConfig(target); err != nil {
					return err
				}
			}
		}
		if client == domain.ClientClaude {
			if err := app.Shims.DisableClaude(); err != nil {
				return err
			}
		}
		delete(cfg.Adapters, client)
		if err := app.Config.Save(cfg); err != nil {
			return err
		}
		fmt.Fprintf(app.Out, "Disabled %s adapter.\n", title(client))
		return nil
	}}
}

func newConfigCommand(app *App) *cobra.Command {
	root := &cobra.Command{Use: "config", Short: "Import, export, and inspect AIGW configuration"}
	root.AddCommand(
		&cobra.Command{Use: "path", Short: "Print the local config path", Args: cobra.NoArgs, Run: func(_ *cobra.Command, _ []string) { fmt.Fprintln(app.Out, app.Config.Path()) }},
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
		&cobra.Command{Use: "import <team-profiles.toml>", Short: "Merge a secret-free team manifest", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read team manifest: %w", err)
			}
			team, err := manifest.Parse(data)
			if err != nil {
				return err
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			cfg, err = manifest.Merge(cfg, team)
			if err != nil {
				return err
			}
			if err := app.Config.Save(cfg); err != nil {
				return err
			}
			missing := []string{}
			for name := range team.Profiles {
				if !app.Secrets.Has(name) {
					missing = append(missing, name)
				}
			}
			sort.Strings(missing)
			fmt.Fprintf(app.Out, "Imported %d profiles.\n", len(team.Profiles))
			for _, name := range missing {
				fmt.Fprintf(app.Out, "Next: aigw rotate %s\n", name)
			}
			return nil
		}},
		&cobra.Command{Use: "migrate <legacy-config.json>", Short: "Migrate the pre-product AIGW JSON structure", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read legacy config: %w", err)
			}
			cfg, err := manifest.MigrateLegacyV2(data)
			if err != nil {
				return err
			}
			if current, loadErr := app.Config.Load(); loadErr == nil && len(current.Profiles) > 0 {
				return fmt.Errorf("current AIGW config is not empty; refusing to overwrite %s", app.Config.Path())
			}
			if err := app.Config.Save(cfg); err != nil {
				return err
			}
			fmt.Fprintf(app.Out, "Migrated %d profiles. Secrets remained in the system keyring.\n", len(cfg.Profiles))
			return nil
		}},
	)
	return root
}

func RunClaude(app *App, args []string) error {
	cfg, err := app.Config.Load()
	if err != nil {
		return err
	}
	adapter := cfg.Adapters[domain.ClientClaude]
	if !adapter.Enabled || adapter.Executable == "" {
		return fmt.Errorf("Claude adapter is disabled; run `aigw adapter enable claude --executable <real-claude>`")
	}
	profile, _, err := cfg.Resolve(domain.ClientClaude, "")
	if err != nil {
		return err
	}
	token, err := app.Secrets.Get(profile.ID)
	if err != nil {
		return fmt.Errorf("Claude route token unavailable: %w; repair with `aigw rotate %s`", err, profile.ID)
	}
	plan, err := adapters.ClaudePlan(adapter.Executable, args, os.Environ(), profile, token)
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

func title(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
