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
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
	surfaceidentity "aigw-cli/internal/surface"
)

const configurationManifestFixture = `version = 3
recommended_default = "dmxapi-gpt"

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
[profiles.aihubmix-claude.models]
claude = "claude-test"

[profiles.dmxapi-claude]
label = "DMXAPI Claude"
account = "dmxapi"
client = "claude"
[profiles.dmxapi-claude.models]
claude = "claude-test"

[profiles.dmxapi-gpt]
label = "DMXAPI GPT"
account = "dmxapi"
client = "codex"
[profiles.dmxapi-gpt.models]
codex = "gpt-test"
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
	if secretStore.Has("aihubmix") || secretStore.Has("dmxapi") {
		t.Fatal("catalogue import wrote a Token")
	}
	if len(cfg.Adapters) != 0 || len(runner.plans) != 0 {
		t.Fatalf("catalogue import activated absent clients: adapters=%#v plans=%#v", cfg.Adapters, runner.plans)
	}
	for _, want := range []string{"Configuration catalogue imported", "Connected accounts", "0 of 2", "Clients", "Not installed", "aigw rotate <account>"} {
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
		Catalogue struct {
			Accounts int `json:"accounts"`
			Profiles int `json:"profiles"`
		} `json:"catalogue"`
		Accounts []struct {
			ID        string `json:"id"`
			Connected bool   `json:"connected"`
		} `json:"accounts"`
		Routes     map[string]string `json:"routes"`
		Clients    map[string]string `json:"clients"`
		Deferred   []string          `json:"deferred"`
		NextAction string            `json:"next_action"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode setup JSON: %v\n%s", err, out.String())
	}
	if result.Catalogue.Accounts != 2 || result.Catalogue.Profiles != 3 || len(result.Accounts) != 2 {
		t.Fatalf("setup JSON catalogue state = %#v", result)
	}
	for _, account := range result.Accounts {
		if account.Connected {
			t.Fatalf("setup JSON unexpectedly connected Account %#v", account)
		}
	}
	if result.Routes[configuration.ClientClaude] != "aihubmix-claude" || result.Routes[configuration.ClientCodex] != "dmxapi-gpt" {
		t.Fatalf("setup JSON routes = %#v", result.Routes)
	}
	if result.Clients[configuration.ClientClaude] != "not_installed" || result.Clients[configuration.ClientCodex] != "not_installed" {
		t.Fatalf("setup JSON clients = %#v", result.Clients)
	}
	if len(result.Deferred) != 0 || result.NextAction != "aigw rotate <account>" {
		t.Fatalf("setup JSON continuation = %#v", result)
	}
	for _, forbidden := range []string{"aigw-test", "token", "secret"} {
		if strings.Contains(strings.ToLower(out.String()), forbidden) {
			t.Fatalf("setup JSON exposed credential material %q: %s", forbidden, out.String())
		}
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
	for _, want := range []string{secrets.EnvironmentKey("aihubmix"), secrets.EnvironmentKey("dmxapi"), "aigw use dmxapi-gpt"} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "aigw rotate") {
		t.Fatalf("read-only environment backend received impossible rotate guidance:\n%s", text)
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
	if !strings.Contains(out.String(), "Installed · connect a compatible Account to configure") {
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
	if secretStore.Has("aihubmix") || secretStore.Has("dmxapi") {
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
	if fixture.secretStore.Has("aihubmix") {
		t.Fatal("setup required an unselected AIHubMix Token")
	}
	if token, err := fixture.secretStore.Get("dmxapi"); err != nil || token != "aigw-test-dmxapi-token" {
		t.Fatalf("DMXAPI Token = %q, %v", token, err)
	}
	cfg, err := fixture.app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Routes.Overrides[configuration.ClientClaude] != "dmxapi-claude" || cfg.Routes.Overrides[configuration.ClientCodex] != "dmxapi-gpt" {
		t.Fatalf("selected Account routes = %#v", cfg.Routes)
	}
	if !strings.Contains(fixture.out.String(), "aihubmix") || !strings.Contains(fixture.out.String(), "Not connected") {
		t.Fatalf("deferred Account is not explained:\n%s", fixture.out.String())
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
	app, _, _, _ := testApp(t, "")
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
	if cfg.Routes.Overrides[configuration.ClientClaude] != "dmxapi-claude" || cfg.Routes.Overrides[configuration.ClientCodex] != "dmxapi-gpt" {
		t.Fatalf("available Account did not become usable: %#v", cfg.Routes)
	}
}

func TestSetupFromConfigurationManifestRequiresAccountForTokenStdin(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "aigw-test-one-token\n")
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err := execute(t, app, "setup", "--from", manifestPath, "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "--account") {
		t.Fatalf("error = %v", err)
	}
	if secretStore.Has("aihubmix") || secretStore.Has("dmxapi") {
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
	if secretStore.Has("aihubmix") {
		t.Fatal("setup stored an unselected Account Token")
	}
	if got, err := secretStore.Get("dmxapi"); err != nil || got != "aigw-test-dmxapi-token" {
		t.Fatalf("dmxapi token = %q, %v", got, err)
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Accounts) != 2 || len(cfg.Profiles) != 3 || cfg.Routes.Default != "dmxapi-gpt" {
		t.Fatalf("team config = %#v", cfg)
	}
	if cfg.Routes.Overrides["claude"] != "dmxapi-claude" || cfg.Routes.Overrides["codex"] != "dmxapi-gpt" {
		t.Fatalf("connected Account routes = %#v", cfg.Routes.Overrides)
	}
	if !cfg.Adapters["claude"].Enabled || !cfg.Adapters["codex"].Enabled {
		t.Fatalf("discovered clients were not configured: %#v", cfg.Adapters)
	}
	if len(runner.plans) != 2 || runner.plans[0].Executable != "/opt/claude-real" || runner.plans[1].Executable != "/opt/codex-real" {
		executables := make([]string, 0, len(runner.plans))
		for _, plan := range runner.plans {
			executables = append(executables, plan.Executable)
		}
		t.Fatalf("client plan executables = %#v", executables)
	}
	if len(runner.captureDeadlines) != 1 || !runner.captureDeadlines[0] {
		t.Fatalf("Claude validation deadlines = %#v", runner.captureDeadlines)
	}
	for _, plan := range runner.plans[:1] {
		for _, value := range plan.Env {
			if strings.HasPrefix(value, "AIGW_TOKEN_") || strings.Contains(value, "aigw-test-unrelated-token") {
				t.Fatal("Claude validation inherited an unrelated Account Token")
			}
		}
	}
	wantValidationRequests := map[string]int{
		"dmxapi.test/openai": 1,
	}
	for key, want := range wantValidationRequests {
		if fixture.validationRequests[key] != want {
			t.Errorf("validation request %s = %d, want %d; all=%#v", key, fixture.validationRequests[key], want, fixture.validationRequests)
		}
	}
	if len(fixture.validationRequests) != len(wantValidationRequests) {
		t.Fatalf("unexpected validation requests: %#v", fixture.validationRequests)
	}
	if cfg.Profiles["aihubmix-claude"].Models["claude"] != "claude-test" || cfg.Profiles["dmxapi-gpt"].Models["codex"] != "gpt-test" {
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
	if err != nil || len(cfg.Profiles) != 3 || cfg.Routes.Default != "dmxapi-gpt" {
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
	if len(prompt.secretCalls) != 0 || secretStore.Has("aihubmix") || secretStore.Has("dmxapi") {
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
	if secretStore.Has("aihubmix") || secretStore.Has("dmxapi") {
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
	if secretStore.Has("aihubmix") || secretStore.Has("dmxapi") {
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
	if len(prompt.secretCalls) != 0 || secretStore.Has("aihubmix") || secretStore.Has("dmxapi") {
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
	if secretStore.Has("aihubmix") || secretStore.Has("dmxapi") {
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
			if !secretStore.Has("aihubmix") || secretStore.Has("dmxapi") {
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
	manifestPath := writeConfigurationManifest(t, `version = 3
recommended_default = "team-claude"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
anthropic = "`+source.URL+`"
[profiles.team-claude]
label = "Team Claude"
account = "team"
client = "claude"
[profiles.team-claude.models]
claude = "claude-test"
`)

	if err := execute(t, app, "setup", "--from", manifestPath, "--account", "team"); err != nil {
		t.Fatal(err)
	}
	if targetSawToken {
		t.Fatal("credential probe forwarded X-Api-Key across a redirect")
	}
	if !secretStore.Has("team") {
		t.Fatal("explicitly connected Account Token was not stored")
	}
}

func TestSetupFromConfigurationManifestRejectsUnreferencedAccountBeforePrompt(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	app.Interactive = true
	prompt := &manifestSetupPrompt{secrets: []string{"must-not-be-read"}}
	app.Prompt = prompt
	manifestPath := writeConfigurationManifest(t, `version = 3
recommended_default = "used"
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
[profiles.used.models]
claude = "claude-test"
`)

	err := execute(t, app, "setup", "--from", manifestPath)
	if err == nil || !strings.Contains(err.Error(), "Account \"unused\" is not referenced") {
		t.Fatalf("error = %v", err)
	}
	if len(prompt.secretCalls) != 0 || secretStore.Has("used") || secretStore.Has("unused") {
		t.Fatalf("unreferenced Account preflight touched credentials: prompts=%#v", prompt.secretCalls)
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

func TestSetupFromLegacyGenericProfileValidatesBothAccountProtocols(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &manifestSetupPrompt{secrets: []string{"aigw-test-team-token"}}
	requests := map[string]int{}
	app.HTTP = &fakeHTTP{handler: func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("X-Api-Key") != "" {
			requests["anthropic"]++
		}
		if req.Header.Get("Authorization") != "" {
			requests["openai"]++
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Request: req}, nil
	}}
	manifestPath := writeConfigurationManifest(t, `version = 3
recommended_default = "team"
[accounts.team]
label = "Team"
[accounts.team.endpoints]
openai_responses = "https://team.test/v1"
anthropic = "https://team.test"
[profiles.team]
label = "Team"
account = "team"
`)

	if err := execute(t, app, "setup", "--from", manifestPath, "--account", "team"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 0 {
		t.Fatalf("absent clients triggered validation requests = %#v", requests)
	}
}

func TestSetupFromConfigurationManifestClientFailureRollsBackCredentialsAndConfig(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
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

	err := execute(t, app, "setup", "--from", manifestPath, "--account", "dmxapi")
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("error = %v", err)
	}
	if secretStore.Has("aihubmix") || secretStore.Has("dmxapi") {
		t.Fatal("failed setup left an Account Token")
	}
	assertManifestSetupLeavesNoConfig(t, app)
	data, err := os.ReadFile(codexTarget)
	if err != nil || string(data) != original {
		t.Fatalf("failed setup changed Codex config: %q, %v", data, err)
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
	if secretStore.Has("aihubmix") || secretStore.Has("dmxapi") {
		t.Fatal("failed setup left an Account Token")
	}
	gotExecutable, readErr := os.ReadFile(claudeExecutable)
	if readErr != nil || string(gotExecutable) != string(originalExecutable) {
		t.Fatalf("foreign Claude executable changed: error=%v bytes=%q", readErr, gotExecutable)
	}
}
