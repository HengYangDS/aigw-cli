package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestEndpointTestKeepsCredentialsAtTheirSelectedOrigin(t *testing.T) {
	for _, client := range []string{configuration.ClientCodex, configuration.ClientClaude} {
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
			app, _, credentials, _, _ := testApp(t, "")
			cfg := configuration.NewConfig()
			addAccountProfile(&cfg, "selected", "team", "Team", configuration.Endpoints{Anthropic: origin.URL, OpenAIResponses: origin.URL}, client, "model")
			cfg.Routes[client] = "selected"
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := credentials.Set("team", "synthetic-token"); err != nil {
				t.Fatal(err)
			}
			app.HTTP = origin.Client()
			err := cli.Execute(app, []string{"test", "--for", client})
			if followed.Load() != 0 || err == nil || !strings.Contains(err.Error(), "HTTP 302") {
				t.Fatalf("credential probe followed=%d, error=%v; want selected-origin redirect rejection", followed.Load(), err)
			}
		})
	}
}

func TestTestCommandSurfacesOperationalFailures(t *testing.T) {
	t.Run("config load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := cli.Execute(app, []string{"test"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("not configured", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		err := cli.Execute(app, []string{"test"})
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "not configured") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		err := cli.Execute(app, []string{"test", "--for", "codex"})
		if err == nil || !strings.Contains(err.Error(), "is unavailable") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("transport", func(t *testing.T) {
		app, _, secretStore, _, httpClient := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		_ = secretStore.Set("one", "token")
		want := errors.New("network down")
		httpClient.handler = func(*http.Request) (*http.Response, error) { return nil, want }
		if err := cli.Execute(app, []string{"test", "--for", "codex"}); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("response close", func(t *testing.T) {
		app, _, secretStore, _, httpClient := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		_ = secretStore.Set("one", "token")
		want := errors.New("close failed")
		httpClient.handler = func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: closeFailingBody{Reader: strings.NewReader("ok"), err: want}, Request: req}, nil
		}
		if err := cli.Execute(app, []string{"test", "--for", "codex"}); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("server status", func(t *testing.T) {
		app, _, secretStore, _, httpClient := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		_ = secretStore.Set("one", "token")
		httpClient.status = http.StatusInternalServerError
		err := cli.Execute(app, []string{"test", "--for", "codex"})
		if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestTestCommandAuthenticatesWithoutPrintingAuthorizationHeader(t *testing.T) {
	app, out, secretStore, _, httpClient := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMX", configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}, configuration.ClientCodex, "gpt-test")
	cfg.Routes[configuration.ClientCodex] = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "unused-secret")
	if err := cli.Execute(app, []string{"test", "--for", "codex"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Connectivity test") || !strings.Contains(out.String(), "Codex") || !strings.Contains(out.String(), "HTTP 200") {
		t.Fatalf("test output = %s", out.String())
	}
	if httpClient.headers.Get("Authorization") != "Bearer unused-secret" {
		t.Fatalf("authorization header = %q", httpClient.headers.Get("Authorization"))
	}
	if strings.Contains(out.String(), "unused-secret") || strings.Contains(strings.ToLower(out.String()), "authorization") {
		t.Fatalf("credential leaked in output: %s", out.String())
	}
}

func TestTestCommandReturnsResponseReadFailure(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMX", configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}, configuration.ClientCodex, "gpt-test")
	cfg.Routes[configuration.ClientCodex] = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "unused-secret"); err != nil {
		t.Fatal(err)
	}
	want := errors.New("response interrupted")
	app.HTTP = &fakeHTTP{status: http.StatusOK, handler: func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: failingReadCloser{err: want}, Request: request}, nil
	}}

	err := cli.Execute(app, []string{"test", "--for", "codex"})
	if !errors.Is(err, want) {
		t.Fatalf("test command error = %v, want %v", err, want)
	}
}

func TestTestCommandKeepsRequestContextAliveUntilResponseIsDrained(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMX", configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}, configuration.ClientCodex, "gpt-test")
	cfg.Routes[configuration.ClientCodex] = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "unused-secret"); err != nil {
		t.Fatal(err)
	}
	app.HTTP = &fakeHTTP{status: http.StatusOK, handler: func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       &contextBoundReadCloser{ctx: request.Context(), reader: strings.NewReader("ok")},
			Request:    request,
		}, nil
	}}

	if err := cli.Execute(app, []string{"test", "--for", "codex"}); err != nil {
		t.Fatalf("test command error = %v", err)
	}
}

