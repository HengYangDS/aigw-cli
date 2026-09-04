package cli_test

import (
	"aigw-cli/internal/prompt"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
	surfaceidentity "aigw-cli/internal/surface"
)

const configurationManifestFixture = `version = 4
[recommended_routes]
claude = "aihubmix-claude"
codex = "dmxapi-gpt"

[accounts.aihubmix]
label = "AIHubMix"
[accounts.aihubmix.endpoints]
openai_responses = "https://aihubmix.test/v1"
anthropic = "https://aihubmix.test"

[accounts.dmxapi]
label = "DMXAPI"
[accounts.dmxapi.endpoints]
openai_responses = "https://dmxapi.test/v1"
anthropic = "https://dmxapi.test"

[profiles.aihubmix-claude]
label = "AIHubMix Claude"
account = "aihubmix"
client = "claude"
model = "claude-test"

[profiles.dmxapi-claude]
label = "DMXAPI Claude"
account = "dmxapi"
client = "claude"
model = "claude-test"

[profiles.dmxapi-gpt]
label = "DMXAPI GPT"
account = "dmxapi"
client = "codex"
model = "gpt-test"
`

type manifestSetupPrompt struct {
	secrets     []string
	secretCalls []string
	textCalls   int
}

func (p *manifestSetupPrompt) Secret(label string) (string, error) {
	p.secretCalls = append(p.secretCalls, label)
	if len(p.secrets) == 0 {
		return "", errors.New("unexpected secret prompt")
	}
	value := p.secrets[0]
	p.secrets = p.secrets[1:]
	return value, nil
}

func (p *manifestSetupPrompt) Text(string) (string, error) {
	p.textCalls++
	return "", errors.New("unexpected text prompt")
}

func (p *manifestSetupPrompt) Select(string, []prompt.Choice) (string, error) {
	return "", errors.New("unexpected select prompt")
}

func writeConfigurationManifest(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertManifestSetupLeavesNoConfig(t *testing.T, app *cli.App) {
	t.Helper()
	for _, path := range []string{app.Config.Path(), app.Config.Path() + ".bak"} {
		if _, err := os.Stat(path); err == nil || !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unexpected configuration residue at %s: %v", path, err)
		}
	}
}

type manifestSetupFixture struct {
	app                *cli.App
	out                *bytes.Buffer
	secretStore        *secrets.MemoryStore
	runner             *fakeRunner
	prompt             *manifestSetupPrompt
	validationRequests map[string]int
}

func newManifestSetupFixture(t *testing.T) manifestSetupFixture {
	t.Helper()
	t.Setenv("AIGW_TOKEN_UNRELATED", "aigw-test-unrelated-token")
	app, out, secretStore, runner := testApp(t, "")
	prompt := &manifestSetupPrompt{secrets: []string{"aigw-test-aihubmix-token", "aigw-test-dmxapi-token"}}
	app.Interactive = true
	app.Prompt = prompt
	codexTarget := filepath.Join(t.TempDir(), "codex", "configuration.toml")
	if err := os.MkdirAll(filepath.Dir(codexTarget), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexTarget, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientClaude: "/opt/claude-real", configuration.ClientCodex: "/opt/codex-real"},
		Surfaces: []discovery.Surface{{
			ID:          string(surfaceidentity.CodexHomeDefault),
			Authority:   string(surfaceidentity.AuthorityAIGW),
			ConfigPath:  codexTarget,
			Present:     true,
			AutoManaged: true,
		}},
	}}
	requests := map[string]int{}
	app.HTTP = &fakeHTTP{handler: func(req *http.Request) (*http.Response, error) {
		auth := req.Header.Get("Authorization")
		apiKey := req.Header.Get("X-Api-Key")
		protocol := ""
		switch {
		case auth != "" && apiKey == "":
			protocol = "openai"
		case auth == "" && apiKey != "":
			protocol = "anthropic"
		default:
			t.Fatalf("credential verification headers = %#v", req.Header)
		}
		if req.URL.Path != "/v1/models" {
			t.Fatalf("credential verification URL = %s, want /v1/models", req.URL)
		}
		requests[req.URL.Host+"/"+protocol]++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Request: req}, nil
	}}
	return manifestSetupFixture{app: app, out: out, secretStore: secretStore, runner: runner, prompt: prompt, validationRequests: requests}
}

