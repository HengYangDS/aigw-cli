package invocation

import (
	"errors"
	"strings"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/terminal"
)

// Stopped is the one report of a command whose transition stopped clients,
// once it saved what it could: the first stop's problem and fix, in the words
// of the client's local check, the others as evidence, and as its impact what
// the command wrote, given by written, and which clients' files it did not.
// It is nil when the invocation's last transition stopped no client. Every
// command that writes client files returns it, so a stop exits as `aigw sync`
// does.
func Stopped(runtime Context, written string) error {
	stops := runtime.Stops.Stopped()
	if len(stops) == 0 {
		return nil
	}
	evidence := make([]string, 0, len(stops)-1)
	names := make([]string, 0, len(stops))
	causes := make([]error, 0, len(stops))
	for index, stop := range stops {
		causes = append(causes, stop)
		names = append(names, runtime.Clients.Admission().ProductName(stop.Client()))
		if index > 0 {
			evidence = append(evidence, stop.Error()+"; its fix: "+stop.NextAction()+".")
		}
	}
	impact := written + " The files of " + strings.Join(names, " and ") + " were not written, apart from the withdrawal of a file that the change drops."
	return terminal.ProblemError(stops[0].Error(), strings.Join(evidence, " "), impact, stops[0].NextAction(), errors.Join(causes...))
}

// StoppedClient is a client that a preview's transition would stop, with its
// problem and its one fix, as a JSON document lists it.
type StoppedClient struct {
	Client     string `json:"client"`
	Problem    string `json:"problem"`
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
}

// StoppedClients lists stops as a document does.
func StoppedClients(stops []*adapter.StopError) []StoppedClient {
	listed := make([]StoppedClient, 0, len(stops))
	for _, stop := range stops {
		item := StoppedClient{Client: stop.Client(), Problem: stop.Error()}
		item.NextAction, item.NextDetail = terminal.NextSteps(stop.NextAction())
		listed = append(listed, item)
	}
	return listed
}