func TestTestCommandDistinguishesReachabilityFromCredentialAcceptance(t *testing.T) {
	app, out, secretStore, _, httpClient := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMX", configuration.Endpoints{Anthropic: "https://example.test"}, configuration.ClientClaude, "claude-test")
	cfg.Routes[configuration.ClientClaude] = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "probe-secret")
	httpClient.status = http.StatusNotFound
	if err := cli.Execute(app, []string{"test", "--for", "claude"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "HTTP 404 · Service is reachable; model discovery is unavailable and credential acceptance is unverified") {
		t.Fatalf("Claude model-discovery 404 probe result = %s", out.String())
	}
}

func TestTestCommandRejectsAuthenticationFailure(t *testing.T) {
	app, _, secretStore, _, httpClient := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMX", configuration.Endpoints{Anthropic: "https://example.test"}, configuration.ClientClaude, "claude-test")
	cfg.Routes[configuration.ClientClaude] = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "rejected-secret")
	httpClient.status = 401
	err := cli.Execute(app, []string{"test", "--for", "claude"})
	if err == nil || !strings.Contains(err.Error(), "authentication was rejected") || strings.Contains(err.Error(), "rejected-secret") {
		t.Fatalf("error = %v", err)
	}
}

func TestTerminalErrorRejectsRedundantProfileAndClientSelectors(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Team", Endpoints: configuration.Endpoints{OpenAIResponses: "https://team.test/v1", Anthropic: "https://team.test"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "team", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	err := cli.Execute(app, []string{"test", "--for", "claude", "--profile", "gpt"})
	if err == nil {
		t.Fatal("test command unexpectedly succeeded")
	}
	text := out.String()
	for _, want := range []string{"[for profile] were all set", "Recommended action", "aigw check"} {
		if !strings.Contains(text, want) {
			t.Fatalf("localized terminal error lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Connectivity test") {
		t.Fatalf("failed test command emitted partial success view:\n%s", text)
	}
}

func TestTestCommandExplainsUnconfiguredStateBeforeResolvingRoutes(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	err := cli.Execute(app, []string{"test", "--for", "claude"})
	if err == nil {
		t.Fatal("test command unexpectedly succeeded")
	}
	text := out.String()
	for _, want := range []string{"Not configured", "No service profiles have been created.", "aigw setup"} {
		if !strings.Contains(text, want) {
			t.Fatalf("unconfigured test output lacks %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"Connectivity test", `unknown profile ""`} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("unconfigured test output retained %q:\n%s", unwanted, text)
		}
	}
}

func TestEndpointTestAdmitsSelectorsBeforeConfigurationAndCredentials(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"empty client", []string{"test", "--for="}, "--for requires a non-empty value"},
		{"empty profile", []string{"test", "--profile="}, "--profile requires a non-empty value"},
		{"blank profile", []string{"test", "--profile", " "}, "--profile requires a non-empty value"},
		{"unknown client", []string{"test", "--for", "future"}, "--for must be"},
		{"conflicting selectors", []string{"test", "--profile", "gpt", "--for", "codex"}, "[for profile] were all set"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, _, secrets, runner, httpClient := testApp(t, "")
			credentials := &recordingCredentialStore[string]{backend: secrets}
			app.Secrets = credentials
			const original = "invalid configuration ["
			if err := os.WriteFile(app.Config.Path(), []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			err := cli.Execute(app, test.args)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v; want argument diagnostic %q", err, test.want)
			}
			if got, err := os.ReadFile(app.Config.Path()); err != nil || string(got) != original {
				t.Fatalf("configuration changed: %q, %v", got, err)
			}
			entries, err := os.ReadDir(filepath.Dir(app.Config.Path()))
			if err != nil || len(entries) != 1 {
				t.Fatalf("configuration storage changed: %v, %v", entries, err)
			}
			if len(credentials.getCalls)+len(credentials.existsCalls)+len(credentials.setCalls)+len(credentials.deleteCalls)+len(runner.plans)+httpClient.calls != 0 {
				t.Fatal("invalid selectors accessed credentials, client or network")
			}
		})
	}
}

func TestTestCommandUsesAnthropicAPIKeyHeader(t *testing.T) {
	app, out, secretStore, _, httpClient := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "anthropic", "Anthropic", configuration.Endpoints{Anthropic: "https://example.test"}, configuration.ClientClaude, "claude-test")
	cfg.Routes[configuration.ClientClaude] = "claude"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("anthropic", "anthropic-test-secret"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"test", "--for", "claude"}); err != nil {
		t.Fatal(err)
	}
	if httpClient.headers.Get("X-Api-Key") != "anthropic-test-secret" {
		t.Fatalf("x-api-key header = %q", httpClient.headers.Get("X-Api-Key"))
	}
	if httpClient.headers.Get("Authorization") != "" {
		t.Fatalf("authorization header = %q", httpClient.headers.Get("Authorization"))
	}
	if strings.Contains(out.String(), "anthropic-test-secret") || strings.Contains(strings.ToLower(out.String()), "x-api-key") {
		t.Fatalf("credential leaked in output: %s", out.String())
	}
}

