// Package adapter owns client-adapter lifecycle commands.
package adapter

import (
	"fmt"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/presentation"
	surfaceidentity "aigw-cli/internal/surface"
	"github.com/spf13/cobra"
)

// NewCommand constructs the adapter command tree.
func NewCommand(runtime invocation.Context) *cobra.Command {
	root := &cobra.Command{Use: "adapter", Short: "Manage client adapters"}
	root.AddCommand(newListCommand(runtime), newDiscoverCommand(runtime), newEnableCommand(runtime), newAuthCommand(runtime), newDisableCommand(runtime))
	return root
}

func newListCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List adapter status", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := runtime.Config.Load()
		if err != nil {
			return err
		}
		r := renderer(runtime)
		r.ProductTitle("Client adapters")
		r.Section("Adapter")
		for _, spec := range configuration.AdmittedClientSpecs() {
			adapter := cfg.Adapters[spec.ID]
			state, text := presentation.Info, "Disabled"
			if adapter.Enabled {
				state, text = presentation.OK, "Enabled"
			}
			r.Status(state, spec.Label, text)
			if adapter.Executable != "" {
				r.Detail(adapter.Executable)
			}
		}
		return nil
	}}
}

func newDiscoverCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{Use: "discover", Short: "Discover installed client executables", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		discovered, err := discover(runtime)
		if err != nil {
			return err
		}
		r := renderer(runtime)
		r.ProductTitle("Client discovery")
		r.Section("Installed clients")
		for _, spec := range configuration.AdmittedClientSpecs() {
			path := discovered.Executable(spec.ID)
			if path == "" {
				r.Status(presentation.Info, spec.Label, "Not found")
				continue
			}
			r.Status(presentation.OK, spec.Label, path)
		}
		return nil
	}}
}

func newEnableCommand(runtime invocation.Context) *cobra.Command {
	var executable string
	var targets []string
	cmd := &cobra.Command{Use: "enable <client>", Short: "Enable a client adapter", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		client := args[0]
		spec, ok := configuration.ClientSpecFor(client)
		if !ok {
			return fmt.Errorf("Client must be %s; run `aigw adapter enable --help`", configuration.AdmittedClientUsage())
		}
		if executable == "" {
			return fmt.Errorf("--executable is required; run `aigw adapter discover`")
		}
		if client == configuration.ClientCodex && len(targets) == 0 {
			return fmt.Errorf("Codex adapter requires at least one --target config.toml")
		}
		cfg, err := runtime.Config.Load()
		if err != nil {
			return err
		}
		before := cfg.Clone()
		if before.Adapters[client].Enabled {
			return fmt.Errorf("%s adapter is already enabled; disable it before changing the executable or config targets", spec.Label)
		}
		clientRuntime, _, err := cfg.ResolveRuntime(client, "")
		if err != nil {
			return err
		}
		if !runtime.Secrets.Has(clientRuntime.AccountID) {
			return fmt.Errorf("Account %q is missing a token; run `aigw rotate %s`", clientRuntime.AccountID, clientRuntime.AccountID)
		}
		if client == configuration.ClientCodex {
			discovered, err := discover(runtime)
			if err != nil {
				return err
			}
			for _, target := range targets {
				if err := ValidateCodexTarget(discovered, target); err != nil {
					return err
				}
			}
		}
		cfg.Adapters[client] = configuration.AdapterConfig{Enabled: true, Executable: executable, Targets: append([]string(nil), targets...)}
		if err := invocation.Synchronizer(runtime).Commit(cmd.Context(), before, cfg, "adapter enable"); err != nil {
			return fmt.Errorf("Adapter enablement failed and was rolled back: %w", err)
		}
		r := renderer(runtime)
		r.ProductTitle("Client enabled")
		r.Row("Client", spec.Label)
		r.Status(presentation.OK, "Adapter", "Configured")
		r.Next("aigw check")
		return nil
	}}
	cmd.Flags().StringVar(&executable, "executable", "", "Path to the real client executable")
	cmd.Flags().StringSliceVar(&targets, "target", nil, "Client configuration path; repeat for multiple Codex homes")
	return cmd
}

func newAuthCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{Use: "auth codex", Short: "Bind the current account token to Codex", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] != configuration.ClientCodex {
			return fmt.Errorf("Native credential binding is available only for codex; run `aigw adapter auth codex`")
		}
		cfg, err := runtime.Config.Load()
		if err != nil {
			return err
		}
		if !cfg.Adapters[configuration.ClientCodex].Enabled {
			return fmt.Errorf("Codex adapter is not enabled; first run `aigw adapter enable codex ...`")
		}
		if err := invocation.Synchronizer(runtime).BindAuthentication(cmd.Context(), cfg); err != nil {
			return fmt.Errorf("Failed to bind Codex authentication: %w", err)
		}
		r := renderer(runtime)
		r.ProductTitle("Codex authentication bound")
		r.Success("The current account token was written to Codex native credential storage")
		r.Next("aigw doctor")
		return nil
	}}
}

func newDisableCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{Use: "disable <client>", Short: "Disable a client adapter and remove AIGW-owned projections", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		client := args[0]
		spec, ok := configuration.ClientSpecFor(client)
		if !ok {
			return fmt.Errorf("Client must be %s; run `aigw adapter disable --help`", configuration.AdmittedClientUsage())
		}
		cfg, err := runtime.Config.Load()
		if err != nil {
			return err
		}
		before := cfg.Clone()
		adapter, ok := cfg.Adapters[client]
		if !ok || !adapter.Enabled {
			r := renderer(runtime)
			r.ProductTitle("Client adapters")
			r.Status(presentation.Info, spec.Label, "Already disabled")
			return nil
		}
		delete(cfg.Adapters, client)
		if err := invocation.Synchronizer(runtime).Commit(cmd.Context(), before, cfg, "adapter disable"); err != nil {
			return err
		}
		r := renderer(runtime)
		r.ProductTitle("Client disabled")
		r.Row("Client", spec.Label)
		r.Success("All AIGW-owned projections were safely removed")
		return nil
	}}
}

// ValidateCodexTarget rejects executable paths and non-Codex-Home
// surfaces where a writable configuration target is required.
func ValidateCodexTarget(discovered discovery.Result, path string) error {
	if _, ok := discovered.SurfaceForExecutablePath(path); ok {
		return fmt.Errorf("an executable is not a Codex configuration target")
	}
	if surface, ok := discovered.SurfaceForConfigPath(path); ok && surface.ID != string(surfaceidentity.CodexHomeDefault) {
		return fmt.Errorf("surface %s is not an AIGW Codex target", surface.ID)
	}
	return nil
}

func discover(runtime invocation.Context) (discovery.Result, error) {
	if runtime.Discovery == nil {
		return discovery.Result{}, fmt.Errorf("client discovery is unavailable")
	}
	return runtime.Discovery.Discover(), nil
}

func renderer(runtime invocation.Context) *presentation.Renderer {
	out := runtime.RenderOut
	if out == nil {
		out = runtime.Out
	}
	return presentation.NewWithWidth(out, runtime.Color, runtime.Width)
}
