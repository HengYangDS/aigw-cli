package cli_test

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
)

// selDocument runs a command and decodes the JSON document that it printed,
// whether or not the command failed.
func selDocument(t *testing.T, app *cli.App, output *bytes.Buffer, args ...string) (map[string]any, error) {
	t.Helper()
	output.Reset()
	err := cli.Execute(app, args)
	var document map[string]any
	if decode := json.Unmarshal(output.Bytes(), &document); decode != nil {
		t.Logf("%v printed no JSON document (err=%v): %.300s", args, err, output)
	}
	return document, err
}

// Ninth review, probe 6: the stored selection forwards to a local address, the
// operator picks a model of another upstream in Hermes (following it ends the
// forwarding in the working configuration), and then selects explicitly a
// model that is neither the stored one nor reachable through the stored
// upstream. The stored forwarding ends with this command: does its result say
// so? The entry of a followed selection is left out for the client that an
// explicit selection names, so the selection's own document is the only place.
func TestSelectionAnExplicitSelectionOfAnotherUpstreamAfterAChoiceSaysThatForwardingEnded(t *testing.T) {
	t.Parallel()
	for _, selects := range [][]string{
		{"use", "hermes", "dmxapi/claude-opus-5-5", "--protocol", "anthropic", "--json"},
		{"use", "hermes", "ucloud/claude-opus-5-5", "--protocol", "anthropic", "--json"},
	} {
		// The control: the same selection without a choice in between.
		for _, choice := range []bool{false, true} {
			app, output, _, target := shippedHermesAppWith(t, true)
			if err := cli.Execute(app, []string{"client", "edit", hermes.ID, "--via", "http://127.0.0.1:9000"}); err != nil {
				t.Fatalf("forward Hermes: %v\n%s", err, output)
			}
			if choice {
				hermesNativeSelection(t, target, "aigw-ucloud-anthropic", "claude-opus-5-5", nil)
			}
			stored := selection(t, app, hermes.ID).ForwardingEndpoint
			for _, command := range [][]string{append(slices.Clone(selects), "--dry-run"), selects} {
				document, err := selDocument(t, app, output, command...)
				if err != nil {
					t.Fatalf("%v: %v\n%s", command, err, output)
				}
				saved := selection(t, app, hermes.ID).ForwardingEndpoint
				t.Logf("%v choice=%t: stored forwarding %q, after %q, forwarding_cleared=%v, followed_selections=%v", command, choice, stored, saved, document["forwarding_cleared"], document["followed_selections"])
				ends := !slices.Contains(command, "--dry-run") && stored != "" && saved == ""
				if (ends || slices.Contains(command, "--dry-run")) && document["forwarding_cleared"] != true && document["followed_selections"] == nil {
					t.Errorf("%v choice=%t: the stored forwarding %q ends with this selection and neither forwarding_cleared nor a followed selection says so", command, choice, stored)
				}
			}
		}
	}
}

