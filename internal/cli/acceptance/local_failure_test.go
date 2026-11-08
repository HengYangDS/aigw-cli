package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
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
		"a mutation where AIGW's folder is a file": {[]string{"use", "claude", "one/m", "--json"},
			"Make AIGW's folder and its files readable and writable by you; `aigw config path` prints where the folder is. Then run the same command again.",
			func(t *testing.T, app *cli.App) {
				t.Helper()
				folder := filepath.Dir(app.Config.Path())
				if err := os.RemoveAll(folder); err != nil {
					t.Fatal(err)
				}
				writeFile(t, folder, []byte("x"), 0o600)
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
