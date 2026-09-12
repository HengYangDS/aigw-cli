package readiness

import (
	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestEndpointTestDefaultsToSelectedRoutesOnly(t *testing.T) {
	runtime, cfg, _ := configuredReadinessRuntime(t)
	delete(cfg.Routes, configuration.ClientClaude)
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Secrets.Set("one", "token"); err != nil {
		t.Fatal(err)
	}

	requests := 0
	runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.URL.Host != "codex.example.test" {
			t.Fatalf("unexpected unselected endpoint request: %s", request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}, nil
	})

	if err := executeCommand(NewTestCommand(runtime)); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1 selected Route", requests)
	}
}

func TestEndpointTestRequiresASelectedRouteByDefault(t *testing.T) {
	runtime, cfg, _ := configuredReadinessRuntime(t)
	cfg.Routes = configuration.Routes{}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("endpoint request must not run without a selected Route")
		return nil, nil
	})
	runtime.Problem = func(title, evidence, impact, fix string, cause error) error {
		if title != "No Route is selected" || evidence != "Profiles exist, but no client has an active Route." || impact != "There is no selected service endpoint to test." || fix != "aigw use <profile>" {
			t.Fatalf("problem = %q, %q, %q, %q", title, evidence, impact, fix)
		}
		return cause
	}

	err := executeCommand(NewTestCommand(runtime))
	if err == nil || !strings.Contains(err.Error(), "no route selected") {
		t.Fatalf("error = %v", err)
	}
}

func TestEndpointTestCommandCoversSuccessAndTransportFailures(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		runtime, _, buffer := configuredReadinessRuntime(t)
		if err := runtime.Secrets.Set("one", "token"); err != nil {
			t.Fatal(err)
		}
		requests := 0
		runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests++
			if strings.Contains(request.URL.Host, "claude") && request.Header.Get("X-Api-Key") != "token" {
				t.Fatalf("Claude credential header = %q", request.Header.Get("X-Api-Key"))
			}
			if strings.Contains(request.URL.Host, "codex") && request.Header.Get("Authorization") != "Bearer token" {
				t.Fatalf("Codex credential header = %q", request.Header.Get("Authorization"))
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}, nil
		})
		command := NewTestCommand(runtime)
		if err := executeCommand(command); err != nil {
			t.Fatal(err)
		}
		if requests != 2 || !strings.Contains(buffer.String(), "Connectivity test") {
			t.Fatalf("requests=%d output=%q", requests, buffer.String())
		}
	})

	tests := []struct {
		name         string
		status       int
		body         io.ReadCloser
		roundTripErr error
		want         string
	}{
		{name: "network", roundTripErr: errors.New("offline"), want: "unreachable"},
		{name: "authentication", status: http.StatusUnauthorized, body: io.NopCloser(strings.NewReader("denied")), want: "authentication was rejected"},
		{name: "unexpected status", status: http.StatusBadGateway, body: io.NopCloser(strings.NewReader("failed")), want: "HTTP 502"},
		{name: "read", status: http.StatusOK, body: io.NopCloser(failingReader{err: errors.New("read failed")}), want: "read Codex endpoint response"},
		{name: "close", status: http.StatusOK, body: closeErrorBody{Reader: strings.NewReader("ok")}, want: "close Codex endpoint response"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtime, _, _ := configuredReadinessRuntime(t)
			if err := runtime.Secrets.Set("one", "token"); err != nil {
				t.Fatal(err)
			}
			runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) {
				if test.roundTripErr != nil {
					return nil, test.roundTripErr
				}
				return &http.Response{StatusCode: test.status, Body: test.body}, nil
			})
			command := NewTestCommand(runtime)
			command.SetArgs([]string{"--for", "codex"})
			if err := executeCommand(command); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestEndpointTestRejectsClientNativeBeforeCredentialOrNetworkAccess(t *testing.T) {
	runtime, cfg, _ := configuredReadinessRuntime(t)
	profile := cfg.Profiles["codex"]
	profile.ModelProvider = "amazon-bedrock"
	profile.Authentication = configuration.AuthenticationClientNative
	cfg.Profiles["codex"] = profile
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	store := &observingSecretStore{getErr: errors.New("client-native credential access")}
	runtime.Secrets = store
	requests := 0
	runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, errors.New("client-native endpoint probe")
	})
	command := NewTestCommand(runtime)
	command.SetArgs([]string{"--for", configuration.ClientCodex})

	err := executeCommand(command)
	if err == nil || !strings.Contains(err.Error(), "aigw verify --for codex") {
		t.Fatalf("client-native endpoint test error = %v", err)
	}
	if store.getCalls != 0 || store.existsCalls != 0 || requests != 0 {
		t.Fatalf("client-native endpoint test used AIGW authentication capabilities: get=%d exists=%d HTTP=%d", store.getCalls, store.existsCalls, requests)
	}
}