// Ninth review, probe 7: after a choice was recorded without its Token, the
// reports agree on Hermes's model and on that it waits for the Token.
func TestSelectionReportsAgreeAfterARecordOnlyWrite(t *testing.T) {
	t.Parallel()
	app, output, target, chosen := waitingHermes(t)
	if err := selRun(t, app, output, "sync", "--json"); err != nil {
		t.Fatalf("sync: %v\n%s", err, output)
	}
	if stored, behind := selection(t, app, hermes.ID).Model.String(), selHermesBehind(t, target); stored != "ucloud/claude-opus-5-5" || behind != "" || !bytes.Equal(readFile(t, target), chosen) {
		t.Fatalf("the record-only write: S=%s, %s", stored, behind)
	}
	const uses = "ucloud/claude-opus-5-5"
	entry := func(document map[string]any) map[string]any {
		switch clients := document["clients"].(type) {
		case map[string]any:
			found, _ := clients[hermes.ID].(map[string]any)
			return found
		case []any:
			for _, client := range clients {
				if found, _ := client.(map[string]any); found["id"] == hermes.ID {
					return found
				}
			}
		}
		return nil
	}
	status, statusErr := selDocument(t, app, output, "status", "--json")
	check, checkErr := selDocument(t, app, output, "check", "hermes", "--depth", "local", "--json")
	preview, previewErr := selDocument(t, app, output, "sync", "--dry-run", "--json")
	list, listErr := selDocument(t, app, output, "client", "list", "--json")
	t.Logf("status (err=%v): %v | next=%v", statusErr, entry(status), status["next_action"])
	t.Logf("check (err=%v): %v | next=%v %v", checkErr, entry(check), check["next_action"], check["next_detail"])
	t.Logf("sync --dry-run (err=%v): selections=%v state=%v next=%v %v files=%v followed=%v", previewErr, preview["selections"], preview["state"], preview["next_action"], preview["next_detail"], preview["client_files"], preview["followed_selections"])
	t.Logf("client list (err=%v): %v", listErr, entry(list))
	for name, model := range map[string]any{
		"status": entry(status)["model"], "check": entry(check)["model"], "client list": entry(list)["model"],
		"sync --dry-run": func() any { selections, _ := preview["selections"].(map[string]any); return selections[hermes.ID] }(),
	} {
		if model != uses {
			t.Errorf("%s names %v for Hermes; the others name %s", name, model, uses)
		}
	}
	for name, document := range map[string]map[string]any{"status": status, "check": check, "sync --dry-run": preview, "client list": list} {
		if document["followed_selections"] != nil {
			t.Errorf("%s still names a followed selection after it was recorded: %v", name, document["followed_selections"])
		}
	}
}

// selRollback is what a rollback over a choice leaves, read back.
type selRollback struct {
	Selections []restoredSelection `json:"selections"`
	Followed   []followed          `json:"followed_selections"`
	NextAction string              `json:"next_action"`
	NextDetail string              `json:"next_detail"`
}

// The models of the rollback table: the one that the configuration stores for
// Hermes, the one picked in Hermes, and a third that neither names.
const selRollbackStored, selRollbackPicked, selRollbackThird = "dmxapi/gpt-6.2-sol", "dmxapi/claude-opus-5-5", "dmxapi/gpt-6.1-terra"

// selRollbackCase is one row of the rollback table: the commands that leave
// the configuration that the rollback restores, and the model that it selects
// for Hermes.
type selRollbackCase struct {
	name     string
	prepare  [][]string
	restores string
}

// selOfHermes keeps the restored selections that name Hermes.
func selOfHermes(entries []restoredSelection) []restoredSelection {
	return slices.DeleteFunc(slices.Clone(entries), func(entry restoredSelection) bool { return entry.Client != hermes.ID })
}

// Ninth review, probe 8: a rollback over a choice in Hermes, with and without
// the Token: to the stored model, to the chosen model, and to a third model.
// S, R and the report against the tables of versions eight and nine.
func TestSelectionARollbackOverAChoiceByTheTable(t *testing.T) {
	t.Parallel()
	const stored, picked, third = selRollbackStored, selRollbackPicked, selRollbackThird
	for _, scenario := range []selRollbackCase{
		{"to the stored model", [][]string{{"provider", "edit", "ucloud", "--label", "Other"}}, stored},
		{"to the chosen model", [][]string{{"use", "hermes", picked, "--protocol", "anthropic"}, {"use", "hermes", stored, "--protocol", "responses"}}, picked},
		{"to a third model", [][]string{{"use", "hermes", third, "--protocol", "responses"}, {"use", "hermes", stored, "--protocol", "responses"}}, third},
	} {
		for _, token := range []bool{true, false} {
			scenario.check(t, token)
		}
	}
}

