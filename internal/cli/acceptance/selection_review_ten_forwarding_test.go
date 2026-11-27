package cli_test

import (
	"fmt"
	"slices"
	"testing"

	"aigw-cli/internal/adapter/hermes"
)

// This file holds the regression tests for what the tenth review of version
// ten found for Hermes (2026-11-22), both about a stored forwarding that a
// choice in Hermes ends: the chosen model is on the Anthropic protocol, which
// the forwarding service does not relay. Each test states its sequence with
// three values: the
// model that Hermes's file selects, the model that AIGW's record of the file
// names, and the selection that AIGW has stored.

// The person ends a forwarding with `--direct` after a choice in Hermes had
// already ended it, with the Token of the stored selection's provider and
// without it.
//
// Before: dmxapi/gpt-6.2-sol on the Responses protocol is stored and forwards
// through a local forwarding service. Hermes's file selects that model and
// holds the service's address, and the record names the model. The person
// picks ucloud's claude-opus-5-5 on the Anthropic protocol in Hermes: the
// file selects it, and the record and the stored selection stay.
//
// `aigw client edit hermes --direct --json` succeeds. It reports
// `changed: true` and the followed selection, whose forwarding the choice
// ended (`forwarding_cleared`). After it ucloud/claude-opus-5-5 is stored
// without a forwarding, the file still selects it, and the record names it.
//
// Then the person picks dmxapi/gpt-6.2-sol again in Hermes and `aigw sync`
// runs with every Token: that model is stored again, without a forwarding,
// and Hermes's file does not hold the forwarding address. The person can pick
// only a model whose entry Hermes's file lists (returnTo).
//
// The tenth review found that the edit answered "Nothing to change" and saved
// nothing, because it compared its result with a configuration in which the
// choice had already ended the forwarding. The stored selection still
// forwarded, so Hermes forwarded again after the person's return to it.
func TestReviewTenEndingAForwardingAfterAChoiceIsSaved(t *testing.T) {
	t.Parallel()
	for _, token := range []bool{true, false} {
		t.Run(fmt.Sprintf("token=%t", token), func(t *testing.T) {
			world := forwardedThenChose(t, "ucloud")
			world.setToken(t, token)
			direct := runIn(t, world, "client", "edit", hermes.ID, "--direct", "--json")
			if direct.err != nil {
				t.Fatalf("client edit hermes --direct: %v\n%s", direct.err, direct.output)
			}
			earlier, chosen := direct.before.stored, direct.before.shows[0]
			if saved := direct.after.binding; saved.ForwardingRoot != "" {
				t.Errorf("`aigw client edit hermes --direct` succeeded, and the stored selection %s still forwards to %s", saved.Model, saved.ForwardingRoot)
			}
			if !settledOn(direct.after, chosen) {
				t.Errorf("an edit of Hermes's own setting did not save the chosen %s with its record:\n  before: %s\n  after:  %s",
					chosen, shownLine(direct.before), shownLine(direct.after))
			}
			var report struct {
				Changed  bool       `json:"changed"`
				Followed []followed `json:"followed_selections"`
			}
			direct.decode(t, &report)
			named, isNamed := followedOf(report.Followed, hermes.ID)
			if !report.Changed || !isNamed || !named.ForwardingCleared || named.Model != direct.after.binding.Model.String() {
				t.Errorf("the edit reports changed=%t and follows %+v; want a change, and the saved choice as followed with its forwarding ended", report.Changed, report.Followed)
			}

			returnTo(t, world, earlier, token)
			world.setToken(t, true)
			after := runIn(t, world, "sync", "--json")
			if after.err != nil {
				t.Fatalf("sync after the return to %s: %v\n%s", earlier, after.err, after.output)
			}
			if after.after.stored != earlier {
				t.Errorf("the person returned to %s in Hermes, and the synchronization left: %s", earlier, shownLine(after.after))
			}
			if saved := after.after.binding; saved.ForwardingRoot != "" {
				t.Errorf("the person removed the forwarding and returned to %s in Hermes, and Hermes forwards to %s again", saved.Model, saved.ForwardingRoot)
			}
			if forwardsInFile(t, world) {
				t.Errorf("Hermes's file still holds the forwarding address after `--direct` and a synchronization")
			}
		})
	}
}

