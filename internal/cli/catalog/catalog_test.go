package catalog

import (
	"aigw-cli/internal/app"
	"bytes"

	"errors"
	"io"
	"net/http"

	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"

	"github.com/spf13/cobra"
)

func catalogDependencies(t *testing.T, cfg domain.Config, secretValues map[string]string, client app.HTTPDoer) (invocation.Context, *bytes.Buffer) {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	if len(cfg.Routes) > 0 {
		if err := configStore.Save(cfg); err != nil {
			t.Fatal(err)
		}
	}
	secretStore := secret.NewMemoryStore()
	for account, value := range secretValues {
		if err := secretStore.Set(account, value); err != nil {
			t.Fatal(err)
		}
	}
	out := new(bytes.Buffer)
	return invocation.Context{Config: configStore, Secrets: secretStore, HTTP: client, Out: out, Width: 120}, out
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

func TestCatalogCommandCoversJSONHumanAndAccountStates(t *testing.T) {
	emptyDeps, emptyOut := catalogDependencies(t, domain.NewConfig(clients.Admission()), nil, catalogHTTPClient(nil))
	command := NewCommand(emptyDeps)
	if err := executeCatalogCommand(t, command, "--json"); err != nil || !strings.Contains(emptyOut.String(), `"observations": []`) {
		t.Fatalf("empty JSON output=%q error=%v", emptyOut.String(), err)
	}
	emptyDeps, _ = catalogDependencies(t, domain.NewConfig(clients.Admission()), nil, catalogHTTPClient(nil))
	if err := executeCatalogCommand(t, NewCommand(emptyDeps)); err == nil || !strings.Contains(err.Error(), "not configured") {
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
	deps := invocation.Context{Config: saveCatalogConfig(t, cfg), Secrets: secretStore, HTTP: client, Out: out, Width: 120}
	command = NewCommand(deps)
	if err := executeCatalogCommand(t, command, "--json", "--all"); err != nil {
		t.Fatalf("complete catalog JSON error = %v", err)
	}
	completeJSON := out.String()
	out.Reset()
	command = NewCommand(deps)
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
	command = NewCommand(deps)
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
	if err := executeCatalogCommand(t, NewCommand(deps)); err != nil {
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
	if err := executeCatalogCommand(t, NewCommand(deps), "--json"); !errors.Is(err, want) {
		t.Fatalf("JSON output error = %v, want %v", err, want)
	}

	deps.RenderOut = errorWriter{err: want}
	if err := executeCatalogCommand(t, NewCommand(deps)); !errors.Is(err, want) {
		t.Fatalf("human output error = %v, want %v", err, want)
	}
	if got := modelTitle(""); got != "" {
		t.Fatalf("empty model title = %q", got)
	}
	if state, detail := catalogModelDisplay(app.CatalogModel{ID: "plain", State: app.CatalogCandidate}); state == 0 || !strings.Contains(detail, "Candidate") {
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
		return "", secret.ErrNotFound
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

type catalogHTTPClient func(*http.Request) (*http.Response, error)

func (client catalogHTTPClient) Do(request *http.Request) (*http.Response, error) {
	return client(request)
}

func TestCatalogCommandName(t *testing.T) {
	if got := NewCommand(invocation.Context{}).Name(); got != "catalog" {
		t.Fatalf("catalog command = %q", got)
	}
}
