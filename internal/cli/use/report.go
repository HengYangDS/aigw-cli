package use

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"aigw-cli/internal/app"
	clientactivation "aigw-cli/internal/app/activation"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
)

// selectionOutcome is what a selection did, or in a preview would do: the
// configuration before and after it, the runtime that it selects, the
// resolved choice, the client files that it writes, and what became of the
// provider's Token. readOnly marks the read-only environment backend, whose
// variable supplies a missing Token.
type selectionOutcome struct {
	before, after domain.Config
	selected      domain.Runtime
	choice        choice
	preview       invocation.Preview
	changed       bool
	tokenMissing  bool
	tokenStored   bool
	readOnly      bool
	// unlisted and catalogChecked say whether the catalog check of a model
	// that the selection adds or gives a protocol was skipped or passed.
	unlisted       bool
	catalogChecked bool
}

// document is `aigw use`'s JSON document.
type document struct {
	DryRun            bool                          `json:"dry_run,omitempty"`
	Changed           bool                          `json:"changed"`
	Client            string                        `json:"client"`
	Model             string                        `json:"model"`
	WasDisabled       bool                          `json:"was_disabled,omitempty"`
	Added             bool                          `json:"added,omitempty"`
	GainedProtocol    string                        `json:"gained_protocol,omitempty"`
	CatalogChecked    *bool                         `json:"catalog_checked,omitempty"`
	Label             string                        `json:"label"`
	Provider          string                        `json:"provider"`
	Family            string                        `json:"family"`
	Protocol          string                        `json:"protocol"`
	Endpoint          string                        `json:"endpoint"`
	ProviderEndpoint  string                        `json:"provider_endpoint"`
	Retries           *retriesItem                  `json:"retries,omitempty"`
	ForwardingCleared bool                          `json:"forwarding_cleared,omitempty"`
	RetriesCleared    bool                          `json:"retries_cleared,omitempty"`
	TokenMissing      bool                          `json:"token_missing,omitempty"`
	TokenStored       bool                          `json:"token_stored,omitempty"`
	ClientFiles       []invocation.ClientFile       `json:"client_files"`
	OtherClients      []otherClientItem             `json:"other_clients,omitempty"`
	ToolSearchOff     []toolSearchItem              `json:"tool_search_off,omitempty"`
	CredentialReader  *app.CredentialEntrypointPlan `json:"credential_reader,omitempty"`
	FilesDeferred     bool                          `json:"files_deferred,omitempty"`
	RestartRequired   bool                          `json:"restart_required,omitempty"`
	NextAction        string                        `json:"next_action,omitempty"`
	NextDetail        string                        `json:"next_detail,omitempty"`
}

// retriesItem is a client's retry budgets, each absent while undeclared.
type retriesItem struct {
	Request *uint64 `json:"request,omitempty"`
	Stream  *uint64 `json:"stream,omitempty"`
}

// otherClientItem is another client whose files the selection changes, such
// as one that lists a model that the selection adds, and whether it reads
// them only when it starts.
type otherClientItem struct {
	Client          string `json:"client"`
	RestartRequired bool   `json:"restart_required,omitempty"`
}

// toolSearchItem is a client whose host tool search the selection turns off,
// with each model that the client then lists without tool_search.
type toolSearchItem struct {
	Client string            `json:"client"`
	Models []toolSearchModel `json:"models"`
}

// toolSearchModel is a listed model without tool_search, and the command
// that declares it once the model is qualified, keeping the capabilities that
// the model already declares.
type toolSearchModel struct {
	Model string `json:"model"`
	Fix   string `json:"fix"`
}

// toolSearchItems names, for each client whose host tool search the
// selection turns off, the models that it lists without tool_search after
// the selection. A search that was off before is readiness's to report.
func (o selectionOutcome) toolSearchItems(runtime invocation.Context) []toolSearchItem {
	var items []toolSearchItem
	for _, loss := range runtime.Clients.ToolSearchLost(o.before, o.after) {
		item := toolSearchItem{Client: loss.Client, Models: make([]toolSearchModel, 0, len(loss.Models))}
		for _, key := range loss.Models {
			item.Models = append(item.Models, toolSearchModel{Model: o.after.Models[key].Ref().String(), Fix: toolSearchFix(o.after.Models[key])})
		}
		items = append(items, item)
	}
	return items
}

