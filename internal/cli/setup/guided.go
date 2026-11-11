package setup

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/app"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/probe"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// The choices that guided setup offers beside the configured providers and a
// catalog's model IDs. Neither is a provider ID, which starts with a letter
// or digit, or a model ID that a catalog lists.
const (
	addProvider  = "+provider"
	typeModelID  = "+model"
	newProviderQ = "Provider ID (for example, dmxapi): "
)

// guidedSetup is `aigw setup` in a terminal: a provider, an installed client
// and a model from the provider's catalog, saved in one transaction, then a
// check of that client. It runs on an empty or a configured home, and leaves
// every provider, Token and selection that the person does not choose as it
// is.
type guidedSetup struct {
	cmd     *cobra.Command
	runtime invocation.Context
	before  domain.Config
}

// guidedProvider is the provider that guided setup uses: one that the
// configuration has, or one that it adds; and the Token that it read for
// the provider, which the transaction stores, or none when the store holds
// one.
type guidedProvider struct {
	domain.Provider
	token string
	added bool
}

// runGuided runs guided setup.
func runGuided(cmd *cobra.Command, runtime invocation.Context) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	g := guidedSetup{cmd: cmd, runtime: runtime, before: cfg}
	provider, err := g.provider(cmd.Context())
	if err != nil {
		return err
	}
	discovered, err := runtime.Discover()
	if err != nil {
		return err
	}
	clients := g.installedClients(provider.Provider, discovered)
	if len(clients) == 0 {
		return g.stopWithoutClient(cmd.Context(), provider)
	}
	spec, err := g.client(clients)
	if err != nil {
		return err
	}
	ref, protocols, err := g.model(cmd.Context(), provider, spec)
	if err != nil {
		return err
	}
	protocol, err := g.protocol(ref, protocols)
	if err != nil {
		return err
	}
	working := g.withProvider(provider)
	withModel(&working, ref, protocol)
	synchronizer := runtime.Synchronizer()
	after, selected, err := synchronizer.PrepareSelection(working, spec.ID, ref, protocol)
	if err != nil {
		return err
	}
	if _, _, _, err := synchronizer.SelectModel(cmd.Context(), cfg, after, spec.ID, provider.token); err != nil {
		return err
	}
	if err := g.renderSaved(provider, spec, selected); err != nil {
		return err
	}
	return g.check(spec.ID)
}

// provider asks for an existing provider or a new one, and for the Token of
// either when the store holds none.
func (g guidedSetup) provider(ctx context.Context) (guidedProvider, error) {
	ids := g.before.ProviderIDs()
	if len(ids) > 0 {
		choices := make([]terminal.Choice, 0, len(ids)+1)
		for _, id := range ids {
			choices = append(choices, terminal.Choice{Value: id, Label: g.before.Providers[id].Label + " (" + id + ")"})
		}
		choices = append(choices, terminal.Choice{Value: addProvider, Label: "Add a new provider"})
		picked, err := g.runtime.Prompt.Select("Provider: ", choices)
		if err != nil {
			return guidedProvider{}, invocation.Prompted("The provider prompt", err)
		}
		if picked != addProvider {
			provider := guidedProvider{Provider: g.before.Providers[picked]}
			provider.ID = picked
			err := g.storedOrAsked(ctx, &provider)
			return provider, err
		}
	}
	return g.newProvider(ctx)
}

