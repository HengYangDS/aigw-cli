package cli_test

import (
	"aigw-cli/internal/prompt"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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

func TestSetupFromConfigurationManifestPromptsOnlyForTokensAndKeepsThemSecret(t *testing.T) {
	t.Setenv("AIGW_TOKEN_UNRELATED", "aigw-test-unrelated-token")
	app, out, secretStore, runner := testApp(t, "")
	app.Interactive = true
	prompt := &manifestSetupPrompt{secrets: []string{"aigw-test-aihubmix-token", "aigw-test-dmxapi-token"}}
	app.Prompt = prompt
	shimDir := t.TempDir()
	app.ClaudeLauncher.BinDir = shimDir
	app.ClaudeLauncher.AIGWExecutable = filepath.Join(shimDir, "aigw")
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
	validationRequests := map[string]int{}
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
		validationRequests[req.URL.Host+"/"+protocol]++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Request: req}, nil
	}}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := execute(t, app, "setup", "--from", manifestPath); err != nil {
		t.Fatal(err)
	}
	if prompt.textCalls != 0 {
		t.Fatalf("endpoint/profile text prompts = %d, want 0", prompt.textCalls)
	}
	if len(prompt.secretCalls) != 2 || !strings.Contains(prompt.secretCalls[0], "AIHubMix") || !strings.Contains(prompt.secretCalls[1], "DMXAPI") {
		t.Fatalf("secret prompts = %#v, want sorted Account prompts", prompt.secretCalls)
	}
	for account, want := range map[string]string{"aihubmix": "aigw-test-aihubmix-token", "dmxapi": "aigw-test-dmxapi-token"} {
		got, err := secretStore.Get(account)
		if err != nil || got != want {
			t.Fatalf("%s token = %q, %v", account, got, err)
		}
	}
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Accounts) != 2 || len(cfg.Profiles) != 3 || cfg.Routes.Default != "dmxapi-gpt" {
		t.Fatalf("team config = %#v", cfg)
	}
	if cfg.Routes.Overrides["claude"] != "aihubmix-claude" || cfg.Routes.Overrides["codex"] != "dmxapi-gpt" {
		t.Fatalf("recommended client routes = %#v", cfg.Routes.Overrides)
	}
	if !cfg.Adapters["claude"].Enabled || !cfg.Adapters["codex"].Enabled {
		t.Fatalf("discovered clients were not configured: %#v", cfg.Adapters)
	}
	if _, err := os.Stat(filepath.Join(shimDir, "claude")); err != nil {
		t.Fatalf("Claude launcher missing: %v", err)
	}
	if len(runner.plans) != 3 || runner.plans[0].Executable != "/opt/claude-real" || runner.plans[1].Executable != "/opt/claude-real" || runner.plans[2].Executable != "/opt/codex-real" {
		executables := make([]string, 0, len(runner.plans))
		for _, plan := range runner.plans {
			executables = append(executables, plan.Executable)
		}
		t.Fatalf("client plan executables = %#v", executables)
	}
	if len(runner.captureDeadlines) != 2 || !runner.captureDeadlines[0] || !runner.captureDeadlines[1] {
		t.Fatalf("Claude validation deadlines = %#v", runner.captureDeadlines)
	}
	for _, plan := range runner.plans[:2] {
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
		if validationRequests[key] != want {
			t.Errorf("validation request %s = %d, want %d; all=%#v", key, validationRequests[key], want, validationRequests)
		}
	}
	if len(validationRequests) != len(wantValidationRequests) {
		t.Fatalf("unexpected validation requests: %#v", validationRequests)
	}
	if cfg.Profiles["aihubmix-claude"].Models["claude"] != "claude-test" || cfg.Profiles["dmxapi-gpt"].Models["codex"] != "gpt-test" {
		t.Fatalf("manifest model matrix was not preserved: %#v", cfg.Profiles)
	}

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

