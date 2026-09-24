package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

type deferredActivationDocument struct {
	OK             bool              `json:"ok"`
	OKScope        string            `json:"ok_scope"`
	State          string            `json:"state"`
	EnabledClients int               `json:"enabled_clients"`
	NextAction     string            `json:"next_action"`
	Selections     map[string]string `json:"selections"`
	Clients        map[string]struct {
		State string `json:"state"`
	} `json:"clients"`
}

func shippedTeamManifest(t *testing.T) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate acceptance source")
	}
	manifest := filepath.Join(filepath.Dir(source), "..", "..", "..", "manifests", "team.toml")
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("locate shipped manifest: %v", err)
	}
	return manifest
}

func assertDeferredJSONCommand(t *testing.T, app *cli.App, out *bytes.Buffer, command, wantAction string) {
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
	if result.State != "deferred" || result.EnabledClients != 0 || result.NextAction != wantAction {
		t.Fatalf("%s activation = %+v\n%s", command, result, out)
	}
	if command == "sync" && len(result.Selections) != 0 {
		t.Fatalf("sync selected an unconnected Account: %+v", result.Selections)
	}
	if command == "check" && result.OK {
		t.Fatalf("empty check reported success: %s", out)
	}
	if command == "doctor" && (!result.OK || result.OKScope != "local_diagnostics") {
		t.Fatalf("doctor did not scope diagnostic success: %s", out)
	}
	if command != "sync" {
		for _, client := range configuration.AdmittedClientIDs() {
			if result.Clients[client].State != "deferred" {
				t.Fatalf("%s client %s = %+v", command, client, result.Clients[client])
			}
		}
	}
}

func assertDeferredHumanCommand(t *testing.T, app *cli.App, out *bytes.Buffer, command, wantAction string) {
	t.Helper()
	out.Reset()
	err := cli.Execute(app, []string{command})
	if (err != nil) != (command == "check") {
		t.Fatalf("human %s error = %v\n%s", command, err, out)
	}
	if !strings.Contains(out.String(), "No client is enabled") || !strings.Contains(out.String(), wantAction) {
		t.Fatalf("human %s implied usable health:\n%s", command, out)
	}
	if command == "doctor" && strings.Contains(out.String(), "No problems found") {
		t.Fatalf("doctor implied product readiness: %s", out)
	}
}

func TestShippedTeamManifestWithoutAccountOrClientIsDeferred(t *testing.T) {
	app, out, _, runner, httpClient := testApp(t, "")
	app.Secrets = secrets.NewEnvironmentStore(func(string) string { return "" })
	app.Discovery = fakeDiscovery{}
	wantAction := "set environment variable " + secrets.EnvironmentKey("dmxapi")

	if err := cli.Execute(app, []string{"setup", "--from", shippedTeamManifest(t), "--json"}); err != nil {
		t.Fatalf("setup shipped catalogue: %v\n%s", err, out)
	}
	var setup struct {
		SelectedBindings map[string]string `json:"selected_bindings"`
		DeferredActions  []string          `json:"deferred_actions"`
	}
	if err := json.Unmarshal(out.Bytes(), &setup); err != nil {
		t.Fatalf("decode setup: %v\n%s", err, out)
	}
	if len(setup.SelectedBindings) != 0 || len(setup.DeferredActions) == 0 {
		t.Fatalf("setup activation = %+v", setup)
	}
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}

	for _, command := range []string{"sync", "status", "check", "doctor"} {
		assertDeferredJSONCommand(t, app, out, command, wantAction)
	}
	for _, command := range []string{"status", "check", "doctor"} {
		assertDeferredHumanCommand(t, app, out, command, wantAction)
	}
	after, err := app.Config.CaptureSnapshot()
	if err != nil || !before.Config.Equal(after.Config) || !before.Backup.Equal(after.Backup) || !before.Verified.Equal(after.Verified) {
		t.Fatalf("read-only acceptance changed configuration: %v", err)
	}
	if len(runner.plans) != 0 || httpClient.calls != 0 {
		t.Fatalf("deferred activation invoked client or endpoint: plans=%d http=%d", len(runner.plans), httpClient.calls)
	}
}
