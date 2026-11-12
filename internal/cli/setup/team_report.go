package setup

import (
	"maps"
	"slices"
	"strings"

	"aigw-cli/internal/app"
	clientactivation "aigw-cli/internal/app/activation"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
)

// The states of a client after team setup: it selects no model, it waits for
// its provider's Token or for the client itself, or its files match its
// selection.
const (
	clientNotSelected    = "not_selected"
	clientWaitsForToken  = "waits_for_token"
	clientWaitsForClient = "waits_for_client"
	clientConfigured     = "configured"
)

// teamDocument is `aigw setup --from`'s JSON document: what the team file
// added, changed and pruned, what stayed local, the capabilities that it
// dropped and the protocols that it left unusable, the Token of each provider
// that a selection or recommendation needs, each client's state, and in a
// preview the client files that it would write.
type teamDocument struct {
	DryRun              bool                          `json:"dry_run,omitempty"`
	Added               teamEntries                   `json:"added"`
	Changed             teamEntries                   `json:"changed"`
	ChangedLabels       []changedLabel                `json:"changed_labels,omitempty"`
	Pruned              *teamEntries                  `json:"pruned,omitempty"`
	KeptEndpoints       []keptEndpoint                `json:"kept_endpoints,omitempty"`
	KeptBalanceProbes   []keptProbe                   `json:"kept_balance_probes,omitempty"`
	KeptModels          []keptModel                   `json:"kept_models,omitempty"`
	DroppedCapabilities []droppedCapabilities         `json:"dropped_capabilities,omitempty"`
	Unusable            []unusableProtocol            `json:"unusable,omitempty"`
	Tokens              []tokenItem                   `json:"tokens"`
	Clients             []clientItem                  `json:"clients"`
	ClientFiles         []invocation.ClientFile       `json:"client_files,omitempty"`
	CredentialReader    *app.CredentialEntrypointPlan `json:"credential_reader,omitempty"`
	NextAction          string                        `json:"next_action,omitempty"`
	NextDetail          string                        `json:"next_detail,omitempty"`
}

// teamEntries are providers, models, families and recommendations, each
// sorted; models are named provider/model and recommendations by client.
type teamEntries struct {
	Providers       []string `json:"providers"`
	Models          []string `json:"models"`
	Families        []string `json:"families"`
	Recommendations []string `json:"recommendations,omitempty"`
}

// changedLabel is a model whose label the team file replaces. Local and File
// are absent for a model without a label, which shows its ID.
type changedLabel struct {
	Model string `json:"model"`
	Local string `json:"local,omitempty"`
	File  string `json:"file,omitempty"`
}

// keptEndpoint is an existing provider's endpoint that stays local; Local is
// absent when the provider has none for the protocol.
type keptEndpoint struct {
	Provider string `json:"provider"`
	Protocol string `json:"protocol"`
	Local    string `json:"local,omitempty"`
	File     string `json:"file"`
}

// keptProbe is an existing provider's balance probe that stays local.
type keptProbe struct {
	Provider string       `json:"provider"`
	Local    balanceProbe `json:"local"`
	File     balanceProbe `json:"file"`
}

// balanceProbe is a balance probe's kind and base URL.
type balanceProbe struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

// keptModel is a model that keeps its local definition, because the file's
// would take the protocol of a client's selection or name another model.
type keptModel struct {
	Model    string `json:"model"`
	Client   string `json:"client"`
	Protocol string `json:"protocol"`
}

// droppedCapabilities are the file's capabilities of one model on one
// protocol whose endpoint stays local.
type droppedCapabilities struct {
	Model        string   `json:"model"`
	Protocol     string   `json:"protocol"`
	Capabilities []string `json:"capabilities"`
}

// unusableProtocol is a model's protocol for which its provider has no
// endpoint, and the step that gives it one.
type unusableProtocol struct {
	Model      string `json:"model"`
	Protocol   string `json:"protocol"`
	NextDetail string `json:"next_detail"`
}

// tokenItem is one provider's Token state, and the step that stores a missing
// one that a recommendation needs.
type tokenItem struct {
	Provider   string `json:"provider"`
	State      string `json:"state"`
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
}

