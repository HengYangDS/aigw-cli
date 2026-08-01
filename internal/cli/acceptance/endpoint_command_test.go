package cli_test

import (
	configuration "aigw-cli/internal/configuration"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestTestCommandSurfacesOperationalFailures(t *testing.T) {
	t.Run("config load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := execute(t, app, "test"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("not configured", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		err := execute(t, app, "test")
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "not configured") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt"})
		err := execute(t, app, "test", "--for", "codex")
		if err == nil || !strings.Contains(err.Error(), "is unavailable") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("transport", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt"})
		_ = secretStore.Set("one", "token")
		want := errors.New("network down")
		app.HTTP.(*fakeHTTP).handler = func(*http.Request) (*http.Response, error) { return nil, want }
		if err := execute(t, app, "test", "--for", "codex"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("response close", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt"})
		_ = secretStore.Set("one", "token")
		want := errors.New("close failed")
		app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: closeFailingBody{Reader: strings.NewReader("ok"), err: want}, Request: req}, nil
		}
		if err := execute(t, app, "test", "--for", "codex"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("server status", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt"})
		_ = secretStore.Set("one", "token")
		app.HTTP.(*fakeHTTP).status = http.StatusInternalServerError
		err := execute(t, app, "test", "--for", "codex")
		if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestTestCommandAuthenticatesWithoutPrintingAuthorizationHeader(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMX", configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}, "", configuration.Models{})
	cfg.Routes.Default = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "unused-secret")
	if err := execute(t, app, "test", "--for", "codex"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Connectivity test") || !strings.Contains(out.String(), "Codex") || !strings.Contains(out.String(), "HTTP 200") {
		t.Fatalf("test output = %s", out.String())
	}
	httpClient := app.HTTP.(*fakeHTTP)
	if httpClient.headers.Get("Authorization") != "Bearer unused-secret" {
		t.Fatalf("authorization header = %q", httpClient.headers.Get("Authorization"))
	}
	if strings.Contains(out.String(), "unused-secret") || strings.Contains(strings.ToLower(out.String()), "authorization") {
		t.Fatalf("credential leaked in output: %s", out.String())
	}
}

func TestTestCommandReturnsResponseReadFailure(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMX", configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}, "", configuration.Models{})
	cfg.Routes.Default = "dmx"
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

	err := execute(t, app, "test", "--for", "codex")
	if !errors.Is(err, want) {
		t.Fatalf("test command error = %v, want %v", err, want)
	}
}

func TestTestCommandKeepsRequestContextAliveUntilResponseIsDrained(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMX", configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}, "", configuration.Models{})
	cfg.Routes.Default = "dmx"
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

	if err := execute(t, app, "test", "--for", "codex"); err != nil {
		t.Fatalf("test command error = %v", err)
	}
}

func TestTestCommandTreatsClaudeBaseURLNotFoundAsTransportReachable(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMX", configuration.Endpoints{Anthropic: "https://example.test"}, "", configuration.Models{})
	cfg.Routes.Default = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "probe-secret")
	app.HTTP.(*fakeHTTP).status = http.StatusNotFound
	if err := execute(t, app, "test", "--for", "claude"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "HTTP 404 · Service is reachable; the base URL does not provide a GET probe") {
		t.Fatalf("Claude base URL 404 probe result = %s", out.String())
	}
}

func TestTestCommandRejectsAuthenticationFailure(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMX", configuration.Endpoints{Anthropic: "https://example.test"}, "", configuration.Models{})
	cfg.Routes.Default = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "rejected-secret")
	app.HTTP.(*fakeHTTP).status = 401
	err := execute(t, app, "test", "--for", "claude")
	if err == nil || !strings.Contains(err.Error(), "authentication was rejected") || strings.Contains(err.Error(), "rejected-secret") {
		t.Fatalf("error = %v", err)
	}
}
