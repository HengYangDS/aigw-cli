package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/secrets"
)

type modelCoverageSecrets struct {
	values  map[string]string
	errors  map[string]error
	present map[string]bool
}

func (store modelCoverageSecrets) Get(id string) (string, error) {
	if err := store.errors[id]; err != nil {
		return "", err
	}
	if value, ok := store.values[id]; ok {
		return value, nil
	}
	return "", secrets.ErrNotFound
}
func (store modelCoverageSecrets) Set(string, string) error { return nil }
func (store modelCoverageSecrets) Delete(string) error      { return nil }
func (store modelCoverageSecrets) Has(id string) bool {
	return store.present[id] || store.values[id] != ""
}

func modelCoverageApp(t *testing.T, cfg domain.Config) *App {
	t.Helper()
	store := config.NewStore(filepath.Join(t.TempDir(), "config.toml"))
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	return &App{Config: store, Secrets: secrets.NewMemoryStore(), Out: out, Err: out}
}

func TestModelsCommandDependencyAndEmptyBranches(t *testing.T) {
	t.Run("config load", func(t *testing.T) {
		out := &bytes.Buffer{}
		app := &App{Config: config.NewStore(t.TempDir()), Out: out, Err: out}
		if err := Execute(app, []string{"models"}); err == nil {
			t.Fatal("expected config load error")
		}
	})

	t.Run("account filtering", func(t *testing.T) {
		cfg := domain.NewConfig()
		cfg.Accounts["anthropic"] = domain.Account{Label: "Anthropic", Endpoints: domain.Endpoints{Anthropic: "https://a.test"}}
		cfg.Accounts["missing"] = domain.Account{Label: "Missing", Endpoints: domain.Endpoints{OpenAIResponses: "https://m.test/v1"}}
		cfg.Accounts["broken"] = domain.Account{Label: "Broken", Endpoints: domain.Endpoints{OpenAIResponses: "https://b.test/v1"}}
		cfg.Profiles["anthropic"] = domain.Profile{Label: "Anthropic", Account: "anthropic", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude"}}
		cfg.Profiles["missing"] = domain.Profile{Label: "Missing", Account: "missing", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-missing"}}
		cfg.Profiles["broken"] = domain.Profile{Label: "Broken", Account: "broken", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-broken"}}
		cfg.Routes.Default = "missing"
		app := modelCoverageApp(t, cfg)
		want := errors.New("secret read failed")
		app.Secrets = modelCoverageSecrets{errors: map[string]error{"broken": want}, present: map[string]bool{"broken": true}}
		if err := Execute(app, []string{"models"}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(app.Out.(*bytes.Buffer).String(), "Unknown") {
			t.Fatalf("output = %q", app.Out.(*bytes.Buffer).String())
		}
	})

	t.Run("no model rows", func(t *testing.T) {
		cfg := domain.NewConfig()
		cfg.Accounts["one"] = domain.Account{Label: "One", Endpoints: domain.Endpoints{OpenAIResponses: "https://one.test/v1"}}
		cfg.Profiles["one"] = domain.Profile{Label: "One", Account: "one"}
		cfg.Routes.Default = "one"
		app := modelCoverageApp(t, cfg)
		if err := Execute(app, []string{"models"}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(app.Out.(*bytes.Buffer).String(), "No model services") {
			t.Fatalf("output = %q", app.Out.(*bytes.Buffer).String())
		}
	})
}

func TestCatalogLoadTokenAndEmptyCatalogBranches(t *testing.T) {
	t.Run("config load", func(t *testing.T) {
		out := &bytes.Buffer{}
		app := &App{Config: config.NewStore(t.TempDir()), Out: out, Err: out}
		if err := Execute(app, []string{"catalog"}); err == nil {
			t.Fatal("expected config load error")
		}
	})

	t.Run("empty JSON", func(t *testing.T) {
		out := &bytes.Buffer{}
		app := &App{Config: config.NewStore(filepath.Join(t.TempDir(), "config.toml")), Out: out, Err: out}
		if err := Execute(app, []string{"catalog", "--json"}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `"accounts": []`) {
			t.Fatalf("output = %q", out.String())
		}
	})

	t.Run("token get", func(t *testing.T) {
		cfg := domain.NewConfig()
		cfg.Accounts["one"] = domain.Account{Label: "One", Endpoints: domain.Endpoints{OpenAIResponses: "https://one.test/v1"}}
		cfg.Profiles["one"] = domain.Profile{Label: "One", Account: "one", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt"}}
		cfg.Routes.Default = "one"
		app := modelCoverageApp(t, cfg)
		app.Secrets = modelCoverageSecrets{errors: map[string]error{"one": errors.New("read failed")}, present: map[string]bool{"one": true}}
		if err := Execute(app, []string{"catalog"}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(app.Out.(*bytes.Buffer).String(), "Token unavailable") {
			t.Fatalf("output = %q", app.Out.(*bytes.Buffer).String())
		}
	})

	t.Run("empty upstream", func(t *testing.T) {
		cfg := domain.NewConfig()
		cfg.Accounts["one"] = domain.Account{Label: "One", Endpoints: domain.Endpoints{OpenAIResponses: "https://one.test/v1"}}
		cfg.Profiles["one"] = domain.Profile{Label: "One", Account: "one", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt"}}
		cfg.Routes.Default = "one"
		app := modelCoverageApp(t, cfg)
		_ = app.Secrets.Set("one", "token")
		app.HTTP = setupCoverageHTTP(func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[]}`)), Request: request}, nil
		})
		if err := Execute(app, []string{"catalog"}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(app.Out.(*bytes.Buffer).String(), "empty catalog") {
			t.Fatalf("output = %q", app.Out.(*bytes.Buffer).String())
		}
	})
}

func TestModelCatalogHelpersAndResponseParsing(t *testing.T) {
	cfg := domain.NewConfig()
	cfg.Profiles["other"] = domain.Profile{Account: "other", Models: domain.Models{domain.ClientCodex: "gpt"}}
	cfg.Profiles["matching"] = domain.Profile{Account: "one", Models: domain.Models{domain.ClientCodex: "gpt"}}
	if got := configuredProfilesForModel(cfg, "one", "gpt"); len(got) != 1 || got[0] != "matching" {
		t.Fatalf("profiles = %#v", got)
	}
	for status, want := range map[string]string{
		"openai_responses_unavailable": "OpenAI Responses",
		"token_unavailable":            "Token unavailable",
		"request_failed":               "Catalog request failed",
		"future":                       "future",
	} {
		if got := catalogStatusText(status); !strings.Contains(got, want) {
			t.Errorf("catalogStatusText(%q) = %q", status, got)
		}
	}

	parseTests := []struct {
		name string
		body string
		want []string
		fail bool
	}{
		{name: "invalid JSON", body: `{`, fail: true},
		{name: "missing data", body: `{}`, fail: true},
		{name: "non-array", body: `{"data":{}}`, fail: true},
		{name: "all shapes", body: `{"data":["z",{"id":"a"},{"model":"b"},{"name":"c"},{"id":"a"},{"id":3},null]}`, want: []string{"a", "b", "c", "z"}},
	}
	for _, test := range parseTests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseModelIDs([]byte(test.body))
			if test.fail {
				if err == nil {
					t.Fatal("expected parse failure")
				}
				return
			}
			if err != nil || strings.Join(got, ",") != strings.Join(test.want, ",") {
				t.Fatalf("models=%#v error=%v", got, err)
			}
		})
	}
}

func TestFetchModelIDsRequestErrorsAndEndpointForms(t *testing.T) {
	account := domain.Account{Endpoints: domain.Endpoints{OpenAIResponses: "://bad"}}
	if _, err := fetchModelIDs(context.Background(), setupCoverageHTTP(nil), account, "token"); err == nil {
		t.Fatal("expected URL error")
	}

	want := errors.New("network failed")
	account.Endpoints.OpenAIResponses = "https://one.test/v1"
	client := setupCoverageHTTP(func(*http.Request) (*http.Response, error) { return nil, want })
	if _, err := fetchModelSet(context.Background(), client, account, "token"); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}

	for _, endpoint := range []string{"https://one.test", "https://one.test/models"} {
		t.Run(endpoint, func(t *testing.T) {
			client := setupCoverageHTTP(func(request *http.Request) (*http.Response, error) {
				if request.URL.Path != "/models" || request.Header.Get("Authorization") != "Bearer token" {
					t.Fatalf("request = %#v", request)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":["gpt"]}`)), Request: request}, nil
			})
			account.Endpoints.OpenAIResponses = endpoint
			ids, err := fetchModelIDs(context.Background(), client, account, "token")
			if err != nil || len(ids) != 1 || ids[0] != "gpt" {
				t.Fatalf("ids=%#v error=%v", ids, err)
			}
		})
	}

	account.Endpoints.OpenAIResponses = "https://one.test/v1"
	client = setupCoverageHTTP(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("bad")), Request: request}, nil
	})
	if _, err := fetchModelIDs(context.Background(), client, account, "token"); err == nil || !strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("error = %v", err)
	}
}