// clientItem is one client's state: the model that it selects, if any, and
// the step that it waits for.
type clientItem struct {
	ID         string `json:"id"`
	State      string `json:"state"`
	Model      string `json:"model,omitempty"`
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
}

// document reports the merge, the Tokens and the clients of after.
func (t teamSetup) document(after domain.Config, states map[string]string, connected []string) teamDocument {
	report := t.report
	document := teamDocument{
		Added:   teamEntries{Providers: nonNil(report.AddedProviders), Models: refs(report.AddedModels), Families: nonNil(report.AddedFamilies), Recommendations: report.AddedRecommendations},
		Changed: teamEntries{Providers: nonNil(report.ChangedProviders), Models: refs(report.ChangedModels), Families: nonNil(report.ChangedFamilies), Recommendations: report.ChangedRecommendations},
		Tokens:  []tokenItem{},
		Clients: []clientItem{},
	}
	for _, label := range report.ChangedLabels {
		document.ChangedLabels = append(document.ChangedLabels, changedLabel{Model: label.Model.String(), Local: label.Local, File: label.File})
	}
	if t.request.Prune {
		document.Pruned = &teamEntries{Providers: []string{}, Models: refs(report.PrunedModels), Families: nonNil(report.PrunedFamilies)}
	}
	for _, kept := range report.KeptEndpoints {
		document.KeptEndpoints = append(document.KeptEndpoints, keptEndpoint{Provider: kept.Provider, Protocol: kept.Protocol.Name(), Local: kept.Local, File: kept.File})
	}
	for _, kept := range report.KeptProbes {
		document.KeptBalanceProbes = append(document.KeptBalanceProbes, keptProbe{Provider: kept.Provider,
			Local: balanceProbe{Kind: kept.Local.Kind, URL: kept.Local.BaseURL}, File: balanceProbe{Kind: kept.File.Kind, URL: kept.File.BaseURL}})
	}
	for _, kept := range report.KeptModels {
		document.KeptModels = append(document.KeptModels, keptModel{Model: kept.Model.String(), Client: kept.Client, Protocol: kept.Protocol.Name()})
	}
	for _, dropped := range report.DroppedCapabilities {
		capabilities := make([]string, 0, len(dropped.Capabilities))
		for _, capability := range dropped.Capabilities {
			capabilities = append(capabilities, string(capability))
		}
		document.DroppedCapabilities = append(document.DroppedCapabilities, droppedCapabilities{Model: dropped.Model.String(), Protocol: dropped.Protocol.Name(), Capabilities: capabilities})
	}
	for _, unusable := range report.Unusable {
		document.Unusable = append(document.Unusable, unusableProtocol{Model: unusable.Model.String(), Protocol: unusable.Protocol.Name(),
			NextDetail: "aigw provider edit " + unusable.Model.Provider() + " --endpoint " + unusable.Protocol.Name() + "=<url>"})
	}
	for _, id := range slices.Sorted(maps.Keys(states)) {
		item := tokenItem{Provider: id, State: states[id]}
		// A named provider's Token is the command's own input.
		if !slices.Contains(t.request.Providers, id) {
			item.NextAction, item.NextDetail = terminal.NextSteps(tokenStep(t.runtime.Secrets, id, item.State))
		}
		document.Tokens = append(document.Tokens, item)
	}
	for _, spec := range after.Admission().Specs() {
		document.Clients = append(document.Clients, t.client(after, spec, connected, states))
	}
	return document
}

// client reports one client's state in after: the Token comes before the
// client's installation, because a client installed later reads it.
func (t teamSetup) client(after domain.Config, spec domain.ClientSpec, connected []string, states map[string]string) clientItem {
	item := clientItem{ID: spec.ID, State: clientNotSelected}
	selected, err := after.ResolveRuntime(spec.ID, domain.ModelRef{})
	if err != nil {
		return item
	}
	item.Model, item.State = selected.Model.String(), clientConfigured
	switch next := clientactivation.ProjectionPrerequisites(after)[spec.ID]; {
	case selected.RequiresProviderToken() && !slices.Contains(connected, selected.ProviderID):
		item.State = clientWaitsForToken
		item.NextAction, item.NextDetail = terminal.NextSteps(tokenStep(t.runtime.Secrets, selected.ProviderID, states[selected.ProviderID]))
	case next != "":
		item.State = clientWaitsForClient
		item.NextAction, item.NextDetail = terminal.NextSteps(next)
	}
	return item
}

