package cli_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/native"
	"aigw-cli/internal/store"
)

func TestCheckDoesNotRecommendRotationForUnreadableToken(t *testing.T) {
	app, out, secretStore, _, httpClient := testApp(t, "")
	saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("one", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	want := fmt.Errorf("private credential detail: %w", native.ErrUnavailable)
	app.Secrets = &recordingCredentialStore[string]{backend: secretStore, getErr: want}

	err = cli.Execute(app, []string{"check", "claude"})
	if !errors.Is(err, native.ErrUnavailable) {
		t.Fatalf("check error = %v, want unreadable credential", err)
	}
	for _, text := range []string{"The credential backend could not return provider one's Token", "Fix: Make the credential store available to this session"} {
		if !strings.Contains(out.String(), text) {
			t.Fatalf("check output lacks %q: %s", text, out.String())
		}
	}
	for _, forbidden := range []string{"rotate", "private credential detail", "fixture-token"} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("check output contains %q: %s", forbidden, out.String())
		}
	}
	if httpClient.calls != 0 {
		t.Fatalf("unreadable credential sent %d provider requests", httpClient.calls)
	}
}

// The environment backend cannot rotate a Token, so a rejected Token names the
// variable that supplies it, in human and JSON output alike.
func TestCheckNamesTheEnvironmentTokenForARejectedToken(t *testing.T) {
	app, out, _, _, httpClient := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "dmx", "dmx", "DMXAPI", domain.Endpoints{Anthropic: "https://dmx.test"}, claude.ID, "claude-test")
	cfg.SetSelectedModel(claude.ID, "dmx", "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	variable := secret.EnvironmentKey("dmx")
	app.Secrets = secret.NewEnvironmentStore(func(key string) string {
		if key == variable {
			return "rejected-fixture-token"
		}
		return ""
	})
	httpClient.handler = func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`{"message":"invalid token"}`)), Request: req}, nil
	}
	for _, args := range [][]string{{"check", "--depth", "endpoint"}, {"check", "--depth", "endpoint", "--json"}} {
		out.Reset()
		_ = cli.Execute(app, args)
		if !strings.Contains(out.String(), variable) || strings.Contains(out.String(), "rotate") || strings.Contains(out.String(), "rejected-fixture-token") {
			t.Fatalf("%v recovery does not follow the environment backend:\n%s", args, out.String())
		}
	}
}

// storeBytes is every file below root with its bytes, so that a test can show
// that a command left a file Token store exactly as it was.
func storeBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, readErr := os.ReadFile(path)
		files[path] = string(data)
		return readErr
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// A one-time Token from standard input authenticates only the check that read
// it: a file Token store holds the same files with the same bytes afterwards,
// whether the provider accepts the Token or rejects it, and the stored Token
// was never replaced by it.
func TestACheckWithAOneTimeTokenLeavesAFileStoreAsItWas(t *testing.T) {
	for name, status := range map[string]int{"accepted": http.StatusOK, "rejected": http.StatusUnauthorized} {
		t.Run(name, func(t *testing.T) {
			client := newBlockedStoreClient(t)
			before := storeBytes(t, client.secrets)
			if len(before) == 0 {
				t.Fatal("the file store holds no Token to compare")
			}
			var tokens []string
			client.app.HTTP = &fakeHTTP{handler: func(req *http.Request) (*http.Response, error) {
				tokens = append(tokens, req.Header.Get("X-Api-Key"))
				return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"claude-test"}]}`)), Request: req}, nil
			}}
			client.app.In = strings.NewReader("sk-once-0001\n")
			client.output.Reset()
			_ = cli.Execute(client.app, []string{"check", claude.ID, "--model", "gateway/claude-test", "--token-stdin", "--depth", "endpoint", "--json"})
			if len(tokens) == 0 || tokens[0] != "sk-once-0001" {
				t.Fatalf("the check sent %q, not the one-time Token\n%s", tokens, client.output)
			}
			if after := storeBytes(t, client.secrets); !maps.Equal(after, before) {
				t.Fatalf("the check changed the file store:\nbefore %q\nafter  %q", before, after)
			}
		})
	}
}

func TestCheckUsesOneBoundedAuthenticationProbeWithoutMutation(t *testing.T) {
	tests := []struct {
		name       string
		statuses   []int
		networkErr error
		wantError  bool
		wantText   []string
		rejectText []string
	}{
		{
			name:       "healthy first observation",
			statuses:   []int{http.StatusOK},
			wantText:   []string{"✓ Claude", "dmx/claude-test · endpoint", "No checked client failed"},
			rejectText: []string{"transient response", "rotate"},
		},
		{
			name:      "invalid token",
			statuses:  []int{http.StatusUnauthorized},
			wantError: true,
			wantText:  []string{"HTTP 401: the provider rejected the Token", "Fix: aigw provider rotate dmx"},
		},
		{
			name:       "unreachable endpoint",
			statuses:   []int{0},
			networkErr: errors.New("dial tcp: connection refused"),
			wantError:  true,
			wantText:   []string{"The endpoint did not answer", "Fix: aigw check claude --depth endpoint"},
			rejectText: []string{"rotate"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, out, secretStore, _, httpClient := testApp(t, "")
			cfg := domain.NewConfig(clients.Admission())
			addProviderModel(&cfg, "dmx", "dmx", "DMXAPI", domain.Endpoints{Anthropic: "https://dmx.test"}, claude.ID, "claude-test")
			cfg.SetSelectedModel(claude.ID, "dmx", "")
			cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
			synchronizeClaudeProjection(t, app, cfg)
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := secretStore.Set("dmx", "stable-token"); err != nil {
				t.Fatal(err)
			}
			beforeConfig := readFile(t, app.Config.Path())
			prompt := &scriptedPrompt{}
			app.Interactive = true
			app.Prompt = prompt
			calls := 0
			httpClient.handler = func(req *http.Request) (*http.Response, error) {
				if calls >= len(tt.statuses) {
					t.Fatalf("unexpected authentication probe %d", calls+1)
				}
				status := tt.statuses[calls]
				calls++
				if tt.networkErr != nil {
					return nil, tt.networkErr
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"message":"bounded observation"}`)), Request: req}, nil
			}

			err := cli.Execute(app, []string{"check", "--depth", "endpoint"})
			if (err != nil) != tt.wantError {
				t.Fatalf("check error = %v, wantError=%v\n%s", err, tt.wantError, out.String())
			}
			if calls != len(tt.statuses) {
				t.Fatalf("probe calls = %d, want %d", calls, len(tt.statuses))
			}
			for _, want := range tt.wantText {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("check output lacks %q:\n%s", want, out.String())
				}
			}
			for _, reject := range tt.rejectText {
				if strings.Contains(out.String(), reject) {
					t.Fatalf("check output unexpectedly contains %q:\n%s", reject, out.String())
				}
			}
			if prompt.callCount() != 0 {
				t.Fatalf("check prompted %d times", prompt.callCount())
			}
			afterConfig := readFile(t, app.Config.Path())
			if string(afterConfig) != string(beforeConfig) {
				t.Fatalf("check changed configuration\nbefore:\n%s\nafter:\n%s", beforeConfig, afterConfig)
			}
			if token, err := secretStore.Get("dmx"); err != nil || token != "stable-token" {
				t.Fatalf("stored token = %q, %v; want unchanged", token, err)
			}
		})
	}
}

