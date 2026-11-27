package invocation

import (
	"aigw-cli/internal/adapter"
	"aigw-cli/internal/app/synchronization"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"
)

// followedSelection is one client's own choice in a command's document: the
// model that the client's file selects, the selection that AIGW held before,
// and the command that selects that one again, which is absent when the
// model would need quoting or the configuration no longer holds it. The
// protocols are named for a client that speaks several.
type followedSelection struct {
	Client            string `json:"client"`
	Model             string `json:"model"`
	Protocol          string `json:"protocol,omitempty"`
	Earlier           string `json:"earlier"`
	EarlierProtocol   string `json:"earlier_protocol,omitempty"`
	ReturnsWith       string `json:"returns_with,omitempty"`
	ForwardingCleared bool   `json:"forwarding_cleared,omitempty"`
	RetriesCleared    bool   `json:"retries_cleared,omitempty"`
}

// FollowedReport is one invocation's report of the clients' own choices: its
// command's document carries them, and its human output names them once.
type FollowedReport struct {
	follows         *synchronization.Follows
	reports, failed bool
	written         bool
}

// NewFollowedReport reports what follows records for one invocation.
func NewFollowedReport(follows *synchronization.Follows) *FollowedReport {
	return &FollowedReport{follows: follows}
}

// ReportedConfiguration reads the working configuration for a command that
// saves nothing and reports it, such as status and check: the invocation's
// output names every choice of a client that the configuration follows. A
// list or a view reads the same configuration through LoadConfiguration and
// names no choice.
func (runtime Context) ReportedConfiguration() (domain.Config, error) {
	if runtime.Followed != nil {
		runtime.Followed.reports = true
	}
	return runtime.LoadConfiguration()
}

// Failed marks an invocation whose command failed: it recorded no choice, so
// only a report still names what its configuration follows.
func (report *FollowedReport) Failed() {
	if report != nil {
		report.failed = true
	}
}

// selections are the choices that the invocation names: every choice that a
// report's configuration follows, and for any other command those that its
// transition, or the preview of it, records. A command that neither reports
// nor ran a transition, and one that failed, names none.
func (report *FollowedReport) selections() []synchronization.Followed {
	switch {
	case report == nil:
		return nil
	case report.reports:
		return report.follows.Observed()
	case report.failed:
		return nil
	}
	recorded, _ := report.follows.Recorded()
	return recorded
}

// Document is the field that the invocation adds to its command's document
// for the choices that it names, and no field when it names none.
func (report *FollowedReport) Document() []terminal.DocumentField {
	selections := report.selections()
	if len(selections) == 0 {
		return nil
	}
	document := make([]followedSelection, 0, len(selections))
	for _, choice := range selections {
		entry := followedSelection{
			Client: choice.Now.Client, Model: choice.Now.Model.String(), Protocol: choice.Now.Protocol,
			Earlier: choice.Earlier.Model.String(), EarlierProtocol: choice.Earlier.Protocol,
			ForwardingCleared: choice.ForwardingCleared, RetriesCleared: choice.RetriesCleared,
		}
		entry.ReturnsWith, _ = terminal.NextSteps(choice.Returns)
		document = append(document, entry)
	}
	return []terminal.DocumentField{{Name: "followed_selections", Value: document}}
}

// Render names the choices once in the invocation's human output: each as a
// sentence, with the command that returns to the earlier selection on a line
// of its own, as a command to copy.
func (report *FollowedReport) Render(r *terminal.Renderer) {
	if report == nil || report.written {
		return
	}
	report.written = true
	selections := report.selections()
	if len(selections) == 0 {
		return
	}
	r.Section("Followed selection")
	for _, choice := range selections {
		sentence := choice.Now.Product + " uses " + named(choice.Now) + ", chosen in " + choice.Now.Product + "; AIGW held " + named(choice.Earlier) + "."
		if choice.ForwardingCleared {
			sentence += " Its forwarding ended, because the forwarding service cannot reach the chosen model's upstream."
		}
		if choice.RetriesCleared {
			sentence += " Its retry budgets ended, because they belonged to the earlier provider."
		}
		action, detail := terminal.NextSteps(choice.Returns)
		switch {
		case action != "":
			r.Text(sentence + " To return to it:")
			r.Command(action)
		case detail != "":
			r.Text(sentence + " " + detail)
		default:
			r.Text(sentence + " The configuration no longer holds that model.")
		}
	}
}

// named is a selection as human output names it: the model, and for a client
// that speaks several protocols the protocol's label.
func named(selection adapter.RecordedSelection) string {
	if protocol, known := domain.ProtocolNamed(selection.Protocol); known {
		return selection.Model.String() + " on " + domain.ProtocolLabel(protocol)
	}
	return selection.Model.String()
}