// tokenStep is the step that a provider's Token in state waits for: none
// once one is stored, the local check, which inspects a credential store that
// cannot tell, or else the step that stores one.
func tokenStep(store secret.Store, id, state string) string {
	switch state {
	case tokenStored, tokenPresent, tokenEnvironment:
		return ""
	case tokenUnobserved:
		return domainreadiness.CredentialBackendRecovery
	}
	return tokenFix(store, id)
}

// tokenFix is the step that stores provider's Token: piping it into provider
// rotate, which validates it, or setting the environment backend's variable.
func tokenFix(store secret.Store, id string) string {
	if secret.IsReadOnly(store) {
		return "Set " + secret.EnvironmentKey(id) + " to the provider's Token, then run `aigw sync`."
	}
	return "aigw provider rotate " + id
}

// write writes the document, or the same report for a person.
func (t teamSetup) write(document teamDocument) error {
	if t.request.JSON {
		document.NextAction, document.NextDetail = terminal.NextSteps(document.NextAction)
		return terminal.WriteDocument(t.runtime.Out, teamSchemaVersion, true, document)
	}
	r := invocation.Renderer(t.runtime)
	if document.DryRun {
		r.ProductTitle("Team file preview")
	} else {
		r.ProductTitle("Team file applied")
	}
	changed := renderEntries(r, "Added", document.Added)
	changed = renderEntries(r, "Changed", document.Changed) || changed
	renderLabels(r, document.ChangedLabels)
	if document.Pruned != nil {
		changed = renderEntries(r, "Pruned", *document.Pruned) || changed
	}
	if !changed {
		r.Section("Team file")
		r.Rows(terminal.Field{State: terminal.OK, Label: "Configuration", Value: "Already holds everything that the team file sets"})
	}
	renderKept(r, document)
	r.Section("Tokens")
	tokens := make([]terminal.Field, 0, len(document.Tokens))
	for _, token := range document.Tokens {
		tokens = append(tokens, tokenField(token))
	}
	r.Rows(tokens...)
	r.Section("Clients")
	clients := make([]terminal.Field, 0, len(document.Clients))
	for _, client := range document.Clients {
		clients = append(clients, clientField(t.runtime.Clients.Admission(), client))
	}
	r.Rows(clients...)
	invocation.RenderClientFiles(r, t.runtime, document.ClientFiles)
	invocation.RenderCredentialReader(r, t.runtime, document.CredentialReader)
	if document.DryRun {
		r.Success("Nothing was written: configuration, credentials and client files are unchanged")
	} else {
		r.Success("Saved; no client was run, so whether each one works is unchecked")
	}
	r.Next(document.NextAction)
	return r.Err()
}

// renderEntries names the providers, models, families and recommendations of
// one section, which it leaves out when it names none, and reports whether it
// named any.
func renderEntries(r *terminal.Renderer, title string, entries teamEntries) bool {
	var rows []terminal.Field
	for _, row := range []struct {
		label string
		names []string
	}{{"Providers", entries.Providers}, {"Models", entries.Models}, {"Families", entries.Families}, {"Recommendations", entries.Recommendations}} {
		if len(row.names) > 0 {
			rows = append(rows, terminal.Field{Label: row.label, Value: strings.Join(row.names, ", ")})
		}
	}
	if len(rows) == 0 {
		return false
	}
	r.Section(title)
	r.Rows(rows...)
	return true
}

// renderLabels names the label that the team file replaces on each model, in
// a section of its own: the Changed section names the model alone.
func renderLabels(r *terminal.Renderer, labels []changedLabel) {
	if len(labels) == 0 {
		return
	}
	shown := func(label string) string {
		if label == "" {
			return "none"
		}
		return label
	}
	rows := make([]terminal.Field, 0, len(labels))
	for _, label := range labels {
		rows = append(rows, terminal.Field{Label: label.Model, Value: shown(label.Local) + " becomes " + shown(label.File)})
	}
	r.Section("Model labels")
	r.Rows(rows...)
}

