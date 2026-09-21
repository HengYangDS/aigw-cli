package readiness

import (
	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestEndpointTestExplicitKeyringFormatDecodesExactlyOnce(t *testing.T) {
	const token = "public-fixture-token"
	envelope := "go-keyring-base64:" + base64.StdEncoding.EncodeToString([]byte(token))
	for _, explicit := range []bool{false, true} {
		runtime, _, _ := configuredReadinessRuntime(t)
		runtime.In = strings.NewReader(envelope)
		runtime.Secrets = nil
		want := envelope
		args := []string{"--for", "codex", "--profile", "codex", "--token-stdin"}
		if explicit {
			want = token
			args = append(args, "--token-format", "go-keyring-base64")
		}
		requests := 0
		runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests++
			if request.Header.Get("Authorization") != "Bearer "+want {
				t.Fatal("explicit input format did not preserve its exact interpretation")
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}"))}, nil
		})
		command := NewTestCommand(runtime)
		command.SetArgs(args)
		if err := executeCommand(command); err != nil || requests != 1 {
			t.Fatalf("format explicit=%v requests=%d error=%v", explicit, requests, err)
		}
	}
}

func TestEndpointTestRejectsMalformedKeyringFormatBeforeHTTP(t *testing.T) {
	for _, token := range []string{"", "token\r\nX-Injected: value", "token\x00", "token\n", " token", "秘密"} {
		runtime, _, _ := configuredReadinessRuntime(t)
		runtime.In = strings.NewReader("go-keyring-base64:" + base64.StdEncoding.EncodeToString([]byte(token)))
		runtime.Secrets = nil
		requests := 0
		runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) { requests++; return nil, errors.New("unexpected request") })
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--for", "codex", "--profile", "codex", "--token-stdin", "--token-format", "go-keyring-base64"})
		if err := executeCommand(command); err == nil || requests != 0 {
			t.Fatalf("invalid decoded token reached HTTP: error=%v requests=%d", err, requests)
		}
	}
}

func TestEndpointTestDefaultsToSelectedBindingsOnly(t *testing.T) {
	runtime, cfg, _ := configuredReadinessRuntime(t)
	delete(cfg.Clients, configuration.ClientClaude)
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
		t.Fatalf("requests = %d, want 1 selected Client Binding", requests)
	}
}

func TestEndpointTestUsesOneEphemeralTokenAndExplicitConfig(t *testing.T) {
	for _, client := range []string{configuration.ClientClaude, configuration.ClientCodex} {
		t.Run(client, func(t *testing.T) {
			runtime, _, output := configuredReadinessRuntime(t)
			configPath := runtime.Config.Path()
			before, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			runtime.Config = configuration.NewStore(filepath.Join(t.TempDir(), "unrelated.toml"))
			runtime.Secrets = nil
			runtime.In = io.MultiReader(strings.NewReader("ephemeral-"), strings.NewReader("token\r\n"))
			requests := 0
			runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
				requests++
				if client == configuration.ClientCodex && request.Header.Get("Authorization") != "Bearer ephemeral-token" {
					t.Fatal("request did not use the supplied Token")
				}
				if client == configuration.ClientClaude && request.Header.Get("X-Api-Key") != "ephemeral-token" {
					t.Fatal("request did not use the supplied Token")
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}"))}, nil
			})
			command := NewTestCommand(runtime)
			command.SetArgs([]string{"--for", client, "--profile", client, "--token-stdin", "--config", configPath})
			if err := executeCommand(command); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(configPath)
			if err != nil || !bytes.Equal(before, after) || requests != 1 {
				t.Fatalf("ephemeral request changed config or request count: requests=%d error=%v", requests, err)
			}
			if strings.Contains(output.String(), "ephemeral-token") || !strings.Contains(output.String(), "not model inference") {
				t.Fatalf("result did not preserve evidence or secret boundary: %s", output.String())
			}
			entries, err := os.ReadDir(filepath.Dir(configPath))
			if err != nil || len(entries) != 1 {
				t.Fatalf("read-only test created state: entries=%d error=%v", len(entries), err)
			}
		})
	}
}

