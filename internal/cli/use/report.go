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
	"aigw-cli/internal/terminal"
)

// selectionOutcome is what a selection did, or in a preview would do: the
// configuration before and after it, the runtime that it selects, the
// resolved choice, the client files that it writes, and what became of the
// provider's Token.
type selectionOutcome struct {
	before, after domain.Config
	selected      domain.Runtime
	choice        choice
	preview       invocation.Preview
	changed       bool
	tokenMissing  bool
	tokenStored   bool
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
	OtherClients      []string                      `json:"other_clients,omitempty"`
	ToolSearchOff     []string                      `json:"tool_search_off,omitempty"`
	ToolSearchFix     string                        `json:"tool_search_fix,omitempty"`
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

// describe reports the outcome as the document does.
func (o selectionOutcome) describe(runtime invocation.Context, dryRun bool) document {
	client := o.choice.client
	previous, current := o.before.Clients[client.ID], o.after.Clients[client.ID]
	doc := document{
		DryRun: dryRun, Changed: o.changed, Client: client.ID, Model: o.choice.ref.String(), Label: o.selected.ModelLabel,
		Provider: o.selected.ProviderID, Family: o.selected.FamilyID, Protocol: o.selected.Protocol.Name(),
		Endpoint: o.selected.Endpoint, ProviderEndpoint: o.selected.UpstreamEndpoint,
		ForwardingCleared: previous.ForwardingEndpoint != "" && current.ForwardingEndpoint == "",
		RetriesCleared:    previous.RetryPolicy != nil && current.RetryPolicy == nil,
		TokenMissing:      dryRun && o.tokenMissing, TokenStored: o.tokenStored,
		ClientFiles: invocation.ClientFiles(runtime, o.preview), CredentialReader: o.preview.CredentialEntrypoint,
	}
	if policy := current.RetryPolicy; policy != nil {
		doc.Retries = &retriesItem{Request: policy.RequestMaxRetries, Stream: policy.StreamMaxRetries}
	}
	for _, plan := range o.preview.Plans() {
		if plan.Client != client.ID && plan.ChangesState && !slices.Contains(doc.OtherClients, plan.Client) {
			doc.OtherClients = append(doc.OtherClients, plan.Client)
		}
	}
	if o.choice.adds || o.choice.gains {
		checked := o.catalogChecked
		doc.Added, doc.CatalogChecked = o.choice.adds, &checked
		if o.choice.gains {
			doc.GainedProtocol = o.choice.protocol.Name()
		}
		// What the selection adds declares no capability, which turns off a
		// host tool search that needs every listed model to declare it.
		if lost := runtime.Clients.ToolSearchLost(o.before, o.after); len(lost) > 0 {
			doc.ToolSearchOff = lost
			doc.ToolSearchFix = "aigw model edit " + doc.Model + " --capability tool_search"
		}
	}
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
// after a preview; the step that writes the files of a client whose files
// wait; restarting a client that reads its files at start; and otherwise a
// check of the client.
func (o selectionOutcome) next(doc document) string {
	client := o.choice.client
	switch {
	case doc.DryRun:
		line := o.choice.command()
		switch {
		case line == "":
			return "Run the same command without --dry-run to apply this selection."
		case doc.TokenMissing:
			// Applying asks for the Token, so a script cannot run it as it is.
			return "Run `" + line + "` in a terminal, which asks for the provider's Token."
		}
		return line
	case doc.FilesDeferred:
		return clientactivation.ProjectionPrerequisites(o.after)[client.ID]
	case doc.RestartRequired:
		return fmt.Sprintf("Restart %s, then run `aigw check %s`", client.Label, client.ID)
	default:
		return "aigw check " + client.ID
	}
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
	if len(doc.OtherClients) > 0 {
		rows = append(rows, terminal.Field{Label: "Other clients", Value: o.labels(doc.OtherClients) + ": their files change too"})
	}
	if len(doc.ToolSearchOff) > 0 {
		rows = append(rows, terminal.Field{State: terminal.Warn, Label: "Tool search", Value: "Off in " + o.labels(doc.ToolSearchOff) + ", because " + doc.Model +
			" declares no tool_search; once it is qualified, `" + doc.ToolSearchFix + "` declares it"})
	}
	return rows
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
	switch len(labels) {
	case 1:
		return labels[0]
	default:
		return strings.Join(labels[:len(labels)-1], ", ") + " and " + labels[len(labels)-1]
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
