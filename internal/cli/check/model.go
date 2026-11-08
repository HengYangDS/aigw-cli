package check

import (
	"fmt"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"
)

// named is the model that --model names for the one named client, which the
// check tries without selecting it: its name, the key under which the
// configuration stores it, and the one-time Token from --token-stdin, if any.
type named struct {
	ref   domain.ModelRef
	key   string
	token string
}

// name is the named model as provider/model, or "" when the check uses each
// client's own selection.
func (n named) name() string {
	if n.key == "" {
		return ""
	}
	return n.ref.String()
}

// resolveNamed resolves --model for client. A name without a provider is
// refused with the names that it could mean, an unknown provider or model
// with the closest ones, and a model that the client cannot use with the
// model's own description as the fix. A name that several stored models
// carry, which only a configuration from before models were unique per
// provider holds, takes the one on the client's protocol.
func resolveNamed(cfg domain.Config, client, text string) (named, error) {
	ref, err := invocation.ParseModelArgument(cfg, text)
	if err != nil {
		return named{}, err
	}
	if _, err := invocation.ResolveProvider(cfg, ref.Provider()); err != nil {
		return named{}, err
	}
	keys := cfg.ResolveModelRef(ref)
	if len(keys) == 0 {
		_, _, err := invocation.ResolveModel(cfg, ref, "", nil)
		return named{}, err
	}
	var usable []string
	var cause error
	for _, key := range keys {
		if _, err := cfg.ResolveRuntime(client, key); err != nil {
			cause = err
			continue
		}
		usable = append(usable, key)
	}
	spec, _ := cfg.Admission().Spec(client)
	switch len(usable) {
	case 0:
		return named{}, terminal.ProblemError(fmt.Sprintf("%s cannot use %s: it declares no protocol that %s speaks and its provider serves", spec.Label, ref, spec.Label),
			"", "", invocation.Command("model", "show", ref.String()), cause)
	case 1:
		return named{ref: ref, key: usable[0]}, nil
	}
	for _, key := range usable {
		if _, declared := cfg.Models[key].Protocols[cfg.Clients[client].Protocol]; declared {
			return named{ref: ref, key: key}, nil
		}
	}
	_, _, err = invocation.ResolveModel(cfg, ref, "", nil)
	return named{}, err
}

// tokenOwner refuses a one-time Token that no request of the check would
// send: a client that authenticates itself, or whose Token an external helper
// supplies, sends none from AIGW, and only a session proves it.
func tokenOwner(cfg domain.Config, client string, model named) error {
	runtime, err := cfg.ResolveRuntime(client, model.key)
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
