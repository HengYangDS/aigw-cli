package cli_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
)

func TestSyncAndCheckTreatDirectAndLoopbackEndpointsAsOrdinaryAccountChoices(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
	}{
		{name: "direct HTTPS", endpoint: "https://provider.test/v1"},
		{name: "explicit loopback", endpoint: "http://127.0.0.1:48721/v1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, out, secretStore, runner, httpClient := testApp(t, "")
			target := filepath.Join(t.TempDir(), "configuration.toml")
			if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg := configuration.NewConfig()
			addAccountRoute(&cfg, "codex", "provider", "Provider", configuration.Endpoints{OpenAIResponses: test.endpoint}, configuration.ClientCodex, "gpt-test")
			cfg.SetSelectedRoute(configuration.ClientCodex, "codex")
			cfg.SetClientActivation(configuration.ClientCodex, true, "/usr/local/bin/codex", []string{target})
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := secretStore.Set("provider", "test-token"); err != nil {
				t.Fatal(err)
			}

			if err := cli.Execute(app, []string{"sync"}); err != nil {
				t.Fatalf("sync: %v", err)
			}
			projection, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(projection), `base_url = "`+test.endpoint+`"`) {
				t.Fatalf("projection does not contain selected Account endpoint:\n%s", projection)
			}
			if len(runner.plans) != 0 {
				t.Fatalf("sync started an external process: %#v", runner.plans)
			}

			var requestURL string
			httpClient.handler = func(request *http.Request) (*http.Response, error) {
				requestURL = request.URL.String()
				return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: request}, nil
			}
			out.Reset()
			if err := cli.Execute(app, []string{"check", "--endpoint-only"}); err != nil {
				t.Fatalf("check: %v\n%s", err, out.String())
			}
			if want := strings.TrimRight(test.endpoint, "/") + "/models"; requestURL != want {
				t.Fatalf("diagnostic URL = %q, want %q", requestURL, want)
			}
			if len(runner.plans) != 0 {
				t.Fatalf("check started an external process: %#v", runner.plans)
			}
		})
	}
}

func TestCheckIdentifiesExternalLoopbackTransportWithoutClaimingOwnership(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountRoute(&cfg, "local", "local", "Local Endpoint", configuration.Endpoints{Anthropic: "http://127.0.0.2:4567"}, configuration.ClientClaude, "model-test")
	cfg.SetSelectedRoute(configuration.ClientClaude, "local")
	cfg.SetClientActivation(configuration.ClientClaude, true, executableFixture(t, "claude"), nil)
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
	for _, want := range []string{"Claude", "uses a loopback endpoint; AIGW does not manage the endpoint runtime"} {
		if !strings.Contains(text, want) {
			t.Fatalf("check lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "4567") {
		t.Fatalf("check exposed the loopback transport port:\n%s", text)
	}
}

func TestCheckDoesNotDescribeRemoteHTTPSAsExternalLoopbackTransport(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountRoute(&cfg, "remote", "remote", "Remote Gateway", configuration.Endpoints{Anthropic: "https://gateway.test"}, configuration.ClientClaude, "model-test")
	cfg.SetSelectedRoute(configuration.ClientClaude, "remote")
	cfg.SetClientActivation(configuration.ClientClaude, true, executableFixture(t, "claude"), nil)
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("remote", "token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"check"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "uses a loopback endpoint") {
		t.Fatalf("check misclassified remote endpoint:\n%s", out.String())
	}
}

func TestEndpointOnlyCheckUsesSelectedTLSOrLoopbackAccount(t *testing.T) {
	for _, test := range []struct {
		name      string
		newServer func(http.Handler) *httptest.Server
		tls       bool
	}{
		{name: "direct TLS", newServer: httptest.NewTLSServer, tls: true},
		{name: "external loopback", newServer: httptest.NewServer},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests := make(chan *http.Request, 2)
			server := test.newServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				requests <- request
				writer.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(server.Close)
			app, _, secrets, runner, _ := testApp(t, "")
			app.HTTP = server.Client()
			target := filepath.Join(t.TempDir(), "codex.toml")
			if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg := configuration.NewConfig()
			addAccountRoute(&cfg, "codex", "selected", "Selected", configuration.Endpoints{OpenAIResponses: server.URL + "/v1"}, configuration.ClientCodex, "gpt-test")
			cfg.SetSelectedRoute(configuration.ClientCodex, "codex")
			cfg.SetClientActivation(configuration.ClientCodex, true, "/usr/local/bin/codex", []string{target})
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := secrets.Set("selected", "synthetic-token"); err != nil {
				t.Fatal(err)
			}
			if err := cli.Execute(app, []string{"sync"}); err != nil {
				t.Fatal(err)
			}
			if err := cli.Execute(app, []string{"check", "--endpoint-only"}); err != nil {
				t.Fatal(err)
			}
			select {
			case request := <-requests:
				if request.URL.Path != "/v1/models" || request.Header.Get("Authorization") != "Bearer synthetic-token" || (request.TLS != nil) != test.tls {
					t.Fatalf("selected Account request = %s, auth=%q, TLS=%t", request.URL.Path, request.Header.Get("Authorization"), request.TLS != nil)
				}
			default:
				t.Fatal("selected Account endpoint was not called")
			}
			if len(runner.plans) != 0 {
				t.Fatalf("control-plane check started an external service: %#v", runner.plans)
			}
		})
	}
}

func TestSelectedAccountWorksWithoutInstalledClient(t *testing.T) {
	t.Chdir(t.TempDir())
	requests := make(chan *http.Request, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests <- request
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	app, _, secrets, runner, _ := testApp(t, "")
	app.Discovery = fakeDiscovery{}
	app.HTTP = server.Client()
	cfg := configuration.NewConfig()
	addAccountRoute(&cfg, "codex", "provider", "Provider", configuration.Endpoints{OpenAIResponses: server.URL + "/v1"}, configuration.ClientCodex, "gpt-test")
	cfg.SetSelectedRoute(configuration.ClientCodex, "codex")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secrets.Set("provider", "synthetic-token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync without installed client: %v", err)
	}
	if err := cli.Execute(app, []string{"test", "--for", "codex"}); err != nil {
		t.Fatalf("test selected Account without installed client: %v", err)
	}
	select {
	case request := <-requests:
		if request.URL.Path != "/v1/models" || request.Header.Get("Authorization") != "Bearer synthetic-token" || request.TLS == nil {
			t.Fatalf("selected Account request = %s, auth=%q, TLS=%t", request.URL.Path, request.Header.Get("Authorization"), request.TLS != nil)
		}
	default:
		t.Fatal("selected Account endpoint was not called")
	}
	if len(runner.plans) != 0 {
		t.Fatalf("Account journey started a client or external process: %#v", runner.plans)
	}
}