// toolSearchFix is the command that declares tool_search on model. The
// capabilities that it gives replace the model's, so it repeats every
// capability that the model declares on any protocol, and each protocol keeps
// what it declares. A model name that would need quoting makes it prose.
func toolSearchFix(model domain.Model) string {
	declared := map[domain.Capability]bool{domain.CapabilityToolSearch: true}
	for _, capabilities := range model.Protocols {
		for _, capability := range capabilities {
			declared[capability] = true
		}
	}
	var flags []string
	for _, capability := range domain.Capabilities() {
		if declared[capability] {
			flags = append(flags, "--capability", string(capability))
		}
	}
	name := model.Ref().String()
	if line := invocation.Command("model", "edit", name); line != "" {
		return line + " " + strings.Join(flags, " ")
	}
	return "Run aigw model edit for " + name + " with " + strings.Join(flags, " ") + "."
}

// describe reports the outcome as the document does.
func (o selectionOutcome) describe(runtime invocation.Context, dryRun bool) document {
	client := o.choice.client
	previous, current := o.before.Clients[client.ID], o.after.Clients[client.ID]
	doc := document{
		DryRun: dryRun, Changed: o.changed, Client: client.ID, Model: o.choice.ref.String(), Label: o.selected.ModelLabel,
		Provider: o.selected.ProviderID, Family: o.selected.FamilyID, Protocol: o.selected.Protocol.Name(),
		Endpoint: o.selected.Endpoint, ProviderEndpoint: o.selected.UpstreamEndpoint,
		// A selection enables its client, which the result says.
		WasDisabled:       !previous.Enabled,
		ForwardingCleared: previous.ForwardingEndpoint != "" && current.ForwardingEndpoint == "",
		RetriesCleared:    previous.RetryPolicy != nil && current.RetryPolicy == nil,
		TokenMissing:      dryRun && o.tokenMissing, TokenStored: o.tokenStored,
		ClientFiles: invocation.ClientFiles(runtime, o.preview), CredentialReader: o.preview.CredentialEntrypoint,
	}
	if policy := current.RetryPolicy; policy != nil {
		doc.Retries = &retriesItem{Request: policy.RequestMaxRetries, Stream: policy.StreamMaxRetries}
	}
	for _, plan := range o.preview.Plans() {
		if plan.Client == client.ID || !plan.ChangesState || slices.ContainsFunc(doc.OtherClients, func(item otherClientItem) bool { return item.Client == plan.Client }) {
			continue
		}
		spec, _ := o.before.Admission().Spec(plan.Client)
		doc.OtherClients = append(doc.OtherClients, otherClientItem{Client: plan.Client, RestartRequired: spec.RestartAfterProjection})
	}
	if o.choice.adds || o.choice.gains {
		checked := o.catalogChecked
		doc.Added, doc.CatalogChecked = o.choice.adds, &checked
		if o.choice.gains {
			doc.GainedProtocol = o.choice.protocol.Name()
		}
	}
	// A model that the selection adds, gives a protocol or selects can declare
	// no tool_search, which turns off a host tool search that needs every
	// listed model to declare it.
	doc.ToolSearchOff = o.toolSearchItems(runtime)
	if clientactivation.ProjectionPrerequisites(o.after)[client.ID] != "" {
		doc.FilesDeferred = true
	} else {
		doc.RestartRequired = client.RestartAfterProjection && o.filesChange(client.ID)
	}
	return doc
}

// filesChange reports whether the selection changes a file of client.
func (o selectionOutcome) filesChange(client string) bool {
	for _, plan := range o.preview.Plans() {
		if plan.Client == client && plan.ChangesState {
			return true
		}
	}
	return false
}

// next is the step after the selection: the same selection without --dry-run
// after a preview, with the Token step first when the Token is missing; the
// step that writes the files of a client whose files wait; restarting a
// client that reads its files at start; and otherwise a check of the client.
func (o selectionOutcome) next(doc document) string {
	client := o.choice.client
	switch {
	case doc.DryRun:
		line := o.choice.command()
		switch {
		case doc.TokenMissing && o.readOnly:
			return setTokenVariable(o.selected.ProviderID, line, "the same command without --dry-run")
		case line == "":
			return "Run the same command without --dry-run to apply this selection."
		case doc.TokenMissing:
			// Applying asks for the Token, so a script cannot run it as it is.
			return "Run `" + line + "` in a terminal, which asks for the provider's Token."
		}
		return line
	case doc.FilesDeferred:
		return clientactivation.ProjectionPrerequisites(o.after)[client.ID]
	}
	if restart := o.restarts(doc); len(restart) > 0 {
		return fmt.Sprintf("Restart %s, then run `aigw check %s`", o.labels(restart), client.ID)
	}
	return "aigw check " + client.ID
}

