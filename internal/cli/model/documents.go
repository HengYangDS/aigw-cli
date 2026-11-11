package model

import (
	"aigw-cli/internal/app"
	"aigw-cli/internal/cli/invocation"
)

// The JSON documents of the model subcommands. list and show replace those of
// `aigw route list` and `aigw route show`, add, edit and remove replace those
// of the route writes, and search replaces those of `aigw catalog` and
// `aigw models`, so each continues from its predecessors' schema_version 1.
const (
	listSchemaVersion   = 2
	showSchemaVersion   = 2
	addSchemaVersion    = 2
	editSchemaVersion   = 2
	removeSchemaVersion = 2
	searchSchemaVersion = 2
)

// searchDocument is `aigw model search`'s JSON document: each selected
// provider once, in configuration order, with the models of its catalog that
// match the query, and the first failure's fix.
type searchDocument struct {
	Providers  []searchProvider `json:"providers"`
	NextAction string           `json:"next_action,omitempty"`
	NextDetail string           `json:"next_detail,omitempty"`
}

// searchProvider is one provider's search result: its protocol sources as
// they were read, the models that match, and, when a source could not be
// read, why and the fix.
type searchProvider struct {
	ID         string         `json:"id"`
	Label      string         `json:"label"`
	Sources    []searchSource `json:"sources"`
	Models     []searchModel  `json:"models"`
	Problem    string         `json:"problem,omitempty"`
	NextAction string         `json:"next_action,omitempty"`
	NextDetail string         `json:"next_detail,omitempty"`
	// fix is the step that human output names for the problem.
	fix string
	// read says that at least one source was read.
	read bool
}

// searchSource is one of the provider's protocol sources: its protocol, the
// provider's endpoint for it, and how reading its catalog went.
type searchSource struct {
	Protocol string `json:"protocol"`
	Endpoint string `json:"endpoint"`
	Status   string `json:"status"`
}

// searchModel is one model that a catalog lists, or an added model that a
// complete read no longer finds: its name, its state, an added model's label,
// and the protocol sources that listed it.
type searchModel struct {
	ID       string   `json:"id"`
	State    string   `json:"state"`
	Label    string   `json:"label,omitempty"`
	ListedBy []string `json:"listed_by"`
}

// listDocument is `aigw model list`'s JSON document: the added models in
// configuration order.
type listDocument struct {
	Models []modelItem `json:"models"`
}

// modelItem is one added model as list and show report it, identified by its
// name, `provider/model`.
type modelItem struct {
	ID        string         `json:"id"`
	Provider  string         `json:"provider"`
	Label     string         `json:"label"`
	Family    string         `json:"family"`
	Protocols []protocolItem `json:"protocols"`
	// ProtocolClients are the clients that speak one of the model's
	// protocols; protocol overlap permits selection and qualifies nothing.
	ProtocolClients []string         `json:"protocol_clients"`
	SelectedBy      []string         `json:"selected_by"`
	Deprecated      bool             `json:"deprecated,omitempty"`
	Endpoints       []endpointDetail `json:"endpoints,omitempty"`
}

// protocolItem is one protocol that a model declares, with the capabilities
// qualified on it.
type protocolItem struct {
	Protocol     string   `json:"protocol"`
	Capabilities []string `json:"capabilities"`
}

// endpointDetail is the provider's endpoint for one of the model's protocols.
type endpointDetail struct {
	Protocol string `json:"protocol"`
	URL      string `json:"url"`
}

// changeDocument is the JSON document of `aigw model add`, `edit` and
// `remove`: the model after the change, or the model removed, what the change
// does beyond the configuration, and in a preview that it was not applied.
type changeDocument struct {
	DryRun bool `json:"dry_run,omitempty"`
	modelItem
	// FamilyCreated names a family that the change creates.
	FamilyCreated string `json:"family_created,omitempty"`
	// CatalogChecked says whether add read the provider's catalog and found
	// the model there; a preview and --unlisted read no catalog. Only add
	// reports it.
	CatalogChecked *bool `json:"catalog_checked,omitempty"`
	// Recommendations names the clients whose recommendation the change drops
	// the model from.
	Recommendations []string `json:"recommendations,omitempty"`
	// PinnedRecommendations names the clients whose recommendation of the
	// model now records the protocol that it reached, which a change of the
	// model's protocols would otherwise make ambiguous.
	PinnedRecommendations []string                      `json:"pinned_recommendations,omitempty"`
	ClientFiles           []invocation.ClientFile       `json:"client_files"`
	CredentialReader      *app.CredentialEntrypointPlan `json:"credential_reader,omitempty"`
	NextAction            string                        `json:"next_action,omitempty"`
	NextDetail            string                        `json:"next_detail,omitempty"`
}
