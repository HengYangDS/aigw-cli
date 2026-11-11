// Package credential exposes the narrow token-helper surface consumed by
// admitted native clients. It never prints diagnostics to standard output.
package credential

import (
	"context"
	"errors"
	"fmt"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/native"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// NewCommand builds the private credential-helper command used by admitted clients.
func NewCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{
		Use:    "credential <client> <fingerprint>",
		Short:  "Read the provider Token that a client file's fingerprint names",
		Hidden: true,
		Args:   cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			client := args[0]
			spec, admitted := runtime.Clients.Admission().Spec(client)
			if !admitted {
				return fmt.Errorf("credential helper requires an admitted client: %s", runtime.Clients.Admission().Usage())
			}
			// The reader decodes only the view that this release freezes, so
			// that a later release's configuration keeps serving a running
			// session.
			view, err := runtime.Config.LoadCredentialView()
			if err != nil {
				return configurationProblem(runtime.Version, err)
			}
			providerID, err := view.TokenProvider(spec, args[1])
			if err != nil {
				return refusal(client, err)
			}
			token, err := runtime.Secrets.Get(providerID)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return terminal.ProblemError("Account Token read exceeded its deadline", "", "The credential subprocess was stopped; no Token was returned.", "Check the selected credential service before retrying; configuration synchronization cannot repair a stalled read.", err)
				}
				if errors.Is(err, secret.ErrNotFound) {
					return terminal.ProblemError(
						fmt.Sprintf("%s Account Token is not configured", client), "",
						"No Token was returned.",
						"Set the selected Account Token in the configured backend; run `aigw check --depth local` for backend guidance.",
						err,
					)
				}
				evidence := ""
				action := "Restore read access to the selected backend (unlock the native store if locked), then retry; AIGW will not switch backends."
				if failure, ok := errors.AsType[*native.KeychainError](err); ok {
					evidence = fmt.Sprintf("Keychain OSStatus %d; the native read did not permit user interaction.", failure.Status)
					action = "Inspect the selected native store, item authorization and credential-reader identity; repeated helper calls cannot grant access."
				}
				return terminal.ProblemError(
					fmt.Sprintf("%s Account Token could not be read", client), evidence,
					"No Token was returned.",
					action,
					err,
				)
			}
			_, err = fmt.Fprintln(runtime.Out, token)
			if err != nil {
				return terminal.ProblemError("Cannot write the Account Token to the client", "", "Credential delivery did not complete.", "Check the client's credential-helper connection before retrying.", err)
			}
			return err
		},
	}
}

// configurationProblem names this reader's version and the configuration stage
// that failed. Client logs receive it, so it omits the path and content. Its
// fix is the installed AIGW's sync, which installs a reader for a newer
// configuration and names the stage of one that it cannot load either; a
// read failure needs the client's read access instead.
func configurationProblem(version string, err error) error {
	reader := "This AIGW credential reader"
	if version != "" {
		reader = "AIGW " + version + " credential reader"
	}
	evidence := reader + " could not load the configuration."
	fix := "Run `aigw sync` with the installed AIGW and reload the client to use its current reader."
	if stage, ok := terminal.ConfigurationLoadEvidence(reader, err); ok {
		evidence = stage
	}
	if failure, ok := errors.AsType[*store.LoadError](err); ok && failure.Phase == store.LoadPhaseRead {
		fix = "Restore the client's read access to the configuration file; `aigw config path` prints its location."
	}
	return terminal.ProblemError("Cannot read AIGW configuration", evidence, "Credential lookup did not start.", fix, err)
}

// refusal explains why the view names no provider for the client, before any
// Token is read.
func refusal(client string, err error) error {
	switch {
	case errors.Is(err, domain.ErrCredentialClientDisabled):
		return terminal.ProblemError(fmt.Sprintf("%s adapter is not enabled", client), "", "Credential lookup did not start.", fmt.Sprintf("Enable the %s adapter before using its credential helper.", client), nil)
	case errors.Is(err, domain.ErrCredentialClientOwned):
		return terminal.ProblemError(fmt.Sprintf("%s uses client-owned authentication", client), "", "AIGW does not supply an Account Token for this route.", fmt.Sprintf("Run `aigw check %s --depth client` to verify authentication through the client.", client), nil)
	}
	if unknown, ok := errors.AsType[*domain.UnknownAuthenticationError](err); ok {
		return terminal.ProblemError(fmt.Sprintf("%s uses the authentication %q, which this credential reader does not know", client, unknown.Authentication), "", "No Token was read or returned.", "Run `aigw sync` with the installed AIGW and reload the client to use its current reader.", err)
	}
	return terminal.ProblemError(fmt.Sprintf("%s credential projection no longer matches an available Account and endpoint", client), "", "No Account Token was read or returned.", "Run `aigw sync` and reload the client's configuration.", err)
}
