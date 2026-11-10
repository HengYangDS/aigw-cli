// Package setup implements `aigw setup`: guided setup of a provider, a
// client and a model in a terminal, and team setup, which applies a team
// file.
package setup

import (
	"errors"
	"fmt"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"

	"github.com/spf13/cobra"
)

// setupRequest contains the explicit setup inputs before discovery and validation resolve their effects.
type setupRequest struct {
	From string
	// Providers are the providers whose Tokens team setup reads, and Prune
	// removes the unused local-only models and families.
	Providers  []string
	Prune      bool
	TokenStdin bool
	JSON       bool
	// DryRun previews setup; apply is the command that applies the preview.
	DryRun bool
	apply  string
}

// NewCommand constructs the setup command and binds it to one invocation context.
func NewCommand(runtime invocation.Context) *cobra.Command {
	request := setupRequest{}
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Set up a provider, a client and a model, or apply a team file",
		Long: "Without --from, ask in a terminal for a provider, an installed client and a model from the\n" +
			"provider's catalog, save them, and check the client; it can run again. With --from, apply a\n" +
			"team file: its models, families and recommendations win, and an existing provider keeps its\n" +
			"endpoints and Token.",
		Example: "aigw setup\n" +
			"aigw setup --from team.toml --dry-run\n" +
			"aigw setup --from team.toml --provider dmxapi --token-stdin",
		Args: cobra.MatchAll(cobra.NoArgs, func(cmd *cobra.Command, _ []string) error {
			request.From = strings.TrimSpace(request.From)
			if cmd.Flags().Changed("from") {
				return admitTeamRequest(cmd, runtime, request)
			}
			if len(request.Providers) > 0 || request.Prune || request.TokenStdin {
				return fmt.Errorf("--provider, --prune and --token-stdin apply a team file, so they need --from; run `%s --help`", cmd.CommandPath())
			}
			return admitGuided(runtime, request)
		}),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if request.DryRun {
				request.apply = invocation.ApplyCommand(cmd, nil)
			}
			operation := runtime
			operation.Secrets = secret.ObserveAvailability(runtime.Secrets)
			// A JSON caller reads one document, so setup never prompts for it.
			operation.Interactive = operation.Interactive && !request.JSON
			if cmd.Flags().Changed("from") {
				return invocation.CancelledPrompt(cmd, nil, runTeamSetup(cmd, operation, request))
			}
			return invocation.CancelledPrompt(cmd, nil, runGuided(cmd, operation))
		},
	}
	cmd.Flags().StringVar(&request.From, "from", "", "Apply a team file: its providers, models, families and recommendations")
	cmd.Flags().StringArrayVar(&request.Providers, "provider", nil, "With --from, a provider whose Token to read, at a prompt or from --token-stdin; repeatable")
	cmd.Flags().BoolVar(&request.Prune, "prune", false, "With --from, remove the local models that no client selects and no recommendation names, and unused families")
	cmd.Flags().BoolVar(&request.TokenStdin, "token-stdin", false, "With --from, read the one --provider's Token from standard input")
	cmd.Flags().BoolVar(&request.JSON, "json", false, "Write the setup result as JSON")
	cmd.Flags().BoolVar(&request.DryRun, "dry-run", false, "Show what setup would change without reading a Token or writing anything")
	invocation.CompleteFlag(cmd, "provider", cobra.NoFileCompletions)
	return cmd
}

// admitGuided refuses guided setup where it cannot ask: it is the one write
// without a preview, it prompts, which --json never does, and it needs a
// terminal. A team file needs none of them, so each refusal names it.
func admitGuided(runtime invocation.Context, request setupRequest) error {
	switch {
	case request.DryRun:
		return errors.New("guided setup is the one write without a preview; preview a team file with `aigw setup --from <file> --dry-run`")
	case request.JSON:
		return errors.New("guided setup asks questions, and --json never prompts; apply a team file with `aigw setup --from <file> --json`")
	case !runtime.Interactive:
		return errors.New("guided setup asks questions in a terminal; without one, apply a team file with `aigw setup --from <file>`")
	}
	return nil
}

// admitTeamRequest refuses, before the team file or standard input is read, a
// team setup whose Token input cannot run: a blank path, an invalid provider
// ID, standard input without exactly one provider or with the read-only
// environment backend, and a named provider whose Token would need a prompt
// where none may appear.
func admitTeamRequest(cmd *cobra.Command, runtime invocation.Context, request setupRequest) error {
	path := cmd.CommandPath()
	if request.From == "" {
		return fmt.Errorf("--from requires a team file path; run `%s --help`", path)
	}
	for _, id := range request.Providers {
		if !domain.ValidIdentifier(id) || id != strings.ToLower(id) {
			return fmt.Errorf("--provider %q is not a provider ID; use lowercase letters, numbers, dots, hyphens or underscores; run `%s --help`", id, path)
		}
	}
	readOnly := secret.IsReadOnly(runtime.Secrets)
	switch {
	case request.TokenStdin && len(request.Providers) != 1:
		return fmt.Errorf("--token-stdin reads one Token, so it needs exactly one --provider; run `%s --help`", path)
	case request.TokenStdin && readOnly:
		return readOnlyTokenInput(request.Providers[0], invocation.Rerun(cmd, nil, []string{"token-stdin"}))
	case len(request.Providers) > 0 && !request.TokenStdin && !request.DryRun && !readOnly && (!runtime.Interactive || request.JSON):
		reason := "a named provider's Token is read from --token-stdin without a terminal or with --json"
		if len(request.Providers) == 1 {
			if line := invocation.Rerun(cmd, nil, nil, "--token-stdin"); line != "" {
				return fmt.Errorf("%s; pipe the Token to `%s`", reason, line)
			}
		}
		return fmt.Errorf("%s, one provider at a time; run `%s --help`", reason, path)
	}
	return nil
}
