package client

import (
	"aigw-cli/internal/app"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/terminal"
)

// listSchemaVersion and the other constants are the versions of the client
// subcommands' JSON documents. list replaced the document of `aigw client
// discover`, version 1, so it continues at 2; enable and disable name their
// client files as client_files, which replaced projections, so they continue
// at 2; edit has no predecessor.
const (
	listSchemaVersion    = 2
	enableSchemaVersion  = 2
	disableSchemaVersion = 2
	editSchemaVersion    = 1
)

// editDocument is `aigw client edit`'s JSON document: the client's settings
// after the edit, whether the edit changes them, the client files that it
// writes, and in a preview that it was not applied.
type editDocument struct {
	DryRun     bool     `json:"dry_run,omitempty"`
	Client     string   `json:"client"`
	Changed    bool     `json:"changed"`
	Enabled    bool     `json:"enabled"`
	Model      string   `json:"model,omitempty"`
	Executable string   `json:"executable,omitempty"`
	Targets    []string `json:"targets"`
	// Forwarding is the endpoint to which the client is sent instead of its
	// provider's, absent when it reaches the provider directly.
	Forwarding string `json:"forwarding,omitempty"`
	// Retries are the client's native retry budgets, absent when it uses
	// its own defaults.
	Retries          *retriesItem                  `json:"retries,omitempty"`
	ClientFiles      []invocation.ClientFile       `json:"client_files"`
	CredentialReader *app.CredentialEntrypointPlan `json:"credential_reader,omitempty"`
	RestartRequired  bool                          `json:"restart_required,omitempty"`
	NextAction       string                        `json:"next_action,omitempty"`
	NextDetail       string                        `json:"next_detail,omitempty"`
}

// retriesItem is a client's native retry budgets; a budget that it does not
// declare is absent.
type retriesItem struct {
	Request *uint64 `json:"request,omitempty"`
	Stream  *uint64 `json:"stream,omitempty"`
}

// listDocument is `aigw client list`'s JSON document: every client in
// admission order.
type listDocument struct {
	Clients []clientItem `json:"clients"`
}

// clientItem is one client as list reports it: whether it is installed and
// where, whether it is enabled, and its model as provider/model, absent when
// it selects none.
type clientItem struct {
	ID         string `json:"id"`
	Installed  bool   `json:"installed"`
	Executable string `json:"executable,omitempty"`
	Enabled    bool   `json:"enabled"`
	Model      string `json:"model,omitempty"`
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