// newProvider asks for a new provider's ID, name, one endpoint and its Token.
func (g guidedSetup) newProvider(ctx context.Context) (guidedProvider, error) {
	id, err := g.runtime.Prompt.Text(newProviderQ)
	if err != nil {
		return guidedProvider{}, invocation.Prompted("The provider ID prompt", err)
	}
	if !domain.ValidIdentifier(id) || id != strings.ToLower(id) {
		return guidedProvider{}, fmt.Errorf("%q is not a provider ID; use lowercase letters, numbers, dots, hyphens or underscores, then run `aigw setup` again", id)
	}
	if _, exists := g.before.Providers[id]; exists {
		return guidedProvider{}, fmt.Errorf("provider %s already exists; run `aigw setup` again and choose it from the list", id)
	}
	label, err := g.runtime.Prompt.Text("Provider name: ")
	if err != nil {
		return guidedProvider{}, invocation.Prompted("The provider name prompt", err)
	}
	choices := make([]terminal.Choice, 0, len(knownProtocols))
	for _, protocol := range knownProtocols {
		choices = append(choices, terminal.Choice{Value: string(protocol), Label: domain.ProtocolLabel(protocol)})
	}
	picked, err := g.runtime.Prompt.Select("Endpoint protocol: ", choices)
	if err != nil {
		return guidedProvider{}, invocation.Prompted("The protocol prompt", err)
	}
	protocol := domain.EndpointProtocol(picked)
	url, err := g.runtime.Prompt.Text(domain.ProtocolLabel(protocol) + " URL: ")
	if err != nil {
		return guidedProvider{}, invocation.Prompted("The endpoint prompt", err)
	}
	url = strings.TrimRight(strings.TrimSpace(url), "/")
	if err := domain.ValidateEndpoint(url); err != nil {
		return guidedProvider{}, fmt.Errorf("the %s URL %w; run `aigw setup` again with a URL that the provider gives", domain.ProtocolLabel(protocol), err)
	}
	provider := guidedProvider{ID: id, Label: label, added: true}
	setEndpoint(&provider.Endpoints, protocol, url)
	err = g.storedOrAsked(ctx, &provider)
	return provider, err
}

// knownProtocols are the protocols in the order in which setup offers them.
var knownProtocols = []domain.EndpointProtocol{domain.ProtocolAnthropic, domain.ProtocolResponses, domain.ProtocolChat}

// setEndpoint gives endpoints the URL of protocol.
func setEndpoint(endpoints *domain.Endpoints, protocol domain.EndpointProtocol, url string) {
	switch protocol {
	case domain.ProtocolAnthropic:
		endpoints.Anthropic = url
	case domain.ProtocolResponses:
		endpoints.Responses = url
	case domain.ProtocolChat:
		endpoints.Chat = url
	}
}

// storedOrAsked keeps the provider's stored Token, or asks for one and
// validates it as `aigw provider add` does. A new provider always asks: a
// Token stored for an ID that the configuration does not name does not make
// a provider. The read-only environment backend supplies the Token itself.
func (g guidedSetup) storedOrAsked(ctx context.Context, provider *guidedProvider) error {
	present, err := g.runtime.Secrets.Exists(provider.ID)
	if err != nil {
		return fmt.Errorf("observe the Token of provider %s: %w", provider.ID, err)
	}
	stored := present && !secret.TokenSlotEmpty(g.runtime.Secrets, provider.ID)
	if secret.IsReadOnly(g.runtime.Secrets) {
		if !stored {
			return readOnlyTokenInput(provider.ID, "aigw setup")
		}
		return nil
	}
	if stored && !provider.added {
		return nil
	}
	token, err := g.runtime.Prompt.Secret("Paste the " + provider.Label + " Token: ")
	if err != nil {
		return invocation.Prompted("The Token prompt for provider "+provider.ID, err)
	}
	if err := invocation.ValidateToken(token); err != nil {
		return err
	}
	if err := probe.Validate(ctx, g.runtime.HTTP, g.runtime.Clients.Admission(), provider.Provider, token); err != nil {
		return invocation.WithImpact(invocation.TokenValidationProblem(g.cmd, nil, g.runtime.Secrets, provider.ID, func(protocol domain.EndpointProtocol) string {
			return "Run `aigw setup` again with the " + domain.ProtocolLabel(protocol) + " URL that the provider gives."
		}, err), "Nothing changed.")
	}
	provider.token = token
	return ctx.Err()
}

// installedClients are the installed clients that speak a protocol for which
// the provider has an endpoint, in admission order.
func (g guidedSetup) installedClients(provider domain.Provider, discovered discovery.Result) []domain.ClientSpec {
	var installed []domain.ClientSpec
	for _, spec := range g.runtime.Clients.Admission().Specs() {
		if discovered.Executables[spec.ID] != "" && len(spec.CompatibleProtocols(provider)) > 0 {
			installed = append(installed, spec)
		}
	}
	return installed
}

