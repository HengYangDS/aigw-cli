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
	"testing/iotest"

	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/store"

	"aigw-cli/internal/clients"

	"github.com/spf13/cobra"
)

func catalogDependencies(t *testing.T, cfg domain.Config, secretValues map[string]string, client HTTPDoer) (Dependencies, *bytes.Buffer) {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	if len(cfg.Routes) > 0 {
		if err := configStore.Save(cfg); err != nil {
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
	return Dependencies{Config: configStore, Secrets: secretStore, HTTP: client, Out: out, Width: 120}, out
}

func executeCatalogCommand(t *testing.T, command *cobra.Command, args ...string) error {
	t.Helper()
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs(args)
	return command.Execute()
}

func configuredCatalog() domain.Config {
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1", Anthropic: "https://gateway.test"}}
	cfg.Routes["codex"] = domain.Route{
		Label: "Codex", Account: "gateway", Model: "gpt-codex",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}},
	}
	cfg.Routes["claude"] = domain.Route{
		Label: "Claude", Account: "gateway", Model: "gpt-claude",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	cfg.SetSelectedRoute(codex.ID, "codex", "")
	return cfg
}

func TestModelsCommandCoversConfigurationAndCatalogMembership(t *testing.T) {
	loadFailure := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	if err := writeUnreadableConfig(loadFailure.Path()); err != nil {
		t.Fatal(err)
	}
	deps := Dependencies{Config: loadFailure, Secrets: secrets.NewMemoryStore(), HTTP: catalogHTTPClient(nil), Out: io.Discard}
	if err := executeCatalogCommand(t, NewModelsCommand(deps)); err == nil {
		t.Fatal("expected config load failure")
	}

	emptyDeps, _ := catalogDependencies(t, domain.NewConfig(clients.Admission()), nil, catalogHTTPClient(nil))
	if err := executeCatalogCommand(t, NewModelsCommand(emptyDeps)); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("empty config error = %v", err)
	}

	cfg := configuredCatalog()
	cfg.Accounts["anthropic"] = domain.Account{Label: "Anthropic", Endpoints: domain.Endpoints{Anthropic: "https://anthropic.test"}}
	cfg.Routes["anthropic"] = domain.Route{Label: "Anthropic", Account: "anthropic", Model: "claude-only"}
	client := catalogHTTPClient(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":["gpt-codex"]}`)), Request: request}, nil
	})
	deps, out := catalogDependencies(t, cfg, map[string]string{"gateway": "token", "anthropic": "ignored"}, client)
	deps.RenderOut = out
	if err := executeCatalogCommand(t, NewModelsCommand(deps)); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Listed", "Catalog not observed", "gpt-codex", "claude-only", "OpenAI Responses"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("models output lacks %q:\n%s", want, out.String())
		}
	}

	if rows := modelRows(domain.NewConfig(clients.Admission()), catalogOutput{}); len(rows) != 0 {
		t.Fatalf("empty model rows = %#v", rows)
	}
}

func TestModelRowsGroupAccountsThenProtocolsThenDeclaredModels(t *testing.T) {
	cfg := configuredCatalog()
	cfg.Accounts["gateway"] = domain.Account{Label: "Gateway", Order: 10, Endpoints: cfg.Accounts["gateway"].Endpoints}
	cfg.Models["first"] = domain.Model{Label: "First", Order: 10}
	cfg.Models["second"] = domain.Model{Label: "Second", Order: 20}
	cfg.Routes = map[string]domain.Route{}
	for _, model := range []string{"second", "first"} {
		cfg.Routes[model] = domain.Route{Account: "gateway", Model: model, Interfaces: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolAnthropic: {}, domain.ProtocolOpenAIResponses: {}, domain.ProtocolOpenAIChatCompletions: {},
		}}
	}
	rows := modelRows(cfg, catalogOutput{})
	index := 0
	for _, protocol := range []domain.EndpointProtocol{domain.ProtocolAnthropic, domain.ProtocolOpenAIResponses, domain.ProtocolOpenAIChatCompletions} {
		for _, model := range []string{"first", "second"} {
			if rows[index].Protocol != protocol || rows[index].Model != model {
				t.Fatalf("catalog row %d = %#v; want protocol=%s model=%s", index, rows[index], protocol, model)
			}
			index++
		}
	}
}

