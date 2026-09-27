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
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
	surfaceidentity "aigw-cli/internal/surface"
)

type deferredActivationDocument struct {
	OK             bool              `json:"ok"`
	OKScope        string            `json:"ok_scope"`
	State          string            `json:"state"`
	EnabledClients int               `json:"enabled_clients"`
	NextAction     string            `json:"next_action"`
	Selections     map[string]string `json:"selections"`
	Clients        map[string]struct {
		State              string `json:"state"`
		NextAction         string `json:"next_action"`
		ProjectionDeferred bool   `json:"projection_deferred"`
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
	human := strings.Join(strings.Fields(out.String()), " ")
	action := strings.Join(strings.Fields(wantAction), " ")
	if !strings.Contains(human, "No client is enabled") || !strings.Contains(human, action) {
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

	if err := cli.Execute(app, []string{"setup", "--from", shippedTeamManifest(t), "--json"}); err != nil {
		t.Fatalf("setup shipped catalogue: %v\n%s", err, out)
	}
	var setup struct {
		SelectedBindings map[string]string `json:"selected_bindings"`
		DeferredActions  []string          `json:"deferred_actions"`
		NextAction       string            `json:"next_action"`
	}
	if err := json.Unmarshal(out.Bytes(), &setup); err != nil {
		t.Fatalf("decode setup: %v\n%s", err, out)
	}
	if len(setup.SelectedBindings) != 0 || len(setup.DeferredActions) != 1 {
		t.Fatalf("setup activation = %+v", setup)
	}
	wantAction := setup.DeferredActions[0]
	for _, account := range []string{"aihubmix", "dmxapi", "ucloud"} {
		if !strings.Contains(wantAction, secrets.EnvironmentKey(account)) {
			t.Fatalf("missing compatible Account %q in %q", account, wantAction)
		}
	}
	if setup.NextAction != wantAction {
		t.Fatalf("setup next action %q differs from prerequisite %q", setup.NextAction, wantAction)
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

func TestShippedTeamManifestWithWritableStoreRequiresOneAccountChoice(t *testing.T) {
	app, out, _, runner, httpClient := testApp(t, "")
	app.Discovery = fakeDiscovery{}

	if err := cli.Execute(app, []string{"setup", "--from", shippedTeamManifest(t), "--json"}); err != nil {
		t.Fatalf("setup shipped catalogue: %v\n%s", err, out)
	}
	var setup struct {
		SelectedBindings map[string]string `json:"selected_bindings"`
		DeferredActions  []string          `json:"deferred_actions"`
		NextAction       string            `json:"next_action"`
	}
	if err := json.Unmarshal(out.Bytes(), &setup); err != nil {
		t.Fatalf("decode setup: %v\n%s", err, out)
	}
	if len(setup.SelectedBindings) != 0 || len(setup.DeferredActions) != 1 || setup.NextAction != setup.DeferredActions[0] {
		t.Fatalf("setup activation = %+v", setup)
	}
	for _, account := range []string{"aihubmix", "dmxapi", "ucloud"} {
		if !strings.Contains(setup.NextAction, "aigw rotate "+account) {
			t.Fatalf("setup omitted Account choice %q: %q", account, setup.NextAction)
		}
	}
	if strings.Contains(setup.NextAction, "aigw sync") {
		t.Fatalf("setup recommended synchronization before its prerequisite: %q", setup.NextAction)
	}
	for _, command := range []string{"sync", "status", "check", "doctor"} {
		assertDeferredJSONCommand(t, app, out, command, setup.NextAction)
	}
	for _, command := range []string{"status", "check", "doctor"} {
		assertDeferredHumanCommand(t, app, out, command, setup.NextAction)
	}
	if len(runner.plans) != 0 || httpClient.calls != 0 {
		t.Fatalf("deferred activation invoked client or endpoint: plans=%d http=%d", len(runner.plans), httpClient.calls)
	}

	app.In = strings.NewReader("new-token\n")
	out.Reset()
	if err := cli.Execute(app, []string{"rotate", "dmxapi", "--token-stdin"}); err != nil {
		t.Fatalf("connect one deferred Account: %v\n%s", err, out)
	}
	want := "aigw use --for claude dmxapi-claude-opus-5-5"
	if !strings.Contains(out.String(), want) || strings.Contains(out.String(), "Next\n  aigw check") {
		t.Fatalf("rotation did not name its next executable selection: %s", out)
	}
	current, err := app.Config.Load()
	if err != nil || len(current.EnabledClientIDs()) != 0 {
		t.Fatalf("rotation silently activated clients: %#v, %v", current.Clients, err)
	}
	out.Reset()
	if err := cli.Execute(app, []string{"use", "--for", "claude", "dmxapi-claude-opus-5-5"}); err != nil {
		t.Fatalf("suggested Route selection failed: %v\n%s", err, out)
	}
	selected, err := app.Config.Load()
	if err != nil || selected.SelectedRoute(configuration.ClientClaude) != "dmxapi-claude-opus-5-5" {
		t.Fatalf("suggested Route was not selected: %#v, %v", selected.Clients, err)
	}
}

func TestSelectedClientWithoutExecutableHasOneDeferredContinuation(t *testing.T) {
	app, out, credentials, runner, httpClient := testApp(t, "")
	app.Interactive = true
	app.Discovery = fakeDiscovery{}
	app.Prompt = &scriptedPrompt{
		selections: []string{configuration.ClientHermes, string(configuration.ProtocolAnthropic)},
		secrets:    []string{"fixture-token"},
		texts:      []string{"team", "Team", "https://messages.test", "hermes-route", "hermes-model"},
	}
	if err := cli.Execute(app, nil); err != nil {
		t.Fatal(err)
	}
	if !secretExists(t, credentials, "team") {
		t.Fatal("guided setup did not connect the selected Account")
	}
	wantAction := "Install Hermes if needed, then run `aigw sync`"
	if !strings.Contains(out.String(), wantAction) {
		t.Fatalf("guided setup action = %q", out)
	}
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	requestsBefore := httpClient.calls

	for _, command := range []string{"sync", "status", "check", "doctor"} {
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
		if result.EnabledClients != 1 || result.State != "deferred" || result.NextAction != wantAction {
			t.Fatalf("%s activation = %+v\n%s", command, result, out)
		}
		if command != "sync" && result.Clients[configuration.ClientHermes].State != "deferred" {
			t.Fatalf("%s Hermes state = %+v", command, result.Clients[configuration.ClientHermes])
		}
	}
	for _, command := range []string{"status", "check", "doctor"} {
		out.Reset()
		err := cli.Execute(app, []string{command})
		if (err != nil) != (command == "check") {
			t.Fatalf("human %s error = %v\n%s", command, err, out)
		}
		if !strings.Contains(out.String(), wantAction) || strings.Contains(out.String(), "No client is enabled") {
			t.Fatalf("human %s continuation = %q", command, out)
		}
	}
	after, err := app.Config.CaptureSnapshot()
	if err != nil || !before.Config.Equal(after.Config) || !before.Backup.Equal(after.Backup) || !before.Verified.Equal(after.Verified) {
		t.Fatalf("read-only commands changed configuration: %v", err)
	}
	if len(runner.plans) != 0 || httpClient.calls != requestsBefore {
		t.Fatalf("deferred activation invoked client or endpoint: plans=%d http=%d, before=%d", len(runner.plans), httpClient.calls, requestsBefore)
	}
}

func TestShippedTeamManifestTokenRemovedBeforeClientInstallation(t *testing.T) {
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
	wantAction := "run `aigw rotate ucloud`"
	for _, command := range []string{"sync", "status", "check", "doctor"} {
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

func TestShippedTeamManifestMixedClientProjectionHasOneContinuation(t *testing.T) {
	app, out, credentials, _, httpClient := testApp(t, "")
	if err := credentials.Set("ucloud", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientCodex: "/opt/codex"},
		Surfaces: []discovery.Surface{{
			ID:          string(surfaceidentity.CodexHomeDefault),
			Authority:   string(surfaceidentity.AuthorityAIGW),
			ConfigPath:  target,
			Present:     true,
			AutoManaged: true,
		}},
	}}
	if err := cli.Execute(app, []string{"setup", "--from", shippedTeamManifest(t), "--json"}); err != nil {
		t.Fatalf("setup mixed clients: %v\n%s", err, out)
	}
	wantAction := "Install Claude if needed, then run `aigw sync`"
	var setup deferredActivationDocument
	if err := json.Unmarshal(out.Bytes(), &setup); err != nil {
		t.Fatalf("decode setup: %v\n%s", err, out)
	}
	if setup.NextAction != wantAction {
		t.Fatalf("setup next action = %q, want %q\n%s", setup.NextAction, wantAction, out)
	}
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	beforeProbes := httpClient.calls
	for _, command := range []string{"sync", "status", "check", "doctor"} {
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
		if result.NextAction != wantAction {
			t.Fatalf("%s next action = %q, want %q\n%s", command, result.NextAction, wantAction, out)
		}
	}
	for _, command := range []string{"sync", "status", "check", "doctor"} {
		assertMixedClientHumanCommand(t, app, out, command, wantAction)
	}
	if httpClient.calls != beforeProbes+2 {
		t.Fatalf("mixed-client check probed unavailable clients: before=%d after=%d", beforeProbes, httpClient.calls)
	}
	after, err := app.Config.CaptureSnapshot()
	if err != nil || !before.Config.Equal(after.Config) || !before.Backup.Equal(after.Backup) || !before.Verified.Equal(after.Verified) {
		t.Fatalf("observational commands changed configuration: %v", err)
	}
}

func assertMixedClientHumanCommand(t *testing.T, app *cli.App, out *bytes.Buffer, command, wantAction string) {
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
	if !strings.Contains(out.String(), wantAction) {
		t.Fatalf("human %s hid %q:\n%s", command, wantAction, out)
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
	if (err != nil) != (command == "check" || command == "doctor") {
		t.Fatalf("%s error = %v\n%s", command, err, out)
	}
	var result deferredActivationDocument
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode %s: %v\n%s", command, err, out)
	}
	wantCommandAction := wantAction
	if command == "doctor" {
		wantCommandAction = "aigw rotate ucloud"
	}
	if result.State != "deferred" || result.NextAction != wantCommandAction {
		t.Fatalf("%s continuation = %+v\n%s", command, result, out)
	}
	if command == "sync" {
		return
	}
	codex := result.Clients[configuration.ClientCodex]
	if codex.State != "deferred" || codex.NextAction != wantAction || !codex.ProjectionDeferred {
		t.Fatalf("%s selected Codex = %+v", command, codex)
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
	if (err != nil) != (command == "check" || command == "doctor") {
		t.Fatalf("human %s error = %v\n%s", command, err, out)
	}
	human := strings.ToLower(out.String())
	if !strings.Contains(human, "aigw rotate ucloud") || !strings.Contains(human, "projection") {
		t.Fatalf("human %s hid a prerequisite:\n%s", command, out)
	}
}
