package setup

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"

	clientactivation "aigw-cli/internal/app/activation"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/probe"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// teamSchemaVersion is the version of `aigw setup --from`'s JSON document,
// which replaced version 2, shared with flag-driven setup, and `aigw config
// import`'s.
const teamSchemaVersion = 3

// The states of a provider's Token after team setup, which it observes
// without reading a Token: this run validated and stored it, the store held
// one already, the read-only environment backend supplies one, there is none,
// or the credential store cannot tell.
const (
	tokenStored      = "stored"
	tokenPresent     = "present"
	tokenEnvironment = "environment"
	tokenMissing     = "missing"
	tokenUnobserved  = "unobserved"
)

// storedTokenUsed says that setup used a Token that the store held already,
// which it never prints.
const storedTokenUsed = "Already stored; used without a prompt"

// teamSetup is one `aigw setup --from`: the configuration before it, the
// merge of the team file into it, and the installed clients.
type teamSetup struct {
	cmd       *cobra.Command
	runtime   invocation.Context
	request   setupRequest
	file      domain.Config
	before    domain.Config
	merged    domain.Config
	report    domain.TeamMerge
	installed map[string]bool
	// refused are the refusals of the Keychain items of the providers that
	// wait for their Token because macOS refuses AIGW the item, by provider.
	refused map[string]*secret.KeychainItemRefusedError
}

// runTeamSetup applies a team file in one transaction: it merges the file
// into the configuration, reads the Tokens that it asks for and validates
// each with the provider, selects models for the clients without a selection
// from the recommendations of the providers with a Token, and writes the
// files of the installed clients that it can configure. A preview does the
// same without reading a Token or writing anything.
func runTeamSetup(cmd *cobra.Command, runtime invocation.Context, request setupRequest) error {
	file, err := readTeamFile(runtime, request.From)
	if err != nil {
		return err
	}
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	merged, report, err := domain.MergeTeamFile(cfg, file, request.Prune)
	if err != nil {
		if shared, ok := errors.AsType[*domain.SharedModelLabelError](err); ok {
			return sharedLabelRefusal(shared, file, err)
		}
		if _, named := domain.EntryOf(err); named {
			return teamFileRefusal(err)
		}
		return terminal.ProblemError(err.Error(), "", "Nothing changed.", "Correct the team file, then run the same command again.", err)
	}
	for _, id := range request.Providers {
		if _, err := invocation.ResolveProvider(merged, id); err != nil {
			return err
		}
	}
	discovered, err := runtime.Discover()
	if err != nil {
		return err
	}
	team := teamSetup{cmd: cmd, runtime: runtime, request: request, file: file, before: cfg, merged: merged, report: report,
		installed: manifestSetupAvailableClients(cfg.Admission(), discovered.Executables), refused: map[string]*secret.KeychainItemRefusedError{}}
	if request.DryRun {
		return team.afterDeletion(team.preview())
	}
	return team.afterDeletion(team.apply(cmd.Context()))
}

// afterDeletion completes the fix of a Keychain item that macOS refuses, when
// err is the refusal of one. A refusal that ends a team setup is of a provider
// that the setup names, whose Token it reads and stores; the item of any other
// provider leaves that provider's client waiting. For a named provider that
// the configuration does not hold yet, which `aigw provider rotate` does not
// know, the setup stores the Token again once the item is deleted.
func (t teamSetup) afterDeletion(err error) error {
	refusal, refused := errors.AsType[*secret.KeychainItemRefusedError](err)
	if !refused {
		return err
	}
	return secret.AfterDeletionRun(err, t.storingLine(refusal.Provider), t.before.Providers)
}

