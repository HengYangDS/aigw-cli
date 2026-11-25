package cli_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/domain"
)

// changedWindow is one item of `changed_windows` in `aigw setup --from`'s
// JSON document.
type changedWindow struct {
	Model string `json:"model"`
	Local int    `json:"local"`
	File  int    `json:"file"`
}

// Scenario "A team file changes a window that the operator declared": the
// file's definition of a model wins over the operator's window, and the
// preview and the result of `aigw setup --from`, in human output and in JSON,
// name each model whose window changes with the window before and after; a
// second apply of the file names none.
func TestSetupFromNamesEveryWindowThatTheTeamFileChanges(t *testing.T) {
	t.Parallel()
	app, out := windowApp(t)
	team := writeConfigurationManifest(t, exportedTeamFile(t, app, out))
	declare := func() {
		t.Helper()
		for model, window := range map[string]string{"team/small": "200000", "team/large": "512000"} {
			runWindowText(t, app, out, "model", "edit", model, "--context-window", window)
		}
	}
	want := []changedWindow{{Model: "team/small", Local: 200000, File: 256000}, {Model: "team/large", Local: 512000, File: domain.DefaultContextWindow}}
	rows := []string{"Context windows", "team/small 200000 tokens becomes 256000 tokens",
		fmt.Sprintf("team/large 512000 tokens becomes %d tokens", domain.DefaultContextWindow)}
	windows := func(args ...string) []changedWindow {
		t.Helper()
		var document struct {
			ChangedWindows []changedWindow `json:"changed_windows"`
		}
		out.Reset()
		if err := cli.Execute(app, append([]string{"setup", "--from", team, "--json"}, args...)); err != nil {
			t.Fatalf("setup --from %v: %v\n%s", args, err, out)
		}
		if err := json.Unmarshal(out.Bytes(), &document); err != nil {
			t.Fatalf("setup --from %v wrote no JSON document: %v\n%s", args, err, out)
		}
		return document.ChangedWindows
	}
	requireRows := func(name, text string) {
		t.Helper()
		for _, row := range rows {
			if !strings.Contains(text, row) {
				t.Errorf("%s lacks %q:\n%s", name, row, text)
			}
		}
	}

	declare()
	requireRows("the preview", runWindowText(t, app, out, "setup", "--from", team, "--dry-run"))
	if got := windows("--dry-run"); !reflect.DeepEqual(got, want) {
		t.Errorf("the preview's changed windows = %+v, want %+v", got, want)
	}
	requireRows("the result", runWindowText(t, app, out, "setup", "--from", team))
	if declaredWindow(t, app, "team/small") != 256000 || declaredWindow(t, app, "team/large") != 0 {
		t.Fatalf("after the team file the configuration declares %d and %d", declaredWindow(t, app, "team/small"), declaredWindow(t, app, "team/large"))
	}
	declare()
	if got := windows(); !reflect.DeepEqual(got, want) {
		t.Errorf("the result's changed windows = %+v, want %+v", got, want)
	}
	if got := windows("--dry-run"); len(got) != 0 {
		t.Errorf("a second apply of the file names the windows %+v", got)
	}
	if text := runWindowText(t, app, out, "setup", "--from", team, "--dry-run"); strings.Contains(text, "Context windows") {
		t.Errorf("a second apply of the file names a window:\n%s", text)
	}
}
