package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/program"
)

// Only a failure of the release sources names a verified local archive, and
// never when the update installs one; an error that states its own fix
// reaches the user unchanged.
func TestUpdateFailureNamesTheFixOfItsCause(t *testing.T) {
	ownFix := errors.New("release source is not configured; install an official release or use `aigw self update --archive <archive> --checksums <checksums>`")
	for _, test := range []struct {
		name, want, problem string
		args                []string
		updater             *fakeUpdater
	}{
		{name: "local archive", args: []string{"self", "update", "--archive", "candidate.tar.gz", "--checksums", "checksums.txt"},
			updater: &fakeUpdater{candidateErr: fmt.Errorf("verify archive: %w", program.ErrReleaseSource)},
			want:    "aigw self status", problem: "Program update did not complete"},
		{name: "program replacement", args: []string{"self", "update"}, updater: &fakeUpdater{updateErr: errors.New("activate update: replaced program did not start")},
			want: "aigw self status", problem: "Program update did not complete"},
		{name: "stated fix", args: []string{"self", "update"}, updater: &fakeUpdater{updateErr: ownFix},
			want: "aigw self update --archive <archive> --checksums <checksums>", problem: ownFix.Error()},
		// A command that a wrapped cause names is not the error's own fix, and
		// the cause stays private.
		{name: "cause names a command", args: []string{"self", "update"},
			updater: &fakeUpdater{updateErr: fmt.Errorf("activate update: %w", errors.New("private cause; run `aigw sync`"))},
			want:    "aigw self status", problem: "Program update did not complete"},
		{name: "stated next action", args: []string{"self", "rollback"}, updater: &fakeUpdater{rollbackErr: movedDirectoryError{}},
			want: movedDirectoryError{}.NextAction(), problem: movedDirectoryError{}.Error()},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			app.Updater = test.updater
			if err := cli.Execute(app, test.args); err == nil {
				t.Fatal("failed update succeeded")
			}
			if !strings.Contains(out.String(), "Problem\n  "+test.problem) || !strings.Contains(out.String(), "Fix\n  "+test.want) ||
				strings.Contains(out.String(), "private cause") {
				t.Fatalf("update failure output:\n%s", out.String())
			}
		})
	}
}

// movedDirectoryError states its own next action, as the refusal of a
// directory at the rollback copy's path does.
type movedDirectoryError struct{}

func (movedDirectoryError) Error() string { return "rollback copy path is a directory" }
func (movedDirectoryError) NextAction() string {
	return "Move the directory at the rollback copy path, then run the command again."
}

// A program that Homebrew installed is updated, rolled back and removed with
// Homebrew, and a portable installation from it names Homebrew's command too.
// No updater runs for it.
func TestHomebrewOwnedProgramNamesHomebrew(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"self", "update"}, want: "brew upgrade --cask aigw"},
		{args: []string{"self", "update", "--dry-run"}, want: "brew upgrade --cask aigw"},
		{args: []string{"self", "rollback"}, want: "Install the earlier release with Homebrew."},
		{args: []string{"self", "rollback", "--dry-run"}, want: "Install the earlier release with Homebrew."},
		{args: []string{"self", "uninstall"}, want: "brew uninstall --cask aigw"},
		{args: []string{"self", "install", "--path", filepath.Join(t.TempDir(), "portable", "aigw")}, want: "brew upgrade --cask aigw"},
	} {
		t.Run(strings.Join(test.args[1:min(len(test.args), 3)], " "), func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			homebrewProgram(t, app)
			updater := &fakeUpdater{}
			app.Updater = updater
			if err := cli.Execute(app, test.args); err == nil {
				t.Fatal("a Homebrew-owned program was changed")
			}
			if !strings.Contains(out.String(), "Homebrew manages this") || !strings.Contains(out.String(), "Fix\n  "+test.want+"\n") {
				t.Fatalf("Homebrew-owned program output:\n%s", out.String())
			}
			if updater.updateCalls+updater.candidateCalls+updater.rollbackCalls+updater.previewCalls != 0 {
				t.Fatalf("a Homebrew-owned program reached the updater: %+v", updater)
			}
		})
	}
}

// A refused rollback of a Homebrew-owned program is one JSON document that
// says what did not change and carries Homebrew's operation as guidance.
func TestHomebrewRollbackRefusalIsOneJSONDocument(t *testing.T) {
	for _, args := range [][]string{{"self", "rollback", "--json"}, {"self", "rollback", "--dry-run", "--json"}} {
		app, out, _, _, _ := testApp(t, "")
		homebrewProgram(t, app)
		app.Updater = &fakeUpdater{}
		err := cli.Execute(app, args)
		var document bytes.Buffer
		const want = `{"schema_version":3,"ok":false,"error":"Homebrew manages this AIGW program","evidence":"A Homebrew receipt owns the program file.",` +
			`"impact":"No program file or credential changed.","next_detail":"Install the earlier release with Homebrew."}`
		if compactErr := json.Compact(&document, out.Bytes()); err == nil || compactErr != nil || document.String() != want {
			t.Fatalf("%v = %v, %v\n%s", args, err, compactErr, out.String())
		}
	}
}
