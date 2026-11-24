package cli_test

import (
	"bytes"
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
)

// selHermesBehind says how AIGW's record of Hermes's file stands against the
// file, by the adapter's own observation: "" when the record names what the
// file shows.
func selHermesBehind(t *testing.T, target string) string {
	t.Helper()
	shown, changed, err := hermes.ObserveSelection(target)
	switch {
	case err != nil:
		return "the file is refused against the record: " + err.Error()
	case changed:
		return "the record is behind " + shown.Provider + " " + shown.Model
	}
	return ""
}

// selCodexBehind names the Codex files whose record does not name the model
// that the file selects.
func selCodexBehind(t *testing.T, targets []string) string {
	t.Helper()
	var behind []string
	for index, target := range targets {
		if recorded := recCodexRecorded(t, target); !strings.Contains(recCodexModel(t, target), `"`+recorded+`"`) {
			behind = append(behind, "file"+string(rune('A'+index))+" F=["+recCodexModel(t, target)+"] R="+recorded)
		}
	}
	return strings.Join(behind, "; ")
}

func selRun(t *testing.T, app *cli.App, output *bytes.Buffer, args ...string) error {
	t.Helper()
	output.Reset()
	return cli.Execute(app, args)
}

// Ninth review, probe 2 (the rule's third row), Hermes: the file shows the
// stored selection and the record an earlier model. Every committing command,
// naming Hermes or not, with and without the Token, leaves the record at what
// the file shows.
func TestSelectionEveryCommitBringsUpARecordBehindTheStoredSelectionHermes(t *testing.T) {
	t.Parallel()
	manifest := filepath.Join("..", "..", "..", "manifests", "team.toml")
	for _, command := range [][]string{
		{"provider", "edit", "ucloud", "--label", "Other", "--json"},
		{"model", "remove", "ucloud/gpt-6-luna", "--json"},
		{"model", "add", "ucloud/gpt-unlisted-probe", "--unlisted", "--protocol", "responses", "--json"},
		{"client", "edit", "codex", "--request-retries", "2", "--json"},
		{"client", "edit", "hermes", "--via", "http://127.0.0.1:8792", "--json"},
		{"use", "claude", "ucloud/claude-opus-5-5", "--json"},
		{"setup", "--from", manifest},
		{"config", "rollback", "--last-change", "--json"},
		{"sync", "--json"},
	} {
		for _, token := range []bool{false, true} {
			app, output, target := pendingHermes(t)
			hermesNativeSelection(t, target, "aigw-dmxapi-responses", "gpt-6.2-sol", nil)
			if token {
				if err := app.Secrets.Set("dmxapi", "fixture-token"); err != nil {
					t.Fatal(err)
				}
			}
			if selHermesBehind(t, target) == "" {
				t.Fatal("the fixture's record is not behind the file")
			}
			err := selRun(t, app, output, command...)
			behind := selHermesBehind(t, target)
			provider, model := hermesProjectedSelection(t, target)
			t.Logf("%v token=%t: err=%v S=%s F=%s %s behind=%q", command, token, err, selection(t, app, hermes.ID).Model, provider, model, behind)
			if err != nil {
				t.Errorf("%v token=%t: %v\n%s", command, token, err, output)
				continue
			}
			if behind != "" {
				t.Errorf("%v token=%t committed and %s", command, token, behind)
			}
		}
	}
}

// selPendingCodex leaves Codex with a stored selection that waits for its
// Token and no choice in play: S = gateway/gpt-sol, every file and record
// show gpt-terra, and the gateway has no Token.
func selPendingCodex(t *testing.T, count int) (*cli.App, *bytes.Buffer, []string) {
	t.Helper()
	app, output, targets := recCodexFiles(t, count)
	if err := cli.Execute(app, []string{"use", "codex", "gateway/gpt-terra"}); err != nil {
		t.Fatalf("select: %v\n%s", err, output)
	}
	if err := app.Secrets.Delete("gateway"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"config", "rollback", "--last-change", "--json"}); err != nil {
		t.Fatalf("rollback: %v\n%s", err, output)
	}
	if got := selection(t, app, codex.ID).Model.String(); got != "gateway/gpt-sol" || selCodexBehind(t, targets) != "" {
		t.Fatalf("the fixture: S=%s, %s", got, selCodexBehind(t, targets))
	}
	return app, output, targets
}

