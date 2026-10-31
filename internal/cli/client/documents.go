package client

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/terminal"
)

// discoverSchemaVersion and the other constants are the versions of the client
// subcommands' JSON documents.
const (
	discoverSchemaVersion = 1
	enableSchemaVersion   = 1
	disableSchemaVersion  = 1
)

// discoverDocument is `aigw client discover`'s JSON document.
type discoverDocument struct {
	Clients []discoveredClient `json:"clients"`
}

// discoveredClient is one admitted client and the executable that discovery
// found for it.
type discoveredClient struct {
	ID         string `json:"id"`
	State      string `json:"state"`
	Executable string `json:"executable,omitempty"`
}

// projectionDocument is the JSON document of `aigw client enable` and
// `disable`. A preview reports the state that the command would reach and
// what reaching it would change beyond the configuration.
type projectionDocument struct {
	DryRun          bool   `json:"dry_run,omitempty"`
	Client          string `json:"client"`
	State           string `json:"state"`
	RestartRequired bool   `json:"restart_required"`
	invocation.Preview
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
}

// newProjectionDocument reports a client's projection state and the
// follow-up that its human output names.
func newProjectionDocument(client, state string, restart bool, next string) projectionDocument {
	document := projectionDocument{Client: client, State: state, RestartRequired: restart}
	document.NextAction, document.NextDetail = terminal.NextSteps(next)
	return document
}
