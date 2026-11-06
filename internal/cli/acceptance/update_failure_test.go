package cli_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/program"
)

// Only a failure of the release sources names a verified local archive, and
// never when the update installs one; an error that states its own fix
// reaches the user unchanged.
func TestUpdateFailureNamesTheFixOfItsCause(t *testing.T) {
	ownFix := errors.New("release source is not configured; install an official release or use `aigw update --candidate <archive> --checksums <checksums>`")
	for _, test := range []struct {
		name, want, problem string
		args                []string
		updater             *fakeUpdater
	}{
		{name: "local archive", args: []string{"update", "--candidate", "candidate.tar.gz", "--checksums", "checksums.txt"},
			updater: &fakeUpdater{candidateErr: fmt.Errorf("verify archive: %w", program.ErrReleaseSource)},
			want:    "aigw installation", problem: "Program update did not complete"},
		{name: "program replacement", args: []string{"update"}, updater: &fakeUpdater{updateErr: errors.New("activate update: replaced program did not start")},
			want: "aigw installation", problem: "Program update did not complete"},
		{name: "stated fix", args: []string{"update"}, updater: &fakeUpdater{updateErr: ownFix},
			want: "aigw update --candidate <archive> --checksums <checksums>", problem: ownFix.Error()},
		{name: "stated next action", args: []string{"update", "--rollback"}, updater: &fakeUpdater{rollbackErr: movedDirectoryError{}},
			want: movedDirectoryError{}.NextAction(), problem: movedDirectoryError{}.Error()},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			app.Updater = test.updater
			if err := cli.Execute(app, test.args); err == nil {
				t.Fatal("failed update succeeded")
			}
			if !strings.Contains(out.String(), "Problem\n  "+test.problem) || !strings.Contains(out.String(), "Fix\n  "+test.want) {
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

// A program that Homebrew installed is updated and removed with Homebrew.
func TestHomebrewOwnedProgramNamesHomebrew(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"update"}, want: "brew upgrade --cask aigw"},
		{args: []string{"update", "--dry-run"}, want: "brew upgrade --cask aigw"},
		{args: []string{"uninstall"}, want: "brew uninstall --cask aigw"},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			homebrewProgram(t, app)
			app.Updater = &fakeUpdater{}
			if err := cli.Execute(app, test.args); err == nil {
				t.Fatal("a Homebrew-owned program was changed")
			}
			if !strings.Contains(out.String(), "Homebrew manages this AIGW program") || !strings.Contains(out.String(), "Fix\n  "+test.want) ||
				strings.Contains(out.String(), "aigw update --rollback") {
				t.Fatalf("Homebrew-owned program output:\n%s", out.String())
			}
		})
	}
}