func TestSetupFromConfigurationManifestNonInteractiveListsMissingTokensWithoutWrite(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err := execute(t, app, "setup", "--from", manifestPath)
	if err == nil || !strings.Contains(err.Error(), "aihubmix") || !strings.Contains(err.Error(), "dmxapi") || !strings.Contains(err.Error(), "interactive") {
		t.Fatalf("error = %v", err)
	}
	if secretStore.Has("aihubmix") || secretStore.Has("dmxapi") {
		t.Fatal("non-interactive rejection wrote a token")
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

func TestSetupFromConfigurationManifestRejectsOneStdinTokenForMultipleAccounts(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "aigw-test-one-token\n")
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err := execute(t, app, "setup", "--from", manifestPath, "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "multiple accounts") {
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
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
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

	err := execute(t, app, "setup", "--from", manifestPath)
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
	app.Prompt = &manifestSetupPrompt{secrets: []string{"aigw-test-aihubmix-token", "aigw-test-dmxapi-token"}}
	requests := 0
	app.HTTP = &fakeHTTP{handler: func(req *http.Request) (*http.Response, error) {
		requests++
		status := http.StatusOK
		if requests == 2 {
			status = http.StatusUnauthorized
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("{}")), Request: req}, nil
	}}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err := execute(t, app, "setup", "--from", manifestPath)
	if err == nil || !strings.Contains(err.Error(), "Token validation failed") {
		t.Fatalf("error = %v", err)
	}
	if requests != 2 {
		t.Fatalf("validation requests = %d, want 2", requests)
	}
	if secretStore.Has("aihubmix") || secretStore.Has("dmxapi") {
		t.Fatal("failed validation left a token")
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

func TestSetupFromConfigurationManifestRejectsClaudeNonSuccessAsUnvalidated(t *testing.T) {
	for _, status := range []int{http.StatusFound, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			app, _, secretStore, _ := testApp(t, "")
			app.Interactive = true
			app.Prompt = &manifestSetupPrompt{secrets: []string{"aigw-test-aihubmix-token", "aigw-test-dmxapi-token"}}
			app.HTTP = &fakeHTTP{handler: func(req *http.Request) (*http.Response, error) {
				responseStatus := http.StatusOK
				if req.Header.Get("X-Api-Key") != "" {
					responseStatus = status
				}
				return &http.Response{StatusCode: responseStatus, Body: io.NopCloser(strings.NewReader("{}")), Request: req}, nil
			}}
			manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

			err := execute(t, app, "setup", "--from", manifestPath)
			if err == nil || !strings.Contains(err.Error(), "Claude endpoint returned HTTP "+strconv.Itoa(status)) {
				t.Fatalf("error = %v", err)
			}
			if secretStore.Has("aihubmix") || secretStore.Has("dmxapi") {
				t.Fatal("unvalidated Claude credential was stored")
			}
			assertManifestSetupLeavesNoConfig(t, app)
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

	err := execute(t, app, "setup", "--from", manifestPath)
	if err == nil || !strings.Contains(err.Error(), "Claude endpoint returned HTTP 302") {
		t.Fatalf("error = %v", err)
	}
	if targetSawToken {
		t.Fatal("credential probe forwarded X-Api-Key across a redirect")
	}
	if secretStore.Has("team") {
		t.Fatal("redirected credential probe stored a Token")
	}
	assertManifestSetupLeavesNoConfig(t, app)
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

	if err := execute(t, app, "setup", "--from", manifestPath); err != nil {
		t.Fatal(err)
	}
	if requests["anthropic"] != 1 || requests["openai"] != 1 || len(requests) != 2 {
		t.Fatalf("generic profile validation requests = %#v", requests)
	}
}

func TestSetupFromConfigurationManifestClientFailureRollsBackCredentialsConfigAndLauncher(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &manifestSetupPrompt{secrets: []string{"aigw-test-aihubmix-token", "aigw-test-dmxapi-token"}}
	app.Runner = &failingRunner{err: errors.New("Codex login failed"), remaining: 1}
	shimDir := t.TempDir()
	app.ClaudeLauncher.BinDir = shimDir
	app.ClaudeLauncher.AIGWExecutable = filepath.Join(shimDir, "aigw")
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

	err := execute(t, app, "setup", "--from", manifestPath)
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("error = %v", err)
	}
	if secretStore.Has("aihubmix") || secretStore.Has("dmxapi") {
		t.Fatal("failed setup left an Account Token")
	}
	assertManifestSetupLeavesNoConfig(t, app)
	if _, err := os.Stat(filepath.Join(shimDir, "claude")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed setup left Claude launcher: %v", err)
	}
	data, err := os.ReadFile(codexTarget)
	if err != nil || string(data) != original {
		t.Fatalf("failed setup changed Codex config: %q, %v", data, err)
	}
}

func TestSetupFromConfigurationManifestClientFailurePreservesExistingClaudeLauncher(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &manifestSetupPrompt{secrets: []string{"aigw-test-aihubmix-token", "aigw-test-dmxapi-token"}}
	app.Runner = &failingRunner{err: errors.New("Codex login failed"), remaining: 1}
	oldLauncherDir := t.TempDir()
	newLauncherDir := t.TempDir()
	home := t.TempDir()
	app.ClaudeLauncher.GOOS = "darwin"
	app.ClaudeLauncher.Home = home
	app.ClaudeLauncher.Shell = "/bin/zsh"
	app.ClaudeLauncher.BinDir = oldLauncherDir
	app.ClaudeLauncher.AIGWExecutable = "/bin/sh"
	if _, err := app.ClaudeLauncher.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	oldLauncherPath := filepath.Join(oldLauncherDir, "claude")
	oldActivationPath := filepath.Join(home, ".zshrc")
	oldLauncher, err := os.ReadFile(oldLauncherPath)
	if err != nil {
		t.Fatal(err)
	}
	oldActivation, err := os.ReadFile(oldActivationPath)
	if err != nil {
		t.Fatal(err)
	}
	app.ClaudeLauncher.BinDir = newLauncherDir
	app.ClaudeLauncher.AIGWExecutable = "/bin/echo"
	codexTarget := filepath.Join(t.TempDir(), "configuration.toml")
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
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err = execute(t, app, "setup", "--from", manifestPath)
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("error = %v", err)
	}
	if secretStore.Has("aihubmix") || secretStore.Has("dmxapi") {
		t.Fatal("failed setup left an Account Token")
	}
	gotOldLauncher, shimErr := os.ReadFile(oldLauncherPath)
	gotActivation, activationErr := os.ReadFile(oldActivationPath)
	if shimErr != nil || string(gotOldLauncher) != string(oldLauncher) || activationErr != nil || string(gotActivation) != string(oldActivation) {
		t.Fatalf("existing Claude launcher state was not restored exactly: shim=%v activation=%v", shimErr, activationErr)
	}
	if _, err := os.Stat(filepath.Join(newLauncherDir, "claude")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed setup left new Claude launcher: %v", err)
	}
}
