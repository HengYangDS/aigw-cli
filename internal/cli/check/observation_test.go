package check

import (
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
)

// A passing observation that states something of its own, as the context
// window that a client's table gives a model, shows that statement; one that
// states nothing matches the selection. Every observation of one file names
// that file alike: by its path under the home directory, or, outside it, by
// the file's position among the client's files, whichever of its observations
// is shown and wherever it stands among them.
func TestObservedChecksShowAnObservationsOwnStatementAndNameItsFile(t *testing.T) {
	home := t.TempDir()
	inside, outside := filepath.Join(home, ".codex", "config.toml"), filepath.Join(t.TempDir(), "config.toml")
	spec := domain.ClientSpec{ID: "codex", Label: "Codex"}
	const stated, fallback = "the table gives the model a window", "the table has no entry for the model"
	got := observedChecks(spec, home, []adapter.Check{
		{ID: "codex:target-1", Ready: true, Target: inside},
		{ID: "codex:target-2", Ready: true, Target: outside},
		{ID: "codex:window-1", Ready: true, Target: inside, Detail: stated},
		{ID: "codex:window-2", Ready: true, Target: outside, Detail: fallback},
	})
	if len(got) != 4 {
		t.Fatalf("observed checks = %+v, want one for each observation", got)
	}
	for index, want := range []struct{ name, detail string }{
		{"codex:target-1", "Matches the selection"},
		{"codex:target-2", "Matches the selection"},
		{"codex:window-1", stated},
		{"codex:window-2", fallback},
	} {
		if got[index].Name != want.name || got[index].State != checkPassed || got[index].Detail != want.detail {
			t.Errorf("check %d = %+v, want %s passed with %q", index, got[index], want.name, want.detail)
		}
	}
	if got[0].Target == "" || got[2].Target != got[0].Target || got[2].label != got[0].label {
		t.Errorf("the two observations of the file under home name it %q (%q) and %q (%q)", got[0].label, got[0].Target, got[2].label, got[2].Target)
	}
	if got[1].Target != "" || got[3].Target != "" || got[3].label != got[1].label || got[1].label != "Codex configuration target 2 (outside the home directory)" {
		t.Errorf("the two observations of the file outside home name it %q and %q", got[1].label, got[3].label)
	}
}

// A failing observation keeps the problem and the fix that its adapter gave.
func TestObservedChecksKeepAFailingObservationsProblemAndFix(t *testing.T) {
	home := t.TempDir()
	got := observedChecks(domain.ClientSpec{ID: "codex", Label: "Codex"}, home, []adapter.Check{
		{ID: "codex:target-1", Target: filepath.Join(home, ".codex", "config.toml"), Detail: "Differs from the selection", RepairAction: "aigw sync"},
	})
	if len(got) != 1 || got[0].State != checkFailed || got[0].Detail != "Differs from the selection" || got[0].fix != "aigw sync" {
		t.Fatalf("a failing observation = %+v", got)
	}
}