// stopWithoutClient saves a new provider and its Token, or a Token that it
// read for an existing one, and stops before a model: no installed client
// can use the provider.
func (g guidedSetup) stopWithoutClient(ctx context.Context, provider guidedProvider) error {
	if provider.token != "" {
		if _, err := secret.Replace(g.runtime.Secrets, map[string]string{provider.ID: provider.token}); err != nil {
			return err
		}
	}
	if provider.added {
		if err := g.runtime.Synchronizer().Commit(ctx, g.before, g.withProvider(provider), "setup"); err != nil {
			return err
		}
	}
	r := invocation.Renderer(g.runtime)
	r.ProductTitle("Setup stopped before a model")
	r.Rows(g.providerFields(provider)...)
	r.Section("Clients")
	r.Rows(terminal.Field{State: terminal.Info, Label: "Installed clients", Value: "None that speaks a protocol for which provider " + provider.ID + " has an endpoint"})
	r.Success("Install a client, then run `aigw setup` again to choose it and a model")
	r.Next("aigw client list")
	return r.Err()
}

// client asks for one of the installed clients, and takes the only one
// without asking.
func (g guidedSetup) client(installed []domain.ClientSpec) (domain.ClientSpec, error) {
	if len(installed) == 1 {
		return installed[0], nil
	}
	choices := make([]terminal.Choice, 0, len(installed))
	for _, spec := range installed {
		choices = append(choices, terminal.Choice{Value: spec.ID, Label: spec.Label})
	}
	picked, err := g.runtime.Prompt.Select("Client: ", choices)
	if err != nil {
		return domain.ClientSpec{}, invocation.Prompted("The client prompt", err)
	}
	spec, _ := g.runtime.Clients.Admission().Spec(picked)
	return spec, nil
}

// model asks for a model that the provider's catalog lists on a protocol
// that the client speaks, or for a model ID when the catalog cannot be read
// or lists none, and returns the protocols on which the client could use it.
func (g guidedSetup) model(ctx context.Context, provider guidedProvider, spec domain.ClientSpec) (domain.ModelRef, []domain.EndpointProtocol, error) {
	usable := spec.CompatibleProtocols(provider.Provider)
	reader := g.runtime.Secrets
	if provider.token != "" {
		reader = pendingToken{Store: reader, provider: provider.ID, token: provider.token}
	}
	scoped := domain.NewConfig(g.before.Admission())
	scoped.Providers[provider.ID] = provider.Provider
	listed := map[string][]domain.EndpointProtocol{}
	for _, observation := range app.DiscoverCatalog(ctx, reader, g.runtime.HTTP, scoped).Observations {
		if observation.Status != app.CatalogOK || !slices.Contains(usable, observation.Source.Protocol) {
			continue
		}
		for _, model := range observation.Models {
			if !slices.Contains(listed[model.ID], observation.Source.Protocol) {
				listed[model.ID] = append(listed[model.ID], observation.Source.Protocol)
			}
		}
	}
	id := typeModelID
	if len(listed) > 0 {
		ids := slices.Sorted(maps.Keys(listed))
		choices := make([]terminal.Choice, 0, len(ids)+1)
		for _, listedID := range ids {
			choices = append(choices, terminal.Choice{Value: listedID, Label: listedID})
		}
		choices = append(choices, terminal.Choice{Value: typeModelID, Label: "Type a model ID that the catalog does not list"})
		picked, err := g.runtime.Prompt.Select("Model for "+spec.Label+": ", choices)
		if err != nil {
			return domain.ModelRef{}, nil, invocation.Prompted("The model prompt", err)
		}
		id = picked
	}
	if id != typeModelID {
		return domain.NewModelRef(provider.ID, id), listed[id], nil
	}
	typed, err := g.runtime.Prompt.Text("Model ID, as provider " + provider.ID + " names it: ")
	if err != nil {
		return domain.ModelRef{}, nil, invocation.Prompted("The model ID prompt", err)
	}
	return domain.NewModelRef(provider.ID, strings.TrimSpace(typed)), usable, nil
}

