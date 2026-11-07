package model

import (
	"aigw-cli/internal/app"
	"aigw-cli/internal/cli/invocation"
)

// The JSON documents of the model subcommands. list and show replace those of
// `aigw route list` and `aigw route show`, and add, edit and remove replace
// those of the route writes, so each continues from its predecessor's
// schema_version 1.
const (
	listSchemaVersion   = 2
	showSchemaVersion   = 2
	addSchemaVersion    = 2
	editSchemaVersion   = 2
	removeSchemaVersion = 2
)

// listDocument is `aigw model list`'s JSON document: the added models in
// configuration order.
type listDocument struct {
	Models []modelItem `json:"models"`
}

// modelItem is one added model as list and show report it.
type modelItem struct {
	Model     string         `json:"model"`
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
	// CatalogChecked says whether the provider's catalog listed the model; a
	// preview and --unlisted read no catalog.
	CatalogChecked *bool `json:"catalog_checked,omitempty"`
	// Recommendations names the clients whose recommendation the change drops
	// the model from.
	Recommendations  []string                      `json:"recommendations,omitempty"`
	ClientFiles      []clientFile                  `json:"client_files"`
	CredentialReader *app.CredentialEntrypointPlan `json:"credential_reader,omitempty"`
	NextAction       string                        `json:"next_action,omitempty"`
	NextDetail       string                        `json:"next_detail,omitempty"`
}

// clientFile is one client file that a change writes or would write.
type clientFile struct {
	Client string `json:"client"`
	File   string `json:"file,omitempty"`
	Action string `json:"action"`
}

// clientFiles names the client files of a preview as documents do.
func clientFiles(runtime invocation.Context, preview invocation.Preview) []clientFile {
	files := make([]clientFile, 0, len(preview.Projections))
	for _, projection := range invocation.Projections(runtime, preview.Plans()) {
		files = append(files, clientFile{Client: projection.Client, File: projection.Target, Action: projection.Action})
	}
	return files
}
