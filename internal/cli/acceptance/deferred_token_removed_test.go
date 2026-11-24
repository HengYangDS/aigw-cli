package cli_test

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
)

func TestShippedTeamManifestTokenRemovedBeforeClientInstallation(t *testing.T) {
	t.Parallel()
	app, out, credentials, runner, httpClient := testApp(t, "")
	app.Discovery = fakeDiscovery{}
	if err := credentials.Set("ucloud", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"setup", "--from", shippedTeamManifest(t), "--json"}); err != nil {
		t.Fatalf("setup before client installation: %v\n%s", err, out)
	}
	if err := credentials.Delete("ucloud"); err != nil {
		t.Fatal(err)
	}
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	wantAction := "run `aigw provider rotate ucloud`"
	for _, command := range []string{"sync", "status", "check"} {
		assertTokenRemovedJSONCommand(t, app, out, command, wantAction)
		assertTokenRemovedHumanCommand(t, app, out, command)
	}
	after, err := app.Config.CaptureSnapshot()
	if err != nil || !before.Config.Equal(after.Config) || !before.Backup.Equal(after.Backup) || !before.Verified.Equal(after.Verified) {
		t.Fatalf("read-only diagnostics changed configuration: %v", err)
	}
	if len(runner.plans) != 0 || httpClient.calls != 0 {
		t.Fatalf("missing client or Token triggered external work: plans=%d http=%d", len(runner.plans), httpClient.calls)
	}
}

func assertTokenRemovedJSONCommand(t *testing.T, app *cli.App, out *bytes.Buffer, command, wantAction string) {
	t.Helper()
	out.Reset()
	args := []string{command, "--json"}
	if command == "sync" {
		args = []string{"sync", "--dry-run", "--json"}
	}
	err := cli.Execute(app, args)
	if (err != nil) != (command == "check") {
		t.Fatalf("%s error = %v\n%s", command, err, out)
	}
	var result deferredActivationDocument
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode %s: %v\n%s", command, err, out)
	}
	wantCommandAction := wantAction
	if command == "check" {
		wantCommandAction = "aigw provider rotate ucloud"
	}
	if command == "check" {
		// Check reports each checked client's state, and the client files that
		// wait for the client, which a missing Token comes before, as a local
		// check.
		result.State = result.Clients[codex.ID].State
		checked, _ := decodeCheck(t, out).client(codex.ID)
		files := slices.IndexFunc(checked.Checks, func(check checkedLocal) bool {
			return check.Name == "codex:files" && check.State == "failed" && strings.HasPrefix(check.Detail, "Codex is not installed")
		})
		if files < 0 || !wantNext(result.Clients[codex.ID].NextAction, result.Clients[codex.ID].NextDetail, wantCommandAction) {
			t.Fatalf("check selected Codex = %+v", checked)
		}
	}
	if result.State != "deferred" || !wantNext(result.NextAction, result.NextDetail, wantCommandAction) {
		t.Fatalf("%s continuation = %+v\n%s", command, result, out)
	}
	if command == "sync" || command == "check" {
		return
	}
	selected := result.Clients[codex.ID]
	if selected.State != "deferred" || !wantNext(selected.NextAction, selected.NextDetail, wantAction) || !selected.ProjectionDeferred && !selected.FilesDeferred {
		t.Fatalf("%s selected Codex = %+v", command, selected)
	}
}

func assertTokenRemovedHumanCommand(t *testing.T, app *cli.App, out *bytes.Buffer, command string) {
	t.Helper()
	out.Reset()
	args := []string{command}
	if command == "sync" {
		args = []string{"sync", "--dry-run"}
	}
	err := cli.Execute(app, args)
	if (err != nil) != (command == "check") {
		t.Fatalf("human %s error = %v\n%s", command, err, out)
	}
	// Status says that AIGW has not written the clients' files, sync that it
	// cannot write them yet, and doctor still calls them projections. Check
	// stops each client at the missing Token, which comes before the client
	// files that wait for it.
	human := strings.ToLower(out.String())
	if !strings.Contains(human, "aigw provider rotate ucloud") || command != "check" && !strings.Contains(human, "projection") &&
		!strings.Contains(human, "has not written") && !strings.Contains(human, "files can be written yet") {
		t.Fatalf("human %s hid a prerequisite:\n%s", command, out)
	}
}
