// Package client owns explicit client discovery and projection lifecycle commands.
package client

import (
	"fmt"
	"strings"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/presentation"
	surfaceidentity "aigw-cli/internal/surface"

	"github.com/spf13/cobra"
)

// NewCommand constructs the client lifecycle command tree.
func NewCommand(runtime invocation.Context) *cobra.Command {
	root := &cobra.Command{Use: "client", Short: "Manage client projections"}
	root.AddCommand(newDiscoverCommand(runtime), newEnableCommand(runtime), newDisableCommand(runtime))
	return root
}

func newDiscoverCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{Use: "discover", Short: "Discover installed client executables", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		discovered, err := discover(runtime)
		if err != nil {
			return err
		}
		r := invocation.Renderer(runtime)
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
	cmd := &cobra.Command{Use: "enable <client>", Short: "Enable a client projection", ValidArgs: configuration.AdmittedClientIDs(), Args: cobra.MatchAll(cobra.ExactArgs(1), validateClientArgument, func(_ *cobra.Command, args []string) error {
		if strings.TrimSpace(executable) == "" {
			return fmt.Errorf("--executable is required; run `aigw client discover`")
		}
		if args[0] == configuration.ClientCodex && len(targets) == 0 {
			return fmt.Errorf("Codex projection requires at least one --target config.toml; run `aigw client enable --help`")
		}
		for _, target := range targets {
			if strings.TrimSpace(target) == "" {
				return fmt.Errorf("--target requires a non-empty path; run `aigw client enable --help`")
			}
		}
		return nil
	}), RunE: func(cmd *cobra.Command, args []string) error {
		client := args[0]
		spec, _ := configuration.ClientSpecFor(client)
		cfg, err := runtime.Config.Load()
		if err != nil {
			return err
		}
		before := cfg.Clone()
		current := before.Clients[client]
		if current.Enabled && (current.Executable != "" || len(current.Targets) > 0) {
			return fmt.Errorf("%s client projection is already enabled; disable it before changing the executable or config targets", spec.Label)
		}
		clientRuntime, err := cfg.ResolveRuntime(client, "")
		if err != nil {
			return err
		}
		if clientRuntime.UsesAIGWCredentialStore() {
			available, err := runtime.Secrets.Exists(clientRuntime.AccountID)
			if err != nil {
				return fmt.Errorf("Cannot inspect Account %q credential: %w", clientRuntime.AccountID, err)
			}
			if !available {
				instruction, _ := credential.TokenRecovery(runtime.Secrets, clientRuntime.AccountID)
				return fmt.Errorf("Account %q is missing a token; %s", clientRuntime.AccountID, instruction)
			}
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
		if len(targets) == 0 {
			targets = current.Targets
		}
		cfg.SetClientActivation(client, true, executable, targets)
		if err := invocation.Synchronizer(runtime).Commit(cmd.Context(), before, cfg, "client enable"); err != nil {
			return fmt.Errorf("Client enablement failed and was rolled back: %w", err)
		}
		r := invocation.Renderer(runtime)
		r.ProductTitle("Client enabled")
		r.Row("Client", spec.Label)
		r.Status(presentation.OK, "Projection", "Configured")
		r.Next("aigw check")
		return nil
	}}
	cmd.Flags().StringVar(&executable, "executable", "", "Path to the real client executable")
	cmd.Flags().StringSliceVar(&targets, "target", nil, "Client configuration path; repeat for multiple Codex homes")
	return cmd
}

func newDisableCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{Use: "disable <client>", Short: "Disable a client and remove AIGW-owned projections", ValidArgs: configuration.AdmittedClientIDs(), Args: cobra.MatchAll(cobra.ExactArgs(1), validateClientArgument), RunE: func(cmd *cobra.Command, args []string) error {
		client := args[0]
		spec, _ := configuration.ClientSpecFor(client)
		cfg, err := runtime.Config.Load()
		if err != nil {
			return err
		}
		before := cfg.Clone()
		binding, ok := cfg.Clients[client]
		if !ok || !binding.Enabled {
			r := invocation.Renderer(runtime)
			r.ProductTitle("Client projections")
			r.Status(presentation.Info, spec.Label, "Already disabled")
			return nil
		}
		if err := invocation.Synchronizer(runtime).Withdraw(&cfg, client); err != nil {
			return err
		}
		// Disabled intent is durable; discovery must not turn it back into activation.
		// Full uninstall uses Withdraw without retaining this client binding.
		binding.Enabled = false
		cfg.Clients[client] = binding

		if err := invocation.Synchronizer(runtime).CommitProjection(cmd.Context(), before, cfg, "client disable", client); err != nil {
			return err
		}
		r := invocation.Renderer(runtime)
		r.ProductTitle("Client disabled")
		r.Row("Client", spec.Label)
		r.Success("All AIGW-owned projections were safely removed")
		return nil
	}}
}

func validateClientArgument(command *cobra.Command, args []string) error {
	if err := cobra.OnlyValidArgs(command, args); err != nil {
		return fmt.Errorf("Client must be %s; run `%s --help`: %w", configuration.AdmittedClientUsage(), command.CommandPath(), err)
	}
	return nil
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
