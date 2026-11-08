package use

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/probe"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// tokenMissing reports whether the selection reads its provider's Token from
// AIGW's credential store and the store holds none. It reads the Token's
// metadata, never its value.
func tokenMissing(runtime invocation.Context, selected domain.Runtime) (bool, error) {
	if !selected.UsesAIGWCredentialStore() {
		return false, nil
	}
	available, err := runtime.Secrets.Exists(selected.ProviderID)
	if err != nil {
		return false, terminal.ProblemError("the Token of provider "+selected.ProviderID+" cannot be observed", "", "", "aigw provider show "+selected.ProviderID, err)
	}
	return !available, nil
}

// acquireToken asks for the provider's missing Token in a terminal and
// validates it with one request to the selected protocol's endpoint, before
// anything is stored. With the read-only environment backend it refuses,
// naming the variable that holds the Token. Without a terminal, and with
// --json, which never prompts, it refuses with the step that stores the
// Token from standard input. Each refusal then names the same selection.
func acquireToken(cmd *cobra.Command, runtime invocation.Context, cfg domain.Config, chosen choice, selected domain.Runtime, jsonMode bool) (string, error) {
	id := selected.ProviderID
	if secret.IsReadOnly(runtime.Secrets) {
		return "", terminal.ProblemError("provider "+id+" has no Token", "", "", setTokenVariable(id, chosen.command(), "the same command again"), nil)
	}
	if !runtime.Interactive {
		why := "aigw use asks for one only in a terminal"
		if jsonMode {
			why = "aigw use --json never prompts for one"
		}
		return "", terminal.ProblemError("provider "+id+" has no Token, and "+why, "", "", pipeToken(id, chosen.command()), nil)
	}
	token, err := runtime.Prompt.Secret("Paste the " + cfg.Providers[id].Label + " Token: ")
	if err != nil {
		return "", invocation.Prompted("The Token prompt for provider "+id, err)
	}
	if err := probe.ValidateRuntime(cmd.Context(), runtime.HTTP, runtime.Clients.Admission(), selected, token); err != nil {
		endpointFix := func(protocol domain.EndpointProtocol) string {
			return "aigw provider edit " + id + " --endpoint " + protocol.Name() + "=<url>"
		}
		return "", invocation.TokenValidationProblem(cmd, []string{chosen.client.ID, chosen.ref.String()}, runtime.Secrets, id, endpointFix, err)
	}
	return token, nil
}

// setTokenVariable is the step that supplies provider's missing Token from
// the read-only environment backend: setting its variable, and then the
// selection's command line, or else, as prose, rerun.
func setTokenVariable(provider, line, rerun string) string {
	step := "Set " + secret.EnvironmentKey(provider) + ", then run "
	if line == "" {
		return step + rerun + "."
	}
	return step + "`" + line + "`."
}

// pipeToken is the step that stores provider's missing Token without a
// prompt: piping it into provider rotate, which validates it first, and then
// the selection's command line, or else the same command again.
func pipeToken(provider, line string) string {
	step := "Pipe the Token into `aigw provider rotate " + provider + " --token-stdin`, then run "
	if line == "" {
		return step + "the same command again."
	}
	return step + "`" + line + "`."
}

// tokenNotKept adds to a failed commit of a selection that obtained a Token
// that the Token was not kept: the commit stores it with the selection and
// removes it again, and the credential store, which nothing else wrote,
// confirms that it holds no Token for the provider. A removal that failed is
// the failure's own report, and a store that cannot be read leaves the
// failure as it is.
func tokenNotKept(runtime invocation.Context, provider, token string, err error) error {
	if token == "" {
		return err
	}
	if kept, observeErr := runtime.Secrets.Exists(provider); observeErr != nil || kept {
		return err
	}
	return invocation.WithImpact(err, "The Token that you entered was not kept.")
}

// obtainedToken is the credential store with the Token of one provider that
// the selection obtained and has not stored yet, which only the catalog check
// reads; every other provider's Token comes from the store.
type obtainedToken struct {
	secret.Store
	provider, token string
}

// Exists reports the obtained Token, or the store's.
func (s obtainedToken) Exists(provider string) (bool, error) {
	if provider == s.provider {
		return true, nil
	}
	return s.Store.Exists(provider)
}

// Get returns the obtained Token, or the store's.
func (s obtainedToken) Get(provider string) (string, error) {
	if provider == s.provider {
		return s.token, nil
	}
	return s.Store.Get(provider)
}
