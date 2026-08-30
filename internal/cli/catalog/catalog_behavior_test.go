package catalog

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"

	"github.com/spf13/cobra"
)

func catalogDependencies(t *testing.T, cfg configuration.Config, secretValues map[string]string, client HTTPDoer) (Dependencies, *bytes.Buffer) {
	t.Helper()
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	if len(cfg.Profiles) > 0 {
		if err := store.Save(cfg); err != nil {
			t.Fatal(err)
		}
	}
	secretStore := secrets.NewMemoryStore()
	for account, value := range secretValues {
		if err := secretStore.Set(account, value); err != nil {
			t.Fatal(err)
		}
	}
	out := new(bytes.Buffer)
	return Dependencies{Config: store, Secrets: secretStore, HTTP: client, Out: out, Width: 120}, out
}

func executeCatalogCommand(t *testing.T, command *cobra.Command, args ...string) error {
	t.Helper()
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs(args)
	return command.Execute()
}

func configuredCatalog() configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Profiles["codex"] = configuration.Profile{Label: "Codex", Account: "gateway", Client: configuration.ClientCodex, Model: "gpt-codex"}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "gateway", Client: configuration.ClientClaude, Model: "gpt-claude"}
	cfg.Routes[configuration.ClientCodex] = "codex"
	return cfg
}

func TestModelsCommandCoversConfigurationAndReachabilityStates(t *testing.T) {
	loadFailure := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	if err := writeUnreadableConfig(loadFailure.Path()); err != nil {
		t.Fatal(err)
	}
	deps := Dependencies{Config: loadFailure, Secrets: secrets.NewMemoryStore(), HTTP: catalogHTTPClient(nil), Out: io.Discard}
	if err := executeCatalogCommand(t, NewModelsCommand(deps)); err == nil {
		t.Fatal("expected config load failure")
	}

	emptyDeps, _ := catalogDependencies(t, configuration.NewConfig(), nil, catalogHTTPClient(nil))
	if err := executeCatalogCommand(t, NewModelsCommand(emptyDeps)); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("empty config error = %v", err)
	}

	cfg := configuredCatalog()
	cfg.Accounts["anthropic"] = configuration.Account{Label: "Anthropic", Endpoints: configuration.Endpoints{Anthropic: "https://anthropic.test"}}
	cfg.Profiles["anthropic"] = configuration.Profile{Label: "Anthropic", Account: "anthropic", Client: configuration.ClientClaude, Model: "claude-only"}
	client := catalogHTTPClient(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":["gpt-codex"]}`)), Request: request}, nil
	})
	deps, out := catalogDependencies(t, cfg, map[string]string{"gateway": "token", "anthropic": "ignored"}, client)
	deps.RenderOut = out
	if err := executeCatalogCommand(t, NewModelsCommand(deps)); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Reachable", "Unknown", "Codex", "Claude"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("models output lacks %q:\n%s", want, out.String())
		}
	}

	if rows := modelRows(configuration.NewConfig(), nil); len(rows) != 0 {
		t.Fatalf("empty model rows = %#v", rows)
	}
}

func TestModelsCommandIgnoresCredentialAndCatalogFailures(t *testing.T) {
	cfg := configuredCatalog()
	secretStore := &faultingSecrets{values: map[string]string{"gateway": "token"}, getErr: errors.New("credential unavailable")}
	out := new(bytes.Buffer)
	deps := Dependencies{Config: saveCatalogConfig(t, cfg), Secrets: secretStore, HTTP: catalogHTTPClient(nil), Out: out}
	if err := executeCatalogCommand(t, NewModelsCommand(deps)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Unknown") {
		t.Fatalf("credential failure output = %s", out.String())
	}

	secretStore.getErr = nil
	deps.HTTP = catalogHTTPClient(func(*http.Request) (*http.Response, error) { return nil, errors.New("network unavailable") })
	out.Reset()
	if err := executeCatalogCommand(t, NewModelsCommand(deps)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Unknown") {
		t.Fatalf("catalog failure output = %s", out.String())
	}
}

