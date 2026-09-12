package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	surfaceidentity "aigw-cli/internal/surface"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func assertManifestSetupLeavesNoConfig(t *testing.T, app *cli.App) {
	t.Helper()
	for _, path := range []string{app.Config.Path(), app.Config.Path() + ".bak"} {
		if _, err := os.Stat(path); err == nil || !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unexpected configuration residue at %s: %v", path, err)
		}
	}
}

func TestSetupFromConfigurationManifestRejectsUnknownSelectedAccountBeforeMutation(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err := cli.Execute(app, []string{"setup", "--from", manifestPath, "--account", "missing"})
	if err == nil || !strings.Contains(err.Error(), "unknown Account") {
		t.Fatalf("error = %v", err)
	}
	if secretExists(t, secretStore, "aihubmix") || secretExists(t, secretStore, "dmxapi") {
		t.Fatal("unknown selected Account mutated credentials")
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

func TestSetupAdmitsArgumentsBeforeCreatingConfiguration(t *testing.T) {
	for _, args := range [][]string{
		{"--from="},
		{"--from", " "},
		{"--from", "team.toml", "--account="},
		{"--from", "team.toml", "--account", "\t"},
		{"--from", "team.toml", "--profile="},
		{"--from", "team.toml", "--label="},
		{"--from", "team.toml", "--openai-url="},
		{"--from", "team.toml", "--anthropic-url="},
		{"--from", "team.toml", "--for="},
		{"--from", "team.toml", "--model="},
		{"--json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			app, _, _, _, _ := testApp(t, "")
			root := filepath.Join(t.TempDir(), "unconfigured")
			app.Config = configuration.NewStore(filepath.Join(root, "config.toml"))
			if err := cli.Execute(app, append([]string{"setup"}, args...)); err == nil {
				t.Fatal("invalid setup arguments were admitted")
			}
			if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid invocation created configuration state: %v", err)
			}
		})
	}
}

func TestSetupFromConfigurationManifestValidationFailureLeavesNoCredentialsOrConfig(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &scriptedPrompt{secrets: []string{"aigw-test-dmxapi-token"}}
	requests := 0
	app.HTTP = &fakeHTTP{handler: func(req *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader("{}")), Request: req}, nil
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

	err := cli.Execute(app, []string{"setup", "--from", manifestPath, "--account", "dmxapi"})
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
			app, _, secretStore, _, _ := testApp(t, "")
			app.Interactive = true
			app.Prompt = &scriptedPrompt{secrets: []string{"aigw-test-aihubmix-token"}}
			app.HTTP = &fakeHTTP{handler: func(req *http.Request) (*http.Response, error) {
				responseStatus := http.StatusOK
				if req.Header.Get("X-Api-Key") != "" {
					responseStatus = status
				}
				return &http.Response{StatusCode: responseStatus, Body: io.NopCloser(strings.NewReader("{}")), Request: req}, nil
			}}
			manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

			if err := cli.Execute(app, []string{"setup", "--from", manifestPath, "--account", "aihubmix"}); err != nil {
				t.Fatal(err)
			}
			if !secretExists(t, secretStore, "aihubmix") || secretExists(t, secretStore, "dmxapi") {
				t.Fatal("setup did not preserve only the explicitly connected Account")
			}
		})
	}
}

func TestSetupFromConfigurationManifestRejectsUnreferencedAccountBeforePrompt(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	app.Interactive = true
	prompt := &scriptedPrompt{secrets: []string{"must-not-be-read"}}
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

	err := cli.Execute(app, []string{"setup", "--from", manifestPath})
	if err == nil || !strings.Contains(err.Error(), "Account \"unused\" is not referenced") {
		t.Fatalf("error = %v", err)
	}
	if len(prompt.secretCalls) != 0 || secretExists(t, secretStore, "used") || secretExists(t, secretStore, "unused") {
		t.Fatalf("unreferenced Account preflight touched credentials: prompts=%#v", prompt.secretCalls)
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

func TestSetupFromConfigurationManifestRejectsProfileWithoutClient(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
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

	err := cli.Execute(app, []string{"setup", "--from", manifestPath, "--account", "team"})
	if err == nil || !strings.Contains(err.Error(), `profile "team" has unknown client ""`) {
		t.Fatalf("error = %v", err)
	}
	if secretExists(t, secretStore, "team") {
		t.Fatal("invalid manifest wrote a Token")
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

func TestManifestSetupSurfacesManifestAndConfigFailures(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		if err := cli.Execute(app, []string{"setup", "--from", filepath.Join(t.TempDir(), "missing.toml")}); err == nil {
			t.Fatal("expected manifest read failure")
		}
	})

	t.Run("parse", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		path := writeConfigurationManifest(t, "not = [valid")
		if err := cli.Execute(app, []string{"setup", "--from", path}); err == nil {
			t.Fatal("expected manifest parse failure")
		}
	})

	t.Run("config load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		path := writeConfigurationManifest(t, configurationManifestFixture)
		if err := cli.Execute(app, []string{"setup", "--from", path}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("already configured", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		path := writeConfigurationManifest(t, configurationManifestFixture)
		err := cli.Execute(app, []string{"setup", "--from", path})
		if err == nil || !strings.Contains(err.Error(), "already configured") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("discovery", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Discovery = nil
		path := writeConfigurationManifest(t, configurationManifestFixture)
		err := cli.Execute(app, []string{"setup", "--from", path})
		if err == nil || !strings.Contains(err.Error(), "discovery is unavailable") {
			t.Fatalf("error = %v", err)
		}
	})
}