// returnTo lets the person pick shown again in Hermes. With every Token
// Hermes's file must list the entry of shown. A command that ran without the
// Token of that entry's provider wrote Hermes's file without the provider's
// entries: there the Token arrives first, and a synchronization lists the
// entry before the person picks.
func returnTo(t *testing.T, world selectionWorld, shown string, withEveryToken bool) {
	t.Helper()
	if pickShown(t, world, shown) {
		return
	}
	if withEveryToken {
		t.Fatalf("with every Token Hermes's file does not list the entry of %s", shown)
	}
	world.setToken(t, true)
	if arrived := runIn(t, world, "sync", "--json"); arrived.err != nil {
		t.Fatalf("sync after the Token arrived: %v\n%s", arrived.err, arrived.output)
	}
	if !pickShown(t, world, shown) {
		t.Fatalf("after the Token and a synchronization Hermes's file does not list the entry of %s", shown)
	}
}

// The local check after a choice in Hermes that ends a stored forwarding.
//
// Before: dmxapi/gpt-6.2-sol on the Responses protocol is stored and forwards
// through a local forwarding service. Hermes's file selects that model and
// holds the service's address, and the record names the model. The person
// picks claude-opus-5-5 on the Anthropic protocol in Hermes, of ucloud
// (another provider) or of dmxapi (the same provider): the file selects it
// and still holds the address, and the record and the stored selection stay.
//
// `aigw check hermes --depth local --json` does not pass and changes no file.
// Hermes is `invalid`, its check `hermes:files` failed, and the next step is
// `aigw sync`. The document names the followed selection, whose forwarding
// the choice ended (`forwarding_cleared`).
//
// `aigw sync` then succeeds: the chosen model is stored without a forwarding,
// the file still selects it and no longer holds the address, and the record
// names it. The same check passes after it.
//
// The tenth review found that the specification promised a passing local
// check after every choice, while this check fails until a synchronization
// has run. The specification is narrowed to what the check does, and this
// test holds the check to it.
func TestReviewTenTheLocalCheckAfterAChoiceThatEndsAForwarding(t *testing.T) {
	t.Parallel()
	check := []string{"check", hermes.ID, "--depth", "local", "--json"}
	for _, account := range []string{"ucloud", "dmxapi"} {
		t.Run(account, func(t *testing.T) {
			world := forwardedThenChose(t, account)
			first := runIn(t, world, check...)
			var report followingCheck
			first.decode(t, &report)
			if first.err == nil || report.OK {
				t.Errorf("the local check passes while Hermes's file holds the forwarding address beside the choice: err=%v, ok=%t", first.err, report.OK)
			}
			if len(first.changed) != 0 {
				t.Errorf("the check changed files: %v", first.changed)
			}
			entry, _ := report.client(hermes.ID)
			filesFailed := slices.ContainsFunc(entry.Checks, func(local checkedLocal) bool {
				return local.Name == hermes.ID+":files" && local.State == "failed"
			})
			if entry.State != "invalid" || !filesFailed {
				t.Errorf("the check's entry of Hermes is %q with the checks %+v; want invalid, with hermes:files failed", entry.State, entry.Checks)
			}
			if !wantNext(entry.NextAction, entry.NextDetail, "aigw sync") || !wantNext(report.NextAction, report.NextDetail, "aigw sync") {
				t.Errorf("the check's next step is not `aigw sync`: for Hermes %q %q, for the document %q %q",
					entry.NextAction, entry.NextDetail, report.NextAction, report.NextDetail)
			}
			if named, isNamed := followedOf(report.Followed, hermes.ID); !isNamed || !named.ForwardingCleared || named.Model != account+"/claude-opus-5-5" {
				t.Errorf("the check follows %+v; want the choice, with its forwarding ended", report.Followed)
			}

			chosen := first.before.shows[0]
			sync := runIn(t, world, "sync", "--json")
			if sync.err != nil {
				t.Fatalf("sync after the check: %v\n%s", sync.err, sync.output)
			}
			if saved := sync.after.binding; !settledOn(sync.after, chosen) || saved.ForwardingRoot != "" {
				t.Errorf("the synchronization did not save the chosen %s without a forwarding (it forwards to %q): %s", chosen, saved.ForwardingRoot, shownLine(sync.after))
			}
			if forwardsInFile(t, world) {
				t.Errorf("Hermes's file still holds the forwarding address after the synchronization")
			}
			second := runIn(t, world, check...)
			var passed followingCheck
			second.decode(t, &passed)
			if second.err != nil || !passed.OK {
				t.Errorf("the local check after the synchronization: err=%v, ok=%t\n%s", second.err, passed.OK, second.output)
			}
		})
	}
}

// followingCheck is the document of `aigw check` with the selections that it
// follows.
type followingCheck struct {
	checkReport
	Followed []followed `json:"followed_selections"`
}