// storingLine is the setup that stores provider's Token once its refused
// item is deleted: this command's line, without --dry-run, because a preview
// stores nothing; naming that provider alone where this command does not name
// it, because a setup reads the Token of a provider that it names; and
// reading the Token of that provider alone from standard input where this
// command neither has that input nor can prompt, which would end a rerun as
// promptRefusal ends it. Where the line then differs from the one given and
// its team file would need quoting, it names the file to fill in, because the
// same command again would not be that setup.
func (t teamSetup) storingLine(provider string) string {
	replaced, added := []string{"dry-run"}, []string(nil)
	piped := !t.request.TokenStdin && !t.runtime.Interactive
	if piped || !slices.Contains(t.request.Providers, provider) {
		replaced, added = append(replaced, "provider"), []string{"--provider", provider}
	}
	if piped {
		added = append(added, "--token-stdin")
	}
	line := invocation.Rerun(t.cmd, nil, replaced, added...)
	if line == "" && (t.request.DryRun || added != nil) {
		line = invocation.Rerun(t.cmd, nil, append(replaced, "from"), append([]string{"--from", "<file>"}, added...)...)
	}
	return line
}

// readTeamFile reads and parses the team file at path. A file that cannot be
// read is named as every path failure is; one that does not parse names what
// to correct.
func readTeamFile(runtime invocation.Context, path string) (domain.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.Config{}, terminal.ManifestReadFailure(path, platform.PathContext{Home: invocation.Home(runtime.Dependencies)}, err)
	}
	file, err := store.ParseTeamFile(data, runtime.Config.Admission())
	if err != nil {
		return domain.Config{}, teamFileRefusal(err)
	}
	return file, nil
}

// teamFileRefusal presents the refusal of a team file that cannot be applied:
// a version or a [clients] section by its own fix, a failure at one entry by
// the entry to correct, and a document that does not parse as it is, with the
// location it names.
func teamFileRefusal(err error) error {
	if version, ok := errors.AsType[*domain.UnsupportedTeamFileVersionError](err); ok {
		return terminal.ProblemError(version.Error(), "", "Nothing changed.",
			fmt.Sprintf("Use a team file of version %d, which `aigw config export` writes, then run the same command again.", domain.ConfigVersion), err)
	}
	if errors.Is(err, store.ErrTeamFileClients) {
		return terminal.ProblemError(err.Error(), "", "Nothing changed.",
			"Remove the [clients] section from the team file, or write the file again with `aigw config export`, then run the same command again.", err)
	}
	if shared, ok := errors.AsType[*domain.SharedModelLabelError](err); ok {
		return terminal.ProblemError(err.Error(), "", "Nothing changed.",
			fmt.Sprintf("Give %s or %s a label of its own in the team file, then run the same command again.", shared.Models[0], shared.Models[1]), err)
	}
	if entry, ok := domain.EntryOf(err); ok {
		return terminal.ProblemError(err.Error(), "", "Nothing changed.", "Correct "+entry+" in the team file, then run the same command again.", err)
	}
	return err
}

// sharedLabelRefusal refuses a team file whose model would share a label
// with another model of its provider, and names where to correct it: a model
// that only the local configuration holds, as one that a client keeps after
// the file replaced it, is relabelled or removed by its own command, which
// loads the configuration as it is.
func sharedLabelRefusal(shared *domain.SharedModelLabelError, file domain.Config, err error) error {
	for _, ref := range shared.Models {
		if _, inFile := file.Models[ref]; !inFile {
			return terminal.ProblemError(err.Error(), "", "Nothing changed.",
				fmt.Sprintf("Give %s a label of its own with `aigw model edit %s --label <label>`, or remove it with `aigw model remove %s`, then run the same command again.", ref, ref, ref), err)
		}
	}
	return teamFileRefusal(err)
}

