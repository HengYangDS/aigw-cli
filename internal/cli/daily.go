package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/secrets"
)

func newAddCommand(app *App) *cobra.Command {
	var label, openAIURL, anthropicURL string
	var tokenStdin bool
	cmd := &cobra.Command{
		Use:   "add <profile>",
		Short: "Add a provider profile and its token",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]
			if !domain.ValidProfileName(name) {
				return fmt.Errorf("invalid profile name %q; use letters, numbers, dot, dash, or underscore", name)
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			if _, exists := cfg.Profiles[name]; exists {
				return fmt.Errorf("profile %q already exists; use `aigw profile edit %s` or `aigw rotate %s`", name, name, name)
			}
			if label == "" {
				label = name
			}
			profile := domain.Profile{Label: label, Endpoints: domain.Endpoints{
				OpenAIResponses: strings.TrimRight(openAIURL, "/"),
				Anthropic:       strings.TrimRight(anthropicURL, "/"),
			}}
			token, err := app.readToken(tokenStdin, true)
			if err != nil {
				return err
			}
			cfg.Profiles[name] = profile
			if cfg.Routes.Default == "" {
				cfg.Routes.Default = name
			}
			if err := cfg.Validate(); err != nil {
				return err
			}
			if err := app.Secrets.Set(name, token); err != nil {
				return err
			}
			if err := app.Config.Save(cfg); err != nil {
				_ = app.Secrets.Delete(name)
				return err
			}
			fmt.Fprintf(app.Out, "Added %s.\nNext: aigw test\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "human-readable provider label")
	cmd.Flags().StringVar(&openAIURL, "openai-url", "", "OpenAI Responses base URL")
	cmd.Flags().StringVar(&anthropicURL, "anthropic-url", "", "Anthropic base URL")
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "read one token line from stdin")
	return cmd
}

func newUseCommand(app *App) *cobra.Command {
	var client string
	var all bool
	cmd := &cobra.Command{
		Use:   "use <profile>",
		Short: "Switch the default or one client route",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if all && client != "" {
				return fmt.Errorf("--all and --for cannot be used together")
			}
			if client != "" && client != domain.ClientClaude && client != domain.ClientCodex {
				return fmt.Errorf("--for must be claude or codex")
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			before := cloneConfig(cfg)
			name := args[0]
			if _, ok := cfg.Profiles[name]; !ok {
				return fmt.Errorf("unknown profile %q; inspect `aigw profile list`", name)
			}
			if !app.Secrets.Has(name) {
				return fmt.Errorf("profile %q has no token; repair with `aigw rotate %s`", name, name)
			}
			switch {
			case all:
				cfg.Routes.Default = name
				cfg.Routes.Overrides = map[string]string{}
			case client != "":
				cfg.Routes.Overrides[client] = name
			default:
				cfg.Routes.Default = name
			}
			if err := commitConfigAndSync(context.Background(), app, before, cfg, "route"); err != nil {
				return err
			}
			fmt.Fprintf(app.Out, "Using %s", name)
			if client != "" {
				fmt.Fprintf(app.Out, " for %s", client)
			} else if all {
				fmt.Fprint(app.Out, " for all clients")
			} else {
				fmt.Fprint(app.Out, " by default")
			}
			fmt.Fprintln(app.Out, ".")
			return nil
		},
	}
	cmd.Flags().StringVar(&client, "for", "", "set only claude or codex")
	cmd.Flags().BoolVar(&all, "all", false, "set default and clear client overrides")
	return cmd
}

func newRotateCommand(app *App) *cobra.Command {
	var tokenStdin bool
	cmd := &cobra.Command{
		Use:   "rotate <profile>",
		Short: "Replace one profile token",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			name := args[0]
			if _, ok := cfg.Profiles[name]; !ok {
				return fmt.Errorf("unknown profile %q", name)
			}
			oldToken, oldErr := app.Secrets.Get(name)
			if oldErr != nil && !errors.Is(oldErr, secrets.ErrNotFound) {
				return oldErr
			}
			token, err := app.readToken(tokenStdin, true)
			if err != nil {
				return err
			}
			if err := app.Secrets.Set(name, token); err != nil {
				return err
			}
			if err := syncAdapters(context.Background(), app, cfg); err != nil {
				var rollbackErr error
				if errors.Is(oldErr, secrets.ErrNotFound) {
					rollbackErr = app.Secrets.Delete(name)
				} else {
					rollbackErr = app.Secrets.Set(name, oldToken)
				}
				if rollbackErr == nil {
					rollbackErr = syncAdapters(context.Background(), app, cfg)
				}
				if rollbackErr != nil {
					return fmt.Errorf("token sync failed: %w; rollback also failed: %v", err, rollbackErr)
				}
				return fmt.Errorf("token sync failed and was rolled back: %w", err)
			}
			fmt.Fprintf(app.Out, "Rotated %s.\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "read one token line from stdin")
	return cmd
}

type routeStatus struct {
	Profile         string `json:"profile,omitempty"`
	Inherited       bool   `json:"inherited"`
	SecretAvailable bool   `json:"secret_available"`
	EndpointReady   bool   `json:"endpoint_ready"`
}

type statusOutput struct {
	ConfigPath string                 `json:"config_path"`
	Default    string                 `json:"default,omitempty"`
	Routes     map[string]routeStatus `json:"routes"`
	Profiles   int                    `json:"profiles"`
}

func newStatusCommand(app *App) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{Use: "status", Short: "Show profiles, routes, and readiness", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error { return runStatus(cmd, app, jsonMode) }
	cmd.Flags().BoolVar(&jsonMode, "json", false, "emit machine-readable JSON")
	return cmd
}

func runStatus(_ *cobra.Command, app *App, jsonMode bool) error {
	cfg, err := app.Config.Load()
	if err != nil {
		return err
	}
	result := statusOutput{ConfigPath: app.Config.Path(), Default: cfg.Routes.Default, Profiles: len(cfg.Profiles), Routes: map[string]routeStatus{}}
	for _, client := range []string{domain.ClientClaude, domain.ClientCodex} {
		profile, inherited, resolveErr := cfg.Resolve(client, "")
		if resolveErr != nil {
			result.Routes[client] = routeStatus{Inherited: true}
			continue
		}
		_, endpointErr := profile.EndpointFor(client)
		result.Routes[client] = routeStatus{Profile: profile.ID, Inherited: inherited, SecretAvailable: app.Secrets.Has(profile.ID), EndpointReady: endpointErr == nil}
	}
	if jsonMode {
		enc := json.NewEncoder(app.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	if len(cfg.Profiles) == 0 {
		fmt.Fprintln(app.Out, "AIGW is not configured.\n\nNext        aigw setup")
		return nil
	}
	fmt.Fprintln(app.Out, "Current")
	fmt.Fprintf(app.Out, "  Default   %s\n", result.Default)
	for _, client := range []string{domain.ClientClaude, domain.ClientCodex} {
		route := result.Routes[client]
		mode := "override"
		if route.Inherited {
			mode = "inherited"
		}
		readiness := "ready"
		if !route.SecretAvailable || !route.EndpointReady {
			readiness = "needs attention"
		}
		fmt.Fprintf(app.Out, "  %-9s %-12s %s · %s\n", strings.Title(client), route.Profile, mode, readiness)
	}
	fmt.Fprintf(app.Out, "\nProfiles    %d\n", result.Profiles)
	fmt.Fprintln(app.Out, "Next        aigw test")
	return nil
}

func newTestCommand(app *App) *cobra.Command {
	var client, profileName string
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test the resolved provider endpoint",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			clients := []string{domain.ClientClaude, domain.ClientCodex}
			if client != "" {
				if client != domain.ClientClaude && client != domain.ClientCodex {
					return fmt.Errorf("--for must be claude or codex")
				}
				clients = []string{client}
			}
			checked := 0
			for _, target := range clients {
				profile, _, err := cfg.Resolve(target, profileName)
				if err != nil {
					return err
				}
				endpoint, err := profile.EndpointFor(target)
				if err != nil {
					if client == "" {
						continue
					}
					return err
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), 12*time.Second)
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
				if err != nil {
					cancel()
					return err
				}
				token, err := app.Secrets.Get(profile.ID)
				if err != nil {
					cancel()
					return fmt.Errorf("profile %q token unavailable: %w; repair with `aigw rotate %s`", profile.ID, err, profile.ID)
				}
				req.Header.Set("Authorization", "Bearer "+token)
				resp, err := app.HTTP.Do(req)
				cancel()
				if err != nil {
					return fmt.Errorf("%s endpoint unreachable: %w", target, err)
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
					return fmt.Errorf("%s authentication rejected (HTTP %d); repair with `aigw rotate %s`", target, resp.StatusCode, profile.ID)
				}
				if resp.StatusCode >= 500 {
					return fmt.Errorf("%s endpoint returned HTTP %d", target, resp.StatusCode)
				}
				fmt.Fprintf(app.Out, "%s     %s reachable (HTTP %d)\n", strings.Title(target), profile.ID, resp.StatusCode)
				checked++
			}
			if checked == 0 {
				return fmt.Errorf("resolved profiles have no testable client endpoint")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&client, "for", "", "test only claude or codex")
	cmd.Flags().StringVar(&profileName, "profile", "", "test one profile without changing routes")
	return cmd
}

func newSyncCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Repair enabled client projections",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			if err := syncAdapters(cmd.Context(), app, cfg); err != nil {
				return err
			}
			fmt.Fprintln(app.Out, "Client projections are synchronized.")
			return nil
		},
	}
}

