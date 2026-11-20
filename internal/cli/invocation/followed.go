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

// FollowedDocument is the field that an invocation adds to its command's
// document for the clients' own choices that it followed, and no field when
// it followed none.
func FollowedDocument(follows *synchronization.Follows) []terminal.DocumentField {
	selections := follows.Selections()
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

// RenderFollowed names, once per invocation, the clients' own choices that it
// followed: each as a sentence, with the command that returns to the earlier
// selection on a line of its own, as a command to copy.
func RenderFollowed(r *terminal.Renderer, follows *synchronization.Follows) {
	if follows == nil || follows.Reported {
		return
	}
	follows.Reported = true
	selections := follows.Selections()
	if len(selections) == 0 {
		return
	}
	r.Section("Followed selection")
	for _, choice := range selections {
		sentence := choice.Now.Product + " uses " + named(choice.Now) + ", chosen in " + choice.Now.Product + "; AIGW held " + named(choice.Earlier) + "."
		if choice.ForwardingCleared {
			sentence += " Its forwarding ended, because the client's upstream endpoint changed."
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