// Ninth review, probe 2, Codex with one file and with two: file A shows the
// stored selection and its record an earlier model.
func TestSelectionEveryCommitBringsUpARecordBehindTheStoredSelectionCodex(t *testing.T) {
	t.Parallel()
	for _, command := range [][]string{
		{"provider", "edit", "spare", "--label", "Other", "--json"},
		{"model", "remove", "gateway/gpt-luna", "--json"},
		{"model", "remove", "spare/gpt-spare", "--json"},
		{"client", "edit", "codex", "--request-retries", "2", "--json"},
		{"config", "rollback", "--last-change", "--json"},
		{"sync", "--json"},
	} {
		for _, count := range []int{1, 2} {
			for _, token := range []bool{false, true} {
				app, output, targets := selPendingCodex(t, count)
				recPick(t, targets[0], "gpt-sol")
				if token {
					if err := app.Secrets.Set("gateway", "fixture-token"); err != nil {
						t.Fatal(err)
					}
				}
				err := selRun(t, app, output, command...)
				behind := selCodexBehind(t, targets)
				t.Logf("%v files=%d token=%t: err=%v S=%s behind=%q", command, count, token, err, selection(t, app, codex.ID).Model, behind)
				if err != nil {
					t.Errorf("%v files=%d token=%t: %v\n%s", command, count, token, err, output)
					continue
				}
				if behind != "" {
					t.Errorf("%v files=%d token=%t committed and left %s", command, count, token, behind)
				}
			}
		}
	}
}

// Ninth review, probe 4: while a record is behind the stored selection, a
// preview and a report write nothing: not the record, not the file, not the
// stored configuration.
func TestSelectionPreviewsAndReportsWriteNothingWhileARecordIsBehind(t *testing.T) {
	t.Parallel()
	manifest := filepath.Join("..", "..", "..", "manifests", "team.toml")
	app, output, target := pendingHermes(t)
	hermesNativeSelection(t, target, "aigw-dmxapi-responses", "gpt-6.2-sol", nil)
	before := ownedFileSnapshot(t, filepath.Dir(target), app.Config.Path())
	for _, command := range [][]string{
		nil,
		{"status", "--json"},
		{"check", "--depth", "local"},
		{"check", "hermes", "--depth", "local", "--json"},
		{"client", "list", "--json"},
		{"model", "list", "--json"},
		{"provider", "list", "--json"},
		{"sync", "--dry-run", "--json"},
		{"provider", "edit", "ucloud", "--label", "Other", "--dry-run", "--json"},
		{"model", "remove", "ucloud/gpt-6-luna", "--dry-run", "--json"},
		{"model", "add", "ucloud/gpt-unlisted-probe", "--unlisted", "--protocol", "responses", "--dry-run", "--json"},
		{"client", "edit", "codex", "--request-retries", "2", "--dry-run", "--json"},
		{"client", "edit", "hermes", "--via", "http://127.0.0.1:8792", "--dry-run", "--json"},
		{"client", "disable", "hermes", "--dry-run", "--json"},
		{"use", "claude", "ucloud/claude-opus-5-5", "--dry-run", "--json"},
		{"use", "hermes", "ucloud/claude-opus-5-5", "--protocol", "anthropic", "--dry-run", "--json"},
		{"setup", "--from", manifest, "--dry-run"},
		{"config", "rollback", "--last-change", "--dry-run", "--json"},
		{"self", "uninstall", "--dry-run", "--json"},
	} {
		err := selRun(t, app, output, command...)
		if after := ownedFileSnapshot(t, filepath.Dir(target), app.Config.Path()); !equalSnapshots(after, before) {
			t.Errorf("%v (err=%v) wrote Hermes's file, AIGW's record of it or the stored configuration", command, err)
			before = after
		}
	}
	// Codex, two files, file A behind.
	capp, coutput, targets := selPendingCodex(t, 2)
	recPick(t, targets[0], "gpt-sol")
	snapshot := func() map[string]string {
		both := ownedFileSnapshot(t, filepath.Dir(targets[0]), capp.Config.Path())
		maps.Copy(both, ownedFileSnapshot(t, filepath.Dir(targets[1]), capp.Config.Path()))
		return both
	}
	cbefore := snapshot()
	for _, command := range [][]string{
		{"status", "--json"},
		{"check", "codex", "--depth", "local", "--json"},
		{"sync", "--dry-run", "--json"},
		{"provider", "edit", "spare", "--label", "Other", "--dry-run", "--json"},
		{"model", "remove", "gateway/gpt-luna", "--dry-run", "--json"},
		{"client", "edit", "codex", "--request-retries", "2", "--dry-run", "--json"},
		{"client", "disable", "codex", "--dry-run", "--json"},
		{"use", "codex", "gateway/gpt-luna", "--dry-run", "--json"},
		{"config", "rollback", "--last-change", "--dry-run", "--json"},
		{"self", "uninstall", "--dry-run", "--json"},
	} {
		err := selRun(t, capp, coutput, command...)
		if after := snapshot(); !equalSnapshots(after, cbefore) {
			t.Errorf("%v (err=%v) wrote a Codex file, a record or the stored configuration", command, err)
			cbefore = after
		}
	}
}

