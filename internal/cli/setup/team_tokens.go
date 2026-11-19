package setup

import (
	"context"
	"errors"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/probe"
	"aigw-cli/internal/terminal"
)

// acquire reads and validates the Tokens that setup asks for, and records in
// failed each provider whose Token it read and did not store: a named
// provider's from standard input or at a prompt, or, in a terminal without a
// named provider, those of the providers that the person says they hold a
// Token for. A named provider whose Token is stored already uses it without a
// prompt, unless standard input replaces it; one without a Token needs a
// terminal or standard input. Without a terminal it reads none, and the
// read-only environment backend supplies every Token.
func (t teamSetup) acquire(ctx context.Context, states map[string]string) (map[string]string, error) {
	tokens := map[string]string{}
	asked := t.request.Providers
	if secret.IsReadOnly(t.runtime.Secrets) {
		return tokens, t.admitEnvironmentTokens(states)
	}
	if len(asked) == 0 {
		if !t.runtime.Interactive {
			return tokens, nil
		}
		var err error
		if asked, err = t.chooseProviders(states); err != nil {
			return nil, err
		}
	}
	for _, id := range asked {
		if states[id] == tokenPresent && !t.request.TokenStdin {
			continue
		}
		if !t.request.TokenStdin && !t.runtime.Interactive {
			return nil, t.promptRefusal()
		}
		outcome, err := t.readToken(ctx, id)
		switch {
		case err != nil:
			return nil, err
		case outcome.token != "":
			tokens[id] = outcome.token
		case outcome.failure != nil:
			t.failed[id] = outcome.failure
		}
	}
	return tokens, nil
}

// chooseProviders asks which of the team file's providers without a Token the
// person holds a Token for, in the file's order, and accepts none: an answer
// that names no choice in accessible mode chooses none too. A provider whose
// store cannot tell, or whose Keychain item macOS refuses, keeps the fix that
// its state names and is not offered.
func (t teamSetup) chooseProviders(states map[string]string) ([]string, error) {
	var choices []terminal.Choice
	for _, id := range t.merged.ProviderIDs() {
		if _, named := t.file.Providers[id]; !named {
			continue
		}
		state, observed := states[id]
		if !observed {
			observation, err := secret.ObserveToken(t.runtime.Secrets, id)
			if err != nil || observation.Refusal != nil || observation.Usable() {
				continue
			}
			state = tokenMissing
		}
		if state == tokenMissing {
			choices = append(choices, terminal.Choice{Value: id, Label: t.providerLabel(id)})
		}
	}
	if len(choices) == 0 {
		return nil, nil
	}
	chosen, err := t.runtime.Prompt.MultiSelect(providerQuestion, choices)
	if errors.Is(err, terminal.ErrNoChoice) {
		return nil, nil
	}
	if err != nil {
		return nil, invocation.Prompted("The provider question", err)
	}
	for _, id := range chosen {
		states[id] = tokenMissing
	}
	return chosen, nil
}

// tokenOutcome is what became of one provider's Token that setup asked for:
// the Token that the provider accepted, or the validation failure of one that
// it did not store, or neither when the person skipped the provider.
type tokenOutcome struct {
	token   string
	failure error
}

// readToken reads provider id's Token and validates it with the provider. A
// piped Token is validated once; standard input that holds no Token is an
// input error, which refuses the command before any write. In a terminal,
// Enter alone skips the provider, and a Token that is not one or that the
// validation does not accept offers to enter it again, to skip the provider,
// or, when the validation got no verdict that a later request may give, to try
// the same Token again.
func (t teamSetup) readToken(ctx context.Context, id string) (tokenOutcome, error) {
	if t.request.TokenStdin {
		token, err := invocation.ReadToken(t.runtime, true, false)
		if err != nil {
			return tokenOutcome{}, err
		}
		if failure := t.validate(ctx, id, token); failure != nil {
			return tokenOutcome{failure: failure}, ctx.Err()
		}
		return tokenOutcome{token: token}, ctx.Err()
	}
	label := t.providerLabel(id)
	token := ""
	for {
		if token == "" {
			entered, err := t.runtime.Prompt.OptionalSecret("Paste the " + label + " Token, or press Enter to skip " + label + ": ")
			if err != nil {
				return tokenOutcome{}, invocation.Prompted("The Token prompt for provider "+id, err)
			}
			if entered == "" {
				return tokenOutcome{}, nil
			}
			token = entered
		}
		malformed := invocation.ValidateToken(token)
		failure := malformed
		if failure == nil {
			failure = t.validate(ctx, id, token)
		}
		if err := ctx.Err(); err != nil {
			return tokenOutcome{}, err
		}
		if failure == nil {
			return tokenOutcome{token: token}, nil
		}
		question := failure.Error() + ". What now?"
		if malformed != nil {
			question = "That is not a Token: " + malformed.Error() + ". What now?"
		}
		next, err := t.runtime.Prompt.Select(question, failureChoices(label, failure))
		if err != nil {
			return tokenOutcome{}, invocation.Prompted("The choice for provider "+id, err)
		}
		switch next {
		case "skip":
			// Text that is not a Token tells nothing about the provider, which
			// stays without one, as Enter alone leaves it.
			if malformed != nil {
				return tokenOutcome{}, nil
			}
			return tokenOutcome{failure: failure}, nil
		case "again":
			token = ""
		}
	}
}

// failureChoices are the answers to a Token that was not accepted: enter it
// again or skip the provider, and first, when its validation got no verdict
// that a later request may give, try the same Token again.
func failureChoices(label string, failure error) []terminal.Choice {
	choices := []terminal.Choice{{Value: "again", Label: "Enter the " + label + " Token again"}, {Value: "skip", Label: "Skip " + label}}
	if transientFailure(failure) {
		choices = append([]terminal.Choice{{Value: "retry", Label: "Try this Token again"}}, choices...)
	}
	return choices
}

// transientFailure reports a validation that a later request may answer
// otherwise: one that got no response, or a status that a later request may
// not repeat.
func transientFailure(failure error) bool {
	if _, unreachable := errors.AsType[*probe.UnreachableError](failure); unreachable {
		return true
	}
	status, answered := errors.AsType[*probe.EndpointStatusError](failure)
	return answered && invocation.TransientStatus(status.Status)
}

// providerLabel is the name by which setup asks for a provider's Token.
func (t teamSetup) providerLabel(id string) string {
	if label := t.merged.Providers[id].Label; label != "" {
		return label
	}
	return id
}

// failureState is the state of a Token that this run read and did not store:
// rejected when the provider refused it, unverified when the validation got no
// verdict.
func failureState(failure error) string {
	if _, refused := errors.AsType[*probe.TokenRejectedError](failure); refused {
		return tokenRejected
	}
	return tokenUnverified
}
