package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
)

func TestCatalogUnconfiguredPointsToSetup(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	err := cli.Execute(app, []string{"catalog"})
	if err == nil {
		t.Fatal("catalog succeeded without configuration")
	}
	text := out.String() + "\n" + err.Error()
	if !strings.Contains(text, "aigw setup") || strings.Contains(text, "aigw profile add") {
		t.Fatalf("catalog should direct first use to setup:\n%s", text)
	}
}

func TestCatalogJSONUnconfiguredIsEmptyAndMachineReadable(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	if err := cli.Execute(app, []string{"catalog", "--json"}); err != nil {
		t.Fatalf("catalog --json error = %v", err)
	}
	if strings.TrimSpace(out.String()) != "{\n  \"observations\": []\n}" {
		t.Fatalf("catalog --json = %q", out.String())
	}
}

func TestCatalogDiscoversSortedModelsWithoutWritingConfigOrLeakingToken(t *testing.T) {
	app, out, secretStore, _, httpClient := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://responses.dmx.test/v1", Anthropic: "https://anthropic.dmx.test"}}
	cfg.Routes["gpt-configured"] = qualifiedRoute("GPT", "dmx", "gpt-5.6", configuration.ProtocolOpenAIResponses)
	cfg.Routes["claude-configured"] = qualifiedRoute("Claude", "dmx", "gpt-5.6", configuration.ProtocolAnthropic)
	cfg.SetSelectedRoute(configuration.ClientCodex, "gpt-configured")
	cfg.SetSelectedRoute(configuration.ClientClaude, "claude-configured")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	const token = "catalog-token-must-not-appear"
	if err := secretStore.Set("dmx", token); err != nil {
		t.Fatal(err)
	}
	httpClient.handler = func(req *http.Request) (*http.Response, error) {
		assertCatalogRequest(t, req, token)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"z-model"},{"id":"gpt-5.6"}]}`)), Request: req}, nil
	}

	if err := cli.Execute(app, []string{"catalog", "--json"}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("catalog changed config\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if strings.Contains(out.String(), token) || strings.Contains(strings.ToLower(out.String()), "authorization") {
		t.Fatalf("catalog leaked secret material: %s", out.String())
	}
	var result catalogJSON
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	assertCatalogObservations(t, result)
}

type catalogJSON struct {
	Observations []catalogJSONObservation `json:"observations"`
}

type catalogJSONObservation struct {
	Account string `json:"account"`
	Source  struct {
		Protocol string `json:"protocol"`
		Endpoint string `json:"endpoint"`
	} `json:"source"`
	Status string `json:"status"`
	Models []struct {
		ID     string `json:"id"`
		State  string `json:"state"`
		Routes []struct {
			ID           string   `json:"id"`
			Capabilities []string `json:"capabilities"`
		} `json:"routes"`
	} `json:"models"`
}

func assertCatalogRequest(t *testing.T, request *http.Request, token string) {
	t.Helper()
	if request.URL.Path != "/v1/models" {
		t.Fatalf("catalog request = %s", request.URL)
	}
	switch request.URL.Host {
	case "anthropic.dmx.test":
		if request.Header.Get("X-Api-Key") != token || request.Header.Get("Authorization") != "" {
			t.Fatalf("Anthropic headers = %#v", request.Header)
		}
	case "responses.dmx.test":
		if request.Header.Get("Authorization") != "Bearer "+token || request.Header.Get("X-Api-Key") != "" {
			t.Fatalf("Responses headers = %#v", request.Header)
		}
	default:
		t.Fatalf("unexpected catalog source %s", request.URL)
	}
}

func assertCatalogObservations(t *testing.T, result catalogJSON) {
	t.Helper()
	if len(result.Observations) != 2 {
		t.Fatalf("catalog result = %#v", result)
	}
	wantRoutes := map[string]string{"anthropic": "claude-configured", "openai_responses": "gpt-configured"}
	for _, observation := range result.Observations {
		if observation.Account != "dmx" || observation.Status != "ok" || len(observation.Models) != 2 {
			t.Fatalf("catalog observation = %#v", observation)
		}
		models := observation.Models
		if models[0].ID != "gpt-5.6" || models[0].State != "admitted" || len(models[0].Routes) != 1 || models[1].ID != "z-model" || models[1].State != "candidate" {
			t.Fatalf("catalog models = %#v", models)
		}
		if models[0].Routes[0].ID != wantRoutes[observation.Source.Protocol] {
			t.Fatalf("protocol %q admitted Routes = %#v", observation.Source.Protocol, models[0].Routes)
		}
	}
}

func TestCatalogDefaultHumanOutputShowsOnlyConfiguredModels(t *testing.T) {
	app, out, secretStore, _, httpClient := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Routes["configured"] = qualifiedRoute("Configured", "gateway", "configured-model", configuration.ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(configuration.ClientCodex, "configured")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "catalog-token"); err != nil {
		t.Fatal(err)
	}
	httpClient.handler = func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"configured-model"},{"id":"unconfigured-model"}]}`)), Request: req}, nil
	}

	if err := cli.Execute(app, []string{"catalog"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"2 observed", "1 admitted", "configured-model", "1 candidate models require qualification", "aigw catalog --all"} {
		if !strings.Contains(text, want) {
			t.Fatalf("compact catalog lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "unconfigured-model") {
		t.Fatalf("compact catalog leaked an unconfigured model:\n%s", text)
	}
}

func TestCatalogAllHumanOutputIncludesEveryModelAsReadableRecord(t *testing.T) {
	app, out, secretStore, _, httpClient := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Routes["configured"] = qualifiedRoute("Configured", "gateway", "configured-model", configuration.ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(configuration.ClientCodex, "configured")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "catalog-token"); err != nil {
		t.Fatal(err)
	}
	httpClient.handler = func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"configured-model"},{"id":"unconfigured-model"}]}`)), Request: req}, nil
	}

	if err := cli.Execute(app, []string{"catalog", "--all"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"configured-model", "Admitted Routes: configured", "unconfigured-model", "Candidate; protocol and client capabilities are unqualified"} {
		if !strings.Contains(text, want) {
			t.Fatalf("full catalog lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "unconfigured-modelCandidate") {
		t.Fatalf("full catalog ran together the model and its status:\n%s", text)
	}
}

func TestCatalogRejectsAllWithJSON(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	err := cli.Execute(app, []string{"catalog", "--all", "--json"})
	if err == nil || !strings.Contains(err.Error(), "--all cannot be used with --json") {
		t.Fatalf("catalog flags error = %v", err)
	}
}

func TestCatalogReportsUnavailableAccountWithoutBlockingHealthyAccount(t *testing.T) {
	app, out, secretStore, _, httpClient := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["healthy"] = configuration.Account{Label: "Healthy", Endpoints: configuration.Endpoints{OpenAIResponses: "https://healthy.test/v1"}}
	cfg.Accounts["missing-token"] = configuration.Account{Label: "Missing Token", Endpoints: configuration.Endpoints{OpenAIResponses: "https://missing.test/v1"}}
	cfg.Accounts["anthropic-only"] = configuration.Account{Label: "Anthropic Only", Endpoints: configuration.Endpoints{Anthropic: "https://anthropic.test"}}
	cfg.Routes["healthy-model"] = qualifiedRoute("Healthy", "healthy", "healthy-model", configuration.ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(configuration.ClientCodex, "healthy-model")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("healthy", "healthy-token"); err != nil {
		t.Fatal(err)
	}
	httpClient.handler = func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "healthy.test" {
			t.Fatalf("unexpected request to %s", req.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"healthy-model"}]}`)), Request: req}, nil
	}

	if err := cli.Execute(app, []string{"catalog", "--json"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"account": "healthy"`, `"status": "ok"`, `"account": "missing-token"`, `"status": "token_unavailable"`, `"account": "anthropic-only"`, `"protocol": "anthropic"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("catalog output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestCatalogReportsMalformedAccountPayloadWithoutBlockingHealthyAccount(t *testing.T) {
	app, out, secretStore, _, httpClient := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["broken"] = configuration.Account{Label: "Broken", Endpoints: configuration.Endpoints{OpenAIResponses: "https://broken.test/v1"}}
	cfg.Accounts["healthy"] = configuration.Account{Label: "Healthy", Endpoints: configuration.Endpoints{OpenAIResponses: "https://healthy.test/v1"}}
	cfg.Routes["healthy-model"] = qualifiedRoute("Healthy", "healthy", "healthy-model", configuration.ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(configuration.ClientCodex, "healthy-model")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	for _, account := range []string{"broken", "healthy"} {
		if err := secretStore.Set(account, account+"-token"); err != nil {
			t.Fatal(err)
		}
	}
	httpClient.handler = func(req *http.Request) (*http.Response, error) {
		body := `{"data":[{"id":"healthy-model"}]}`
		if req.URL.Host == "broken.test" {
			body = `{}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	}

	if err := cli.Execute(app, []string{"catalog", "--json"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"account": "broken"`, `"status": "request_failed"`, `"account": "healthy"`, `"status": "ok"`, `"id": "healthy-model"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("catalog output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestModelsCommandReportsCatalogMembershipWithoutClaimingReachability(t *testing.T) {
	app, out, secretStore, _, httpClient := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Routes["gpt-5.6-sol"] = qualifiedRoute("GPT-5.6 Sol Codex", "dmx", "gpt-5.6-sol", configuration.ProtocolOpenAIResponses)
	cfg.Routes["gpt-5.6"] = qualifiedRoute("GPT-5.6", "dmx", "gpt-5.6", configuration.ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(configuration.ClientCodex, "gpt-5.6-sol")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	httpClient.handler = func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/models" {
			t.Fatalf("unexpected model-discovery request: %s %s", req.Method, req.URL.Path)
		}
		body := `{"data":[{"id":"gpt-5.6-sol"},{"id":"gpt-5.5"}]}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	}
	if err := cli.Execute(app, []string{"models"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "gpt-5.6-sol") || !strings.Contains(text, "Listed") || !strings.Contains(text, "gpt-5.6") || !strings.Contains(text, "Not listed") || !strings.Contains(text, "does not prove protocol capabilities, inference, or client readiness") {
		t.Fatalf("models output = %s", text)
	}
}

func TestModelsCommandKeepsLongProfileNamesOnOneLine(t *testing.T) {
	app, out, secretStore, _, httpClient := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
	cfg.Routes["claude-opus-5"] = qualifiedRoute("Claude Opus 5", "dmx", "claude-opus-5", configuration.ProtocolAnthropic)
	cfg.SetSelectedRoute(configuration.ClientClaude, "claude-opus-5")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	httpClient.handler = func(req *http.Request) (*http.Response, error) {
		body := `{"data":[{"id":"claude-opus-5"}]}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	}
	if err := cli.Execute(app, []string{"models"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Contains(text, "claude-opus-4-8-\n") || strings.Contains(text, "thinking      ") {
		t.Fatalf("long profile name was wrapped or column-padded badly:\n%s", text)
	}
	if !strings.Contains(text, "Route  claude-opus-5") || !strings.Contains(text, "claude-opus-5 · Anthropic Messages · Listed · account dmx") {
		t.Fatalf("models output should use detail layout for long Route names:\n%s", text)
	}
}