func TestCatalogCommandCoversJSONHumanAndAccountStates(t *testing.T) {
	emptyDeps, emptyOut := catalogDependencies(t, configuration.NewConfig(), nil, catalogHTTPClient(nil))
	command := NewCatalogCommand(emptyDeps)
	if err := executeCatalogCommand(t, command, "--json"); err != nil || !strings.Contains(emptyOut.String(), `"accounts": []`) {
		t.Fatalf("empty JSON output=%q error=%v", emptyOut.String(), err)
	}
	emptyDeps, _ = catalogDependencies(t, configuration.NewConfig(), nil, catalogHTTPClient(nil))
	if err := executeCatalogCommand(t, NewCatalogCommand(emptyDeps)); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("empty human error = %v", err)
	}

	cfg := configuredCatalog()
	cfg.Accounts["anthropic"] = configuration.Account{Label: "Anthropic", Endpoints: configuration.Endpoints{Anthropic: "https://anthropic.test"}}
	cfg.Accounts["missing"] = configuration.Account{Label: "Missing", Endpoints: configuration.Endpoints{OpenAIResponses: "https://missing.test/v1"}}
	cfg.Accounts["denied"] = configuration.Account{Label: "Denied", Endpoints: configuration.Endpoints{OpenAIResponses: "https://denied.test/v1"}}
	cfg.Accounts["broken"] = configuration.Account{Label: "Broken", Endpoints: configuration.Endpoints{OpenAIResponses: "https://broken.test/v1"}}
	cfg.Profiles["anthropic"] = configuration.Profile{Label: "Anthropic", Account: "anthropic", Client: configuration.ClientClaude, Model: "claude"}
	cfg.Profiles["missing"] = configuration.Profile{Label: "Missing", Account: "missing", Client: configuration.ClientCodex, Model: "missing"}
	cfg.Profiles["denied"] = configuration.Profile{Label: "Denied", Account: "denied", Client: configuration.ClientCodex, Model: "denied"}
	cfg.Profiles["broken"] = configuration.Profile{Label: "Broken", Account: "broken", Client: configuration.ClientCodex, Model: "broken"}
	secretStore := &faultingSecrets{values: map[string]string{"gateway": "token", "denied": "token", "broken": "token"}, failAccount: "denied", getErr: errors.New("denied")}
	client := catalogHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "broken.test" {
			return nil, errors.New("network unavailable")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":["gpt-codex","gpt-claude","other"]}`)), Request: request}, nil
	})
	out := new(bytes.Buffer)
	deps := Dependencies{Config: saveCatalogConfig(t, cfg), Secrets: secretStore, HTTP: client, Out: out, Width: 120}
	command = NewCatalogCommand(deps)
	if err := executeCatalogCommand(t, command, "--json", "--all"); err == nil || !strings.Contains(err.Error(), "cannot be used") {
		t.Fatalf("flag conflict error = %v", err)
	}
	out.Reset()
	command = NewCatalogCommand(deps)
	if err := executeCatalogCommand(t, command, "--json"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"status": "ok"`, `"status": "openai_responses_unavailable"`, `"status": "token_unavailable"`, `"status": "request_failed"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("catalog JSON lacks %q:\n%s", want, out.String())
		}
	}

	out.Reset()
	command = NewCatalogCommand(deps)
	if err := executeCatalogCommand(t, command, "--all"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"OpenAI Responses endpoint is not configured", "Token unavailable", "Catalog request failed", "Not configured", "Configured:"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("catalog human output lacks %q:\n%s", want, out.String())
		}
	}

	emptyCfg := configuredCatalog()
	emptyClient := catalogHTTPClient(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[]}`)), Request: request}, nil
	})
	deps, out = catalogDependencies(t, emptyCfg, map[string]string{"gateway": "token"}, emptyClient)
	if err := executeCatalogCommand(t, NewCatalogCommand(deps)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Upstream returned an empty catalog") {
		t.Fatalf("empty catalog output = %s", out.String())
	}
}

func TestCatalogOutputFailuresAndHelpers(t *testing.T) {
	cfg := configuredCatalog()
	deps, _ := catalogDependencies(t, cfg, map[string]string{"gateway": "token"}, catalogHTTPClient(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":["gpt-codex"]}`)), Request: request}, nil
	}))
	want := errors.New("output unavailable")
	deps.Out = errorWriter{err: want}
	if err := executeCatalogCommand(t, NewCatalogCommand(deps), "--json"); !errors.Is(err, want) {
		t.Fatalf("JSON output error = %v, want %v", err, want)
	}

	deps.RenderOut = errorWriter{err: want}
	if err := executeCatalogCommand(t, NewCatalogCommand(deps)); !errors.Is(err, want) {
		t.Fatalf("human output error = %v, want %v", err, want)
	}
	if got := modelTitle(""); got != "" {
		t.Fatalf("empty model title = %q", got)
	}
	if got := strings.Join(sortedProfileNames(cfg), ","); got != "claude,codex" {
		t.Fatalf("profile order = %q", got)
	}
	if got := strings.Join(sortedModelAccountNames(cfg), ","); got != "gateway" {
		t.Fatalf("account order = %q", got)
	}
	if state, detail := catalogModelDisplay(catalogModel{ID: "plain"}); state == 0 || detail != "Not configured" {
		t.Fatalf("model display state=%v detail=%q", state, detail)
	}
}

