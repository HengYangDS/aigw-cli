package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/claudedesktop"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/client/discovery"
	"aigw-cli/internal/client/hermes"
	"aigw-cli/internal/domain"

	"go.yaml.in/yaml/v3"
)

func TestHermesSetupDeferredSyncCredentialCheckAndWithdrawal(t *testing.T) {
	app, output, credentials, runner, httpClient := testApp(t, "")
	root := t.TempDir()
	manifest := filepath.Join(root, "team.toml")
	writeFile(t, manifest, []byte(`version = 7
[recommendations.hermes.primary]
route = "team-model"
protocol = "anthropic"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "https://provider.test"
[models.model-test]
label = "Model Test"
[routes.team-model]
label = "Team Model"
account = "team"
model = "model-test"
upstream_model = "model-test"
interfaces = { anthropic = ["text", "streaming", "tools"] }
`), 0o600)
	if err := credentials.Set("team", "public-fixture-token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"setup", "--from", manifest}); err != nil {
		t.Fatal(err)
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Clients[hermes.ID].Enabled || cfg.SelectedRoute(hermes.ID) != "team-model" {
		t.Fatal("deferred setup lost selected or enabled intent")
	}
	target := filepath.Join(root, "hermes", "config.yaml")
	executable := executableFixture(t, "hermes")
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{hermes.ID: executable},
		Surfaces:    []discovery.Surface{{ID: "hermes-home-default", Product: "Hermes", Authority: "aigw", ConfigPath: target}},
	}}
	for _, args := range [][]string{{"sync"}, {"status", "--json"}, {"check"}, {"test", "--for", "hermes"}, {"verify", "--for", "hermes"}} {
		output.Reset()
		err := cli.Execute(app, args)
		if args[0] == "verify" && runtime.GOOS == "windows" {
			// Installed Windows Hermes ignores the verification-only launcher opt-out.
			if err == nil || !strings.Contains(err.Error()+output.String(), "cannot isolate") {
				t.Fatalf("%v: Windows verification was not refused: %v\n%s", args, err, output.String())
			}
		} else if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, output.String())
		}
		if strings.Contains(output.String(), "public-fixture-token") {
			t.Fatal("public command exposed its fixture credential")
		}
	}
	wantPlans := 2
	if runtime.GOOS == "windows" {
		wantPlans = 0
	}
	if len(runner.plans) != wantPlans {
		t.Fatalf("native verification invocation count = %d, want %d", len(runner.plans), wantPlans)
	}
	if httpClient.headers.Get("X-Api-Key") != "public-fixture-token" || httpClient.headers.Get("Authorization") != "" {
		t.Fatalf("wrong selected protocol headers: %v", httpClient.headers)
	}
	before := readFile(t, target)
	if !bytes.Contains(before, []byte("key_cmd:")) || bytes.Contains(before, []byte("public-fixture-token")) {
		t.Fatal("Hermes did not receive a secret-free native helper projection")
	}
	if err := cli.Execute(app, []string{"client", "disable", "hermes"}); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{target, target + ".aigw-state.json"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("withdrawal left %s: %v", path, err)
		}
	}
}

func TestHermesUseProjectsShippedChannelWireID(t *testing.T) {
	app, output, credentials, _, httpClient := testApp(t, "")
	if err := credentials.Set("dmxapi", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "hermes", "config.yaml")
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{hermes.ID: executableFixture(t, "hermes")},
		Surfaces:    []discovery.Surface{{ID: "hermes-home-default", Product: "Hermes", Authority: "aigw", ConfigPath: target}},
	}}
	manifest := filepath.Join("..", "..", "..", "manifests", "team.toml")
	if err := cli.Execute(app, []string{"setup", "--from", manifest, "--account", "dmxapi"}); err != nil {
		t.Fatalf("shipped setup: %v\n%s", err, output.String())
	}
	output.Reset()
	if err := cli.Execute(app, []string{"use", "--for", "hermes", "--protocol", "openai_responses", "dmxapi-gpt-6.1-sol-cdx"}); err != nil {
		t.Fatalf("select shipped channel: %v\n%s", err, output.String())
	}
	var projected struct {
		Model struct {
			Provider string `yaml:"provider"`
			Default  string `yaml:"default"`
		} `yaml:"model"`
		Providers map[string]struct {
			Models []string `yaml:"models"`
		} `yaml:"providers"`
	}
	if err := yaml.Unmarshal(readFile(t, target), &projected); err != nil {
		t.Fatal(err)
	}
	if projected.Model.Default != "gpt-6.1-sol-cdx" || !slices.Contains(projected.Providers[projected.Model.Provider].Models, projected.Model.Default) {
		t.Fatalf("shipped Hermes projection = %#v", projected)
	}
	if httpClient.calls != 0 || strings.Contains(string(readFile(t, target)), "fixture-token") {
		t.Fatal("Hermes selection sent a provider request or disclosed the Account Token")
	}
}