func TestSetupFromConfigurationManifestImportsWithoutTokensOrClients(t *testing.T) {
	app, out, secretStore, runner := testApp(t, "")
	app.Discovery = emptyDiscovery{}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := execute(t, app, "setup", "--from", manifestPath); err != nil {
		t.Fatal(err)
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Accounts) != 2 || len(cfg.Profiles) != 3 {
		t.Fatalf("imported catalogue = %#v", cfg)
	}
	if secretExists(t, secretStore, "aihubmix") || secretExists(t, secretStore, "dmxapi") {
		t.Fatal("catalogue import wrote a Token")
	}
	if len(cfg.Adapters) != 0 || len(runner.plans) != 0 {
		t.Fatalf("catalogue import activated absent clients: adapters=%#v plans=%#v", cfg.Adapters, runner.plans)
	}
	for _, want := range []string{
		"Configuration catalogue imported",
		"Imported capability",
		"Connected accounts",
		"0 of 2",
		"Selected routes",
		"Projected clients",
		"Connect one compatible Account",
		"aigw rotate <account>",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestSetupFromConfigurationManifestJSONReportsProgressWithoutSecrets(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	app.Discovery = emptyDiscovery{}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := execute(t, app, "setup", "--from", manifestPath, "--json"); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Imported struct {
			Accounts []string `json:"accounts"`
			Profiles []string `json:"profiles"`
		} `json:"imported"`
		ConnectedAccounts []string          `json:"connected_accounts"`
		SelectedRoutes    map[string]string `json:"selected_routes"`
		ProjectedClients  []string          `json:"projected_clients"`
		DeferredActions   []string          `json:"deferred_actions"`
		NextAction        string            `json:"next_action"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode setup JSON: %v\n%s", err, out.String())
	}
	if !slices.Equal(result.Imported.Accounts, []string{"aihubmix", "dmxapi"}) ||
		!slices.Equal(result.Imported.Profiles, []string{"aihubmix-claude", "dmxapi-claude", "dmxapi-gpt"}) ||
		len(result.ConnectedAccounts) != 0 {
		t.Fatalf("setup JSON catalogue state = %#v", result)
	}
	if result.SelectedRoutes[configuration.ClientClaude] != "aihubmix-claude" || result.SelectedRoutes[configuration.ClientCodex] != "dmxapi-gpt" {
		t.Fatalf("setup JSON routes = %#v", result.SelectedRoutes)
	}
	if len(result.ProjectedClients) != 0 {
		t.Fatalf("setup JSON projected clients = %#v", result.ProjectedClients)
	}
	wantDeferred := []string{
		"Connect one compatible Account",
		"Install Claude, then run `aigw sync`",
		"Install Codex, then run `aigw sync`",
	}
	if !slices.Equal(result.DeferredActions, wantDeferred) || result.NextAction != "aigw rotate <account>" {
		t.Fatalf("setup JSON continuation = %#v", result)
	}
	for _, forbidden := range []string{"aigw-test", "token", "secret"} {
		if strings.Contains(strings.ToLower(out.String()), forbidden) {
			t.Fatalf("setup JSON exposed credential material %q: %s", forbidden, out.String())
		}
	}
}

func TestSetupFromConfigurationManifestProjectsOnlyTheUsableClientIntersection(t *testing.T) {
	tests := []struct {
		name      string
		installed map[string]string
	}{
		{name: "neither client", installed: map[string]string{}},
		{name: "Claude only", installed: map[string]string{configuration.ClientClaude: "/opt/claude"}},
		{name: "Codex only", installed: map[string]string{configuration.ClientCodex: "/opt/codex"}},
		{name: "both clients", installed: map[string]string{configuration.ClientClaude: "/opt/claude", configuration.ClientCodex: "/opt/codex"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, out, _, _ := testApp(t, "")
			app.Secrets = secrets.NewEnvironmentStore(func(key string) string {
				if key == secrets.EnvironmentKey("dmxapi") {
					return "aigw-test-dmxapi-token"
				}
				return ""
			})
			discovered := discovery.Result{Executables: test.installed}
			if _, installed := test.installed[configuration.ClientCodex]; installed {
				target := filepath.Join(t.TempDir(), "codex", "configuration.toml")
				if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				discovered.Surfaces = []discovery.Surface{{
					ID:          string(surfaceidentity.CodexHomeDefault),
					Authority:   string(surfaceidentity.AuthorityAIGW),
					ConfigPath:  target,
					Present:     true,
					AutoManaged: true,
				}}
			}
			app.Discovery = fakeDiscovery{result: discovered}
			manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

			if err := execute(t, app, "setup", "--from", manifestPath, "--json"); err != nil {
				t.Fatal(err)
			}
			var result struct {
				SelectedRoutes   map[string]string `json:"selected_routes"`
				ProjectedClients []string          `json:"projected_clients"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("decode setup JSON: %v\n%s", err, out.String())
			}
			if result.SelectedRoutes[configuration.ClientClaude] != "dmxapi-claude" || result.SelectedRoutes[configuration.ClientCodex] != "dmxapi-gpt" {
				t.Fatalf("selected routes = %#v", result.SelectedRoutes)
			}
			cfg, err := app.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			for _, client := range configuration.AdmittedClientIDs() {
				_, installed := test.installed[client]
				if cfg.Adapters[client].Enabled != installed {
					t.Errorf("%s adapter enabled = %v, want %v", client, cfg.Adapters[client].Enabled, installed)
				}
				if slices.Contains(result.ProjectedClients, client) != installed {
					t.Errorf("%s projected = %v, want %v", client, slices.Contains(result.ProjectedClients, client), installed)
				}
			}
		})
	}
}