func TestEndpointTestCommandCoversInputAndResolutionFailures(t *testing.T) {
	t.Run("profile infers client", func(t *testing.T) {
		runtime, _, _ := configuredReadinessRuntime(t)
		if err := runtime.Secrets.Set("one", "token"); err != nil {
			t.Fatal(err)
		}
		var requests int
		runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests++
			if request.URL.Path != "/v1/models" {
				t.Fatalf("request path = %q", request.URL.Path)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}"))}, nil
		})
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--profile", "codex"})
		if err := executeCommand(command); err != nil {
			t.Fatal(err)
		}
		if requests != 1 {
			t.Fatalf("requests = %d, want 1", requests)
		}
	})

	t.Run("unknown profile is rejected", func(t *testing.T) {
		runtime, _, _ := configuredReadinessRuntime(t)
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--profile", "missing"})
		if err := executeCommand(command); err == nil || !strings.Contains(err.Error(), "unknown profile") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("invalid client", func(t *testing.T) {
		runtime, _, _ := configuredReadinessRuntime(t)
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--for", "future"})
		if err := executeCommand(command); err == nil || !strings.Contains(err.Error(), "--for must be") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("not configured", func(t *testing.T) {
		problem := errors.New("structured problem")
		runtime := invocation.Context{Config: configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml")), Out: io.Discard, Problem: func(string, string, string, string, error) error { return problem }}
		if err := executeCommand(NewTestCommand(runtime)); !errors.Is(err, problem) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		runtime, _, _ := configuredReadinessRuntime(t)
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--for", "codex"})
		if err := executeCommand(command); err == nil || !strings.Contains(err.Error(), "token for account") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing explicit endpoint", func(t *testing.T) {
		runtime, cfg, _ := configuredReadinessRuntime(t)
		accountConfig := cfg.Accounts["one"]
		accountConfig.Endpoints.OpenAIResponses = ""
		cfg.Accounts["one"] = accountConfig
		data, err := toml.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(runtime.Config.Path(), data, 0o600); err != nil {
			t.Fatal(err)
		}
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--for", "codex"})
		if err := executeCommand(command); err == nil || !strings.Contains(err.Error(), "no OpenAI Responses endpoint") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestReadinessTransportHelpers(t *testing.T) {
	if got := TransportStatus("%"); got.Kind != "" {
		t.Fatalf("invalid transport = %#v", got)
	}
	if got := TransportStatus("http://LOCALHOST:8791/v1"); got.Kind != "external_loopback" {
		t.Fatalf("loopback transport = %#v", got)
	}
	if got := TransportStatus("https://api.example.test/v1"); got.Kind != "" {
		t.Fatalf("remote transport = %#v", got)
	}
	buffer := &bytes.Buffer{}
	renderer := Renderer(invocation.Context{Out: buffer})
	renderer.Text("fallback writer")
	if !strings.Contains(buffer.String(), "fallback writer") {
		t.Fatalf("renderer output = %q", buffer.String())
	}
	command := NewStatusCommand(invocation.Context{Config: configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml")), Out: io.Discard, Secrets: secrets.NewMemoryStore()})
	command.SetArgs([]string{"--json"})
	if err := executeCommand(command); err != nil {
		t.Fatal(err)
	}
}