func TestModelsCommandExplainsCredentialAndCatalogFailures(t *testing.T) {
	cfg := configuredCatalog()
	secretStore := &faultingSecrets{values: map[string]string{"gateway": "token"}, getErr: errors.New("credential unavailable")}
	out := new(bytes.Buffer)
	deps := Dependencies{Config: saveCatalogConfig(t, cfg), Secrets: secretStore, HTTP: catalogHTTPClient(nil), Out: out}
	if err := executeCatalogCommand(t, NewModelsCommand(deps)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Credential backend failed") {
		t.Fatalf("credential failure output = %s", out.String())
	}

	secretStore.getErr = nil
	deps.HTTP = catalogHTTPClient(func(*http.Request) (*http.Response, error) { return nil, errors.New("network unavailable") })
	out.Reset()
	if err := executeCatalogCommand(t, NewModelsCommand(deps)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Catalog request failed") {
		t.Fatalf("catalog failure output = %s", out.String())
	}
}

func TestCatalogCommandCoversJSONHumanAndAccountStates(t *testing.T) {
	emptyDeps, emptyOut := catalogDependencies(t, domain.NewConfig(clients.Admission()), nil, catalogHTTPClient(nil))
	command := NewCatalogCommand(emptyDeps)
	if err := executeCatalogCommand(t, command, "--json"); err != nil || !strings.Contains(emptyOut.String(), `"observations": []`) {
		t.Fatalf("empty JSON output=%q error=%v", emptyOut.String(), err)
	}
	emptyDeps, _ = catalogDependencies(t, domain.NewConfig(clients.Admission()), nil, catalogHTTPClient(nil))
	if err := executeCatalogCommand(t, NewCatalogCommand(emptyDeps)); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("empty human error = %v", err)
	}

	cfg := configuredCatalog()
	cfg.Accounts["anthropic"] = domain.Account{Label: "Anthropic", Endpoints: domain.Endpoints{Anthropic: "https://anthropic.test"}}
	cfg.Accounts["missing"] = domain.Account{Label: "Missing", Endpoints: domain.Endpoints{OpenAIResponses: "https://missing.test/v1"}}
	cfg.Accounts["denied"] = domain.Account{Label: "Denied", Endpoints: domain.Endpoints{OpenAIResponses: "https://denied.test/v1"}}
	cfg.Accounts["broken"] = domain.Account{Label: "Broken", Endpoints: domain.Endpoints{OpenAIResponses: "https://broken.test/v1"}}
	cfg.Routes["anthropic"] = domain.Route{Label: "Anthropic", Account: "anthropic", Model: "claude"}
	cfg.Routes["missing"] = domain.Route{Label: "Missing", Account: "missing", Model: "missing"}
	cfg.Routes["denied"] = domain.Route{Label: "Denied", Account: "denied", Model: "denied"}
	cfg.Routes["broken"] = domain.Route{Label: "Broken", Account: "broken", Model: "broken"}
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
	if err := executeCatalogCommand(t, command, "--json", "--all"); err != nil {
		t.Fatalf("complete catalog JSON error = %v", err)
	}
	completeJSON := out.String()
	out.Reset()
	command = NewCatalogCommand(deps)
	if err := executeCatalogCommand(t, command, "--json"); err != nil {
		t.Fatal(err)
	}
	if out.String() != completeJSON {
		t.Fatalf("--all changed complete JSON output:\n%s\nwithout --all:\n%s", completeJSON, out.String())
	}
	for _, want := range []string{`"status": "ok"`, `"status": "token_unavailable"`, `"status": "request_failed"`, `"protocol": "anthropic"`, `"protocol": "openai_responses"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("catalog JSON lacks %q:\n%s", want, out.String())
		}
	}

	out.Reset()
	command = NewCatalogCommand(deps)
	if err := executeCatalogCommand(t, command, "--all"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Anthropic Messages", "OpenAI Responses", "Token unavailable", "Catalog request failed", "Candidate", "Admitted Routes:"} {
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
	rows := modelRows(cfg, catalogOutput{})
	if len(rows) != 2 || rows[0].Route != "claude" || rows[1].Route != "codex" {
		t.Fatalf("route rows = %#v", rows)
	}
	cfg.Accounts["alpha"] = domain.Account{Label: "Alpha"}
	observations := discoverCatalog(context.Background(), deps, cfg).Observations
	if len(observations) != 3 || observations[0].Account != "alpha" || observations[1].Account != "gateway" || observations[2].Account != "gateway" {
		t.Fatalf("catalog observations = %#v", observations)
	}
	if state, detail := catalogModelDisplay(catalogModel{ID: "plain", State: catalogCandidate}); state == 0 || !strings.Contains(detail, "Candidate") {
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
func (store *faultingSecrets) Exists(account string) (bool, error) {
	if store.getErr != nil && (store.failAccount == "" || store.failAccount == account) {
		return false, store.getErr
	}
	_, ok := store.values[account]
	return ok, nil
}

func saveCatalogConfig(t *testing.T, cfg domain.Config) store.Store {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return configStore
}

func writeUnreadableConfig(path string) error {
	return os.Mkdir(path, 0o700)
}

func TestModelCatalogHelpersAndResponseParsing(t *testing.T) {
	cfg := domain.NewConfig(clients.Admission())
	cfg.Routes["other"] = domain.Route{Account: "other", Model: "gpt", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.Routes["matching"] = domain.Route{Account: "one", Model: "canonical", UpstreamModel: "gpt", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	models, missing := catalogDifference(cfg, "one", domain.ProtocolOpenAIResponses, []string{"gpt"})
	if len(models) != 1 || models[0].State != catalogAdmitted || len(models[0].Routes) != 1 || models[0].Routes[0].ID != "matching" || len(missing) != 0 {
		t.Fatalf("difference models=%#v missing=%#v", models, missing)
	}
	for status, want := range map[catalogStatus]string{
		catalogEndpointUnavailable: "No supported",
		catalogTokenUnavailable:    "Token unavailable",
		catalogRequestFailed:       "Catalog request failed",
		"future":                   "future",
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
		{name: "all shapes", body: `{"data":["z",{"id":"a"},{"model":"b"},{"name":"c"},{"id":"a"}]}`, want: []string{"a", "b", "c", "z"}},
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
	endpoint := "://bad"
	if _, _, err := FetchIDs(context.Background(), catalogHTTPClient(nil), endpoint, domain.ProtocolOpenAIResponses, "token"); err == nil {
		t.Fatal("expected URL error")
	}

	want := errors.New("network failed")
	endpoint = "https://one.test/v1"
	client := catalogHTTPClient(func(*http.Request) (*http.Response, error) { return nil, want })
	if _, _, err := FetchIDs(context.Background(), client, endpoint, domain.ProtocolOpenAIResponses, "token"); !errors.Is(err, want) {
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
			ids, source, err := FetchIDs(context.Background(), client, endpoint, domain.ProtocolOpenAIResponses, "token")
			if err != nil || len(ids) != 1 || ids[0] != "gpt" {
				t.Fatalf("ids=%#v error=%v", ids, err)
			}
			if !strings.HasSuffix(source, "/models") {
				t.Fatalf("source = %q", source)
			}
		})
	}

	endpoint = "https://one.test/v1"
	client = catalogHTTPClient(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("bad")), Request: request}, nil
	})
	if _, _, err := FetchIDs(context.Background(), client, endpoint, domain.ProtocolOpenAIResponses, "token"); err == nil || !strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("error = %v", err)
	}
}

func TestCatalogMembershipRequiresACompleteResponse(t *testing.T) {
	interrupted := errors.New("catalog read interrupted")
	for _, test := range []struct {
		name string
		body io.Reader
	}{
		{name: "read failure after valid JSON", body: io.MultiReader(strings.NewReader(`{"data":[{"id":"listed"}]}`), iotest.ErrReader(interrupted))},
		{name: "oversized response", body: strings.NewReader(`{"data":[{"id":"listed"}]}` + strings.Repeat(" ", 4<<20))},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := catalogHTTPClient(func(request *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(test.body), Request: request}, nil
			})
			if ids, _, err := FetchIDs(context.Background(), client, "https://catalog.test/v1", domain.ProtocolOpenAIResponses, "token"); err == nil || len(ids) != 0 {
				t.Fatalf("incomplete catalog established membership: ids=%v error=%v", ids, err)
			}
		})
	}
}

type catalogHTTPClient func(*http.Request) (*http.Response, error)

func (client catalogHTTPClient) Do(request *http.Request) (*http.Response, error) {
	return client(request)
}

type recordingClient struct{ path string }

func (client *recordingClient) Do(request *http.Request) (*http.Response, error) {
	client.path = request.URL.Path
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"zeta"},{"id":"alpha"},{"id":"alpha"}]}`)),
		Request:    request,
	}, nil
}

func TestFetchIDsOwnsCatalogProtocol(t *testing.T) {
	client := &recordingClient{}
	ids, _, err := FetchIDs(context.Background(), client, "https://gateway.test/v1", domain.ProtocolOpenAIResponses, "token")
	if err != nil {
		t.Fatal(err)
	}
	if client.path != "/v1/models" {
		t.Fatalf("path = %q, want /v1/models", client.path)
	}
	if got := strings.Join(ids, ","); got != "alpha,zeta" {
		t.Fatalf("IDs = %q, want alpha,zeta", got)
	}
}

func TestCommandConstructorsExposeStableNames(t *testing.T) {
	deps := Dependencies{}
	if got := NewModelsCommand(deps).Name(); got != "models" {
		t.Fatalf("models command = %q", got)
	}
	if got := NewCatalogCommand(deps).Name(); got != "catalog" {
		t.Fatalf("catalog command = %q", got)
	}
}