// Ninth review, probe 1b: a choice (the file differs from the record and from
// the stored selection) whose provider has no Token, and a command that does
// not name the client: the stored selection and the record move to the file
// together.
func TestSelectionACommandAboutSomethingElseRecordsAChoiceWithoutAToken(t *testing.T) {
	t.Parallel()
	for _, command := range [][]string{
		{"client", "edit", "codex", "--request-retries", "2", "--json"},
		{"use", "claude", "dmxapi/claude-sonnet-5-5", "--json"},
		{"model", "remove", "dmxapi/gpt-6-luna", "--json"},
		{"provider", "edit", "dmxapi", "--label", "Other", "--json"},
	} {
		app, output, target, chosen := waitingHermes(t)
		err := selRun(t, app, output, command...)
		stored, behind := selection(t, app, hermes.ID).Model.String(), selHermesBehind(t, target)
		t.Logf("%v: err=%v S=%s behind=%q fileKept=%t", command, err, stored, behind, bytes.Equal(readFile(t, target), chosen))
		if err != nil {
			t.Errorf("%v: %v\n%s", command, err, output)
			continue
		}
		if stored != "ucloud/claude-opus-5-5" || behind != "" || !bytes.Equal(readFile(t, target), chosen) {
			t.Errorf("%v: S=%s, %s, file kept=%t; want the choice recorded with its record and the file left", command, stored, behind, bytes.Equal(readFile(t, target), chosen))
		}
	}
}

// Ninth review, probe 4b: a record-only write that fails on other paths than
// `provider edit`: the command fails and leaves the record and the stored
// configuration as they were.
func TestSelectionAFailedRecordOnOtherPathsLeavesNothing(t *testing.T) {
	t.Parallel()
	for _, command := range [][]string{
		{"sync", "--json"},
		{"client", "edit", "codex", "--request-retries", "2", "--json"},
		{"model", "remove", "gateway/gpt-sol", "--json"},
		{"model", "remove", "spare/gpt-spare", "--json"},
		{"config", "rollback", "--last-change", "--json"},
	} {
		app, output, targets := recCodexFiles(t, 1)
		if err := cli.Execute(app, []string{"provider", "edit", "spare", "--label", "Changed"}); err != nil {
			t.Fatalf("a change for the rollback: %v\n%s", err, output)
		}
		if err := app.Secrets.Delete("gateway"); err != nil {
			t.Fatal(err)
		}
		recPick(t, targets[0], "gpt-terra")
		folder := filepath.Dir(targets[0])
		before := ownedFileSnapshot(t, folder, app.Config.Path())
		release := blockFolder(t, folder)
		err := selRun(t, app, output, command...)
		release()
		after := ownedFileSnapshot(t, folder, app.Config.Path())
		t.Logf("%v: err=%v S=%s R=%s", command, err, selection(t, app, codex.ID).Model, recCodexRecorded(t, targets[0]))
		if err == nil {
			t.Errorf("%v succeeded although AIGW's record cannot be written; S=%s R=%s", command, selection(t, app, codex.ID).Model, recCodexRecorded(t, targets[0]))
		}
		if !equalSnapshots(after, before) {
			t.Errorf("%v (err=%v) left the record, the file or the stored configuration changed", command, err)
		}
	}
}

// Ninth review, probe 1d: Codex, a choice in file A (the file differs from the
// record and from the stored selection), and a command that does not name
// Codex, with one file and with two, with and without the Token: the stored
// selection moves to the choice together with the record of every file that
// shows it.
func TestSelectionACommandAboutSomethingElseRecordsACodexChoice(t *testing.T) {
	t.Parallel()
	for _, command := range [][]string{
		{"provider", "edit", "spare", "--label", "Other", "--json"},
		{"model", "remove", "spare/gpt-spare", "--json"},
		{"model", "remove", "gateway/gpt-luna", "--json"},
		{"model", "remove", "gateway/gpt-sol", "--json"},
	} {
		for _, count := range []int{1, 2} {
			for _, token := range []bool{false, true} {
				app, output, targets := recCodexFiles(t, count)
				if !token {
					if err := app.Secrets.Delete("gateway"); err != nil {
						t.Fatal(err)
					}
				}
				recPick(t, targets[0], "gpt-terra")
				err := selRun(t, app, output, command...)
				stored, behind := selection(t, app, codex.ID).Model.String(), selCodexBehind(t, targets)
				t.Logf("%v files=%d token=%t: err=%v S=%s behind=%q", command, count, token, err, stored, behind)
				if err != nil || stored != "gateway/gpt-terra" || behind != "" {
					t.Errorf("%v files=%d token=%t: err=%v S=%s %s; want the choice saved with its record", command, count, token, err, stored, behind)
				}
			}
		}
	}
}
