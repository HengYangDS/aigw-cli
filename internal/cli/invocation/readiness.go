package invocation

import (
	"strings"

	"aigw-cli/internal/app"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/terminal"
)

// DocumentClient names a client's next step as documents do: a command that
// a script can run as it stands is next_action, and a command with a
// placeholder or prose is next_detail.
func DocumentClient(client domainreadiness.Client) domainreadiness.Client {
	client.NextAction, client.NextDetail = terminal.NextSteps(client.NextAction)
	return client
}

// DocumentClients returns a copy of clients as status and check documents
// write them: each next step split as DocumentClient splits it, and the
// qualified modes and platforms, which human output shows as labels, in
// snake_case.
func DocumentClients(clients map[string]app.ClientStatus) map[string]app.ClientStatus {
	documented := make(map[string]app.ClientStatus, len(clients))
	for id, status := range clients {
		status.Client = DocumentClient(status.Client)
		status.QualifiedModes = snakeLabels(status.QualifiedModes)
		status.QualifiedPlatforms = snakeLabels(status.QualifiedPlatforms)
		documented[id] = status
	}
	return documented
}

func snakeLabels(labels []string) []string {
	if labels == nil {
		return nil
	}
	snake := make([]string, len(labels))
	for index, label := range labels {
		snake[index] = terminal.SnakeValue(strings.ToLower(strings.ReplaceAll(label, " ", "_")))
	}
	return snake
}