func TestSetupFromConfigurationManifestAcceptsOneConnectedAccount(t *testing.T) {
	app, out, store, _ := testApp(t, "")
	if err := store.Set("dmxapi", "aigw-test-dmxapi-token"); err != nil {
		t.Fatal(err)
	}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := execute(t, app, "setup", "--from", manifestPath, "--json"); err != nil {
		t.Fatal(err)
	}
	var result struct {
		ConnectedAccounts []string `json:"connected_accounts"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode setup JSON: %v\n%s", err, out.String())
	}
	if !slices.Equal(result.ConnectedAccounts, []string{"dmxapi"}) {
		t.Fatalf("connected Accounts = %#v", result.ConnectedAccounts)
	}
	if secretExists(t, store, "aihubmix") {
		t.Fatal("setup required an unrelated Account Token")
	}
}

func TestSetupFromConfigurationManifestNamesEnvironmentTokensInsteadOfRotate(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	app.Secrets = secrets.NewEnvironmentStore(func(string) string { return "" })
	app.Discovery = emptyDiscovery{}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := execute(t, app, "setup", "--from", manifestPath); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		secrets.EnvironmentKey("aihubmix"),
		secrets.EnvironmentKey("dmxapi"),
		"one compatible Account variable",
		"aigw sync",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q:\n%s", want, text)
		}
	}
	for _, forbidden := range []string{"listed environment variables", "aigw check", "aigw rotate"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("read-only environment backend received misleading guidance %q:\n%s", forbidden, text)
		}
	}
}

func TestSetupFromConfigurationManifestJSONNamesEveryEnvironmentActivationChoice(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	app.Secrets = secrets.NewEnvironmentStore(func(string) string { return "" })
	app.Discovery = emptyDiscovery{}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := execute(t, app, "setup", "--from", manifestPath, "--json"); err != nil {
		t.Fatal(err)
	}
	var result struct {
		DeferredActions []string `json:"deferred_actions"`
		NextAction      string   `json:"next_action"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode setup JSON: %v\n%s", err, out.String())
	}
	wantDeferred := []string{
		"Set one compatible Account variable: " + secrets.EnvironmentKey("aihubmix") + " or " + secrets.EnvironmentKey("dmxapi"),
		"Install Claude, then run `aigw sync`",
		"Install Codex, then run `aigw sync`",
	}
	if !slices.Equal(result.DeferredActions, wantDeferred) || result.NextAction != "aigw sync" {
		t.Fatalf("setup JSON continuation = %#v", result)
	}
	if strings.Contains(out.String(), "aigw check") {
		t.Fatalf("setup JSON implied all Tokens or verification-as-activation: %s", out.String())
	}
}

func TestSetupFromConfigurationManifestReportsInstalledClientWaitingForAnAccount(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientClaude: "/opt/claude"},
	}}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := execute(t, app, "setup", "--from", manifestPath); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Connect an Account compatible with Claude, then run `aigw sync`") {
		t.Fatalf("installed but deferred client is not explained:\n%s", out.String())
	}
}