// apply connects the Tokens and commits the setup.
func (t teamSetup) apply(ctx context.Context) error {
	states, err := t.tokenStates()
	if err != nil {
		return err
	}
	tokens, err := t.acquire(states)
	if err != nil {
		return err
	}
	for _, id := range slices.Sorted(maps.Keys(tokens)) {
		if err := t.validate(ctx, id, tokens[id]); err != nil {
			return err
		}
		states[id] = tokenStored
	}
	connected := connectedProviders(states)
	selected, err := t.merged.SelectModelsForConnectedProviders(connected)
	if err != nil {
		return err
	}
	clients := manifestSetupSelectedClients(selected, connected, t.installed)
	after, err := t.runtime.Synchronizer().Setup(ctx, t.before, selected, tokens, clients...)
	if err != nil {
		return err
	}
	document := t.document(after, states, connected)
	activation := clientactivation.AssessActivation(after, t.runtime.Secrets)
	document.NextAction = activation.NextActionFor(nil)
	if len(t.request.Providers) == 1 && slices.Contains(connected, t.request.Providers[0]) {
		document.NextAction = clientactivation.NextActionAfterProviderConnection(after, t.request.Providers[0])
	}
	return t.write(document)
}

// preview reports what apply would do without reading, validating or
// storing a Token: it observes, from the credential store's metadata alone,
// which Tokens are present, and plans the providers that have one, and the
// named ones, whose Tokens apply reads; every other provider waits for its
// Token with its fix, as apply leaves it.
func (t teamSetup) preview() error {
	states, err := t.tokenStates()
	if err != nil {
		return err
	}
	if err := t.admitEnvironmentTokens(states); err != nil {
		return err
	}
	presumed := connectedProviders(states)
	for _, id := range t.request.Providers {
		if !slices.Contains(presumed, id) {
			presumed = append(presumed, id)
		}
	}
	planned, err := t.merged.SelectModelsForConnectedProviders(presumed)
	if err != nil {
		return err
	}
	clients := manifestSetupSelectedClients(planned, presumed, t.installed)
	planned, projection, err := t.runtime.Synchronizer().PreviewSetup(t.before, planned, presumed, clients...)
	if err != nil {
		return err
	}
	preview, err := invocation.WithEntrypoint(t.runtime, planned, projection)
	if err != nil {
		return err
	}
	document := t.document(planned, states, presumed)
	document.DryRun = true
	document.ClientFiles = invocation.ClientFiles(t.runtime, preview)
	document.CredentialReader = preview.CredentialEntrypoint
	document.NextAction = t.request.apply
	return t.write(document)
}

// tokenProviders are the providers whose Tokens setup observes and reports:
// the named ones, the ones whose models a recommendation offers a client
// without a selection, and the ones that a selection reads a Token of. A
// provider that no client needs a Token of stays unobserved.
func (t teamSetup) tokenProviders() []string {
	providers := slices.Concat(t.request.Providers, recommendedChoiceProviders(t.merged), t.merged.RequiredProviderTokenIDs())
	slices.Sort(providers)
	return slices.Compact(providers)
}

// tokenStates observes whether each reported provider has a Token, from the
// credential store's metadata alone. An empty stored Token is no Token, and
// a store that cannot tell leaves the Token unobserved, which is not absent:
// such a provider waits, unless the command names it, whose Token it would
// store, which is refused before any input is read. A Keychain item that
// macOS refuses AIGW is such an observation: the provider that the command
// names ends the setup with the item's refusal, and any other one waits with
// the item's title and fix, which refused keeps for the report. The fix ends
// with the credential's own command when the configuration holds the provider
// at the time the fix is printed.
func (t teamSetup) tokenStates() (map[string]string, error) {
	states := map[string]string{}
	for _, id := range t.tokenProviders() {
		observation, err := secret.ObserveToken(t.runtime.Secrets, id)
		if observation.Refusal != nil {
			// For the provider that the command names, the refusal ends the
			// setup as a store that cannot tell does.
			err = observation.Refusal
		}
		switch {
		case err != nil && slices.Contains(t.request.Providers, id):
			return nil, fmt.Errorf("observe the Token of provider %s: %w", id, err)
		case observation.Refusal != nil:
			if t.request.DryRun {
				// A preview saves no provider, so the credential's own command
				// would not know one that only the team file holds: the fix
				// of such a provider's item ends with the setup that stores
				// its Token. An apply saves the provider before it prints.
				_ = secret.AfterDeletionRun(observation.Refusal, t.storingLine(id), t.before.Providers)
			}
			states[id], t.refused[id] = tokenUnobserved, observation.Refusal
		case err != nil:
			states[id] = tokenUnobserved
		case !observation.Usable():
			states[id] = tokenMissing
		case secret.IsReadOnly(t.runtime.Secrets):
			states[id] = tokenEnvironment
		default:
			states[id] = tokenPresent
		}
	}
	return states, nil
}