func TestHermesExplicitSelectionAdoptsNativeSwitchThenSyncs(t *testing.T) {
	app, output, credentials, _, httpClient := testApp(t, "")
	for _, account := range []string{"dmxapi", "ucloud"} {
		if err := credentials.Set(account, "fixture-token"); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(t.TempDir(), "hermes", "config.yaml")
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{hermes.ID: executableFixture(t, "hermes")},
		Surfaces:    []discovery.Surface{{ID: "hermes-home-default", Product: "Hermes", Authority: "aigw", ConfigPath: target}},
	}}
	manifest := filepath.Join("..", "..", "..", "manifests", "team.toml")
	for _, args := range [][]string{{"setup", "--from", manifest}, {"use", "--for", "hermes", "--protocol", "openai_responses", "ucloud-gpt-6.1-sol"}} {
		if err := cli.Execute(app, args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	observed := readFile(t, target)
	var native map[string]any
	if err := yaml.Unmarshal(observed, &native); err != nil {
		t.Fatal(err)
	}
	model, ok := native["model"].(map[string]any)
	if !ok {
		t.Fatal("native Hermes model is not a mapping")
	}
	providerID := "aigw-dmxapi-openai-responses"
	providers, ok := native["providers"].(map[string]any)
	if !ok {
		t.Fatal("native Hermes providers are not a mapping")
	}
	provider, ok := providers[providerID].(map[string]any)
	if !ok {
		t.Fatal("native Hermes selected provider is not a mapping")
	}
	model["provider"], model["base_url"] = providerID, provider["base_url"]
	drifted, err := yaml.Marshal(native)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, target, drifted, 0o600)
	stateBefore := readFile(t, target+".aigw-state.json")
	configBefore, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	// Explicit selection still adopts a proved native switch.
	args := []string{"use", "--for", "hermes", "--protocol", "openai_responses", "dmxapi-gpt-6.1-sol"}
	if err := cli.Execute(app, append(slices.Clone(args), "--dry-run", "--json")); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readFile(t, target), drifted) || !bytes.Equal(readFile(t, target+".aigw-state.json"), stateBefore) {
		t.Fatal("selection preview mutated native state")
	}
	if err := cli.Execute(app, args); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync after explicit adoption: %v", err)
	}
	cfg, err := app.Config.Load()
	if err != nil || cfg.SelectedRoute(hermes.ID) != "dmxapi-gpt-6.1-sol" {
		t.Fatalf("Hermes binding after adoption: %#v, %v", cfg.Clients, err)
	}
	for _, client := range []string{codex.ID, claude.ID, claudedesktop.ID} {
		if cfg.SelectedRoute(client) != configBefore.SelectedRoute(client) {
			t.Fatalf("Hermes adoption changed %s", client)
		}
	}
	if httpClient.calls != 0 || strings.Contains(output.String(), "fixture-token") {
		t.Fatal("selection recovery sent requests or disclosed credentials")
	}
}

// hermesNativeSelection switches Hermes' own model selection to another
// projected model in place, as Hermes' model picker does, and returns the
// drifted bytes. Key order and every other value stay as projected.
func hermesNativeSelection(t *testing.T, target, providerID, model string, edit func(provider *yaml.Node)) []byte {
	t.Helper()
	var document yaml.Node
	if err := yaml.Unmarshal(readFile(t, target), &document); err != nil {
		t.Fatal(err)
	}
	value := func(node *yaml.Node, key string) *yaml.Node {
		for index := 0; node != nil && index+1 < len(node.Content); index += 2 {
			if node.Content[index].Value == key {
				return node.Content[index+1]
			}
		}
		t.Fatalf("native Hermes configuration lacks %q", key)
		return nil
	}
	root := document.Content[0]
	selection, provider := value(root, "model"), value(value(root, "providers"), providerID)
	value(selection, "provider").Value, value(selection, "default").Value = providerID, model
	value(selection, "base_url").Value = value(provider, "base_url").Value
	value(selection, "api_mode").Value = value(provider, "transport").Value
	if edit != nil {
		edit(provider)
	}
	drifted, err := yaml.Marshal(&document)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, target, drifted, 0o600)
	return drifted
}

func shippedHermesApp(t *testing.T) (*cli.App, *bytes.Buffer, *fakeHTTP, string) {
	t.Helper()
	return shippedHermesAppWith(t, false)
}