// protocol asks for the protocol when the client could use the model on
// more than one.
func (g guidedSetup) protocol(ref domain.ModelRef, protocols []domain.EndpointProtocol) (domain.EndpointProtocol, error) {
	if len(protocols) == 1 {
		return protocols[0], nil
	}
	choices := make([]terminal.Choice, 0, len(protocols))
	for _, protocol := range protocols {
		choices = append(choices, terminal.Choice{Value: string(protocol), Label: domain.ProtocolLabel(protocol)})
	}
	picked, err := g.runtime.Prompt.Select("Protocol for "+ref.String()+": ", choices)
	if err != nil {
		return "", invocation.Prompted("The protocol prompt", err)
	}
	return domain.EndpointProtocol(picked), nil
}

// withProvider is the configuration with a provider that setup adds.
func (g guidedSetup) withProvider(provider guidedProvider) domain.Config {
	working := g.before.Clone()
	if provider.added {
		stored := provider.Provider
		stored.ID = ""
		working.Providers[provider.ID] = stored
	}
	return working
}

// withModel adds the model that ref names on protocol to cfg, or the protocol
// to the model when it lacks it, declaring no capability. A new model takes
// the family that its ID names, ignoring case, or is its own family, as
// `aigw use` adds one.
func withModel(cfg *domain.Config, ref domain.ModelRef, protocol domain.EndpointProtocol) {
	model, added := cfg.Models[ref]
	if !added {
		model = domain.Model{Provider: ref.Provider(), ID: ref.ID(), Family: cfg.AddedModelFamily(ref.ID())}
	}
	if _, declared := model.Protocols[protocol]; declared {
		return
	}
	protocols := maps.Clone(model.Protocols)
	if protocols == nil {
		protocols = map[domain.EndpointProtocol][]domain.Capability{}
	}
	protocols[protocol] = []domain.Capability{}
	model.Protocols = protocols
	cfg.Models[ref] = model
}

// renderSaved reports what setup saved, before the check runs.
func (g guidedSetup) renderSaved(provider guidedProvider, spec domain.ClientSpec, selected domain.Runtime) error {
	r := invocation.Renderer(g.runtime)
	r.ProductTitle("Setup saved")
	r.Rows(append(g.providerFields(provider),
		terminal.Field{Label: spec.Label, Value: g.modelRef(selected).String() + " on " + domain.ProtocolLabel(selected.Protocol)})...)
	r.Success("Saved; the check below tells whether " + spec.Label + " works")
	return r.Err()
}

// modelRef names the selected model.
func (g guidedSetup) modelRef(selected domain.Runtime) domain.ModelRef {
	return domain.NewModelRef(selected.ProviderID, selected.UpstreamModel)
}

// providerFields describe the provider and what setup did with its Token.
func (g guidedSetup) providerFields(provider guidedProvider) []terminal.Field {
	token := terminal.Field{State: terminal.OK, Label: "Token", Value: "Stored before"}
	switch {
	case provider.token != "":
		token.Value = "Accepted by the provider and stored"
	case secret.IsReadOnly(g.runtime.Secrets):
		token.Value = "Supplied by " + secret.EnvironmentKey(provider.ID)
	}
	added := "Kept as it was"
	if provider.added {
		added = "Added"
	}
	return []terminal.Field{{Label: "Provider", Value: provider.Label + " (" + provider.ID + "), " + strings.ToLower(added[:1]) + added[1:]}, token}
}

// check checks the client as `aigw check <client>` does, through the command
// tree's check, whose output follows setup's. A tree without one names it.
func (g guidedSetup) check(client string) error {
	check, rest, err := g.cmd.Root().Find([]string{"check", client})
	if err != nil || check == g.cmd.Root() || check.RunE == nil {
		r := invocation.Renderer(g.runtime)
		r.Next("aigw check " + client)
		return r.Err()
	}
	check.SetContext(g.cmd.Context())
	return check.RunE(check, rest)
}

// pendingToken is the credential store with the Token of one provider that
// setup read and has not stored yet, which only the catalog read uses.
type pendingToken struct {
	secret.Store
	provider, token string
}

// Exists reports the pending Token, or the store's.
func (s pendingToken) Exists(provider string) (bool, error) {
	if provider == s.provider {
		return true, nil
	}
	return s.Store.Exists(provider)
}

// Get returns the pending Token, or the store's.
func (s pendingToken) Get(provider string) (string, error) {
	if provider == s.provider {
		return s.token, nil
	}
	return s.Store.Get(provider)
}