func TestSetupFromConfigurationManifestRejectsUnknownSelectedAccountBeforeMutation(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err := execute(t, app, "setup", "--from", manifestPath, "--account", "missing")
	if err == nil || !strings.Contains(err.Error(), "unknown account") {
		t.Fatalf("error = %v", err)
	}
	if secretExists(t, secretStore, "aihubmix") || secretExists(t, secretStore, "dmxapi") {
		t.Fatal("unknown selected Account mutated credentials")
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

func TestSetupFromConfigurationManifestRefusesEnvironmentActivatedMultipleCodexTargets(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	app.Secrets = secrets.NewEnvironmentStore(func(key string) string {
		if key == secrets.EnvironmentKey("dmxapi") {
			return "aigw-test-dmxapi-token"
		}
		return ""
	})
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientCodex: "/opt/codex"},
		Surfaces: []discovery.Surface{
			{ID: string(surfaceidentity.CodexHomeDefault), Authority: string(surfaceidentity.AuthorityAIGW), ConfigPath: filepath.Join(t.TempDir(), "one.toml"), Present: true, AutoManaged: true},
			{ID: "second", Authority: string(surfaceidentity.AuthorityAIGW), ConfigPath: filepath.Join(t.TempDir(), "two.toml"), Present: true, AutoManaged: true},
		},
	}}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err := execute(t, app, "setup", "--from", manifestPath)
	if err == nil || !strings.Contains(err.Error(), "multiple auto-managed Codex targets") {
		t.Fatalf("error = %v", err)
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

func TestSetupFromConfigurationManifestConnectsOnlySelectedAccount(t *testing.T) {
	fixture := newManifestSetupFixture(t)
	fixture.prompt.secrets = []string{"aigw-test-dmxapi-token"}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := execute(t, fixture.app, "setup", "--from", manifestPath, "--account", "dmxapi"); err != nil {
		t.Fatal(err)
	}
	if got := fixture.prompt.secretCalls; len(got) != 1 || !strings.Contains(got[0], "DMXAPI") {
		t.Fatalf("secret prompts = %#v, want only DMXAPI", got)
	}
	if secretExists(t, fixture.secretStore, "aihubmix") {
		t.Fatal("setup required an unselected AIHubMix Token")
	}
	if token, err := fixture.secretStore.Get("dmxapi"); err != nil || token != "aigw-test-dmxapi-token" {
		t.Fatalf("DMXAPI Token = %q, %v", token, err)
	}
	cfg, err := fixture.app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Routes[configuration.ClientClaude] != "dmxapi-claude" || cfg.Routes[configuration.ClientCodex] != "dmxapi-gpt" {
		t.Fatalf("selected Account routes = %#v", cfg.Routes)
	}
	if !strings.Contains(fixture.out.String(), "aihubmix") || !strings.Contains(fixture.out.String(), "Deferred") {
		t.Fatalf("deferred Account is not explained:\n%s", fixture.out.String())
	}
}

func TestSetupFromConfigurationManifestReportsSelectedRoutesAndProjectedClients(t *testing.T) {
	fixture := newManifestSetupFixture(t)
	fixture.prompt.secrets = []string{"aigw-test-dmxapi-token"}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := execute(t, fixture.app, "setup", "--from", manifestPath, "--account", "dmxapi"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Selected routes",
		"Claude",
		"dmxapi-claude",
		"Codex",
		"dmxapi-gpt",
		"Projected",
	} {
		if !strings.Contains(fixture.out.String(), want) {
			t.Errorf("setup output missing %q:\n%s", want, fixture.out.String())
		}
	}
}

func TestSetupFromConfigurationManifestLeavesNoConfigWhenTokenStorageFails(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &manifestSetupPrompt{secrets: []string{"aigw-test-dmxapi-token"}}
	want := errors.New("credential store unavailable")
	app.Secrets = &failingSecretsStore{getErr: secrets.ErrNotFound, setErr: want}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err := execute(t, app, "setup", "--from", manifestPath, "--account", "dmxapi")
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

func TestSetupFromConfigurationManifestUsesAnyAvailableEnvironmentToken(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	app.Secrets = secrets.NewEnvironmentStore(func(key string) string {
		if key == secrets.EnvironmentKey("dmxapi") {
			return "aigw-test-dmxapi-env-token"
		}
		return ""
	})
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := execute(t, app, "setup", "--from", manifestPath); err != nil {
		t.Fatal(err)
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Routes[configuration.ClientClaude] != "dmxapi-claude" || cfg.Routes[configuration.ClientCodex] != "dmxapi-gpt" {
		t.Fatalf("available Account did not become usable: %#v", cfg.Routes)
	}
	for _, want := range []string{"Install Claude, then run `aigw sync`", "Install Codex, then run `aigw sync`", "Next", "aigw sync"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "aigw status") {
		t.Fatalf("setup pointed to an observational command instead of activation:\n%s", out.String())
	}
}

func TestSetupFromConfigurationManifestRequiresAccountForTokenStdin(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "aigw-test-one-token\n")
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err := execute(t, app, "setup", "--from", manifestPath, "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "--account") {
		t.Fatalf("error = %v", err)
	}
	if secretExists(t, secretStore, "aihubmix") || secretExists(t, secretStore, "dmxapi") {
		t.Fatal("ambiguous stdin Token was stored")
	}
}

func TestSetupFromConfigurationManifestConnectsOneAccountAndKeepsItsTokenSecret(t *testing.T) {
	fixture := newManifestSetupFixture(t)
	app, out, secretStore, runner, prompt := fixture.app, fixture.out, fixture.secretStore, fixture.runner, fixture.prompt
	prompt.secrets = []string{"aigw-test-dmxapi-token"}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := execute(t, app, "setup", "--from", manifestPath, "--account", "dmxapi"); err != nil {
		t.Fatal(err)
	}
	if prompt.textCalls != 0 {
		t.Fatalf("endpoint/profile text prompts = %d, want 0", prompt.textCalls)
	}
	if len(prompt.secretCalls) != 1 || !strings.Contains(prompt.secretCalls[0], "DMXAPI") {
		t.Fatalf("secret prompts = %#v, want only the selected Account", prompt.secretCalls)
	}
	if secretExists(t, secretStore, "aihubmix") {
		t.Fatal("setup stored an unselected Account Token")
	}
	if got, err := secretStore.Get("dmxapi"); err != nil || got != "aigw-test-dmxapi-token" {
		t.Fatalf("dmxapi token = %q, %v", got, err)
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Accounts) != 2 || len(cfg.Profiles) != 3 || cfg.Routes[configuration.ClientCodex] != "dmxapi-gpt" {
		t.Fatalf("team config = %#v", cfg)
	}
	if cfg.Routes["claude"] != "dmxapi-claude" || cfg.Routes["codex"] != "dmxapi-gpt" {
		t.Fatalf("connected Account routes = %#v", cfg.Routes)
	}
	if !cfg.Adapters["claude"].Enabled || !cfg.Adapters["codex"].Enabled {
		t.Fatalf("discovered clients were not configured: %#v", cfg.Adapters)
	}
	if len(runner.plans) != 1 || runner.plans[0].Executable != "/opt/codex-real" {
		executables := make([]string, 0, len(runner.plans))
		for _, plan := range runner.plans {
			executables = append(executables, plan.Executable)
		}
		t.Fatalf("client plan executables = %#v", executables)
	}
	if len(runner.captureDeadlines) != 0 {
		t.Fatalf("setup ran a live client verification: %#v", runner.captureDeadlines)
	}
	wantValidationRequests := map[string]int{
		"dmxapi.test/anthropic": 1,
		"dmxapi.test/openai":    1,
	}
	for key, want := range wantValidationRequests {
		if fixture.validationRequests[key] != want {
			t.Errorf("validation request %s = %d, want %d; all=%#v", key, fixture.validationRequests[key], want, fixture.validationRequests)
		}
	}
	if len(fixture.validationRequests) != len(wantValidationRequests) {
		t.Fatalf("unexpected validation requests: %#v", fixture.validationRequests)
	}
	if cfg.Profiles["aihubmix-claude"].Model != "claude-test" || cfg.Profiles["dmxapi-gpt"].Model != "gpt-test" {
		t.Fatalf("manifest model matrix was not preserved: %#v", cfg.Profiles)
	}

	assertManifestSetupDoesNotLeakTokens(t, app, out)
}

func assertManifestSetupDoesNotLeakTokens(t *testing.T, app *cli.App, out *bytes.Buffer) {
	t.Helper()
	for _, path := range []string{app.Config.Path(), app.Config.Path() + ".bak"} {
		data, readErr := os.ReadFile(path)
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, token := range []string{"aigw-test-aihubmix-token", "aigw-test-dmxapi-token"} {
			if strings.Contains(string(data), token) {
				t.Fatalf("token leaked to %s", path)
			}
		}
	}
	if strings.Contains(out.String(), "aigw-test-") {
		t.Fatalf("token leaked to output: %s", out.String())
	}
	out.Reset()
	if err := execute(t, app, "config", "export"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "aigw-test-") {
		t.Fatalf("token leaked to export: %s", out.String())
	}
}

func TestSetupFromConfigurationManifestReusesEnvironmentTokensWithoutPrompting(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	app.Secrets = secrets.NewEnvironmentStore(func(key string) string {
		switch key {
		case secrets.EnvironmentKey("aihubmix"):
			return "aigw-test-aihubmix-env-token"
		case secrets.EnvironmentKey("dmxapi"):
			return "aigw-test-dmxapi-env-token"
		default:
			return ""
		}
	})
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := execute(t, app, "setup", "--from", manifestPath); err != nil {
		t.Fatal(err)
	}
	cfg, err := app.Config.Load()
	if err != nil || len(cfg.Profiles) != 3 || cfg.Routes[configuration.ClientCodex] != "dmxapi-gpt" {
		t.Fatalf("team config = %#v, %v", cfg, err)
	}
}

func TestSetupFromConfigurationManifestRejectsCredentialsBeforePromptOrWrite(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	app.Interactive = true
	prompt := &manifestSetupPrompt{secrets: []string{"must-not-be-read"}}
	app.Prompt = prompt
	manifestPath := writeConfigurationManifest(t, strings.Replace(configurationManifestFixture, "label = \"AIHubMix\"", "label = \"AIHubMix\"\ntoken = \"forbidden\"", 1))

	err := execute(t, app, "setup", "--from", manifestPath)
	if err == nil || !strings.Contains(err.Error(), "forbidden credential") {
		t.Fatalf("error = %v", err)
	}
	if len(prompt.secretCalls) != 0 || secretExists(t, secretStore, "aihubmix") || secretExists(t, secretStore, "dmxapi") {
		t.Fatalf("invalid manifest touched credentials: prompts=%#v", prompt.secretCalls)
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

func TestSetupFromConfigurationManifestNonInteractiveImportDoesNotRequireTokens(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := execute(t, app, "setup", "--from", manifestPath); err != nil {
		t.Fatal(err)
	}
	if secretExists(t, secretStore, "aihubmix") || secretExists(t, secretStore, "dmxapi") {
		t.Fatal("non-interactive rejection wrote a token")
	}
	if cfg, err := app.Config.Load(); err != nil || len(cfg.Profiles) != 3 {
		t.Fatalf("imported config = %#v, %v", cfg, err)
	}
}

func TestSetupFromConfigurationManifestRejectsStdinTokenWithoutAccountOwner(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "aigw-test-one-token\n")
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err := execute(t, app, "setup", "--from", manifestPath, "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "--account") {
		t.Fatalf("error = %v", err)
	}
	if secretExists(t, secretStore, "aihubmix") || secretExists(t, secretStore, "dmxapi") {
		t.Fatal("ambiguous stdin token was stored")
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

func TestSetupFromConfigurationManifestRejectsExplicitEmptySingleProfileFlag(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err := execute(t, app, "setup", "--from", manifestPath, "--account=")
	if err == nil || !strings.Contains(err.Error(), "requires an Account ID") {
		t.Fatalf("error = %v", err)
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

func TestSetupFromConfigurationManifestRefusesMultipleCodexTargetsBeforePromptOrWrite(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	app.Interactive = true
	prompt := &manifestSetupPrompt{secrets: []string{"must-not-be-read"}}
	app.Prompt = prompt
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientCodex: "/opt/codex-real"},
		Surfaces: []discovery.Surface{
			{ID: string(surfaceidentity.CodexHomeDefault), Authority: string(surfaceidentity.AuthorityAIGW), ConfigPath: filepath.Join(t.TempDir(), "one", "configuration.toml"), Present: true, AutoManaged: true},
			{ID: "second-codex", Authority: string(surfaceidentity.AuthorityAIGW), ConfigPath: filepath.Join(t.TempDir(), "two", "configuration.toml"), Present: true, AutoManaged: true},
		},
	}}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err := execute(t, app, "setup", "--from", manifestPath, "--account", "dmxapi")
	if err == nil || !strings.Contains(err.Error(), "multiple auto-managed Codex targets") {
		t.Fatalf("error = %v", err)
	}
	if len(prompt.secretCalls) != 0 || secretExists(t, secretStore, "aihubmix") || secretExists(t, secretStore, "dmxapi") {
		t.Fatalf("multi-target preflight touched credentials: prompts=%#v", prompt.secretCalls)
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

func TestSetupFromConfigurationManifestValidationFailureLeavesNoCredentialsOrConfig(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &manifestSetupPrompt{secrets: []string{"aigw-test-dmxapi-token"}}
	requests := 0
	app.HTTP = &fakeHTTP{handler: func(req *http.Request) (*http.Response, error) {
		requests++
		status := http.StatusOK
		status = http.StatusUnauthorized
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("{}")), Request: req}, nil
	}}
	target := filepath.Join(t.TempDir(), "codex", "configuration.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientCodex: "/opt/codex-real"},
		Surfaces: []discovery.Surface{{
			ID:          string(surfaceidentity.CodexHomeDefault),
			Authority:   string(surfaceidentity.AuthorityAIGW),
			ConfigPath:  target,
			Present:     true,
			AutoManaged: true,
		}},
	}}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err := execute(t, app, "setup", "--from", manifestPath, "--account", "dmxapi")
	if err == nil || !strings.Contains(err.Error(), "Token validation failed") {
		t.Fatalf("error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("validation requests = %d, want 1", requests)
	}
	if secretExists(t, secretStore, "aihubmix") || secretExists(t, secretStore, "dmxapi") {
		t.Fatal("failed validation left a token")
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

func TestSetupFromConfigurationManifestDefersValidationWhenClientIsAbsent(t *testing.T) {
	for _, status := range []int{http.StatusFound, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			app, _, secretStore, _ := testApp(t, "")
			app.Interactive = true
			app.Prompt = &manifestSetupPrompt{secrets: []string{"aigw-test-aihubmix-token"}}
			app.HTTP = &fakeHTTP{handler: func(req *http.Request) (*http.Response, error) {
				responseStatus := http.StatusOK
				if req.Header.Get("X-Api-Key") != "" {
					responseStatus = status
				}
				return &http.Response{StatusCode: responseStatus, Body: io.NopCloser(strings.NewReader("{}")), Request: req}, nil
			}}
			manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

			if err := execute(t, app, "setup", "--from", manifestPath, "--account", "aihubmix"); err != nil {
				t.Fatal(err)
			}
			if !secretExists(t, secretStore, "aihubmix") || secretExists(t, secretStore, "dmxapi") {
				t.Fatal("setup did not preserve only the explicitly connected Account")
			}
		})
	}
}

func TestSetupFromConfigurationManifestDoesNotFollowCredentialProbeRedirects(t *testing.T) {
	targetSawToken := false
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		targetSawToken = req.Header.Get("X-Api-Key") != ""
	}))
	defer target.Close()
	redirectTarget := strings.Replace(target.URL, "127.0.0.1", "localhost", 1)
	source := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, req *http.Request) {
		http.Redirect(response, req, redirectTarget, http.StatusFound)
	}))
	defer source.Close()

	app, _, secretStore, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &manifestSetupPrompt{secrets: []string{"aigw-test-team-token"}}
	app.HTTP = &http.Client{}
	manifestPath := writeConfigurationManifest(t, `version = 4
[recommended_routes]
claude = "team-claude"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "`+source.URL+`"
[profiles.team-claude]
label = "Team Claude"
account = "team"
client = "claude"
model = "claude-test"
`)

	if err := execute(t, app, "setup", "--from", manifestPath, "--account", "team"); err != nil {
		t.Fatal(err)
	}
	if targetSawToken {
		t.Fatal("credential probe forwarded X-Api-Key across a redirect")
	}
	if !secretExists(t, secretStore, "team") {
		t.Fatal("explicitly connected Account Token was not stored")
	}
}