// The checks that `aigw doctor` and `aigw test` covered and that `aigw
// check` now owns: what a Token reaches and what output never shows.

// syncedCheckApp is a home whose provider dmx serves Claude Code and Codex at
// endpoints, both installed, synchronized and holding dmx's Token.
func syncedCheckApp(t *testing.T, endpoints domain.Endpoints) (*cli.App, *bytes.Buffer) {
	t.Helper()
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["dmx"] = domain.Provider{Label: "DMX", Endpoints: endpoints}
	cfg.Models["gpt"] = qualifiedModel("GPT", "dmx", "gpt-test", domain.ProtocolResponses)
	cfg.Models["claude"] = qualifiedModel("Claude", "dmx", "claude-test", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(codex.ID, "gpt", "")
	cfg.SetSelectedModel(claude.ID, "claude", "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	codexTarget := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(codexTarget, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), []string{codexTarget})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "check-token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	return app, out
}

// An endpoint check sends the Token only to the selected provider's origin:
// a redirect elsewhere is a failure, never followed.
func TestCheckKeepsTheTokenAtTheSelectedOrigin(t *testing.T) {
	for _, client := range []string{codex.ID, claude.ID} {
		t.Run(client, func(t *testing.T) {
			var followed atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				followed.Add(1)
				writer.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(target.Close)
			origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				http.Redirect(writer, request, target.URL, http.StatusFound)
			}))
			t.Cleanup(origin.Close)
			app, out := syncedCheckApp(t, domain.Endpoints{Anthropic: origin.URL, Responses: origin.URL + "/v1"})
			app.HTTP = origin.Client()
			err := cli.Execute(app, []string{"check", client, "--depth", "endpoint", "--json"})
			if followed.Load() != 0 || err == nil || !strings.Contains(out.String(), "302") {
				t.Fatalf("followed=%d, error=%v; want the redirect refused:\n%s", followed.Load(), err, out)
			}
		})
	}
}

// An endpoint check authenticates with the Token and prints neither the
// Token nor its header.
func TestCheckAuthenticatesWithoutPrintingTheToken(t *testing.T) {
	app, out := syncedCheckApp(t, domain.Endpoints{Responses: "https://example.test/v1", Anthropic: "https://example.test"})
	httpClient := &fakeHTTP{status: http.StatusOK}
	app.HTTP = httpClient
	if err := cli.Execute(app, []string{"check", "codex", "--depth", "endpoint"}); err != nil {
		t.Fatalf("check: %v\n%s", err, out)
	}
	if httpClient.headers.Get("Authorization") != "Bearer check-token" {
		t.Fatalf("authorization header = %q", httpClient.headers.Get("Authorization"))
	}
	if strings.Contains(out.String(), "check-token") || strings.Contains(strings.ToLower(out.String()), "authorization") {
		t.Fatalf("the Token reached the output:\n%s", out)
	}
}

// A local check names a client Token in the environment without its value,
// and a configuration that cannot be read without its path.
func TestCheckNamesNeitherAnEnvironmentTokenNorTheConfigurationPath(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	const secret = "check-environment-token-must-not-appear"
	app.Env = []string{"ANTHROPIC_AUTH_TOKEN=" + secret}
	saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
	_ = cli.Execute(app, []string{"check", "--depth", "local", "--verbose", "--json"})
	if !strings.Contains(out.String(), "ANTHROPIC_AUTH_TOKEN") || strings.Contains(out.String(), secret) {
		t.Fatalf("the environment check:\n%s", out)
	}
	bad, broken, _, _, _ := testApp(t, "")
	unreadable := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	bad.Config = unreadable
	if err := os.MkdirAll(unreadable.Path(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(bad, []string{"check", "--depth", "local", "--json"}); err == nil || strings.Contains(broken.String(), unreadable.Path()) {
		t.Fatalf("an unreadable configuration: %v\n%s", err, broken)
	}
}
