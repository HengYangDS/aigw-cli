package account

import "aigw-cli/internal/domain"

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
	Account    string           `json:"account"`
	Label      string           `json:"label"`
	Endpoints  domain.Endpoints `json:"endpoints"`
	NextAction string           `json:"next_action"`
}

// diagnosticsDocument is the JSON document of `aigw account diagnostics
// enable` and `disable`.
type diagnosticsDocument struct {
	Account     string `json:"account"`
	Label       string `json:"label,omitempty"`
	Diagnostics string `json:"diagnostics"`
	NextAction  string `json:"next_action,omitempty"`
}
