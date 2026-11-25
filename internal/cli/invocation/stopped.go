package invocation

import (
	"errors"
	"slices"
	"strings"

	"aigw-cli/internal/app/synchronization"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"
)

// stoppedClient is a client that the invocation's transition, or the preview
// of it, stopped, as its command's document lists it: the problem and the one
// fix that the client's local check gives, and what became of its files.
type stoppedClient struct {
	Client     string `json:"client"`
	Problem    string `json:"problem"`
	Files      string `json:"files"`
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
}

// StopReport is one invocation's report of the clients that its transition,
// or the preview of it, stopped: the one place where every command reports a
// stop. The command's own report stays whole; the stop follows it, in the
// document as stopped_clients with ok false and in human output as a section
// after the command's own, and the invocation then fails.
type StopReport struct {
	stops     *synchronization.Stops
	admission domain.Admission
	written   bool
}

// NewStopReport reports what stops records for one invocation of a program
// that admits admission's clients.
func NewStopReport(stops *synchronization.Stops, admission domain.Admission) *StopReport {
	return &StopReport{stops: stops, admission: admission}
}

// Failed reports whether the invocation's transition stopped a client.
func (report *StopReport) Failed() bool {
	return report != nil && len(report.stops.Stopped()) > 0
}

// Document is the field that the invocation adds to its command's document
// for the clients that it stopped, and no field when it stopped none.
func (report *StopReport) Document() []terminal.DocumentField {
	if !report.Failed() {
		return nil
	}
	listed := make([]stoppedClient, 0, len(report.stops.Stopped()))
	for _, stop := range report.stops.Stopped() {
		item := stoppedClient{Client: stop.Client(), Problem: stop.Error(), Files: report.files(stop.Client(), false)}
		item.NextAction, item.NextDetail = terminal.NextSteps(stop.NextAction())
		listed = append(listed, item)
	}
	return []terminal.DocumentField{{Name: "stopped_clients", Value: listed}}
}

// Render names the stopped clients once, after the command's own human
// output: each with its problem, then what the transition wrote and what
// became of the stopped clients' files, as the record holds it, then each
// fix.
func (report *StopReport) Render(r *terminal.Renderer) {
	if !report.Failed() || report.written {
		return
	}
	report.written = true
	r.Section("Stopped clients")
	for _, stop := range report.stops.Stopped() {
		r.Rows(terminal.Field{State: terminal.Fail, Label: report.admission.Label(stop.Client()), Value: stop.Error()})
	}
	r.Text(report.Impact())
	for _, stop := range report.stops.Stopped() {
		if action, detail := terminal.NextSteps(stop.NextAction()); action != "" {
			r.Command(action)
		} else {
			r.Text(detail)
		}
	}
}

// Impact says what the transition wrote and what became of each stopped
// client's files, from what the record holds and nothing else.
func (report *StopReport) Impact() string {
	written, _, preview := report.stops.Written()
	sentences := []string{}
	if len(written) > 0 {
		verb := "AIGW wrote the files of "
		if preview {
			verb = "A run would write the files of "
		}
		sentences = append(sentences, verb+report.names(written)+".")
	}
	for _, stop := range report.stops.Stopped() {
		sentences = append(sentences, report.files(stop.Client(), true))
	}
	return strings.Join(sentences, " ")
}

// files says what became of a stopped client's files: as a sentence that
// names the client, with sentence, and otherwise as a document's phrase.
func (report *StopReport) files(client string, sentence bool) string {
	_, withdrawn, preview := report.stops.Written()
	product := report.admission.ProductName(client)
	dropped := slices.Contains(withdrawn, client)
	switch {
	case !sentence && dropped && preview:
		return "dropped files would be withdrawn, the others left as they are"
	case !sentence && dropped:
		return "dropped files withdrawn, the others left as they were"
	case !sentence && preview:
		return "left as they are"
	case !sentence:
		return "left as they were"
	case dropped && preview:
		return "A run would withdraw AIGW's settings from the files of " + product + " that the change drops and leave the others as they are."
	case dropped:
		return "AIGW withdrew its settings from the files of " + product + " that the change drops and left the others as they were."
	case preview:
		return "A run would leave the files of " + product + " as they are."
	}
	return "AIGW left the files of " + product + " as they were."
}

// names lists clients by their product names.
func (report *StopReport) names(clients []string) string {
	names := make([]string, 0, len(clients))
	for _, client := range clients {
		names = append(names, report.admission.ProductName(client))
	}
	return strings.Join(names, " and ")
}

// Err is the invocation's failure when its transition stopped a client, for
// its exit status; the report already said what happened.
func (report *StopReport) Err() error {
	if !report.Failed() {
		return nil
	}
	causes := make([]error, 0, len(report.stops.Stopped()))
	for _, stop := range report.stops.Stopped() {
		causes = append(causes, stop)
	}
	return terminal.Presented(errors.Join(causes...))
}