// shippedHermesAppWith optionally binds a private credential reader, as
// production does, so that a missing Account Token defers its clients.
func shippedHermesAppWith(t *testing.T, reader bool) (*cli.App, *bytes.Buffer, *fakeHTTP, string) {
	t.Helper()
	app, output, credentials, _, httpClient := testApp(t, "")
	if reader {
		root := t.TempDir()
		app.Executable = filepath.Join(root, "bin", executableName("aigw"))
		app.DataDir = filepath.Join(root, "data")
		writeFile(t, app.Executable, []byte("portable program"), 0o755)
		app.UseExecutableIdentity(nil, executableName("aigw"))
	}
	for _, account := range []string{"dmxapi", "ucloud"} {
		if err := credentials.Set(account, "fixture-token"); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(t.TempDir(), "hermes", "config.yaml")
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{hermes.ID: executableFixture(t, "hermes")},
		Surfaces:    []discovery.Surface{{ID: "hermes-home-default", Product: "Hermes", Authority: "aigw", ConfigPath: target}},
	}}
	manifest := filepath.Join("..", "..", "..", "manifests", "team.toml")
	for _, args := range [][]string{{"setup", "--from", manifest}, {"use", "--for", "hermes", "--protocol", "openai_responses", "dmxapi-gpt-6.1-sol"}} {
		if err := cli.Execute(app, args); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, output)
		}
	}
	return app, output, httpClient, target
}

func hermesProjectedSelection(t *testing.T, target string) (provider, model string) {
	t.Helper()
	var projected struct {
		Model struct {
			Provider string `yaml:"provider"`
			Default  string `yaml:"default"`
		} `yaml:"model"`
	}
	if err := yaml.Unmarshal(readFile(t, target), &projected); err != nil {
		t.Fatal(err)
	}
	return projected.Model.Provider, projected.Model.Default
}