// acquire reads the Tokens that setup asks for: the named provider's from
// standard input, each named provider's at a prompt, or, in a terminal
// without a named provider, at a prompt each missing Token that the
// recommended setup would select. A named provider whose Token is stored
// already uses it without a prompt, unless standard input replaces it; one
// without a Token needs a terminal or standard input. Without a terminal it
// reads none, and the read-only environment backend supplies every Token.
func (t teamSetup) acquire(states map[string]string) (map[string]string, error) {
	tokens := map[string]string{}
	asked := t.request.Providers
	if secret.IsReadOnly(t.runtime.Secrets) {
		return tokens, t.admitEnvironmentTokens(states)
	}
	if len(asked) == 0 {
		if !t.runtime.Interactive {
			return tokens, nil
		}
		for _, id := range recommendedProviders(t.merged) {
			if states[id] == tokenMissing {
				asked = append(asked, id)
			}
		}
	}
	for _, id := range asked {
		if states[id] == tokenPresent && !t.request.TokenStdin {
			continue
		}
		if !t.request.TokenStdin && !t.runtime.Interactive {
			return nil, t.promptRefusal()
		}
		var token string
		var err error
		if t.request.TokenStdin {
			token, err = invocation.ReadToken(t.runtime, true, false)
		} else if token, err = t.runtime.Prompt.Secret("Paste the " + t.merged.Providers[id].Label + " Token: "); err != nil {
			err = invocation.Prompted("The Token prompt for provider "+id, err)
		} else {
			err = invocation.ValidateToken(token)
		}
		if err != nil {
			return nil, err
		}
		tokens[id] = token
	}
	return tokens, nil
}

// promptRefusal refuses a named provider without a stored Token where no
// prompt may appear, without a terminal or with --json, naming the standard
// input that gives it.
func (t teamSetup) promptRefusal() error {
	reason := "a named provider's Token is read from --token-stdin without a terminal or with --json"
	if len(t.request.Providers) == 1 {
		if line := invocation.Rerun(t.cmd, nil, nil, "--token-stdin"); line != "" {
			return fmt.Errorf("%s; pipe the Token to `%s`", reason, line)
		}
	}
	return fmt.Errorf("%s, one provider at a time; run `%s --help`", reason, t.cmd.CommandPath())
}

// admitEnvironmentTokens refuses a named provider whose variable the
// read-only environment backend does not set, which supplies every Token.
func (t teamSetup) admitEnvironmentTokens(states map[string]string) error {
	if !secret.IsReadOnly(t.runtime.Secrets) {
		return nil
	}
	for _, id := range t.request.Providers {
		if states[id] == tokenMissing {
			return readOnlyTokenInput(id, invocation.Rerun(t.cmd, nil, nil))
		}
	}
	return nil
}

// validate sends the one authenticated request that names no model, as
// `aigw provider add` does, and refuses the whole setup when the provider
// does not accept the Token.
func (t teamSetup) validate(ctx context.Context, id, token string) error {
	provider := t.merged.Providers[id]
	provider.ID = id
	err := probe.Validate(ctx, t.runtime.HTTP, t.runtime.Clients.Admission(), provider, token)
	if err == nil {
		return ctx.Err()
	}
	_, existing := t.before.Providers[id]
	endpointFix := func(protocol domain.EndpointProtocol) string {
		if existing {
			return "aigw provider edit " + id + " --endpoint " + protocol.Name() + "=<url>"
		}
		return "Correct the " + protocol.Name() + " endpoint of provider " + id + " in the team file, then run the same command again."
	}
	return invocation.WithImpact(invocation.TokenValidationProblem(t.cmd, nil, t.runtime.Secrets, id, endpointFix, err),
		"The team file was not applied.")
}

