package check

import (
	"fmt"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"
)

// named is the model that --model names for the one named client, which the
// check tries without selecting it, and the one-time Token from
// --token-stdin, if any.
type named struct {
	ref   domain.ModelRef
	token string
}

// name is the named model as provider/model, or "" when the check uses each
// client's own selection.
func (n named) name() string { return n.ref.String() }

// resolveNamed resolves --model for client. A name without a provider is
// refused with the names that it could mean, an unknown provider or model
// with the closest ones, and a model that the client cannot use with the
// model's own description as the fix.
func resolveNamed(cfg domain.Config, client, text string) (named, error) {
	ref, err := invocation.ParseModelArgument(cfg, text)
	if err != nil {
		return named{}, err
	}
	if _, err := invocation.ResolveModel(cfg, ref, ""); err != nil {
		return named{}, err
	}
	if _, err := cfg.ResolveRuntime(client, ref); err != nil {
		spec, _ := cfg.Admission().Spec(client)
		return named{}, terminal.ProblemError(fmt.Sprintf("%s cannot use %s: it declares no protocol that %s speaks and its provider serves", spec.Label, ref, spec.Label),
			"", "", invocation.Command("model", "show", ref.String()), err)
	}
	return named{ref: ref}, nil
}

// tokenOwner refuses a one-time Token that no request of the check would
// send: a client that authenticates itself, or whose Token an external helper
// supplies, sends none from AIGW, and only a session proves it.
func tokenOwner(cfg domain.Config, client string, model named) error {
	runtime, err := cfg.ResolveRuntime(client, model.ref)
	if err != nil || runtime.UsesAIGWCredentialStore() {
		return err
	}
	product := cfg.Admission().ProductName(client)
	reason := product + " uses its own authentication"
	if runtime.RequiresProviderToken() {
		reason = "an external credential helper gives " + product + " its Token"
	}
	return terminal.ProblemError("--token-stdin needs a client that sends a Token from AIGW, and "+reason, "", "Nothing was read.",
		checkCommand(client, model, depthClient), nil)
}
