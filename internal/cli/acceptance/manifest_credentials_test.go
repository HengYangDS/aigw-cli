package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
	surfaceidentity "aigw-cli/internal/surface"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSetupFromConfigurationManifestNamesEnvironmentTokensInsteadOfRotate(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	app.Secrets = secrets.NewEnvironmentStore(func(string) string { return "" })
	app.Discovery = fakeDiscovery{}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := cli.Execute(app, []string{"setup", "--from", manifestPath}); err != nil {
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
	app, out, _, _, _ := testApp(t, "")
	app.Secrets = secrets.NewEnvironmentStore(func(string) string { return "" })
	app.Discovery = fakeDiscovery{}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := cli.Execute(app, []string{"setup", "--from", manifestPath, "--json"}); err != nil {
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

func TestSetupFromConfigurationManifestLeavesNoConfigWhenTokenStorageFails(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &scriptedPrompt{secrets: []string{"aigw-test-dmxapi-token"}}
	want := errors.New("credential store unavailable")
	app.Secrets = &recordingCredentialStore[string]{backend: secrets.NewMemoryStore(), setErr: want}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err := cli.Execute(app, []string{"setup", "--from", manifestPath, "--account", "dmxapi"})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

func TestSetupFromConfigurationManifestUsesAnyAvailableEnvironmentToken(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	app.Secrets = secrets.NewEnvironmentStore(func(key string) string {
		if key == secrets.EnvironmentKey("dmxapi") {
			return "aigw-test-dmxapi-env-token"
		}
		return ""
	})
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := cli.Execute(app, []string{"setup", "--from", manifestPath}); err != nil {
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

func TestSetupFromConfigurationManifestConnectsOneAccountAndKeepsItsTokenSecret(t *testing.T) {
	t.Setenv("AIGW_TOKEN_UNRELATED", "aigw-test-unrelated-token")
	app, out, secretStore, runner, _ := testApp(t, "")
	prompt := &scriptedPrompt{secrets: []string{"aigw-test-dmxapi-token"}}
	app.Interactive = true
	app.Prompt = prompt
	codexTarget := filepath.Join(t.TempDir(), "codex", "configuration.toml")
	writeFile(t, codexTarget, []byte("model_provider = \"native\"\n"), 0o600)
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
		if (auth == "") == (apiKey == "") {
			t.Fatalf("credential verification requires exactly one authentication header: %#v", req.Header)
		}
		protocol := "openai"
		if apiKey != "" {
			protocol = "anthropic"
		}
		if req.URL.Path != "/v1/models" {
			t.Fatalf("credential verification URL = %s, want /v1/models", req.URL)
		}
		requests[req.URL.Host+"/"+protocol]++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Request: req}, nil
	}}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := cli.Execute(app, []string{"setup", "--from", manifestPath, "--account", "dmxapi"}); err != nil {
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
	if len(cfg.Accounts) != 2 {
		t.Fatalf("team config = %#v", cfg)
	}
	if !maps.Equal(cfg.Routes, map[string]string{"claude": "dmxapi-claude", "codex": "dmxapi-gpt"}) {
		t.Fatalf("connected Account routes = %#v", cfg.Routes)
	}
	if !cfg.Adapters["claude"].Enabled || !cfg.Adapters["codex"].Enabled {
		t.Fatalf("discovered clients were not configured: %#v", cfg.Adapters)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("setup invoked a client: %#v", runner.plans)
	}
	projection, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	projected := readFile(t, codexTarget)
	if !strings.Contains(string(projected), projection.CredentialProjectionFingerprint(configuration.ClientCodex)) {
		t.Fatalf("credential helper projection = %s", projected)
	}
	for _, want := range []string{"Selected routes", "Claude", "dmxapi-claude", "Codex", "dmxapi-gpt", "Projected", "aihubmix", "Deferred"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("setup output missing %q:\n%s", want, out.String())
		}
	}
	wantValidationRequests := map[string]int{
		"dmxapi.test/anthropic": 1,
		"dmxapi.test/openai":    1,
	}
	if !maps.Equal(requests, wantValidationRequests) {
		t.Fatalf("validation requests = %#v, want %#v", requests, wantValidationRequests)
	}
	wantProfiles := map[string]configuration.Profile{
		"aihubmix-claude": {Label: "AIHubMix Claude", Account: "aihubmix", Client: "claude", Model: "claude-test"},
		"dmxapi-claude":   {Label: "DMXAPI Claude", Account: "dmxapi", Client: "claude", Model: "claude-test"},
		"dmxapi-gpt":      {Label: "DMXAPI GPT", Account: "dmxapi", Client: "codex", Model: "gpt-test"},
	}
	if !maps.Equal(cfg.Profiles, wantProfiles) {
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
	if err := cli.Execute(app, []string{"config", "export"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "aigw-test-") {
		t.Fatalf("token leaked to export: %s", out.String())
	}
}

func TestSetupFromConfigurationManifestReusesEnvironmentTokensWithoutPrompting(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
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

	if err := cli.Execute(app, []string{"setup", "--from", manifestPath}); err != nil {
		t.Fatal(err)
	}
	cfg, err := app.Config.Load()
	if err != nil || len(cfg.Profiles) != 3 || cfg.Routes[configuration.ClientCodex] != "dmxapi-gpt" {
		t.Fatalf("team config = %#v, %v", cfg, err)
	}
}

func TestSetupFromConfigurationManifestRejectsCredentialsBeforePromptOrWrite(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	app.Interactive = true
	prompt := &scriptedPrompt{secrets: []string{"must-not-be-read"}}
	app.Prompt = prompt
	manifestPath := writeConfigurationManifest(t, strings.Replace(configurationManifestFixture, "label = \"AIHubMix\"", "label = \"AIHubMix\"\ntoken = \"forbidden\"", 1))

	err := cli.Execute(app, []string{"setup", "--from", manifestPath})
	if err == nil || !strings.Contains(err.Error(), "forbidden credential") {
		t.Fatalf("error = %v", err)
	}
	if len(prompt.secretCalls) != 0 || secretExists(t, secretStore, "aihubmix") || secretExists(t, secretStore, "dmxapi") {
		t.Fatalf("invalid manifest touched credentials: prompts=%#v", prompt.secretCalls)
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

func TestSetupFromConfigurationManifestRejectsStdinTokenWithoutAccountOwner(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "aigw-test-one-token\n")
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err := cli.Execute(app, []string{"setup", "--from", manifestPath, "--token-stdin"})
	if err == nil || !strings.Contains(err.Error(), "--account") {
		t.Fatalf("error = %v", err)
	}
	if secretExists(t, secretStore, "aihubmix") || secretExists(t, secretStore, "dmxapi") {
		t.Fatal("ambiguous stdin token was stored")
	}
	assertManifestSetupLeavesNoConfig(t, app)
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

	app, _, secretStore, _, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &scriptedPrompt{secrets: []string{"aigw-test-team-token"}}
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

	if err := cli.Execute(app, []string{"setup", "--from", manifestPath, "--account", "team"}); err != nil {
		t.Fatal(err)
	}
	if targetSawToken {
		t.Fatal("credential probe forwarded X-Api-Key across a redirect")
	}
	if !secretExists(t, secretStore, "team") {
		t.Fatal("explicitly connected Account Token was not stored")
	}
}

func TestSetupFromConfigurationManifestPreservesClientOwnedCredentials(t *testing.T) {
	app, _, _, runner, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &scriptedPrompt{secrets: []string{"aigw-test-dmxapi-token"}}
	codexTarget := filepath.Join(t.TempDir(), "configuration.toml")
	authPath := filepath.Join(filepath.Dir(codexTarget), "auth.json")
	priorAuth := []byte(`{"OPENAI_API_KEY":"prior-client-token"}`)
	if err := os.WriteFile(authPath, priorAuth, 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientClaude: "/opt/claude-real", configuration.ClientCodex: "/opt/codex-real"},
		Surfaces: []discovery.Surface{{
			ID: string(surfaceidentity.CodexHomeDefault), Authority: string(surfaceidentity.AuthorityAIGW),
			ConfigPath: codexTarget, AutoManaged: true,
		}},
	}}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)
	if err := cli.Execute(app, []string{"setup", "--from", manifestPath, "--account", "dmxapi"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 0 {
		t.Fatal("setup invoked a native client instead of projecting the credential helper")
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Routes[configuration.ClientCodex] == "" || !secretExists(t, app.Secrets, "dmxapi") {
		t.Fatal("setup did not persist the selected Account and route")
	}
	authAfter, err := os.ReadFile(authPath)
	if err != nil || string(authAfter) != string(priorAuth) {
		t.Fatalf("setup changed client-owned credentials: %v", err)
	}
	projection, err := os.ReadFile(codexTarget)
	if err != nil || !strings.Contains(string(projection), "[model_providers.aigw.auth]") {
		t.Fatalf("setup did not project command authentication: %s, %v", projection, err)
	}
}