func syncAdapters(ctx context.Context, app *App, cfg domain.Config) error {
	if adapter := cfg.Adapters[domain.ClientCodex]; adapter.Enabled {
		profile, _, err := cfg.Resolve(domain.ClientCodex, "")
		if err != nil {
			return err
		}
		token, err := app.Secrets.Get(profile.ID)
		if err != nil {
			return fmt.Errorf("Codex route token unavailable: %w", err)
		}
		for _, target := range adapter.Targets {
			if err := adapters.SyncCodexConfig(target, profile); err != nil {
				return err
			}
			if adapter.Executable != "" && app.Runner != nil {
				plan, err := adapters.CodexLoginPlan(adapter.Executable, filepath.Dir(target), token)
				if err != nil {
					return err
				}
				if err := app.Runner.Run(ctx, plan); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func cloneConfig(cfg domain.Config) domain.Config {
	clone := cfg
	clone.Profiles = make(map[string]domain.Profile, len(cfg.Profiles))
	for name, profile := range cfg.Profiles {
		clone.Profiles[name] = profile
	}
	clone.Routes.Overrides = make(map[string]string, len(cfg.Routes.Overrides))
	for client, profile := range cfg.Routes.Overrides {
		clone.Routes.Overrides[client] = profile
	}
	clone.Adapters = make(map[string]domain.AdapterConfig, len(cfg.Adapters))
	for name, adapter := range cfg.Adapters {
		adapter.Targets = append([]string(nil), adapter.Targets...)
		clone.Adapters[name] = adapter
	}
	return clone
}

func rollbackConfigAndAdapters(ctx context.Context, app *App, before domain.Config) error {
	if err := app.Config.Save(before); err != nil {
		return err
	}
	return syncAdapters(ctx, app, before)
}

func commitConfigAndSync(ctx context.Context, app *App, before, after domain.Config, subject string) error {
	if err := app.Config.Save(after); err != nil {
		return err
	}
	if err := syncAdapters(ctx, app, after); err != nil {
		rollbackErr := rollbackConfigAndAdapters(ctx, app, before)
		if rollbackErr != nil {
			return fmt.Errorf("%s sync failed: %w; rollback also failed: %v", subject, err, rollbackErr)
		}
		return fmt.Errorf("%s sync failed and was rolled back: %w", subject, err)
	}
	return nil
}

func _processPlanCompileGuard(_ adapters.ProcessPlan) {}
