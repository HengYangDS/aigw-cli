package adapter

import (
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// RecordedSelection is the selection that AIGW recorded for a client, as the
// command that restores it names it.
type RecordedSelection struct {
	// Client is the client's ID in commands, and Product its name in messages.
	Client, Product string
	Model           domain.ModelRef
	// Protocol is the restoring command's --protocol word, for a client that
	// speaks several protocols; empty for a client that needs none.
	Protocol string
}

// Recorded names the selection that runtime resolves for spec's client. A
// client that speaks several protocols needs the protocol in the command that
// restores the selection; one that speaks a single protocol keeps its binding
// free of one.
func Recorded(spec domain.ClientSpec, runtime domain.Runtime) RecordedSelection {
	recorded := RecordedSelection{Client: spec.ID, Product: spec.Label, Model: runtime.Model}
	if len(spec.EndpointProtocols) > 1 {
		recorded.Protocol = runtime.Protocol.Name()
	}
	return recorded
}

// SelectCommand is the command that selects model for client, with protocol
// as its --protocol word for a client that needs one, or "" when the model
// would need quoting in the platform's shells.
func SelectCommand(client string, model domain.ModelRef, protocol string) string {
	command := platform.AIGWCommand("use", client, model.String())
	if command != "" && protocol != "" {
		command += " --protocol " + protocol
	}
	return command
}

// Restore is the command that writes the recorded selection to the client,
// or, for a model that would need quoting, the step in prose.
func (recorded RecordedSelection) Restore() string {
	if command := SelectCommand(recorded.Client, recorded.Model, recorded.Protocol); command != "" {
		return command
	}
	return "Select " + recorded.Model.String() + " for " + recorded.Product + " again with `aigw use`, quoting the model for your shell."
}

// SelectionError refuses a client's own model selection that AIGW does not
// follow, such as a model that AIGW has not added within the client's reach.
// Its next action restores the recorded selection, the one step that works on
// every such state; its message also names the command that adds and selects
// the model that the client names, when there is one.
type SelectionError struct {
	message string
	restore string
}

// RefusedSelection refuses the selection of a client whose recorded selection
// is recorded. what says what the client selects and why AIGW does not follow
// it, after the client's name; adds is the command that adds and selects the
// named model, or empty when no command does.
func RefusedSelection(recorded RecordedSelection, what, adds string) *SelectionError {
	message := recorded.Product + " " + what + "; AIGW's selection for " + recorded.Product + " is " + recorded.Model.String()
	if adds != "" {
		message += "; `" + adds + "` adds and selects the model that " + recorded.Product + " names"
	}
	return &SelectionError{message: message, restore: recorded.Restore()}
}

// Error names what the client selects, why AIGW does not follow it, and the
// recorded selection.
func (e *SelectionError) Error() string { return e.message }

// NextAction is the step that restores the recorded selection.
func (e *SelectionError) NextAction() string { return e.restore }
