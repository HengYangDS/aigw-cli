package cli_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func TestModelsCommandReportsReachabilityFromGatewayModelList(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["gpt-5.6-sol-cdx"] = domain.Profile{Label: "GPT-5.6 Sol Codex", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{Codex: "gpt-5.6-sol-cdx"}}
	cfg.Profiles["gpt-5.6"] = domain.Profile{Label: "GPT-5.6", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{Codex: "gpt-5.6"}}
	cfg.Routes.Default = "gpt-5.6-sol-cdx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		body := `{"data":[{"id":"gpt-5.6-sol-cdx"},{"id":"gpt-5.5"}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	}
	if err := execute(t, app, "models"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "gpt-5.6-sol-cdx") || !strings.Contains(text, "可达") || !strings.Contains(text, "gpt-5.6") || !strings.Contains(text, "不可达") {
		t.Fatalf("models output = %s", text)
	}
}

func TestModelsCommandKeepsLongProfileNamesOnOneLine(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["claude-opus-4-8-thinking"] = domain.Profile{Label: "Claude Opus 4.8 Thinking", Account: "dmx", Client: domain.ClientClaude, Models: domain.Models{Claude: "claude-opus-4-8-thinking"}}
	cfg.Routes.Default = "claude-opus-4-8-thinking"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
		body := `{"data":[{"id":"claude-opus-4-8-thinking"}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	}
	if err := execute(t, app, "models"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Contains(text, "claude-opus-4-8-\n") || strings.Contains(text, "thinking      ") {
		t.Fatalf("long profile name was wrapped or column-padded badly:\n%s", text)
	}
	if !strings.Contains(text, "Profile  claude-opus-4-8-thinking") || !strings.Contains(text, "Claude · claude-opus-4-8-thinking · 可达 · Account dmx") {
		t.Fatalf("models output should use detail layout for long profile names:\n%s", text)
	}
}
