package cli_test

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/account"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

type failingAccountStore struct {
	account.Store
	setErr    error
	deleteErr error
}

func (store failingAccountStore) Set(string, account.Credential) error { return store.setErr }
func (store failingAccountStore) Delete(string) error                  { return store.deleteErr }

func saveProbeProfile(t *testing.T, appConfig config.Store) {
	t.Helper()
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{
		Label:        "DMXAPI",
		Endpoints:    domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"},
		AccountProbe: &domain.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"},
	}
	cfg.Profiles["gpt"] = domain.Profile{Label: "GPT", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "gpt"
	if err := appConfig.Save(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestCheckSurfacesMissingDefaultRuntimeAndToken(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	saveDailyProfile(t, app, domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt"})
	err := execute(t, app, "check")
	if err == nil || !strings.Contains(err.Error(), "System secret is missing") {
		t.Fatalf("error = %v", err)
	}
}

func TestAccountConnectValidationAndDependencyFailures(t *testing.T) {
	t.Run("non-interactive", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		if err := execute(t, app, "account", "connect"); err == nil || !strings.Contains(err.Error(), "interactive") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("config load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Interactive = true
		app.Config = config.NewStore(t.TempDir())
		if err := execute(t, app, "account", "connect"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("unknown explicit account", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveProbeProfile(t, app.Config)
		if err := execute(t, app, "account", "connect", "missing"); err == nil || !strings.Contains(err.Error(), "Unknown") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("no probe", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveDailyProfile(t, app, domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt"})
		if err := execute(t, app, "account", "connect"); err == nil || !strings.Contains(err.Error(), "does not support") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("unsupported probe", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveProbeProfile(t, app.Config)
		cfg, _ := app.Config.Load()
		providerAccount := cfg.Accounts["dmx"]
		providerAccount.AccountProbe.Kind = "future"
		cfg.Accounts["dmx"] = providerAccount
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := execute(t, app, "account", "connect"); err == nil || !strings.Contains(err.Error(), "does not include") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("secret prompt", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveProbeProfile(t, app.Config)
		want := errors.New("cancelled")
		app.Prompt = &fakePrompt{secretErr: want}
		if err := execute(t, app, "account", "connect"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("text prompt", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveProbeProfile(t, app.Config)
		app.Prompt = &fakePrompt{secret: "system-token"}
		if err := execute(t, app, "account", "connect"); err == nil || !strings.Contains(err.Error(), "no text") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("credential write", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveProbeProfile(t, app.Config)
		want := errors.New("credential write failed")
		app.Prompt = &fakePrompt{secret: "system-token", text: "user"}
		app.Accounts = failingAccountStore{Store: account.NewMemoryStore(), setErr: want}
		if err := execute(t, app, "account", "connect"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})
}

func TestAccountDisconnectBranches(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = config.NewStore(t.TempDir())
		if err := execute(t, app, "account", "disconnect"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("unknown", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		if err := execute(t, app, "account", "disconnect", "missing"); err == nil || !strings.Contains(err.Error(), "Unknown") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("delete failure", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		want := errors.New("delete failed")
		app.Accounts = failingAccountStore{Store: account.NewMemoryStore(), deleteErr: want}
		if err := execute(t, app, "account", "disconnect", "dmx"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("success", func(t *testing.T) {
		app, out, _, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		store := account.NewMemoryStore()
		_ = store.Set("dmx", account.Credential{SystemToken: "system", UserID: "user"})
		app.Accounts = store
		if err := execute(t, app, "account", "disconnect", "dmx"); err != nil {
			t.Fatal(err)
		}
		if store.Has("dmx") || !strings.Contains(out.String(), "credentials were removed") {
			t.Fatalf("output=%q has=%v", out.String(), store.Has("dmx"))
		}
	})
}

func TestBalanceOperationalAndRenderingBranches(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = config.NewStore(t.TempDir())
		if err := execute(t, app, "balance"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("unknown explicit account", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		if err := execute(t, app, "balance", "missing"); err == nil || !strings.Contains(err.Error(), "Unknown") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("no probe", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt"})
		if err := execute(t, app, "balance"); err == nil || !strings.Contains(err.Error(), "does not support") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing api token", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		store := account.NewMemoryStore()
		_ = store.Set("dmx", account.Credential{SystemToken: "system", UserID: "user"})
		app.Accounts = store
		if err := execute(t, app, "balance"); err == nil {
			t.Fatal("expected missing API token")
		}
	})

	t.Run("provider failure", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		store := account.NewMemoryStore()
		_ = store.Set("dmx", account.Credential{SystemToken: "system", UserID: "user"})
		app.Accounts = store
		_ = secretStore.Set("dmx", "sk-abcd-middle-wxyz")
		want := errors.New("network failed")
		app.HTTP.(*fakeHTTP).handler = func(*http.Request) (*http.Response, error) { return nil, want }
		if err := execute(t, app, "balance"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("disabled unlimited token", func(t *testing.T) {
		app, out, secretStore, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		store := account.NewMemoryStore()
		_ = store.Set("dmx", account.Credential{SystemToken: "system", UserID: "user"})
		app.Accounts = store
		_ = secretStore.Set("dmx", "sk-abcd-middle-wxyz")
		app.HTTP.(*fakeHTTP).handler = func(request *http.Request) (*http.Response, error) {
			body := `{"success":true,"data":{"quota":6250000}}`
			if strings.Contains(request.URL.Path, "/api/token/search") {
				body = `{"success":true,"data":{"items":[{"name":"Codex","key":"abcd**********wxyz","status":2,"used_quota":1,"remain_quota":0,"unlimited_quota":true,"remain_count":0,"unlimited_count":true,"expired_time":-1}]}}`
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
		}
		if err := execute(t, app, "balance", "dmx"); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"Disabled", "Unlimited", "Unlimited requests"} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("output lacks %q: %s", want, out.String())
			}
		}
	})
}

func TestRepairHumanPreviewAndDependencyFailures(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = config.NewStore(t.TempDir())
		if err := execute(t, app, "repair", "--dry-run"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("discovery", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt"})
		app.Discovery = nil
		if err := execute(t, app, "repair", "--dry-run"); err == nil || !strings.Contains(err.Error(), "discovery") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("human preview", func(t *testing.T) {
		app, out, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt"})
		if err := execute(t, app, "repair", "--dry-run"); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "Repair preview") || !strings.Contains(out.String(), "Preview did not write") {
			t.Fatalf("output = %q", out.String())
		}
	})

	t.Run("shim enable", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "m"})
		_ = secretStore.Set("one", "token")
		blocker := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		app.Shims.BinDir = filepath.Join(blocker, "bin")
		app.Shims.AIGWExecutable = "/bin/aigw"
		app.Discovery = fakeDiscovery{result: discovery.Result{ClaudeExecutable: "/opt/claude"}}
		if err := execute(t, app, "repair"); err == nil {
			t.Fatal("expected shim enable failure")
		}
	})
}

func TestUpdateWithoutUpdaterIsRejected(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	app.Updater = nil
	if err := execute(t, app, "update"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("error = %v", err)
	}
}