// connectedProviders are the providers with a Token.
func connectedProviders(states map[string]string) []string {
	var connected []string
	for _, id := range slices.Sorted(maps.Keys(states)) {
		if state := states[id]; state == tokenStored || state == tokenPresent || state == tokenEnvironment {
			connected = append(connected, id)
		}
	}
	return connected
}

// recommendedProviders are the providers whose models the recommendations
// would select for the clients without a selection, were every Token
// stored, in the order of the clients.
func recommendedProviders(cfg domain.Config) []string {
	selected, err := cfg.SelectModelsForConnectedProviders(slices.Sorted(maps.Keys(cfg.Providers)))
	if err != nil {
		return nil
	}
	var providers []string
	for _, client := range cfg.Admission().IDs() {
		if !cfg.SelectedModel(client).IsZero() {
			continue
		}
		runtime, err := selected.ResolveRuntime(client, domain.ModelRef{})
		if err != nil || !runtime.RequiresProviderToken() || slices.Contains(providers, runtime.ProviderID) {
			continue
		}
		providers = append(providers, runtime.ProviderID)
	}
	return providers
}

// recommendedChoiceProviders are the providers of every recommended choice
// for the clients without a selection, each of which a Token would make
// selectable: equal alternatives, as activation names them.
func recommendedChoiceProviders(cfg domain.Config) []string {
	var providers []string
	for _, client := range cfg.Admission().IDs() {
		if !cfg.SelectedModel(client).IsZero() {
			continue
		}
		for _, choice := range cfg.Recommendations[client].Selections() {
			if choice.Authentication == domain.AuthenticationClientNative {
				continue
			}
			if _, err := cfg.ResolveSelection(client, choice); err == nil && !slices.Contains(providers, cfg.Models[choice.Model].Provider) {
				providers = append(providers, cfg.Models[choice.Model].Provider)
			}
		}
	}
	return providers
}

// readOnlyTokenInput refuses Token input that the read-only environment
// backend cannot take, or a named provider whose variable is not set: the
// variable supplies the Token, so the fix sets it and runs then.
func readOnlyTokenInput(id, then string) error {
	key := secret.EnvironmentKey(id)
	fix := "Set " + key + " in aigw's environment, then run the same command again."
	if then != "" {
		fix = "Set " + key + " in aigw's environment, then run `" + then + "`."
	}
	return terminal.ProblemError("the environment backend reads the "+id+" Token from "+key+", and setup cannot store one", "", "Nothing changed.", fix, nil)
}

func manifestSetupAvailableClients(admission domain.Admission, executables map[string]string) map[string]bool {
	available := make(map[string]bool, len(admission.IDs()))
	for _, clientID := range admission.IDs() {
		available[clientID] = executables[clientID] != ""
	}
	return available
}

// manifestSetupSelectedClients are the installed, enabled clients whose
// selection's provider has a Token, or that authenticate themselves.
func manifestSetupSelectedClients(cfg domain.Config, connected []string, available map[string]bool) []string {
	clients := make([]string, 0, len(cfg.Admission().IDs()))
	for _, client := range cfg.Admission().IDs() {
		if !cfg.Clients[client].Enabled || !available[client] {
			continue
		}
		runtime, err := cfg.ResolveRuntime(client, domain.ModelRef{})
		if err != nil || runtime.ProviderID == "" {
			continue
		}
		if runtime.UsesAIGWCredentialStore() && !slices.Contains(connected, runtime.ProviderID) {
			continue
		}
		clients = append(clients, client)
	}
	return clients
}