func TestEndpointTestRejectsAmbiguousEphemeralInputsBeforeEffects(t *testing.T) {
	for _, args := range [][]string{
		{"--token-stdin"},
		{"--token-stdin", "--for", "all"},
		{"--token-stdin", "--profile", "missing"},
		{"--token-stdin", "--for", "codex", "--profile", "codex", "--config", "relative.toml"},
		{"--token-stdin", "--for", "codex", "--profile", "codex", "--config", ""},
		{"--for", "codex", "--profile", "codex", "--config", "/unrelated/config.toml"},
	} {
		runtime, _, _ := configuredReadinessRuntime(t)
		store := &observingSecretStore{}
		runtime.Secrets = store
		runtime.In = failingReader{err: errors.New("stdin must remain unread")}
		requests := 0
		runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) {
			requests++
			return nil, errors.New("network must remain unused")
		})
		command := NewTestCommand(runtime)
		command.SetArgs(args)
		err := executeCommand(command)
		if err == nil || strings.Contains(err.Error(), "stdin must remain unread") || requests != 0 || store.getCalls != 0 {
			t.Fatalf("invalid selection had effects: args=%v error=%v requests=%d reads=%d", args, err, requests, store.getCalls)
		}
	}
}

func TestEndpointTestRejectsMalformedEphemeralTokenBeforeHTTP(t *testing.T) {
	for _, input := range []string{"", "token\nextra", "token\r\nX-Injected: value", "token\x00", strings.Repeat("t", 65537)} {
		runtime, _, _ := configuredReadinessRuntime(t)
		runtime.In = strings.NewReader(input)
		runtime.Secrets = nil
		requests := 0
		runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) {
			requests++
			return nil, errors.New("network must remain unused")
		})
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--for", "codex", "--token-stdin"})
		if err := executeCommand(command); err == nil || requests != 0 {
			t.Fatalf("malformed input reached HTTP: error=%v requests=%d", err, requests)
		}
	}
}

func TestEndpointTestRequiresASelectedClientBindingByDefault(t *testing.T) {
	runtime, cfg, _ := configuredReadinessRuntime(t)
	cfg.Clients = map[string]configuration.ClientBinding{}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("endpoint request must not run without a selected Client Binding")
		return nil, nil
	})
	runtime.Problem = func(title, evidence, impact, fix string, cause error) error {
		if title != "No Client Binding is selected" || evidence != "Profiles exist, but no client has a selected Profile." || impact != "There is no selected endpoint to test." || fix != "aigw use --for <client> <profile>" {
			t.Fatalf("problem = %q, %q, %q, %q", title, evidence, impact, fix)
		}
		return cause
	}

	err := executeCommand(NewTestCommand(runtime))
	if err == nil || !strings.Contains(err.Error(), "no Client Binding selected") {
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
	binding := cfg.Clients[configuration.ClientCodex]
	binding.ModelProvider = "amazon-bedrock"
	binding.Authentication = configuration.AuthenticationClientNative
	cfg.Clients[configuration.ClientCodex] = binding
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
	t.Run("profile overrides the explicit client selection", func(t *testing.T) {
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
		command.SetArgs([]string{"--for", "codex", "--profile", "codex"})
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
		command.SetArgs([]string{"--for", "codex", "--profile", "missing"})
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
	if got := endpointTransport("%"); got != "" {
		t.Fatalf("invalid transport = %#v", got)
	}
	if got := endpointTransport("http://LOCALHOST:48721/v1"); got != "external_loopback" {
		t.Fatalf("loopback transport = %#v", got)
	}
	if got := endpointTransport("https://api.example.test/v1"); got != "" {
		t.Fatalf("remote transport = %#v", got)
	}
	buffer := &bytes.Buffer{}
	runtime := invocation.Context{Config: configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml")), Out: buffer, Secrets: secrets.NewMemoryStore()}
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buffer.String(), "Not configured") {
		t.Fatalf("status output = %q", buffer.String())
	}
	command := NewStatusCommand(invocation.Context{Config: configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml")), Out: io.Discard, Secrets: secrets.NewMemoryStore()})
	command.SetArgs([]string{"--json"})
	if err := executeCommand(command); err != nil {
		t.Fatal(err)
	}
}
