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
// anything is stored. Without a terminal it refuses, naming the command that
// stores the Token, and with the read-only environment backend the variable
// that holds it.
func acquireToken(cmd *cobra.Command, runtime invocation.Context, cfg domain.Config, chosen choice, selected domain.Runtime) (string, error) {
	id := selected.ProviderID
	if secret.IsReadOnly(runtime.Secrets) {
		fix := "Set " + secret.EnvironmentKey(id) + ", then run the same command again."
		if line := chosen.command(); line != "" {
			fix = "Set " + secret.EnvironmentKey(id) + ", then run `" + line + "`."
		}
		return "", terminal.ProblemError("provider "+id+" has no Token", "", "", fix, nil)
	}
	if !runtime.Interactive {
		return "", terminal.ProblemError("provider "+id+" has no Token, and aigw use asks for one only in a terminal", "", "", "aigw provider rotate "+id, nil)
	}
	token, err := runtime.Prompt.Secret("Paste the " + cfg.Providers[id].Label + " Token: ")
	if err != nil {
		return "", err
	}
	if err := probe.ValidateRuntime(cmd.Context(), runtime.HTTP, runtime.Clients.Admission(), selected, token); err != nil {
		endpointFix := func(protocol domain.EndpointProtocol) string {
			return "aigw provider edit " + id + " --endpoint " + protocol.Name() + "=<url>"
		}
		return "", invocation.TokenValidationProblem(cmd, []string{chosen.client.ID, chosen.ref.String()}, runtime.Secrets, id, endpointFix, err)
	}
	return token, nil
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
