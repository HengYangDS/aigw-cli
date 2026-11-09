package terminal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/platform"
)

// A client file failure whose command then failed to remove the credential
// reader that it installed and the Token that it stored, as a selection's
// transaction joins them, states each failed undo with its cause after the
// configuration's outcome, in text and in JSON, and keeps its own fix.
func TestAFailedUndoIsReportedWithTheClientFileFailure(t *testing.T) {
	home := t.TempDir()
	failed := filepath.Join(home, "b", "config.toml")
	failure := &adapter.ApplyError{
		Client: "Codex", Home: home,
		Failed: &platform.FileError{Path: failed, Err: &os.PathError{Op: "createtemp", Path: failed, Err: os.ErrPermission}},
		Err:    errors.New("internal detail"),
	}
	reader := platform.FailedUndo("removing the credential reader that this command installed",
		&os.PathError{Op: "remove", Path: filepath.Join(home, "reader"), Err: os.ErrPermission})
	token := platform.FailedUndo("removing the Token that this command stored for provider team",
		errors.New("credential rollback also failed: the credential store is locked"))
	projection := presentationProjectionError{restored: true, cause: fmt.Errorf("synchronization failed; configuration was rolled back: %w", errors.Join(failure, reader))}
	err := errors.Join(fmt.Errorf("client selection %w", projection), token)

	problem := ProblemFor(err)
	want := "The configuration was restored. Removing the credential reader that this command installed failed: permission denied. " +
		"Removing the Token that this command stored for provider team failed: credential rollback also failed: the credential store is locked."
	if problem.Impact != want || problem.Fix != "Make ~/b/config.toml and its folder readable and writable, then run the same command again." {
		t.Fatalf("ProblemFor() = %#v\nwant the impact %q", problem, want)
	}
	var document bytes.Buffer
	RenderError(NewRenderer(&document, false), err, true)
	var failureDocument map[string]any
	if json.Unmarshal(document.Bytes(), &failureDocument) != nil || failureDocument["impact"] != want {
		t.Fatalf("failure document = %s", document.String())
	}
	var text bytes.Buffer
	RenderError(NewRenderer(&text, false), err, false)
	for _, sentence := range []string{"Removing the credential reader", "permission denied", "Removing the Token", "the credential store is locked"} {
		if !strings.Contains(text.String(), sentence) {
			t.Fatalf("human output lacks %q:\n%s", sentence, text.String())
		}
	}
	if strings.Contains(text.String(), home) {
		t.Fatalf("human output names a private path:\n%s", text.String())
	}

	// A failure without a file, and one that names no recovery at all, keep
	// their failed undos too, and an undo that succeeded adds nothing.
	for _, joined := range []error{
		errors.Join(presentationProjectionError{restored: false, cause: errors.New("finalization failed")}, reader),
		errors.Join(errors.New("save failed"), reader, platform.FailedUndo("removing the Token", nil)),
	} {
		if impact := ProblemFor(joined).Impact; !strings.HasSuffix(impact, "Removing the credential reader that this command installed failed: permission denied.") ||
			strings.Contains(impact, "removing the Token") {
			t.Fatalf("impact of %v = %q", joined, impact)
		}
	}
	// A problem that its raiser presents states what the raiser reported.
	if impact := ProblemFor(ProblemError("refused", "", "Nothing changed.", "aigw status", reader)).Impact; impact != "Nothing changed." {
		t.Fatalf("a typed problem's impact = %q", impact)
	}
}
