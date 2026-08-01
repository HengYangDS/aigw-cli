package cli_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"aigw-cli/internal/account"
	configuration "aigw-cli/internal/configuration"
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
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, "", configuration.Models{})
	cfg.Routes.Default = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	app.HTTP.(*fakeHTTP).status = 403
	app.HTTP.(*fakeHTTP).body = `{"message":"token quota is insufficient"}`
	err := execute(t, app, "check")
	if err == nil || !strings.Contains(out.String()+err.Error(), "Token quota is exhausted") || !strings.Contains(out.String()+err.Error(), "aigw rotate") {
		t.Fatalf("output=%s error=%v", out.String(), err)
	}
}

func TestBalanceExplainsOptionalAccountBinding(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, "", configuration.Models{})
	account := cfg.Accounts["dmx"]
	account.AccountProbe = &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}
	cfg.Accounts["dmx"] = account
	cfg.Routes.Default = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err := execute(t, app, "balance")
	if err == nil || !strings.Contains(out.String()+err.Error(), "aigw account connect dmx") || !strings.Contains(out.String()+err.Error(), "Precise balance diagnostics are not enabled") {
		t.Fatalf("output=%s error=%v", out.String(), err)
	}
}

func TestAccountConnectStoresSeparateCredentialAndBalanceShowsDetails(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	accountStore := account.NewMemoryStore()
	app.Accounts = accountStore
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, "", configuration.Models{})
	providerAccount := cfg.Accounts["dmx"]
	providerAccount.AccountProbe = &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}
	cfg.Accounts["dmx"] = providerAccount
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
	for _, want := range []string{"Account balance", "$12.5000", "Token status", "Enabled", "Remaining quota", "$5.0000"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("balance lacks %q:\n%s", want, out.String())
		}
	}
}