// renderKept names what stayed local, what was dropped and what is unusable.
func renderKept(r *terminal.Renderer, document teamDocument) {
	var rows []terminal.Field
	for _, kept := range document.KeptEndpoints {
		value := "Kept " + kept.Local + "; the team file has " + kept.File
		if kept.Local == "" {
			value = "Not added; the team file has " + kept.File + ", and `aigw provider edit` adds an endpoint"
		}
		rows = append(rows, terminal.Field{State: terminal.Info, Label: kept.Provider + " " + kept.Protocol + " endpoint", Value: value})
	}
	for _, kept := range document.KeptBalanceProbes {
		rows = append(rows, terminal.Field{State: terminal.Info, Label: kept.Provider + " balance probe",
			Value: "Kept " + kept.Local.Kind + " at " + kept.Local.URL + "; the team file has " + kept.File.Kind + " at " + kept.File.URL})
	}
	for _, kept := range document.KeptModels {
		value := "Kept, because " + kept.Client + " selects it on " + kept.Protocol + ", which the team file's model lacks"
		rows = append(rows, terminal.Field{State: terminal.Info, Label: kept.Model, Value: value})
	}
	// A protocol of a model is one row, even when the file's capabilities
	// were dropped on it and its provider has no endpoint for it.
	unusable := map[string]unusableProtocol{}
	for _, entry := range document.Unusable {
		unusable[entry.Model+" "+entry.Protocol] = entry
	}
	for _, dropped := range document.DroppedCapabilities {
		label := dropped.Model + " " + dropped.Protocol
		value := "Dropped " + strings.Join(dropped.Capabilities, ", ") + ", qualified on the team file's endpoint"
		if entry, both := unusable[label]; both {
			rows = append(rows, terminal.Field{State: terminal.Warn, Label: label,
				Value: "Unusable until the provider has the endpoint: " + entry.NextDetail + "; dropped " + strings.Join(dropped.Capabilities, ", ") + ", qualified on the team file's endpoint"})
			delete(unusable, label)
			continue
		}
		rows = append(rows, terminal.Field{State: terminal.Info, Label: label, Value: value})
	}
	for _, entry := range document.Unusable {
		if _, left := unusable[entry.Model+" "+entry.Protocol]; !left {
			continue
		}
		rows = append(rows, terminal.Field{State: terminal.Warn, Label: entry.Model + " " + entry.Protocol,
			Value: "Unusable until the provider has the endpoint: " + entry.NextDetail})
	}
	if len(rows) == 0 {
		return
	}
	r.Section("Kept local")
	r.Rows(rows...)
}

// tokenField is one provider's Token row.
func tokenField(token tokenItem) terminal.Field {
	field := terminal.Field{State: terminal.OK, Label: token.Provider}
	switch token.State {
	case tokenStored:
		field.Value = "Accepted by the provider and stored"
	case tokenPresent:
		field.Value = storedTokenUsed
	case tokenEnvironment:
		field.Value = "Supplied by " + secret.EnvironmentKey(token.Provider)
	case tokenUnobserved:
		field.State, field.Value = terminal.Warn, "The credential store cannot tell; "+token.NextAction+token.NextDetail
	default:
		field.State, field.Value = terminal.Info, "None"
		if next := token.NextAction + token.NextDetail; next != "" {
			field.Value = "None; " + next
		}
	}
	return field
}

// clientField is one client's row.
func clientField(admission domain.Admission, client clientItem) terminal.Field {
	field := terminal.Field{State: terminal.OK, Label: admission.Label(client.ID), Value: client.Model}
	switch client.State {
	case clientNotSelected:
		field.State, field.Value = terminal.Info, "No model selected"
	case clientWaitsForToken, clientWaitsForClient:
		field.State, field.Value = terminal.Info, client.Model+"; "+client.NextAction+client.NextDetail
	}
	return field
}

// refs names models by reference.
func refs(models []domain.ModelRef) []string {
	names := make([]string, 0, len(models))
	for _, model := range models {
		names = append(names, model.String())
	}
	return names
}

// nonNil returns names, or an empty list for none, which a document writes
// as [].
func nonNil(names []string) []string {
	if names == nil {
		return []string{}
	}
	return names
}