func TestHermesSyncAdoptsNativeModelChoice(t *testing.T) {
	app, output, httpClient, target := shippedHermesApp(t)
	before, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	drifted := hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
	output.Reset()
	if err := cli.Execute(app, []string{"sync", "--json"}); err != nil {
		t.Fatalf("sync after a native Hermes model choice: %v\n%s", err, output)
	}
	var result struct {
		Adopted map[string]string `json:"adopted_selections"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("sync result: %v\n%s", err, output)
	}
	if result.Adopted[hermes.ID] != "dmxapi-claude-opus-5-5" {
		t.Fatalf("adopted selections = %v", result.Adopted)
	}
	after, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := after.Clients[hermes.ID]
	if binding.Route != "dmxapi-claude-opus-5-5" || binding.Protocol != domain.ProtocolAnthropic {
		t.Fatalf("Hermes binding = %#v", binding)
	}
	for _, client := range []string{codex.ID, claude.ID, claudedesktop.ID} {
		if after.SelectedRoute(client) != before.SelectedRoute(client) {
			t.Fatalf("Hermes adoption changed %s", client)
		}
	}
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-dmxapi-anthropic" || model != "claude-opus-5-5" {
		t.Fatalf("sync replaced the native choice: %s %s", provider, model)
	}
	if !bytes.Equal(readFile(t, target), drifted) {
		t.Fatal("adoption rewrote the native Hermes configuration")
	}
	output.Reset()
	if err := cli.Execute(app, []string{"sync", "--dry-run", "--json"}); err != nil {
		t.Fatalf("second sync: %v\n%s", err, output)
	}
	var preview struct {
		Adopted map[string]string `json:"adopted_selections"`
		Targets []struct {
			Client string `json:"client"`
			Action string `json:"action"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(output.Bytes(), &preview); err != nil {
		t.Fatalf("second sync preview: %v\n%s", err, output)
	}
	for _, target := range preview.Targets {
		if target.Client == hermes.ID && target.Action != "unchanged" {
			t.Fatalf("adopted Hermes state did not converge: %+v", preview)
		}
	}
	if len(preview.Adopted) != 0 {
		t.Fatalf("second sync adopted again: %v", preview.Adopted)
	}
	if httpClient.calls != 0 || strings.Contains(output.String(), "fixture-token") {
		t.Fatal("adoption sent requests or disclosed credentials")
	}
}

func TestHermesSyncAdoptsAChoiceWhoseTokenReturnsLater(t *testing.T) {
	app, output, httpClient, target := shippedHermesAppWith(t, true)
	drifted := hermesNativeSelection(t, target, "aigw-ucloud-anthropic", "claude-opus-5-5", nil)
	stateBefore := readFile(t, target+".aigw-state.json")
	if err := app.Secrets.Delete("ucloud"); err != nil {
		t.Fatal(err)
	}
	for _, step := range []string{"token-missing", "token-restored", "converged"} {
		if step == "token-restored" {
			if err := app.Secrets.Set("ucloud", "fixture-token"); err != nil {
				t.Fatal(err)
			}
		}
		output.Reset()
		if err := cli.Execute(app, []string{"sync", "--json"}); err != nil {
			t.Fatalf("%s sync: %v\n%s", step, err, output)
		}
		var result struct {
			Adopted map[string]string `json:"adopted_selections"`
		}
		if err := json.Unmarshal(output.Bytes(), &result); err != nil {
			t.Fatalf("%s sync result: %v\n%s", step, err, output)
		}
		wantAdopted := map[string]string{}
		if step == "token-missing" {
			wantAdopted[hermes.ID] = "ucloud-claude-opus-5-5"
		}
		if len(result.Adopted) != len(wantAdopted) || result.Adopted[hermes.ID] != wantAdopted[hermes.ID] {
			t.Fatalf("%s adopted selections = %v, want %v", step, result.Adopted, wantAdopted)
		}
		cfg, err := app.Config.Load()
		if err != nil || cfg.SelectedRoute(hermes.ID) != "ucloud-claude-opus-5-5" {
			t.Fatalf("%s Hermes binding = %#v, %v", step, cfg.Clients[hermes.ID], err)
		}
		if !bytes.Equal(readFile(t, target), drifted) {
			t.Fatalf("%s sync rewrote Hermes' own selection", step)
		}
		// A missing Token defers Hermes, so its ownership record stays stale.
		if step == "token-missing" && !bytes.Equal(readFile(t, target+".aigw-state.json"), stateBefore) {
			t.Fatal("a deferred Hermes projection changed its ownership record")
		}
	}
	output.Reset()
	if err := cli.Execute(app, []string{"sync", "--dry-run", "--json"}); err != nil {
		t.Fatalf("converged preview: %v\n%s", err, output)
	}
	var preview struct {
		Targets []struct {
			Client string `json:"client"`
			Action string `json:"action"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(output.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	for _, target := range preview.Targets {
		if target.Client == hermes.ID && target.Action != "unchanged" {
			t.Fatalf("restored-Token adoption did not converge: %+v", preview.Targets)
		}
	}
	if httpClient.calls != 0 {
		t.Fatalf("adoption sent %d provider requests", httpClient.calls)
	}
}

func TestHermesSyncRefusesChangedManagedProvider(t *testing.T) {
	app, output, _, target := shippedHermesApp(t)
	before, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	drifted := hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", func(provider *yaml.Node) {
		for index := 0; index+1 < len(provider.Content); index += 2 {
			if provider.Content[index].Value == "base_url" {
				provider.Content[index+1].Value = "https://foreign.test"
			}
		}
	})
	stateBefore := readFile(t, target+".aigw-state.json")
	output.Reset()
	err = cli.Execute(app, []string{"sync"})
	if err == nil || !strings.Contains(err.Error(), "changed outside AIGW") {
		t.Fatalf("sync accepted a changed managed provider: %v\n%s", err, output)
	}
	after, loadErr := app.Config.Load()
	if loadErr != nil || after.SelectedRoute(hermes.ID) != before.SelectedRoute(hermes.ID) {
		t.Fatalf("refused sync changed the Hermes binding: %#v, %v", after.Clients[hermes.ID], loadErr)
	}
	if !bytes.Equal(readFile(t, target), drifted) || !bytes.Equal(readFile(t, target+".aigw-state.json"), stateBefore) {
		t.Fatal("refused sync changed native Hermes files")
	}
}

func TestHermesExplicitSelectionOverridesNativeModelChoice(t *testing.T) {
	app, output, _, target := shippedHermesApp(t)
	hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
	if err := cli.Execute(app, []string{"use", "--for", "hermes", "--protocol", "openai_responses", "ucloud-gpt-6.1-sol"}); err != nil {
		t.Fatalf("explicit selection after a native choice: %v\n%s", err, output)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync after explicit selection: %v\n%s", err, output)
	}
	cfg, err := app.Config.Load()
	if err != nil || cfg.SelectedRoute(hermes.ID) != "ucloud-gpt-6.1-sol" {
		t.Fatalf("explicit selection lost to the native choice: %#v, %v", cfg.Clients[hermes.ID], err)
	}
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-ucloud-openai-responses" || model != "gpt-6.1-sol" {
		t.Fatalf("Hermes did not receive the explicitly selected Route: %s %s", provider, model)
	}
}