// check prepares the row's configuration, with the picked model's Token or
// without it, picks that model in Hermes, rolls back over the pick, and checks
// S, R and the report. A preview or a rollback that fails ends the row.
func (scenario selRollbackCase) check(t *testing.T, token bool) {
	app, output, _, target := shippedHermesAppWith(t, true)
	for _, args := range scenario.prepare {
		if err := cli.Execute(app, args); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, output)
		}
	}
	if !token {
		if err := app.Secrets.Delete("dmxapi"); err != nil {
			t.Fatal(err)
		}
	}
	chosen := hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
	record, configuration := readFile(t, target+".aigw-state.json"), readFile(t, app.Config.Path())
	var preview, result selRollback
	if err := runDocument(t, app, output, &preview, "config", "rollback", "--last-change", "--dry-run", "--json"); err != nil {
		t.Errorf("%s token=%t: the preview: %v\n%s", scenario.name, token, err, output)
		return
	}
	if !bytes.Equal(readFile(t, target), chosen) || !bytes.Equal(readFile(t, target+".aigw-state.json"), record) || !bytes.Equal(readFile(t, app.Config.Path()), configuration) {
		t.Errorf("%s token=%t: the preview wrote", scenario.name, token)
	}
	if err := runDocument(t, app, output, &result, "config", "rollback", "--last-change", "--json"); err != nil {
		t.Errorf("%s token=%t: %v\n%s", scenario.name, token, err, output)
		return
	}
	saved, behind := selection(t, app, hermes.ID).Model.String(), selHermesBehind(t, target)
	provider, model := hermesProjectedSelection(t, target)
	t.Logf("%s token=%t: S=%s F=%s %s behind=%q selections=%+v followed=%+v next=%q %q", scenario.name, token, saved, provider, model, behind, selOfHermes(result.Selections), result.Followed, result.NextAction, result.NextDetail)
	if saved != scenario.restores {
		t.Errorf("%s token=%t: the rollback saved %s, the restored configuration selects %s", scenario.name, token, saved, scenario.restores)
	}
	if behind != "" {
		t.Errorf("%s token=%t: after the rollback %s", scenario.name, token, behind)
	}
	scenario.checkReport(t, token, preview, result, target, chosen)
	if scenario.restores != selRollbackStored && !token {
		// A later pick of the model that the record named before the
		// rollback is a choice.
		hermesNativeSelection(t, target, "aigw-dmxapi-responses", "gpt-6.2-sol", nil)
		if got := followedIn(t, app, output, "status", "--json"); len(got) != 1 || got[0].Model != selRollbackStored {
			t.Errorf("%s token=%t: after a pick of the model that the record named before, status follows %+v", scenario.name, token, got)
		}
	}
}

// checkReport checks what the rollback's preview and its run name for Hermes.
// Where the rollback restores a selection instead of following the pick,
// Hermes's file at target, which held chosen, is written exactly when the
// Token is there.
func (scenario selRollbackCase) checkReport(t *testing.T, token bool, preview, result selRollback, target string, chosen []byte) {
	if !slices.Equal(selOfHermes(preview.Selections), selOfHermes(result.Selections)) || len(preview.Followed) != len(result.Followed) {
		t.Errorf("%s token=%t: the preview names %+v / %+v, the run %+v / %+v", scenario.name, token, selOfHermes(preview.Selections), preview.Followed, selOfHermes(result.Selections), result.Followed)
	}
	switch scenario.restores {
	case selRollbackPicked:
		// The restored selection is the choice: followed, not restored.
		if len(selOfHermes(result.Selections)) != 0 || len(result.Followed) != 1 || result.Followed[0].Model != selRollbackPicked {
			t.Errorf("%s token=%t: names restored %+v and followed %+v; want the choice named as followed alone", scenario.name, token, selOfHermes(result.Selections), result.Followed)
		}
	default:
		want := []restoredSelection{{Client: hermes.ID, Model: scenario.restores, Replaces: selRollbackPicked}}
		if !slices.Equal(selOfHermes(result.Selections), want) || len(result.Followed) != 0 {
			t.Errorf("%s token=%t: names restored %+v and followed %+v; want %+v and none", scenario.name, token, selOfHermes(result.Selections), result.Followed, want)
		}
		if written := !bytes.Equal(readFile(t, target), chosen); written != token {
			t.Errorf("%s token=%t: Hermes's file written = %t", scenario.name, token, written)
		}
	}
}

// selStore counts what a command asks the credential store.
type selStore struct {
	inner interface {
		Get(account string) (string, error)
		Set(account, value string) error
		Delete(account string) error
		Exists(account string) (bool, error)
	}
	asked *[]string
}

