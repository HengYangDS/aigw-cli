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
// one already, the read-only environment backend supplies one, or there is
// none. A preview reads none.
const (
	tokenStored      = "stored"
	tokenPresent     = "present"
	tokenEnvironment = "environment"
	tokenMissing     = "missing"
	tokenNotRead     = "not_read"
)

// teamSetup is one `aigw setup --from`: the configuration before it, the
// merge of the team file into it, and the installed clients.
type teamSetup struct {
	cmd       *cobra.Command
	runtime   invocation.Context
	request   setupRequest
	file      domain.Manifest
	before    domain.Config
	merged    domain.Config
	report    domain.TeamMerge
	installed map[string]bool
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
		return err
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
		installed: manifestSetupAvailableClients(cfg.Admission(), discovered.Executables)}
	if request.DryRun {
		return team.preview()
	}
	return team.apply(cmd.Context())
}

// readTeamFile reads and parses the team file at path. A file that cannot be
// read is named as every path failure is; one that does not parse names what
// to correct.
func readTeamFile(runtime invocation.Context, path string) (domain.Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.Manifest{}, terminal.ManifestReadFailure(path, platform.PathContext{Home: invocation.Home(runtime.Dependencies)}, err)
	}
	file, err := store.Parse(data, runtime.Config.Admission())
	if version, ok := errors.AsType[*domain.UnsupportedManifestVersionError](err); ok {
		return domain.Manifest{}, terminal.ProblemError(version.Error(), "", "Nothing changed.",
			"Export the team file again with the AIGW release that wrote it, then run the same command again.", err)
	}
	if errors.Is(err, store.ErrTeamFileClients) {
		return domain.Manifest{}, terminal.ProblemError(err.Error(), "", "Nothing changed.",
			"Remove the [clients] section from the team file, or write the file again with `aigw config export`, then run the same command again.", err)
	}
	return file, err
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

// preview reports what apply would do were every provider's Token stored,
// without reading, observing, validating or storing a Token: a preview
// never uses the credential store.
func (t teamSetup) preview() error {
	presumed := slices.Sorted(maps.Keys(t.merged.Providers))
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
	states := map[string]string{}
	for _, id := range t.tokenProviders() {
		states[id] = tokenNotRead
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
// credential store's metadata alone. An empty stored Token is no Token.
func (t teamSetup) tokenStates() (map[string]string, error) {
	states := map[string]string{}
	for _, id := range t.tokenProviders() {
		present, err := t.runtime.Secrets.Exists(id)
		switch {
		case err != nil:
			return nil, fmt.Errorf("observe the Token of provider %s: %w", id, err)
		case !present || secret.TokenSlotEmpty(t.runtime.Secrets, id):
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
// recommended setup would select. Without a terminal it reads none, and the
// read-only environment backend supplies every Token.
func (t teamSetup) acquire(states map[string]string) (map[string]string, error) {
	tokens := map[string]string{}
	asked := t.request.Providers
	if secret.IsReadOnly(t.runtime.Secrets) {
		for _, id := range asked {
			if states[id] == tokenMissing {
				return nil, readOnlyTokenInput(id, invocation.Rerun(t.cmd, nil, nil))
			}
		}
		return tokens, nil
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
		if cfg.SelectedModel(client) != "" {
			continue
		}
		runtime, err := selected.ResolveRuntime(client, "")
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
		if cfg.SelectedModel(client) != "" {
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
		runtime, err := cfg.ResolveRuntime(client, "")
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
