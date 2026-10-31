package route

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
)

// listSchemaVersion and the other constants are the versions of the route
// subcommands' JSON documents.
const (
	listSchemaVersion   = 1
	showSchemaVersion   = 1
	addSchemaVersion    = 1
	editSchemaVersion   = 1
	removeSchemaVersion = 1
)

// addDocument is `aigw route add`'s JSON document. A preview also reports what
// adding the Route would change beyond the configuration.
type addDocument struct {
	DryRun        bool                    `json:"dry_run,omitempty"`
	Route         string                  `json:"route"`
	Account       string                  `json:"account"`
	Label         string                  `json:"label"`
	Purpose       string                  `json:"purpose,omitempty"`
	Model         string                  `json:"model"`
	UpstreamModel string                  `json:"upstream_model"`
	Protocol      domain.EndpointProtocol `json:"protocol"`
	invocation.Preview
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
}

// editDocument is `aigw route edit`'s JSON document: the Route's metadata
// after the edit.
type editDocument struct {
	DryRun  bool   `json:"dry_run,omitempty"`
	Route   string `json:"route"`
	Label   string `json:"label"`
	Purpose string `json:"purpose,omitempty"`
	invocation.Preview
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
}

// removeDocument is `aigw route remove`'s JSON document. A preview also names
// the clients whose recommendation the removal would change.
type removeDocument struct {
	DryRun          bool     `json:"dry_run,omitempty"`
	Route           string   `json:"route"`
	Account         string   `json:"account"`
	Recommendations []string `json:"recommendations,omitempty"`
	invocation.Preview
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
}
