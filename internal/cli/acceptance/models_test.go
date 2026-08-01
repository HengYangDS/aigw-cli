package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
)

func TestCatalogDiscoversSortedModelsWithoutWritingConfigOrLeakingToken(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["gpt-configured"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-5.6"}}
	cfg.Profiles["claude-configured"] = configuration.Profile{Label: "Claude", Account: "dmx", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "gpt-5.6"}}
	cfg.Routes.Default = "gpt-configured"
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
	app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/models" || req.Header.Get("Authorization") != "Bearer "+token {
			t.Fatalf("catalog request = %s authorization=%q", req.URL, req.Header.Get("Authorization"))
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"z-model"},{"id":"gpt-5.6"}]}`)), Request: req}, nil
	}

	if err := execute(t, app, "catalog", "--json"); err != nil {
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
	var result struct {
		Accounts []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Models []struct {
				ID       string   `json:"id"`
				Profiles []string `json:"profiles"`
			} `json:"models"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Accounts) != 1 || result.Accounts[0].ID != "dmx" || result.Accounts[0].Status != "ok" || len(result.Accounts[0].Models) != 2 {
		t.Fatalf("catalog result = %#v", result)
	}
	models := result.Accounts[0].Models
	if models[0].ID != "gpt-5.6" || strings.Join(models[0].Profiles, ",") != "claude-configured,gpt-configured" || models[1].ID != "z-model" || len(models[1].Profiles) != 0 {
		t.Fatalf("catalog models = %#v", models)
	}
}

func TestCatalogDefaultHumanOutputShowsOnlyConfiguredModels(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Profiles["configured"] = configuration.Profile{Label: "Configured", Account: "gateway", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "configured-model"}}
	cfg.Routes.Default = "configured"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "catalog-token"); err != nil {
		t.Fatal(err)
	}
	app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"configured-model"},{"id":"unconfigured-model"}]}`)), Request: req}, nil
	}

	if err := execute(t, app, "catalog"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"2 models", "1 configured", "configured-model", "1 more models are unconfigured", "aigw catalog --all"} {
		if !strings.Contains(text, want) {
			t.Fatalf("compact catalog lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "unconfigured-model") {
		t.Fatalf("compact catalog leaked an unconfigured model:\n%s", text)
	}
}

func TestCatalogAllHumanOutputIncludesEveryModelAsReadableRecord(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Profiles["configured"] = configuration.Profile{Label: "Configured", Account: "gateway", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "configured-model"}}
	cfg.Routes.Default = "configured"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "catalog-token"); err != nil {
		t.Fatal(err)
	}
	app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"configured-model"},{"id":"unconfigured-model"}]}`)), Request: req}, nil
	}

	if err := execute(t, app, "catalog", "--all"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"configured-model", "Configured: configured", "unconfigured-model", "Not configured"} {
		if !strings.Contains(text, want) {
			t.Fatalf("full catalog lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "unconfigured-modelNot configured") {
		t.Fatalf("full catalog ran together the model and its status:\n%s", text)
	}
}

func TestCatalogRejectsAllWithJSON(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	err := execute(t, app, "catalog", "--all", "--json")
	if err == nil || !strings.Contains(err.Error(), "--all cannot be used with --json") {
		t.Fatalf("catalog flags error = %v", err)
	}
}

func TestCatalogReportsUnavailableAccountWithoutBlockingHealthyAccount(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["healthy"] = configuration.Account{Label: "Healthy", Endpoints: configuration.Endpoints{OpenAIResponses: "https://healthy.test/v1"}}
	cfg.Accounts["missing-token"] = configuration.Account{Label: "Missing Token", Endpoints: configuration.Endpoints{OpenAIResponses: "https://missing.test/v1"}}
	cfg.Accounts["anthropic-only"] = configuration.Account{Label: "Anthropic Only", Endpoints: configuration.Endpoints{Anthropic: "https://anthropic.test"}}
	cfg.Profiles["healthy-model"] = configuration.Profile{Label: "Healthy", Account: "healthy", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "healthy-model"}}
	cfg.Routes.Default = "healthy-model"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("healthy", "healthy-token"); err != nil {
		t.Fatal(err)
	}
	app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "healthy.test" {
			t.Fatalf("unexpected request to %s", req.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"healthy-model"}]}`)), Request: req}, nil
	}

	if err := execute(t, app, "catalog", "--json"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"id": "healthy"`, `"status": "ok"`, `"id": "missing-token"`, `"status": "token_unavailable"`, `"id": "anthropic-only"`, `"status": "openai_responses_unavailable"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("catalog output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestCatalogReportsMalformedAccountPayloadWithoutBlockingHealthyAccount(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["broken"] = configuration.Account{Label: "Broken", Endpoints: configuration.Endpoints{OpenAIResponses: "https://broken.test/v1"}}
	cfg.Accounts["healthy"] = configuration.Account{Label: "Healthy", Endpoints: configuration.Endpoints{OpenAIResponses: "https://healthy.test/v1"}}
	cfg.Profiles["healthy-model"] = configuration.Profile{Label: "Healthy", Account: "healthy", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "healthy-model"}}
	cfg.Routes.Default = "healthy-model"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	for _, account := range []string{"broken", "healthy"} {
		if err := secretStore.Set(account, account+"-token"); err != nil {
			t.Fatal(err)
		}
	}
	app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
		body := `{"data":[{"id":"healthy-model"}]}`
		if req.URL.Host == "broken.test" {
			body = `{}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	}

	if err := execute(t, app, "catalog", "--json"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"id": "broken"`, `"status": "request_failed"`, `"id": "healthy"`, `"status": "ok"`, `"id": "healthy-model"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("catalog output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestModelsCommandReportsReachabilityFromGatewayModelList(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["gpt-5.6-sol"] = configuration.Profile{Label: "GPT-5.6 Sol Codex", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-5.6-sol"}}
	cfg.Profiles["gpt-5.6"] = configuration.Profile{Label: "GPT-5.6", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-5.6"}}
	cfg.Routes.Default = "gpt-5.6-sol"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		body := `{"data":[{"id":"gpt-5.6-sol"},{"id":"gpt-5.5"}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	}
	if err := execute(t, app, "models"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "gpt-5.6-sol") || !strings.Contains(text, "Reachable") || !strings.Contains(text, "gpt-5.6") || !strings.Contains(text, "Unavailable") {
		t.Fatalf("models output = %s", text)
	}
}

func TestModelsCommandKeepsLongProfileNamesOnOneLine(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["claude-opus-5"] = configuration.Profile{Label: "Claude Opus 5", Account: "dmx", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "claude-opus-5"}}
	cfg.Routes.Default = "claude-opus-5"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
		body := `{"data":[{"id":"claude-opus-5"}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	}
	if err := execute(t, app, "models"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Contains(text, "claude-opus-4-8-\n") || strings.Contains(text, "thinking      ") {
		t.Fatalf("long profile name was wrapped or column-padded badly:\n%s", text)
	}
	if !strings.Contains(text, "Profile  claude-opus-5") || !strings.Contains(text, "Claude · claude-opus-5 · Reachable · account dmx") {
		t.Fatalf("models output should use detail layout for long profile names:\n%s", text)
	}
}
