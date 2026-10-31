package route

import "aigw-cli/internal/domain"

// listSchemaVersion and the other constants are the versions of the route
// subcommands' JSON documents.
const (
	listSchemaVersion   = 1
	showSchemaVersion   = 1
	addSchemaVersion    = 1
	editSchemaVersion   = 1
	removeSchemaVersion = 1
)

// addDocument is `aigw route add`'s JSON document.
type addDocument struct {
	Route         string                  `json:"route"`
	Account       string                  `json:"account"`
	Label         string                  `json:"label"`
	Purpose       string                  `json:"purpose,omitempty"`
	Model         string                  `json:"model"`
	UpstreamModel string                  `json:"upstream_model"`
	Protocol      domain.EndpointProtocol `json:"protocol"`
	NextAction    string                  `json:"next_action,omitempty"`
	NextDetail    string                  `json:"next_detail,omitempty"`
}

// editDocument is `aigw route edit`'s JSON document: the Route's metadata
// after the edit.
type editDocument struct {
	Route      string `json:"route"`
	Label      string `json:"label"`
	Purpose    string `json:"purpose,omitempty"`
	NextAction string `json:"next_action"`
}

// removeDocument is `aigw route remove`'s JSON document.
type removeDocument struct {
	Route   string `json:"route"`
	Account string `json:"account"`
}
