package cli_test

import (
	"aigw-cli/internal/domain"
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

type deferredActivationDocument struct {
	OK             bool              `json:"ok"`
	OKScope        string            `json:"ok_scope"`
	State          string            `json:"state"`
	EnabledClients int               `json:"enabled_clients"`
	NextAction     string            `json:"next_action"`
	NextDetail     string            `json:"next_detail"`
	Selections     map[string]string `json:"selections"`
	Clients        clientStates      `json:"clients"`
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

// shippedRecommendedProviders are the providers that the shipped team file's
// recommendations name, in ID order: a Token of any one of them lets setup
// fill the clients that it serves.
func shippedRecommendedProviders(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(shippedTeamManifest(t))
	if err != nil {
		t.Fatal(err)
	}
	file, err := store.ParseTeamFile(data, clients.Admission())
	if err != nil {
		t.Fatal(err)
	}
	providers := map[string]bool{}
	for _, recommendation := range file.Recommendations {
		for _, choice := range recommendation.Selections() {
			providers[choice.Model.Provider()] = true
		}
	}
	return slices.Sorted(maps.Keys(providers))
}

// eachProvider joins one phrase for each of providers with " or ".
func eachProvider(providers []string, phrase func(string) string) string {
	phrases := make([]string, 0, len(providers))
	for _, provider := range providers {
		phrases = append(phrases, phrase(provider))
	}
	return strings.Join(phrases, " or ")
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
	if result.State != "deferred" || result.EnabledClients != 0 || !wantNext(result.NextAction, result.NextDetail, wantAction) {
		t.Fatalf("%s activation = %+v\n%s", command, result, out)
	}
	if command == "sync" && len(result.Selections) != 0 {
		t.Fatalf("sync selected an unconnected Account: %+v", result.Selections)
	}
	if command == "check" && result.OK {
		t.Fatalf("empty check reported success: %s", out)
	}
	// Check lists only the clients that it checked, and it checked none.
	if command == "check" && len(result.Clients) != 0 {
		t.Fatalf("check listed clients that it did not check: %+v", result.Clients)
	}
	if command != "sync" && command != "check" {
		for _, client := range clients.Admission().IDs() {
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
}

func TestShippedTeamManifestWithoutProviderOrClientIsDeferred(t *testing.T) {
	t.Parallel()
	app, out, _, runner, httpClient := testApp(t, "")
	app.Secrets = environmentBackend(func(string) string { return "" })
	app.Discovery = fakeDiscovery{}

	if err := cli.Execute(app, []string{"setup", "--from", shippedTeamManifest(t), "--json"}); err != nil {
		t.Fatalf("setup shipped catalog: %v\n%s", err, out)
	}
	var setup teamSetupDocument
	if err := json.Unmarshal(out.Bytes(), &setup); err != nil {
		t.Fatalf("decode setup: %v\n%s", err, out)
	}
	for _, client := range setup.Clients {
		if client.State != "not_selected" {
			t.Fatalf("setup activation = %+v", setup)
		}
	}
	wantAction := "Set one compatible provider's variable: " + eachProvider(shippedRecommendedProviders(t), secret.EnvironmentKey) + ", then run `aigw sync`"
	if !wantNext(setup.NextAction, setup.NextDetail, wantAction) {
		t.Fatalf("setup next action %q differs from prerequisite %q", (setup.NextAction + setup.NextDetail), wantAction)
	}
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}

	for _, command := range []string{"sync", "status", "check"} {
		assertDeferredJSONCommand(t, app, out, command, wantAction)
	}
	for _, command := range []string{"status", "check"} {
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

func TestShippedTeamManifestImportsWithAnOfflineProvider(t *testing.T) {
	t.Parallel()
	app, out, _, runner, httpClient := testApp(t, "")
	app.Secrets = environmentBackend(func(key string) string {
		if key == secret.EnvironmentKey("dmxapi") {
			return "synthetic-offline-token"
		}
		return ""
	})
	httpClient.handler = func(*http.Request) (*http.Response, error) {
		return nil, errors.New("provider is offline")
	}
	target := filepath.Join(t.TempDir(), "codex", "config.toml")
	writeFile(t, target, []byte("model_provider = \"native\"\n"), 0o600)
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{codex.ID: executableFixture(t, "codex")},
		Surfaces: []discovery.Surface{{
			ID:          string(codex.HomeDefault),
			ConfigPath:  target,
			Present:     true,
			AutoManaged: true,
		}},
	}}

	if err := cli.Execute(app, []string{"setup", "--from", shippedTeamManifest(t), "--json"}); err != nil {
		t.Fatalf("offline catalog import: %v\n%s", err, out)
	}
	var setup teamSetupDocument
	if err := json.Unmarshal(out.Bytes(), &setup); err != nil {
		t.Fatalf("decode setup: %v\n%s", err, out)
	}
	if setup.token("dmxapi").State != "environment" || setup.client(codex.ID).State != "configured" ||
		!strings.Contains((setup.NextAction+setup.NextDetail), "aigw sync") {
		t.Fatalf("offline import state = %+v", setup)
	}
	if httpClient.calls != 0 || len(runner.plans) != 0 {
		t.Fatalf("declarative import invoked provider or client: http=%d plans=%d", httpClient.calls, len(runner.plans))
	}
	if err := cli.Execute(app, []string{"check", "--json"}); err == nil {
		t.Fatal("offline provider was reported healthy")
	}
	if httpClient.calls == 0 {
		t.Fatal("explicit check did not observe the provider")
	}
	if strings.Contains(out.String(), "synthetic-offline-token") {
		t.Fatal("credential leaked to CLI output")
	}
}

func TestShippedTeamManifestWithWritableStoreRequiresOneProviderChoice(t *testing.T) {
	t.Parallel()
	app, out, _, runner, httpClient := testApp(t, "")
	app.Discovery = fakeDiscovery{}

	if err := cli.Execute(app, []string{"setup", "--from", shippedTeamManifest(t), "--json"}); err != nil {
		t.Fatalf("setup shipped catalog: %v\n%s", err, out)
	}
	var setup teamSetupDocument
	if err := json.Unmarshal(out.Bytes(), &setup); err != nil {
		t.Fatalf("decode setup: %v\n%s", err, out)
	}
	for _, client := range setup.Clients {
		if client.State != "not_selected" {
			t.Fatalf("setup activation = %+v", setup)
		}
	}
	wantAction := "Choose one compatible provider: " + eachProvider(shippedRecommendedProviders(t), func(provider string) string { return "aigw provider rotate " + provider })
	if !wantNext(setup.NextAction, setup.NextDetail, wantAction) {
		t.Fatalf("setup Account choices = %q, want %q", (setup.NextAction + setup.NextDetail), wantAction)
	}
	if strings.Contains((setup.NextAction + setup.NextDetail), "aigw sync") {
		t.Fatalf("setup recommended synchronization before its prerequisite: %q", (setup.NextAction + setup.NextDetail))
	}
	for _, command := range []string{"sync", "status", "check"} {
		assertDeferredJSONCommand(t, app, out, command, (setup.NextAction + setup.NextDetail))
	}
	for _, command := range []string{"status", "check"} {
		assertDeferredHumanCommand(t, app, out, command, (setup.NextAction + setup.NextDetail))
	}
	if len(runner.plans) != 0 || httpClient.calls != 0 {
		t.Fatalf("deferred activation invoked client or endpoint: plans=%d http=%d", len(runner.plans), httpClient.calls)
	}

	app.In = strings.NewReader("new-token\n")
	out.Reset()
	if err := cli.Execute(app, []string{"provider", "rotate", "dmxapi", "--token-stdin"}); err != nil {
		t.Fatalf("connect one deferred Account: %v\n%s", err, out)
	}
	want := "aigw use claude dmxapi/claude-opus-5-5"
	if !strings.Contains(out.String(), want) || strings.Contains(out.String(), "Next\n  aigw check") {
		t.Fatalf("rotation did not name its next executable selection: %s", out)
	}
	current, err := app.Config.Load()
	if err != nil || len(current.EnabledClientIDs()) != 0 {
		t.Fatalf("rotation silently activated clients: %#v, %v", current.Clients, err)
	}
	out.Reset()
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync after rotation: %v\n%s", err, out)
	}
	if strings.Contains(out.String(), "then synchronize") {
		t.Fatalf("sync after rotation claimed that synchronization activates a writable Account: %s", out)
	}
	out.Reset()
	if err := cli.Execute(app, []string{"use", "claude", "dmxapi/claude-opus-5-5"}); err != nil {
		t.Fatalf("suggested model selection failed: %v\n%s", err, out)
	}
	selected, err := app.Config.Load()
	if err != nil || selected.SelectedModel(claude.ID) != domain.NewModelRef("dmxapi", "claude-opus-5-5") {
		t.Fatalf("suggested Route was not selected: %#v, %v", selected.Clients, err)
	}
}

// selectUninstalledHermes adds provider team with its Token and selects one
// of its models for Hermes, which is not installed.
func selectUninstalledHermes(t *testing.T, app *cli.App, out *bytes.Buffer, credentials secret.Store, wantAction string) {
	t.Helper()
	if err := cli.Execute(app, []string{"provider", "add", "team", "--endpoint", "anthropic=https://messages.test", "--token-stdin"}); err != nil {
		t.Fatalf("provider add: %v\n%s", err, out)
	}
	out.Reset()
	if err := cli.Execute(app, []string{"use", "hermes", "team/hermes-model", "--protocol", "anthropic", "--unlisted"}); err != nil {
		t.Fatalf("use: %v\n%s", err, out)
	}
	if !secretExists(t, credentials, "team") {
		t.Fatal("provider add did not store the Token")
	}
	if !strings.Contains(out.String(), wantAction) {
		t.Fatalf("selection action = %q", out)
	}
}

func TestSelectedClientWithoutExecutableHasOneDeferredContinuation(t *testing.T) {
	t.Parallel()
	app, out, credentials, runner, httpClient := testApp(t, "fixture-token\n")
	app.Discovery = fakeDiscovery{}
	wantAction := "Install Hermes if needed, then run `aigw sync`"
	selectUninstalledHermes(t, app, out, credentials, wantAction)
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	requestsBefore := httpClient.calls

	for _, command := range []string{"sync", "status", "check"} {
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
		// Check reports each checked client's state, not activation's.
		if command == "check" {
			result.State = result.Clients[hermes.ID].State
		}
		if result.EnabledClients != 1 || result.State != "deferred" || !wantNext(result.NextAction, result.NextDetail, wantAction) {
			t.Fatalf("%s activation = %+v\n%s", command, result, out)
		}
		if command != "sync" && result.Clients[hermes.ID].State != "deferred" {
			t.Fatalf("%s Hermes state = %+v", command, result.Clients[hermes.ID])
		}
	}
	for _, command := range []string{"status", "check"} {
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

func TestShippedTeamManifestMixedClientProjectionHasOneContinuation(t *testing.T) {
	t.Parallel()
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
		Executables: map[string]string{codex.ID: fakeProgram("/opt/codex")},
		Surfaces: []discovery.Surface{{
			ID:          string(codex.HomeDefault),
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
	if !wantNext(setup.NextAction, setup.NextDetail, wantAction) {
		t.Fatalf("setup next action = %q, want %q\n%s", (setup.NextAction + setup.NextDetail), wantAction, out)
	}
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	beforeProbes := httpClient.calls
	for _, command := range []string{"sync", "status", "check"} {
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
		if !wantNext(result.NextAction, result.NextDetail, wantAction) {
			t.Fatalf("%s next action = %q, want %q\n%s", command, (result.NextAction + result.NextDetail), wantAction, out)
		}
	}
	for _, command := range []string{"sync", "status", "check"} {
		assertMixedClientHumanCommand(t, app, out, command, wantAction)
	}
	// Each check of the configured client sends one request without a model
	// and one inference request; the clients that wait send none.
	if httpClient.calls != beforeProbes+4 {
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
