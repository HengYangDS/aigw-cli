package cli_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/account"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func dmxBalanceHandler(t *testing.T) func(*http.Request) (*http.Response, error) {
	t.Helper()
	return func(req *http.Request) (*http.Response, error) {
		body := `{"success":true,"data":{"quota":6250000}}`
		if strings.Contains(req.URL.Path, "/api/token/search") {
			body = `{"success":true,"data":{"items":[{"name":"Codex","key":"abcd**********wxyz","status":1,"used_quota":1000000,"remain_quota":2500000,"unlimited_quota":false,"unlimited_count":true,"expired_time":-1}]}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	}
}

func TestCheckExplainsQuotaFailureWithoutGuessingBalance(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Profiles["dmx"] = domain.Profile{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Routes.Default = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	app.HTTP.(*fakeHTTP).status = 403
	app.HTTP.(*fakeHTTP).body = `{"message":"令牌额度不足"}`
	err := execute(t, app, "check")
	if err == nil || !strings.Contains(out.String()+err.Error(), "Token 额度已耗尽") || !strings.Contains(out.String()+err.Error(), "aigw rotate") {
		t.Fatalf("output=%s error=%v", out.String(), err)
	}
}

func TestBalanceExplainsOptionalAccountBinding(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Profiles["dmx"] = domain.Profile{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, AccountProbe: &domain.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
	cfg.Routes.Default = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err := execute(t, app, "balance")
	if err == nil || !strings.Contains(err.Error(), "aigw account connect") {
		t.Fatalf("error = %v", err)
	}
}

func TestAccountConnectStoresSeparateCredentialAndBalanceShowsDetails(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	accountStore := account.NewMemoryStore()
	app.Accounts = accountStore
	cfg := domain.NewConfig()
	cfg.Profiles["dmx"] = domain.Profile{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, AccountProbe: &domain.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
	cfg.Routes.Default = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "sk-abcd-middle-wxyz")
	prompt := &fakePrompt{secret: "system-secret", text: "10000"}
	app.Prompt = prompt
	app.Interactive = true
	if err := execute(t, app, "account", "connect"); err != nil {
		t.Fatal(err)
	}
	if !accountStore.Has("dmx") {
		t.Fatal("account credential not stored")
	}
	out.Reset()
	app.HTTP.(*fakeHTTP).handler = dmxBalanceHandler(t)
	if err := execute(t, app, "balance"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"账户余额", "￥12.5000", "Token 状态", "启用", "剩余额度", "￥5.0000"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("balance lacks %q:\n%s", want, out.String())
		}
	}
}
