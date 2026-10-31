package account

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
)

// listSchemaVersion and the other constants are the versions of the account
// subcommands' JSON documents.
const (
	listSchemaVersion        = 1
	editSchemaVersion        = 1
	diagnosticsSchemaVersion = 1
)

// editDocument is `aigw account edit`'s JSON document: the Account's
// metadata after the edit.
type editDocument struct {
	DryRun    bool             `json:"dry_run,omitempty"`
	Account   string           `json:"account"`
	Label     string           `json:"label"`
	Endpoints domain.Endpoints `json:"endpoints"`
	invocation.Preview
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
}

// diagnosticsDocument is the JSON document of `aigw account diagnostics
// enable` and `disable`.
type diagnosticsDocument struct {
	Account     string `json:"account"`
	Label       string `json:"label,omitempty"`
	Diagnostics string `json:"diagnostics"`
	NextAction  string `json:"next_action,omitempty"`
}
