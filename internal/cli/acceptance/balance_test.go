package cli_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestBalanceOperationalAndRenderingBranches(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := cli.Execute(app, []string{"balance"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("unknown explicit account", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		if err := cli.Execute(app, []string{"balance", "missing"}); err == nil || !strings.Contains(strings.ToLower(err.Error()), "unknown") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("no probe", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		if err := cli.Execute(app, []string{"balance"}); err == nil || !strings.Contains(err.Error(), "does not support") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing api token", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		store := app.Accounts
		_ = store.Set("dmx", secrets.DiagnosticCredential{SystemToken: "system", UserID: "user"})
		if err := cli.Execute(app, []string{"balance"}); err == nil {
			t.Fatal("expected missing API token")
		}
	})

	t.Run("provider failure", func(t *testing.T) {
		app, _, secretStore, _, httpClient := testApp(t, "")
		saveProbeProfile(t, app.Config)
		store := app.Accounts
		_ = store.Set("dmx", secrets.DiagnosticCredential{SystemToken: "system", UserID: "user"})
		_ = secretStore.Set("dmx", "sk-abcd-middle-wxyz")
		want := errors.New("network failed")
		httpClient.handler = func(*http.Request) (*http.Response, error) { return nil, want }
		if err := cli.Execute(app, []string{"balance"}); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("disabled unlimited token", func(t *testing.T) {
		app, out, secretStore, _, httpClient := testApp(t, "")
		saveProbeProfile(t, app.Config)
		store := app.Accounts
		_ = store.Set("dmx", secrets.DiagnosticCredential{SystemToken: "system", UserID: "user"})
		_ = secretStore.Set("dmx", "sk-abcd-middle-wxyz")
		httpClient.handler = func(request *http.Request) (*http.Response, error) {
			body := `{"success":true,"data":{"quota":6250000}}`
			if strings.Contains(request.URL.Path, "/api/token/search") {
				body = `{"success":true,"data":{"items":[{"name":"Codex","key":"abcd**********wxyz","status":2,"used_quota":1,"remain_quota":0,"unlimited_quota":true,"remain_count":0,"unlimited_count":true,"expired_time":-1}]}}`
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
		}
		if err := cli.Execute(app, []string{"balance", "dmx"}); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"Disabled", "Unlimited", "Unlimited requests"} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("output lacks %q: %s", want, out.String())
			}
		}
	})
}

func TestBalanceExplainsWhenConfiguredDiagnosticDriverIsNotBundled(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["future"] = configuration.Account{
		Label:        "Future Gateway",
		Endpoints:    configuration.Endpoints{OpenAIResponses: "https://future.test/v1"},
		AccountProbe: &configuration.AccountProbe{Kind: "future-provider", BaseURL: "https://future.test"},
	}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "future", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("future", "test-token"); err != nil {
		t.Fatal(err)
	}
	err := cli.Execute(app, []string{"balance"})
	if err == nil || !strings.Contains(err.Error(), "is not included in this AIGW version") || !strings.Contains(err.Error(), "aigw check") {
		t.Fatalf("balance error = %v", err)
	}
}

func handleDMXBalance(req *http.Request) (*http.Response, error) {
	body := `{"success":true,"data":{"quota":6250000}}`
	if strings.Contains(req.URL.Path, "/api/token/search") {
		body = `{"success":true,"data":{"items":[{"name":"Codex","key":"abcd**********wxyz","status":1,"used_quota":1000000,"remain_quota":2500000,"unlimited_quota":false,"unlimited_count":true,"expired_time":-1}]}}`
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

func TestBalanceExplainsOptionalAccountBinding(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, configuration.ClientCodex, "gpt-test")
	account := cfg.Accounts["dmx"]
	account.AccountProbe = &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}
	cfg.Accounts["dmx"] = account
	cfg.Routes[configuration.ClientCodex] = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err := cli.Execute(app, []string{"balance"})
	if err == nil || !strings.Contains(out.String()+err.Error(), "aigw account connect dmx") || !strings.Contains(out.String()+err.Error(), "Precise balance diagnostics are not enabled") {
		t.Fatalf("output=%s error=%v", out.String(), err)
	}
}

func TestAccountConnectStoresSeparateCredentialAndBalanceShowsDetails(t *testing.T) {
	app, out, secretStore, _, httpClient := testApp(t, "")
	accountStore := app.Accounts
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, configuration.ClientCodex, "gpt-test")
	providerAccount := cfg.Accounts["dmx"]
	providerAccount.AccountProbe = &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}
	cfg.Accounts["dmx"] = providerAccount
	cfg.Routes[configuration.ClientCodex] = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "sk-abcd-middle-wxyz")
	prompt := &scriptedPrompt{secrets: []string{"system-secret"}, texts: []string{"10000"}}
	app.Prompt = prompt
	app.Interactive = true
	if err := cli.Execute(app, []string{"account", "connect"}); err != nil {
		t.Fatal(err)
	}
	if !accountCredentialExists(t, accountStore, "dmx") {
		t.Fatal("account credential not stored")
	}
	out.Reset()
	httpClient.handler = handleDMXBalance
	if err := cli.Execute(app, []string{"balance"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Account balance", "$12.5000", "Token status", "Enabled", "Remaining quota", "$5.0000"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("balance lacks %q:\n%s", want, out.String())
		}
	}
}
