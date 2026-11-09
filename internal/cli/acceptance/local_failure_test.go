package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/store"
)

// A local file failure names a fix that runs in the state where the command
// failed. In a fresh home a local check fails for want of a configuration, so
// neither a team file that cannot be read nor AIGW's own folder names one.
func TestALocalFileFailureNamesAFixThatRunsInAFreshHome(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "team.toml")
	for name, test := range map[string]struct {
		args []string
		fix  string
		home func(*testing.T, *cli.App)
	}{
		"setup without its team file":         {[]string{"setup", "--from", missing, "--json"}, "Name a team file that exists, then run the same command again.", nil},
		"config import without its team file": {[]string{"config", "import", missing, "--json"}, "Name a team file that exists, then run the same command again.", nil},
		// A file that stands where AIGW's folder belongs is named to move, and how
		// the folder comes back: a permission fix acts on nothing there. The
		// configuration lies in the command's own home, so the fix names it as
		// ~/… on every platform, whatever the process's home is.
		"a mutation where AIGW's folder is a file": {[]string{"use", "claude", "one/m", "--json"},
			"Move the file ~/aigw. If it replaced AIGW's folder, put that folder back from a copy; otherwise run `aigw setup`. Then run the same command again.",
			func(t *testing.T, app *cli.App) {
				t.Helper()
				blockAIGWsFolder(t, app)
			}},
	} {
		t.Run(name, func(t *testing.T) {
			app, output, _, _, _ := testApp(t, "")
			if test.home != nil {
				test.home(t, app)
			}
			if err := cli.Execute(app, test.args); err == nil {
				t.Fatalf("%v passed", test.args)
			}
			var failure struct {
				Error      string `json:"error"`
				NextAction string `json:"next_action"`
				NextDetail string `json:"next_detail"`
			}
			if err := json.Unmarshal(output.Bytes(), &failure); err != nil {
				t.Fatalf("%v: %v\n%s", test.args, err, output)
			}
			if failure.NextDetail != test.fix || failure.NextAction != "" || strings.Contains(failure.Error, "aigw check") {
				t.Fatalf("%v failure = %+v", test.args, failure)
			}
		})
	}
}

// A home without a configuration has nothing to roll back, so rollback names
// setup, while a configuration without a recovery source keeps the local
// check, which inspects it.
func TestRollbackWithoutAConfigurationNamesSetup(t *testing.T) {
	app, output, _, _, _ := testApp(t, "")
	if err := cli.Execute(app, []string{"rollback", "--json"}); err == nil {
		t.Fatal("a rollback without a configuration passed")
	}
	// Bare setup asks a person for a provider, so it is guidance for a script.
	var failure struct {
		NextAction string `json:"next_action"`
		NextDetail string `json:"next_detail"`
	}
	if err := json.Unmarshal(output.Bytes(), &failure); err != nil || failure.NextDetail != "aigw setup" || failure.NextAction != "" {
		t.Fatalf("the rollback = %+v, %v\n%s", failure, err, output)
	}
	if _, err := os.Stat(app.Config.Path()); !os.IsNotExist(err) {
		t.Fatalf("the refusal wrote a configuration: %v", err)
	}
}

// blockAIGWsFolder moves app's configuration into its home's aigw folder, which
// a regular file then replaces, as an operator's mistake would. It returns
// that folder.
func blockAIGWsFolder(t *testing.T, app *cli.App) string {
	t.Helper()
	folder := filepath.Join(app.Host.Home, "aigw")
	app.Config = store.NewStore(filepath.Join(folder, "configuration.toml"), clients.Admission())
	writeFile(t, folder, []byte("x"), 0o600)
	return folder
}

// A file that stands where AIGW's own folder belongs is named to move on every
// command that reaches it, not only the ones that build a configuration
// problem: sync, use, rollback and an uninstall preview would otherwise name a
// permission to restore that cannot help. The reason reads not a directory
// on every platform, though Windows reports the path as missing, and the path
// is named against the command's own home. Following the fix lets each run.
func TestAFileWhereAIGWsFolderBelongsToEveryCommand(t *testing.T) {
	for _, args := range [][]string{
		{"sync", "--json"},
		{"use", "claude", "one/m", "--json"},
		{"rollback", "--json"},
		{"uninstall", "--dry-run", "--json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			app, output, _, _, _ := testApp(t, "")
			folder := blockAIGWsFolder(t, app)
			if err := cli.Execute(app, args); err == nil {
				t.Fatalf("%v passed", args)
			}
			var failure struct {
				Error      string `json:"error"`
				NextAction string `json:"next_action"`
				NextDetail string `json:"next_detail"`
			}
			if err := json.Unmarshal(output.Bytes(), &failure); err != nil {
				t.Fatalf("%v: %v\n%s", args, err, output)
			}
			want := "Move the file ~/aigw. If it replaced AIGW's folder, put that folder back from a copy; otherwise run `aigw setup`. Then run the same command again."
			if failure.NextDetail != want || failure.NextAction != "" || !strings.Contains(failure.Error, "not a directory") {
				t.Fatalf("%v names %+v", args, failure)
			}
			// The file moves away, and setup begins the configuration again in a
			// folder of its own; the command that failed then runs.
			if err := os.Rename(folder, folder+".moved"); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(folder, 0o700); err != nil {
				t.Fatal(err)
			}
		})
	}
}
