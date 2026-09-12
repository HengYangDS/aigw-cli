package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
	surfaceidentity "aigw-cli/internal/surface"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestSetupFromConfigurationManifestImportsWithoutTokensOrClients(t *testing.T) {
	app, out, secretStore, runner, _ := testApp(t, "")
	app.Discovery = fakeDiscovery{}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := cli.Execute(app, []string{"setup", "--from", manifestPath}); err != nil {
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

func TestSetupFromConfigurationManifestProjectsClientNativeCodexWithoutAccountToken(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	credentials := &recordingCredentialStore[string]{backend: secrets.NewMemoryStore()}
	app.Secrets = credentials
	httpCalls := 0
	app.HTTP = &fakeHTTP{handler: func(*http.Request) (*http.Response, error) {
		httpCalls++
		return nil, errors.New("account-token probe must not run")
	}}
	target := filepath.Join(t.TempDir(), "codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientCodex: executableFixture(t, "codex")},
		Surfaces: []discovery.Surface{{
			ID:          string(surfaceidentity.CodexHomeDefault),
			Authority:   string(surfaceidentity.AuthorityAIGW),
			ConfigPath:  target,
			Present:     true,
			AutoManaged: true,
		}},
	}}
	manifestPath := writeConfigurationManifest(t, `version = 4
[recommended_routes]
codex = "bedrock"

[accounts.aws]
label = "AWS"
[accounts.aws.endpoints]
openai_responses = "https://bedrock-runtime.us-east-1.amazonaws.com/openai/v1"

[profiles.bedrock]
label = "AWS Bedrock"
account = "aws"
client = "codex"
model = "openai.gpt-5.6-sol"
model_provider = "amazon-bedrock"
authentication = "client-native"
`)

	if err := cli.Execute(app, []string{"setup", "--from", manifestPath, "--json"}); err != nil {
		t.Fatalf("setup client-native Codex: %v", err)
	}
	if len(credentials.getCalls)+len(credentials.existsCalls)+len(credentials.setCalls)+len(credentials.deleteCalls) != 0 {
		t.Fatal("client-native setup accessed AIGW credentials")
	}
	if httpCalls != 0 {
		t.Fatalf("setup made %d account-token probes", httpCalls)
	}
	var result struct {
		ConnectedAccounts []string          `json:"connected_accounts"`
		SelectedRoutes    map[string]string `json:"selected_routes"`
		ProjectedClients  []string          `json:"projected_clients"`
		DeferredActions   []string          `json:"deferred_actions"`
		NextAction        string            `json:"next_action"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode setup result: %v\n%s", err, out.String())
	}
	if len(result.ConnectedAccounts) != 0 || result.SelectedRoutes[configuration.ClientCodex] != "bedrock" || !slices.Equal(result.ProjectedClients, []string{configuration.ClientCodex}) {
		t.Fatalf("setup state = %#v", result)
	}
	if len(result.DeferredActions) != 0 || result.NextAction != "aigw check" {
		t.Fatalf("setup continuation = %#v", result)
	}
	projection := string(readFile(t, target))
	if !strings.Contains(projection, `model_provider = "amazon-bedrock"`) || strings.Contains(projection, "credential") || strings.Contains(projection, ".auth]") {
		t.Fatalf("client-native Codex projection is not self-owned:\n%s", projection)
	}
}

func TestSetupFromConfigurationManifestJSONReportsProgressWithoutSecrets(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	app.Discovery = fakeDiscovery{}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := cli.Execute(app, []string{"setup", "--from", manifestPath, "--json"}); err != nil {
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
			app, out, _, _, _ := testApp(t, "")
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

			if err := cli.Execute(app, []string{"setup", "--from", manifestPath, "--json"}); err != nil {
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
	app, out, store, _, _ := testApp(t, "")
	if err := store.Set("dmxapi", "aigw-test-dmxapi-token"); err != nil {
		t.Fatal(err)
	}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := cli.Execute(app, []string{"setup", "--from", manifestPath, "--json"}); err != nil {
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

func TestSetupFromConfigurationManifestReportsInstalledClientWaitingForAnAccount(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientClaude: "/opt/claude"},
	}}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	if err := cli.Execute(app, []string{"setup", "--from", manifestPath}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Connect an Account compatible with Claude, then run `aigw sync`") {
		t.Fatalf("installed but deferred client is not explained:\n%s", out.String())
	}
}

func TestSetupFromConfigurationManifestProjectsEveryCodexTarget(t *testing.T) {
	for _, backend := range []string{"environment", "prompt"} {
		t.Run(backend, func(t *testing.T) {
			app, _, _, runner, _ := testApp(t, "")
			const token = "aigw-test-dmxapi-token"
			args := []string{"setup", "--from", writeConfigurationManifest(t, configurationManifestFixture), "--account", "dmxapi"}
			if backend == "environment" {
				app.Secrets = secrets.NewEnvironmentStore(func(key string) string {
					if key == secrets.EnvironmentKey("dmxapi") {
						return token
					}
					return ""
				})
			} else {
				app.Interactive = true
				app.Prompt = &scriptedPrompt{secrets: []string{token}}
			}
			targets := []string{
				filepath.Join(t.TempDir(), "one", "configuration.toml"),
				filepath.Join(t.TempDir(), "two", "configuration.toml"),
			}
			app.Discovery = fakeDiscovery{result: discovery.Result{
				Executables: map[string]string{configuration.ClientCodex: "/opt/codex"},
				Surfaces: []discovery.Surface{
					{ID: string(surfaceidentity.CodexHomeDefault), Authority: string(surfaceidentity.AuthorityAIGW), ConfigPath: targets[0], Present: true, AutoManaged: true},
					{ID: string(surfaceidentity.CodexHomeExplicit("second")), Authority: string(surfaceidentity.AuthorityAIGW), ConfigPath: targets[1], Present: true, AutoManaged: true},
				},
			}}
			if err := cli.Execute(app, args); err != nil {
				t.Fatal(err)
			}
			cfg, err := app.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Routes[configuration.ClientCodex] != "dmxapi-gpt" {
				t.Fatalf("Codex route = %q", cfg.Routes[configuration.ClientCodex])
			}
			projectedRuntime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
			if err != nil {
				t.Fatal(err)
			}
			for _, target := range targets {
				data, err := os.ReadFile(target)
				if err != nil || !strings.Contains(string(data), projectedRuntime.CredentialProjectionFingerprint(configuration.ClientCodex)) || strings.Contains(string(data), token) {
					t.Fatalf("target %s credential projection = %s, %v", target, data, err)
				}
			}
			if got, err := app.Secrets.Get("dmxapi"); err != nil || got != token {
				t.Fatalf("Account Token = %q, %v", got, err)
			}
			if len(runner.plans) != 0 {
				t.Fatal("setup invoked a native client")
			}
		})
	}
}

func TestSetupFromConfigurationManifestOutputFailureKeepsCommittedAutomaticBackendSelection(t *testing.T) {
	app, _, _, _, _ := testApp(t, "token\n")
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

	err = cli.Execute(app, []string{"setup", "--from", manifestPath, "--account", "dmxapi", "--token-stdin", "--json"})
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

func TestSetupFromConfigurationManifestProjectionFailureRestoresOwnedFiles(t *testing.T) {
	app, _, secretStore, runner, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &scriptedPrompt{secrets: []string{"aigw-test-dmxapi-token"}}
	app.Executable = "relative-helper"
	root := t.TempDir()
	app.Config = configuration.NewStore(filepath.Join(root, "aigw", "config.toml"))
	app.ClaudeSettingsPath = filepath.Join(root, "claude", "settings.json")
	claudeExecutable := executableFixture(t, "claude")
	originalExecutable, err := os.ReadFile(claudeExecutable)
	if err != nil {
		t.Fatal(err)
	}
	codexTarget := filepath.Join(root, "codex", "config.toml")
	preserved := map[string]string{
		codexTarget: "model_provider = \"native\"\n",
		filepath.Join(root, "codex", "auth.json"): `{"OPENAI_API_KEY":"client-owned-token"}`,
		app.ClaudeSettingsPath:                    `{"theme":"dark"}`,
	}
	for path, content := range preserved {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientClaude: claudeExecutable, configuration.ClientCodex: "/opt/codex-real"},
		Surfaces: []discovery.Surface{{
			ID: string(surfaceidentity.CodexHomeDefault), Authority: string(surfaceidentity.AuthorityAIGW),
			ConfigPath: codexTarget, Present: true, AutoManaged: true,
		}},
	}}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)
	err = cli.Execute(app, []string{"setup", "--from", manifestPath, "--account", "dmxapi"})
	if err == nil || !strings.Contains(err.Error(), "preflight failed") || strings.Contains(err.Error(), "configuration was rolled back") {
		t.Fatalf("setup error = %v, want rejection before configuration commit", err)
	}
	if secretExists(t, secretStore, "aihubmix") || secretExists(t, secretStore, "dmxapi") {
		t.Fatal("failed setup left an Account Token")
	}
	files := map[string]string{}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		data, readErr := os.ReadFile(path)
		files[path] = string(data)
		return readErr
	})
	if err != nil {
		t.Fatal(err)
	}
	// The shared lock file is an inert synchronization inode, not a held lease.
	preserved[app.Config.Path()+".lock"] = ""
	if !maps.Equal(files, preserved) {
		t.Fatalf("failed setup changed files or left temporary state: got=%#v want=%#v", files, preserved)
	}
	unlock, err := app.Config.Lock(t.Context())
	if err != nil {
		t.Fatalf("failed setup retained its mutation lock: %v", err)
	}
	if err := unlock(); err != nil {
		t.Fatal(err)
	}
	gotExecutable, err := os.ReadFile(claudeExecutable)
	if err != nil || string(gotExecutable) != string(originalExecutable) || len(runner.plans) != 0 {
		t.Fatalf("foreign client changed or executed: error=%v calls=%d", err, len(runner.plans))
	}
}