func TestSetupFromConfigurationManifestRejectsUnreferencedAccountBeforePrompt(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	app.Interactive = true
	prompt := &manifestSetupPrompt{secrets: []string{"must-not-be-read"}}
	app.Prompt = prompt
	manifestPath := writeConfigurationManifest(t, `version = 4
[recommended_routes]
claude = "used"
[accounts.used]
label = "Used"
[accounts.used.endpoints]
anthropic = "https://used.test"
[accounts.unused]
label = "Unused"
[accounts.unused.endpoints]
anthropic = "https://unused.test"
[profiles.used]
label = "Used"
account = "used"
client = "claude"
model = "claude-test"
`)

	err := execute(t, app, "setup", "--from", manifestPath)
	if err == nil || !strings.Contains(err.Error(), "Account \"unused\" is not referenced") {
		t.Fatalf("error = %v", err)
	}
	if len(prompt.secretCalls) != 0 || secretExists(t, secretStore, "used") || secretExists(t, secretStore, "unused") {
		t.Fatalf("unreferenced Account preflight touched credentials: prompts=%#v", prompt.secretCalls)
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

func TestSetupFromConfigurationManifestRejectsProfileWithoutClient(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	manifestPath := writeConfigurationManifest(t, `version = 4
[recommended_routes]
claude = "team"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
openai_responses = "https://team.test/v1"
anthropic = "https://team.test"
[profiles.team]
label = "Team"
account = "team"
`)

	err := execute(t, app, "setup", "--from", manifestPath, "--account", "team")
	if err == nil || !strings.Contains(err.Error(), `profile "team" has unknown client ""`) {
		t.Fatalf("error = %v", err)
	}
	if secretExists(t, secretStore, "team") {
		t.Fatal("invalid manifest wrote a Token")
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

func TestSetupFromConfigurationManifestClientFailureRollsBackCredentialsAndConfig(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	secretsRoot := filepath.Join(t.TempDir(), "secrets")
	secretStore, err := secrets.Select(secrets.Selection{
		GOOS:         runtime.GOOS,
		Root:         secretsRoot,
		KeyringProbe: func(secrets.Store) error { return errors.New("native credential service unavailable") },
	})
	if err != nil {
		t.Fatal(err)
	}
	app.Secrets = secretStore
	app.Interactive = true
	app.Prompt = &manifestSetupPrompt{secrets: []string{"aigw-test-dmxapi-token"}}
	app.Runner = &failingRunner{err: errors.New("Codex login failed"), remaining: 1}
	codexTarget := filepath.Join(t.TempDir(), "configuration.toml")
	original := "model_provider = \"native\"\n"
	if err := os.WriteFile(codexTarget, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientClaude: "/opt/claude-real", configuration.ClientCodex: "/opt/codex-real"},
		Surfaces: []discovery.Surface{{
			ID:          string(surfaceidentity.CodexHomeDefault),
			Authority:   string(surfaceidentity.AuthorityAIGW),
			ConfigPath:  codexTarget,
			Present:     true,
			AutoManaged: true,
		}},
	}}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err = execute(t, app, "setup", "--from", manifestPath, "--account", "dmxapi")
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("error = %v", err)
	}
	if secretExists(t, secretStore, "aihubmix") || secretExists(t, secretStore, "dmxapi") {
		t.Fatal("failed setup left an Account Token")
	}
	if _, err := os.Stat(filepath.Join(secretsRoot, "backend")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed setup left the automatic backend selection: %v", err)
	}
	assertManifestSetupLeavesNoConfig(t, app)
	data, err := os.ReadFile(codexTarget)
	if err != nil || string(data) != original {
		t.Fatalf("failed setup changed Codex config: %q, %v", data, err)
	}
	assertSetupTransactionClosed(
		t,
		app.Config,
		codexTarget+".aigw-state.json",
		app.ClaudeSettingsPath,
		app.ClaudeSettingsPath+".aigw-state.json",
	)
}

func TestSetupFromConfigurationManifestOutputFailureKeepsCommittedAutomaticBackendSelection(t *testing.T) {
	app, _, _, _ := testApp(t, "token\n")
	secretsRoot := filepath.Join(t.TempDir(), "secrets")
	secretStore, err := secrets.Select(secrets.Selection{
		GOOS:         runtime.GOOS,
		Root:         secretsRoot,
		KeyringProbe: func(secrets.Store) error { return errors.New("native credential service unavailable") },
	})
	if err != nil {
		t.Fatal(err)
	}
	app.Secrets = secretStore
	want := errors.New("output failed")
	app.Out = failingOutput{err: want}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err = execute(t, app, "setup", "--from", manifestPath, "--account", "dmxapi", "--token-stdin", "--json")
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if !secretExists(t, secretStore, "dmxapi") {
		t.Fatal("output failure removed the committed Account Token")
	}
	selected, err := os.ReadFile(filepath.Join(secretsRoot, "backend"))
	if err != nil || string(selected) != "file\n" {
		t.Fatalf("committed backend selection = %q, %v; want file", selected, err)
	}
	if _, err := app.Config.Load(); err != nil {
		t.Fatalf("output failure removed the committed configuration: %v", err)
	}
}

func TestSetupFromConfigurationManifestClientFailurePreservesForeignClaudeExecutable(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &manifestSetupPrompt{secrets: []string{"aigw-test-dmxapi-token"}}
	app.Runner = &failingRunner{err: errors.New("Codex login failed"), remaining: 1}
	claudeExecutable := executableFixture(t, "claude")
	originalExecutable, err := os.ReadFile(claudeExecutable)
	if err != nil {
		t.Fatal(err)
	}
	codexTarget := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(codexTarget, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientClaude: claudeExecutable, configuration.ClientCodex: "/opt/codex-real"},
		Surfaces: []discovery.Surface{{
			ID:          string(surfaceidentity.CodexHomeDefault),
			Authority:   string(surfaceidentity.AuthorityAIGW),
			ConfigPath:  codexTarget,
			Present:     true,
			AutoManaged: true,
		}},
	}}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err = execute(t, app, "setup", "--from", manifestPath, "--account", "dmxapi")
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("error = %v", err)
	}
	if secretExists(t, secretStore, "aihubmix") || secretExists(t, secretStore, "dmxapi") {
		t.Fatal("failed setup left an Account Token")
	}
	gotExecutable, readErr := os.ReadFile(claudeExecutable)
	if readErr != nil || string(gotExecutable) != string(originalExecutable) {
		t.Fatalf("foreign Claude executable changed: error=%v bytes=%q", readErr, gotExecutable)
	}
}