type errorWriter struct{ err error }

func (writer errorWriter) Write([]byte) (int, error) { return 0, writer.err }

type faultingSecrets struct {
	values      map[string]string
	failAccount string
	getErr      error
}

func (store *faultingSecrets) Get(account string) (string, error) {
	if store.getErr != nil && (store.failAccount == "" || store.failAccount == account) {
		return "", store.getErr
	}
	value, ok := store.values[account]
	if !ok {
		return "", secrets.ErrNotFound
	}
	return value, nil
}

func (*faultingSecrets) Set(string, string) error { return nil }
func (*faultingSecrets) Delete(string) error      { return nil }
func (store *faultingSecrets) Has(account string) bool {
	_, ok := store.values[account]
	return ok
}

func saveCatalogConfig(t *testing.T, cfg configuration.Config) configuration.Store {
	t.Helper()
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return store
}

func writeUnreadableConfig(path string) error {
	return os.Mkdir(path, 0o700)
}

func TestModelCatalogHelpersAndResponseParsing(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Profiles["other"] = configuration.Profile{Account: "other", Model: "gpt"}
	cfg.Profiles["matching"] = configuration.Profile{Account: "one", Model: "gpt"}
	if got := ConfiguredProfiles(cfg, "one", "gpt"); len(got) != 1 || got[0] != "matching" {
		t.Fatalf("profiles = %#v", got)
	}
	for status, want := range map[string]string{
		"openai_responses_unavailable": "OpenAI Responses",
		"token_unavailable":            "Token unavailable",
		"request_failed":               "Catalog request failed",
		"future":                       "future",
	} {
		if got := StatusText(status); !strings.Contains(got, want) {
			t.Errorf("StatusText(%q) = %q", status, got)
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
			got, err := ParseIDs([]byte(test.body))
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
	account := configuration.Account{Endpoints: configuration.Endpoints{OpenAIResponses: "://bad"}}
	if _, err := FetchIDs(context.Background(), catalogHTTPClient(nil), account, "token"); err == nil {
		t.Fatal("expected URL error")
	}

	want := errors.New("network failed")
	account.Endpoints.OpenAIResponses = "https://one.test/v1"
	client := catalogHTTPClient(func(*http.Request) (*http.Response, error) { return nil, want })
	if _, err := FetchSet(context.Background(), client, account, "token"); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}

	for _, endpoint := range []string{"https://one.test", "https://one.test/models"} {
		t.Run(endpoint, func(t *testing.T) {
			client := catalogHTTPClient(func(request *http.Request) (*http.Response, error) {
				if request.URL.Path != "/models" || request.Header.Get("Authorization") != "Bearer token" {
					t.Fatalf("request = %#v", request)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":["gpt"]}`)), Request: request}, nil
			})
			account.Endpoints.OpenAIResponses = endpoint
			ids, err := FetchIDs(context.Background(), client, account, "token")
			if err != nil || len(ids) != 1 || ids[0] != "gpt" {
				t.Fatalf("ids=%#v error=%v", ids, err)
			}
		})
	}

	account.Endpoints.OpenAIResponses = "https://one.test/v1"
	client = catalogHTTPClient(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("bad")), Request: request}, nil
	})
	if _, err := FetchIDs(context.Background(), client, account, "token"); err == nil || !strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("error = %v", err)
	}
}

type catalogHTTPClient func(*http.Request) (*http.Response, error)

func (client catalogHTTPClient) Do(request *http.Request) (*http.Response, error) {
	return client(request)
}
