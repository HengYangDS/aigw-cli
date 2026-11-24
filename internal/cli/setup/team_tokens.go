package setup

import (
	"context"
	"errors"
	"strings"

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
			if len(tokens) > 0 {
				err = invocation.PromptLost(err, "No Token that this run accepted was stored.")
			}
			return nil, err
		case outcome.token != "":
			tokens[id] = outcome.token
		case outcome.failure != nil:
			t.failed[id] = outcome.failure
		}
	}
	return tokens, nil
}

// chooseProviders asks which of the askable providers the person holds a
// Token for, and accepts none: an answer that names no choice in accessible
// mode chooses none too, which setup says.
func (t teamSetup) chooseProviders(states map[string]string) ([]string, error) {
	choices := t.askable(states)
	if len(choices) == 0 {
		return nil, nil
	}
	chosen, err := t.runtime.Prompt.MultiSelect(providerQuestion, choices)
	if errors.Is(err, terminal.ErrNoChoice) {
		return nil, t.say("No answer named a provider, so setup asks for no Token.")
	}
	if err != nil {
		return nil, invocation.Prompted("The provider question", err)
	}
	for _, id := range chosen {
		states[id] = tokenMissing
	}
	return chosen, nil
}

// say tells the person, between prompts, what setup made of answers that
// named no choice.
func (t teamSetup) say(line string) error {
	r := invocation.Renderer(t.runtime)
	r.Text(line)
	return r.Err()
}

// askable are the team file's providers without a Token, in the file's order,
// which a run in a terminal asks about. A provider whose store cannot tell, or
// whose Keychain item macOS refuses, keeps the fix that its state names and is
// not offered.
func (t teamSetup) askable(states map[string]string) []terminal.Choice {
	var choices []terminal.Choice
	for _, id := range t.merged.ProviderIDs() {
		if _, named := t.file.Providers[id]; !named {
			continue
		}
		state, observed := states[id]
		if !observed {
			observation := secret.ObserveToken(t.runtime.Secrets, id)
			if observation.State() != secret.TokenAbsent || observation.Refusal != nil {
				continue
			}
			state = tokenMissing
		}
		if state == tokenMissing {
			choices = append(choices, terminal.Choice{Value: id, Label: t.providerLabel(id)})
		}
	}
	return choices
}

// previewAsks says in a terminal preview what the run asks a person for,
// which a preview never does: which of the askable providers' Tokens they
// hold, of which the preview assumes none, or the Token of each named
// provider without one, which the preview takes as accepted. Without a
// terminal, with standard input or with the read-only environment backend the
// run asks nothing, and so does a run whose providers all have a Token.
func (t teamSetup) previewAsks(states map[string]string) string {
	if !t.runtime.Interactive || t.request.TokenStdin || secret.IsReadOnly(t.runtime.Secrets) {
		return ""
	}
	var labels []string
	if len(t.request.Providers) == 0 {
		for _, choice := range t.askable(states) {
			labels = append(labels, choice.Label)
		}
		if len(labels) == 0 {
			return ""
		}
		return "The run asks which providers' Tokens you hold (" + strings.Join(labels, ", ") + "); this preview assumes none."
	}
	for _, id := range t.request.Providers {
		if states[id] == tokenMissing {
			labels = append(labels, t.providerLabel(id))
		}
	}
	switch len(labels) {
	case 0:
		return ""
	case 1:
		return "The run asks for the " + labels[0] + " Token; this preview takes it as accepted."
	}
	return "The run asks for the Tokens of " + strings.Join(labels, ", ") + "; this preview takes them as accepted."
}

// tokenOutcome is what became of one provider's Token that setup asked for:
// the Token that the provider accepted, or the validation failure of the last
// one that it did not store, or neither when the person skipped the provider
// before any validation.
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
// the same Token again. The provider keeps the last validation's verdict
// however the person then leaves its prompt: by skipping, by Enter alone, or
// by answers that name no choice, which skip it and which setup says.
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
	var verdict error
	for {
		if token == "" {
			entered, err := t.runtime.Prompt.OptionalSecret("Paste the " + label + " Token, or press Enter to skip " + label + ": ")
			if err != nil {
				return tokenOutcome{}, invocation.Prompted("The Token prompt for provider "+id, err)
			}
			if entered == "" {
				return tokenOutcome{failure: verdict}, nil
			}
			token = entered
		}
		// Text that is not a Token tells nothing about the provider, so it
		// leaves the verdict that an earlier Token got.
		malformed := invocation.ValidateToken(token)
		failure := malformed
		if failure == nil {
			failure = t.validate(ctx, id, token)
			verdict = failure
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
		if errors.Is(err, terminal.ErrNoChoice) {
			next, err = "skip", t.say("No answer named a choice, so "+label+" was skipped.")
		}
		if err != nil {
			return tokenOutcome{}, invocation.Prompted("The choice for provider "+id, err)
		}
		switch next {
		case "skip":
			return tokenOutcome{failure: verdict}, nil
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