// restarts names, the selected client first, the clients whose changed files
// take effect only once they start again.
func (o selectionOutcome) restarts(doc document) []string {
	var restart []string
	if doc.RestartRequired {
		restart = append(restart, doc.Client)
	}
	for _, other := range doc.OtherClients {
		if other.RestartRequired {
			restart = append(restart, other.Client)
		}
	}
	return restart
}

// report writes the outcome as JSON or for a person.
func (o selectionOutcome) report(runtime invocation.Context, options options) error {
	doc := o.describe(runtime, options.dryRun)
	doc.NextAction, doc.NextDetail = terminal.NextSteps(o.next(doc))
	if options.jsonMode {
		return terminal.WriteDocument(runtime.Out, schemaVersion, true, doc)
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle(o.title(doc))
	r.Rows(o.rows(doc)...)
	invocation.RenderClientFiles(r, runtime, doc.ClientFiles)
	renderToolSearchFixes(r, doc.ToolSearchOff)
	if reader := doc.CredentialReader; reader != nil {
		r.Section("Credential reader")
		r.Text("Install " + reader.Path)
	}
	if doc.DryRun {
		r.Success("Nothing was written: configuration, credentials and client files are unchanged")
	} else if doc.Changed || doc.TokenStored {
		r.Success("Saved; the other clients' selections are unchanged")
	} else {
		r.Success(o.choice.client.Label + " already uses " + doc.Model)
	}
	r.Next(o.next(doc))
	return r.Err()
}

// title names what the selection did.
func (o selectionOutcome) title(doc document) string {
	switch {
	case doc.DryRun:
		return "Model selected (preview)"
	case doc.Changed:
		return "Model selected"
	case doc.TokenStored:
		return "Token stored"
	default:
		return "Model already selected"
	}
}

// rows are the selection's rows in human output.
func (o selectionOutcome) rows(doc document) []terminal.Field {
	client := o.choice.client
	rows := []terminal.Field{
		{Label: "Client", Value: client.Label},
		{Label: "Model", Value: doc.Model},
	}
	switch {
	case doc.WasDisabled && doc.DryRun:
		rows = append(rows, terminal.Field{State: terminal.Info, Label: "Enabled", Value: "Was disabled; this selection enables it"})
	case doc.WasDisabled:
		rows = append(rows, terminal.Field{State: terminal.OK, Label: "Enabled", Value: "Was disabled; now enabled"})
	}
	if doc.Added {
		rows = append(rows, terminal.Field{State: terminal.OK, Label: "Added", Value: "To provider " + doc.Provider + ", in family " + doc.Family + ", declaring no capability"})
	}
	protocol := domain.ProtocolLabel(o.selected.Protocol)
	if doc.GainedProtocol != "" {
		protocol += ", which the model gains, declaring no capability"
	}
	rows = append(rows, terminal.Field{Label: "Protocol", Value: protocol}, terminal.Field{Label: "Endpoint", Value: doc.Endpoint})
	if doc.CatalogChecked != nil {
		rows = append(rows, o.catalogRow(doc))
	}
	if doc.Endpoint != doc.ProviderEndpoint {
		rows = append(rows, terminal.Field{Label: "Provider endpoint", Value: doc.ProviderEndpoint})
	}
	if doc.ForwardingCleared {
		rows = append(rows, terminal.Field{State: terminal.Warn, Label: "Forwarding", Value: "Cleared, because the client's upstream endpoint changed"})
	}
	switch {
	case doc.RetriesCleared:
		rows = append(rows, terminal.Field{State: terminal.Warn, Label: "Retries", Value: "Cleared, because they belonged to the previous provider; " + client.Label + " uses its own defaults"})
	case client.RetryBudgets:
		rows = append(rows, retryRows(doc.Retries)...)
	}
	switch {
	case doc.TokenMissing && o.readOnly:
		rows = append(rows, terminal.Field{State: terminal.Info, Label: "Token", Value: "Missing; set " + secret.EnvironmentKey(doc.Provider) + " to supply it"})
	case doc.TokenMissing:
		rows = append(rows, terminal.Field{State: terminal.Info, Label: "Token", Value: "Missing; aigw use asks for it in a terminal and validates it"})
	case doc.TokenStored:
		rows = append(rows, terminal.Field{State: terminal.OK, Label: "Token", Value: "Validated and stored"})
	}
	switch {
	case doc.FilesDeferred:
		rows = append(rows, terminal.Field{Label: "Client files", Value: "Written once " + client.Label + " is installed"})
	case doc.RestartRequired:
		rows = append(rows, terminal.Field{Label: "Activation", Value: "Restart required"})
	}
	if client.SessionsKeepModel && o.filesChange(client.ID) {
		rows = append(rows, terminal.Field{Label: "Existing sessions", Value: "Keep the model they started with; new sessions use this selection"})
	}
	return append(rows, o.otherClientRows(doc)...)
}

// otherClientRows name the other clients whose files the selection changes,
// with the ones that need a restart to apply them, and each client whose host
// tool search it turns off, with the models that turn it off.
func (o selectionOutcome) otherClientRows(doc document) []terminal.Field {
	var rows []terminal.Field
	if len(doc.OtherClients) > 0 {
		var others, restart []string
		for _, other := range doc.OtherClients {
			others = append(others, other.Client)
			if other.RestartRequired {
				restart = append(restart, other.Client)
			}
		}
		value := o.labels(others) + ": their files change too"
		if len(restart) > 0 {
			value += "; restart " + o.labels(restart) + " to apply them"
		}
		rows = append(rows, terminal.Field{Label: "Other clients", Value: value})
	}
	for _, item := range doc.ToolSearchOff {
		models := make([]string, 0, len(item.Models))
		for _, model := range item.Models {
			models = append(models, model.Model)
		}
		verb := " declares"
		if len(models) > 1 {
			verb = " declare"
		}
		rows = append(rows, terminal.Field{State: terminal.Warn, Label: "Tool search", Value: "Off in " + o.labels([]string{item.Client}) + ", because " +
			prose(models) + verb + " no tool_search"})
	}
	return rows
}

// renderToolSearchFixes names the command that declares tool_search on each
// model that turns a host tool search off, once a model is qualified, and
// says that it keeps the model's other capabilities.
func renderToolSearchFixes(r *terminal.Renderer, items []toolSearchItem) {
	var fixes []string
	for _, item := range items {
		for _, model := range item.Models {
			if !slices.Contains(fixes, model.Fix) {
				fixes = append(fixes, model.Fix)
			}
		}
	}
	if len(fixes) == 0 {
		return
	}
	r.Section("Tool search")
	r.Text("Once a model is qualified, its command declares tool_search and keeps its other capabilities:")
	for _, fix := range fixes {
		r.Command(fix)
	}
}

// catalogRow says whether the provider's catalog lists a model that the
// selection adds or gives a protocol.
func (o selectionOutcome) catalogRow(doc document) terminal.Field {
	switch {
	case doc.DryRun:
		return terminal.Field{State: terminal.Info, Label: "Catalog", Value: "Not checked in a preview"}
	case o.unlisted:
		return terminal.Field{State: terminal.Warn, Label: "Catalog", Value: "Not checked: --unlisted"}
	default:
		return terminal.Field{State: terminal.OK, Label: "Catalog", Value: "Listed by the provider"}
	}
}

// labels names clients by their labels, in a list for prose.
func (o selectionOutcome) labels(clients []string) string {
	labels := make([]string, 0, len(clients))
	for _, client := range clients {
		labels = append(labels, o.before.Admission().Label(client))
	}
	return prose(labels)
}

// prose joins items into a list for a sentence: "a", "a and b", "a, b and c".
func prose(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
	}
}

// retryRows show a client's retry budgets, or that it uses its own defaults.
func retryRows(retries *retriesItem) []terminal.Field {
	if retries == nil {
		return []terminal.Field{{Label: "Retries", Value: "The client's own defaults"}}
	}
	var rows []terminal.Field
	if retries.Request != nil {
		rows = append(rows, terminal.Field{Label: "Request retries", Value: strconv.FormatUint(*retries.Request, 10)})
	}
	if retries.Stream != nil {
		rows = append(rows, terminal.Field{Label: "Stream retries", Value: strconv.FormatUint(*retries.Stream, 10)})
	}
	return rows
}
