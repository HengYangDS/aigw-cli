package cli_test

import (
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
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

func TestSetupFromConfigurationManifestRejectsUnknownSelectedProviderBeforeMutation(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "sk-unread\n")
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err := cli.Execute(app, []string{"setup", "--from", manifestPath, "--provider", "missing", "--token-stdin"})
	if err == nil || !strings.Contains(err.Error(), `unknown provider "missing"`) {
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
		{"--from", "team.toml", "--provider="},
		{"--from", "team.toml", "--provider", "Bad ID", "--token-stdin"},
		{"--from", "team.toml", "--provider", "one", "--provider", "two", "--token-stdin"},
		{"--provider", "one"},
		{"--prune"},
		{"--json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			app, _, _, _, _ := testApp(t, "")
			root := filepath.Join(t.TempDir(), "unconfigured")
			app.Config = store.NewStore(filepath.Join(root, "config.toml"), clients.Admission())
			if err := cli.Execute(app, append([]string{"setup"}, args...)); err == nil {
				t.Fatal("invalid setup arguments were admitted")
			}
			if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid invocation created configuration state: %v", err)
			}
		})
	}
}

// A Token that setup reads is validated as `aigw provider add` validates one,
// and one that the provider rejects refuses the whole apply.
func TestSetupFromATeamFileValidatesAPromptedToken(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	app.Interactive = true
	app.Prompt = &scriptedPrompt{secrets: []string{"aigw-test-dmxapi-token"}}
	requests := 0
	app.HTTP = &fakeHTTP{handler: func(req *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader("{}")), Request: req}, nil
	}}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

	err := cli.Execute(app, []string{"setup", "--from", manifestPath, "--provider", "dmxapi"})
	if err == nil || !strings.Contains(err.Error(), "Token validation failed") {
		t.Fatalf("a rejected Token = %v\n%s", err, out)
	}
	if requests == 0 || strings.Contains(out.String(), "aigw-test-dmxapi-token") {
		t.Fatalf("setup did not validate the Token safely: requests=%d output=%s", requests, out)
	}
	if secretExists(t, secretStore, "dmxapi") || secretExists(t, secretStore, "aihubmix") {
		t.Fatal("a rejected Token was stored")
	}
	assertManifestSetupLeavesNoConfig(t, app)
}

// Setup stores only the named provider's Token once the provider accepts it,
// and an endpoint that does not answer as one refuses it.
func TestSetupFromATeamFileStoresOnlyTheNamedProvidersAcceptedToken(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusFound, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			app, _, secretStore, _, _ := testApp(t, "")
			app.Interactive = true
			app.Prompt = &scriptedPrompt{secrets: []string{"aigw-test-aihubmix-token"}}
			app.HTTP = &fakeHTTP{handler: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("{}")), Request: req}, nil
			}}
			manifestPath := writeConfigurationManifest(t, configurationManifestFixture)

			err := cli.Execute(app, []string{"setup", "--from", manifestPath, "--provider", "aihubmix"})
			if accepted := status == http.StatusOK; accepted != (err == nil) || accepted != secretExists(t, secretStore, "aihubmix") {
				t.Fatalf("status %d: error = %v", status, err)
			}
			if secretExists(t, secretStore, "dmxapi") {
				t.Fatal("setup stored a Token of a provider that was not named")
			}
		})
	}
}

// In a terminal, setup asks only for the Tokens that the recommended setup
// would select, and a provider that no model names is added without one.
func TestSetupFromATeamFilePromptsOnlyForTheRecommendedProvider(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	app.Interactive = true
	prompt := &scriptedPrompt{secrets: []string{"aigw-test-used-token"}}
	app.Prompt = prompt
	manifestPath := writeConfigurationManifest(t, `version = 8

[[providers]]
id = "unused"
label = "Unused"
endpoints = {anthropic = "https://unused.test"}

[[providers]]
id = "used"
label = "Used"
endpoints = {anthropic = "https://used.test"}

[[providers.models]]
id = "claude-test"
family = "claude-test"
label = "Used"
protocols = {anthropic = ["text"]}

[[families]]
id = "claude-test"
label = "Claude Test"

[recommendations.claude]
primary = {model = "used/claude-test"}
`)

	if err := cli.Execute(app, []string{"setup", "--from", manifestPath}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(prompt.secretCalls, []string{"Paste the Used Token: "}) || !secretExists(t, secretStore, "used") || secretExists(t, secretStore, "unused") {
		t.Fatalf("prompts=%#v", prompt.secretCalls)
	}
	cfg, err := app.Config.Load()
	if err != nil || cfg.Providers["unused"].Label != "Unused" || cfg.Clients[claude.ID].Model != domain.NewModelRef("used", "claude-test") {
		t.Fatalf("configuration = %+v, %v", cfg, err)
	}
}

func TestSetupFromConfigurationManifestRejectsModelWithoutID(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	manifestPath := writeConfigurationManifest(t, `version = 8

[[providers]]
id = "team"
label = "Team"
endpoints = {anthropic = "https://team.test", responses = "https://team.test/v1"}

[[providers.models]]
label = "Team"
protocols = {anthropic = ["text"]}
`)

	err := cli.Execute(app, []string{"setup", "--from", manifestPath})
	if err == nil || !strings.Contains(err.Error(), `provider "team" has a model with an empty ID`) {
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
		app.Config = store.NewStore(t.TempDir(), clients.Admission())
		path := writeConfigurationManifest(t, configurationManifestFixture)
		if err := cli.Execute(app, []string{"setup", "--from", path}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	// A configured home takes the team file, and its own selection stays.
	t.Run("already configured", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "m")
		before, err := app.Config.Load()
		if err != nil {
			t.Fatal(err)
		}
		path := writeConfigurationManifest(t, configurationManifestFixture)
		if err := cli.Execute(app, []string{"setup", "--from", path}); err != nil {
			t.Fatal(err)
		}
		after, err := app.Config.Load()
		if err != nil || after.Clients[claude.ID].Model != before.Clients[claude.ID].Model || len(after.Providers) != len(before.Providers)+2 {
			t.Fatalf("configured home after setup = %+v, %v", after, err)
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

func TestManifestDecodeFailureKeepsNativeTypesOutOfPublicOutput(t *testing.T) {
	for _, mode := range []struct {
		name string
		json bool
	}{{name: "human"}, {name: "JSON", json: true}} {
		t.Run(mode.name, func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			path := writeConfigurationManifest(t, "version = 8\n[recommendations.codex]\nprimary = 'invalid-shape'\n")
			args := []string{"setup", "--from", path}
			if mode.json {
				args = append(args, "--json")
			}
			if err := cli.Execute(app, args); err == nil {
				t.Fatal("invalid manifest shape was accepted")
			}
			if strings.Contains(out.String(), "configuration.") || strings.Contains(out.String(), path) {
				t.Fatal("manifest decoding exposed a native implementation type or private path")
			}
			if !strings.Contains(out.String(), "line 3") || !strings.Contains(out.String(), "column") {
				t.Fatal("manifest decoding did not identify the document location")
			}
			if mode.json {
				// The fix is prose, so the failure gives it as next_detail.
				var result struct {
					OK         bool   `json:"ok"`
					Error      string `json:"error"`
					NextDetail string `json:"next_detail"`
				}
				if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.OK || result.Error == "" || !strings.Contains(result.NextDetail, "document") || !strings.Contains(result.NextDetail, "rerun") {
					t.Fatalf("manifest decoding did not emit one actionable JSON failure: %v", err)
				}
			}
			assertManifestSetupLeavesNoConfig(t, app)
		})
	}
}
