package terminal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/platform"
)

// A failure of a client file names the client, the file relative to the home
// directory and its operating-system error, says what became of each file
// that the attempt had written and whether the configuration was restored,
// and in JSON says the same. A permission error asks for a writable file and
// folder; any other failure names aigw check. A file outside the home
// directory is not named.
func TestAClientFileFailureNamesEachFile(t *testing.T) {
	// Two directories of one test lie side by side, so the second is outside
	// the first on every platform.
	home, elsewhere := t.TempDir(), t.TempDir()
	failed := filepath.Join(home, "b", "config.toml")
	restoreFailure := &platform.FileError{Path: filepath.Join(home, "c", "config.toml"), Err: os.ErrPermission}
	failure := &adapter.ApplyError{
		Client: "Codex", Home: home,
		Failed: &platform.FileError{Path: failed, Err: &os.PathError{Op: "createtemp", Path: failed, Err: os.ErrPermission}},
		Restored: platform.Restoration{
			{Path: filepath.Join(home, "a", "config.toml.aigw-state.json")},
			{Path: filepath.Join(home, "a", "config.toml")},
			{Path: filepath.Join(home, "c", "config.toml"), Err: restoreFailure},
			{Path: filepath.Join(elsewhere, "config.toml")},
		},
		Err: errors.New("internal detail"),
	}
	err := fmt.Errorf("client edit %w", presentationProjectionError{restored: true, cause: failure})
	problem := ProblemFor(err)
	want := Problem{
		Title: "Codex could not write ~/b/config.toml: permission denied",
		Evidence: "~/a/config.toml.aigw-state.json was restored. ~/a/config.toml was restored. " +
			"~/c/config.toml could not be restored: permission denied. A file outside the home directory was restored. ~/b/config.toml is unchanged.",
		Impact: "The configuration was restored.",
		Fix:    "Make ~/b/config.toml and its folder writable, then run the same command again.",
	}
	if problem != want {
		t.Fatalf("ProblemFor() = %#v\nwant %#v", problem, want)
	}
	var out bytes.Buffer
	RenderError(NewRenderer(&out, false), err, true)
	var document map[string]any
	if json.Unmarshal(out.Bytes(), &document) != nil || document["error"] != want.Title || document["evidence"] != want.Evidence ||
		document["impact"] != want.Impact || document["next_detail"] != want.Fix || bytes.Contains(out.Bytes(), []byte(filepath.Base(home))) {
		t.Fatalf("failure document = %s", out.String())
	}

	failure.Failed = &platform.FileError{Path: filepath.Join(elsewhere, "b.toml"), Changed: true, Err: errors.New("preimage changed")}
	failure.Restored = nil
	problem = ProblemFor(presentationProjectionError{restored: false, cause: failure})
	if problem.Title != "Codex could not write a file outside the home directory: it changed after AIGW read it" ||
		problem.Evidence != "A file outside the home directory keeps the change made outside AIGW." ||
		problem.Impact != "Restoring the configuration failed too, so it may still hold the change." || problem.Fix != "aigw check" {
		t.Fatalf("a changed file outside home = %#v", problem)
	}
}

// A failure that names no file keeps the generic report, whose fix is now
// aigw check.
func TestAClientFilesFailureWithoutAFileNamesCheck(t *testing.T) {
	problem := ProblemFor(presentationProjectionError{restored: true, cause: errors.New("entrypoint finalization failed")})
	if problem.Title != "Updating the client files failed" || problem.Fix != "aigw check" || problem.Impact == "" {
		t.Fatalf("ProblemFor() = %#v", problem)
	}
	home := t.TempDir()
	changed := filepath.Join(home, "x")
	outcome := restoreOutcome(platform.FileOutcome{Path: changed, Err: &platform.FileError{Path: changed, Changed: true, Err: errors.New("postimage changed")}}, home)
	if outcome != "~/x was not restored, because it changed after AIGW wrote it." {
		t.Fatalf("a restore that met a change = %q", outcome)
	}
	if outcome := restoreOutcome(platform.FileOutcome{Err: errors.New("rollback failed")}, home); outcome != "Another client file could not be restored: unexpected error." {
		t.Fatalf("a restore without a file = %q", outcome)
	}
	if name := fileName("relative", ""); name != "a client file" {
		t.Fatalf("a file whose place is unknown = %q", name)
	}
}