func (store selStore) Get(account string) (string, error) {
	*store.asked = append(*store.asked, "get "+account)
	return store.inner.Get(account)
}

func (store selStore) Set(account, value string) error { return store.inner.Set(account, value) }

func (store selStore) Delete(account string) error { return store.inner.Delete(account) }

func (store selStore) Exists(account string) (bool, error) {
	*store.asked = append(*store.asked, "exists "+account)
	return store.inner.Exists(account)
}

// selDiscovery counts the discoveries that a command runs.
type selDiscovery struct {
	inner discovery.Discoverer
	calls *int
}

func (counting selDiscovery) Discover(clients ...string) discovery.Result {
	*counting.calls++
	return counting.inner.Discover(clients...)
}

// Ninth review, probe 5: deciding and writing a record alone asks the
// credential store nothing, and while no record is behind a command discovers
// no more than it does on a converged client.
func TestSelectionARecordAloneAsksTheCredentialStoreNothing(t *testing.T) {
	t.Parallel()
	counted := func(app *cli.App, output *bytes.Buffer, args ...string) ([]string, int) {
		var asked []string
		var discoveries int
		secrets, discoverer := app.Secrets, app.Discovery
		app.Secrets, app.Discovery = selStore{inner: secrets, asked: &asked}, selDiscovery{inner: discoverer, calls: &discoveries}
		defer func() { app.Secrets, app.Discovery = secrets, discoverer }()
		if err := selRun(t, app, output, args...); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, output)
		}
		slices.Sort(asked)
		return asked, discoveries
	}
	// Hermes: a command about Codex, while Hermes's record is behind or not.
	for _, token := range []bool{false, true} {
		var questions [2][]string
		for index, behind := range []bool{false, true} {
			app, output, target := pendingHermes(t)
			if behind {
				hermesNativeSelection(t, target, "aigw-dmxapi-responses", "gpt-6.2-sol", nil)
			}
			if token {
				if err := app.Secrets.Set("dmxapi", "fixture-token"); err != nil {
					t.Fatal(err)
				}
			}
			questions[index], _ = counted(app, output, "client", "edit", "codex", "--request-retries", "2", "--json")
			if behind && selHermesBehind(t, target) != "" {
				t.Fatalf("token=%t: the command did not bring the record up", token)
			}
		}
		t.Logf("Hermes token=%t: without a record behind %v; with one %v", token, questions[0], questions[1])
		if !slices.Equal(questions[0], questions[1]) {
			t.Errorf("Hermes token=%t: bringing a record up changed what the command asks the credential store: %v, without a record behind %v", token, questions[1], questions[0])
		}
	}
	// Codex: a command about another provider, on a converged client, on a
	// pending selection with no record behind, and with a record behind.
	converged, coutput, _ := recCodexFiles(t, 2)
	convergedAsked, convergedDiscoveries := counted(converged, coutput, "provider", "edit", "spare", "--label", "Other", "--json")
	pending, poutput, _ := selPendingCodex(t, 2)
	pendingAsked, pendingDiscoveries := counted(pending, poutput, "provider", "edit", "spare", "--label", "Other", "--json")
	behind, boutput, targets := selPendingCodex(t, 2)
	recPick(t, targets[0], "gpt-sol")
	behindAsked, behindDiscoveries := counted(behind, boutput, "provider", "edit", "spare", "--label", "Other", "--json")
	t.Logf("Codex converged: asks %v, discovers %d; pending, no record behind: asks %v, discovers %d; a record behind: asks %v, discovers %d (S=%s)",
		convergedAsked, convergedDiscoveries, pendingAsked, pendingDiscoveries, behindAsked, behindDiscoveries, selection(t, behind, codex.ID).Model)
	if pendingDiscoveries != convergedDiscoveries {
		t.Errorf("with no record behind, the command ran %d discoveries; on a converged client %d", pendingDiscoveries, convergedDiscoveries)
	}
	if !slices.Equal(pendingAsked, behindAsked) {
		t.Errorf("bringing a Codex record up changed what the command asks the credential store: %v, without a record behind %v", behindAsked, pendingAsked)
	}
}