func TestTestCommandUsesAccountTokenForRuntimeProfile(t *testing.T) {
	app, out, secretStore, _, httpClient := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["gpt-5.6-sol"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-5.6-sol"}
	cfg.Routes[configuration.ClientCodex] = "gpt-5.6-sol"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "account-token")
	if err := cli.Execute(app, []string{"test", "--for", "codex"}); err != nil {
		t.Fatal(err)
	}
	if httpClient.headers.Get("Authorization") != "Bearer account-token" {
		t.Fatalf("authorization header = %q", httpClient.headers.Get("Authorization"))
	}
	if strings.Contains(out.String(), "account-token") || !strings.Contains(out.String(), "gpt-5.6-sol") {
		t.Fatalf("test output = %s", out.String())
	}
}

func TestTestCommandUsesCodexModelsEndpointAndRejectsNotFound(t *testing.T) {
	app, _, secretStore, _, httpClient := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["gpt-5.6-sol"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-5.6-sol"}
	cfg.Routes[configuration.ClientCodex] = "gpt-5.6-sol"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "account-token")
	var gotPath string
	httpClient.handler = func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[]}`)), Request: req}, nil
	}
	if err := cli.Execute(app, []string{"test", "--for", "codex"}); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/models" {
		t.Fatalf("codex test path = %q", gotPath)
	}

	httpClient.handler = func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{"message":"not found"}`)), Request: req}, nil
	}
	err := cli.Execute(app, []string{"test", "--for", "codex"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("error = %v", err)
	}
}

type contextBoundReadCloser struct {
	ctx    context.Context
	reader *strings.Reader
}

func (body *contextBoundReadCloser) Read(data []byte) (int, error) {
	select {
	case <-body.ctx.Done():
		return 0, body.ctx.Err()
	default:
		return body.reader.Read(data)
	}
}

func (body *contextBoundReadCloser) Close() error { return nil }

type closeFailingBody struct {
	io.Reader
	err error
}

func (body closeFailingBody) Close() error { return body.err }

func TestCheckUsesBoundedAuthenticationStabilityWithoutMutation(t *testing.T) {
	tests := []struct {
		name       string
		statuses   []int
		wantError  bool
		wantText   []string
		rejectText []string
	}{
		{
			name:       "healthy first observation",
			statuses:   []int{http.StatusOK},
			wantText:   []string{"Claude", "Endpoint checked", "All enabled route checks passed"},
			rejectText: []string{"transient response", "aigw rotate"},
		},
		{
			name:       "recovered transient",
			statuses:   []int{http.StatusUnauthorized, http.StatusOK, http.StatusOK, http.StatusOK},
			wantText:   []string{"Claude", "Endpoint checked", "Claude authentication recovered after a transient response", "All enabled route checks passed"},
			rejectText: []string{"aigw rotate"},
		},
		{
			name:      "persistent invalid token",
			statuses:  []int{http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized},
			wantError: true,
			wantText:  []string{"Account Token is invalid or belongs to a different service", "aigw rotate dmx"},
		},
		{
			name:       "unstable authentication",
			statuses:   []int{http.StatusUnauthorized, http.StatusOK, http.StatusUnauthorized, http.StatusOK},
			wantError:  true,
			wantText:   []string{"Authentication could not be confirmed consistently", "aigw check"},
			rejectText: []string{"aigw rotate"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, out, secretStore, _, httpClient := testApp(t, "")
			cfg := configuration.NewConfig()
			addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{Anthropic: "https://dmx.test"}, configuration.ClientClaude, "claude-test")
			cfg.Routes[configuration.ClientClaude] = "dmx"
			cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
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
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"message":"bounded observation"}`)), Request: req}, nil
			}

			err := cli.Execute(app, []string{"check"})
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

func TestCheckIdentifiesExternalLoopbackTransportWithoutClaimingOwnership(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "local", "local", "Local Compatibility Layer", configuration.Endpoints{Anthropic: "http://127.0.0.1:4567"}, configuration.ClientClaude, "model-test")
	cfg.Routes[configuration.ClientClaude] = "local"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("local", "token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"check"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"Claude", "external loopback compatibility layer that AIGW does not manage"} {
		if !strings.Contains(text, want) {
			t.Fatalf("check lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "4567") {
		t.Fatalf("check exposed the loopback transport port:\n%s", text)
	}
}
