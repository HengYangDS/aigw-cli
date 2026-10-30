package models

import (
	"aigw-cli/internal/app"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModelsCommandCoversConfigurationAndCatalogMembership(t *testing.T) {
	loadFailure := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	if err := writeUnreadableConfig(loadFailure.Path()); err != nil {
		t.Fatal(err)
	}
	deps := invocation.Context{Config: loadFailure, Secrets: secret.NewMemoryStore(), HTTP: catalogHTTPClient(nil), Out: io.Discard}
	if err := executeCatalogCommand(t, NewCommand(deps)); err == nil {
		t.Fatal("expected config load failure")
	}

	emptyDeps, _ := catalogDependencies(t, domain.NewConfig(clients.Admission()), nil, catalogHTTPClient(nil))
	if err := executeCatalogCommand(t, NewCommand(emptyDeps)); err == nil || !strings.Contains(err.Error(), "not configured") {
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
	if err := executeCatalogCommand(t, NewCommand(deps)); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Listed", "Catalog not observed", "gpt-codex", "claude-only", "OpenAI Responses"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("models output lacks %q:\n%s", want, out.String())
		}
	}

	if rows := modelRows(domain.NewConfig(clients.Admission()), app.CatalogReport{}); len(rows) != 0 {
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
	rows := modelRows(cfg, app.CatalogReport{})
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
	deps := invocation.Context{Config: saveCatalogConfig(t, cfg), Secrets: secretStore, HTTP: catalogHTTPClient(nil), Out: out}
	if err := executeCatalogCommand(t, NewCommand(deps)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Credential backend failed") {
		t.Fatalf("credential failure output = %s", out.String())
	}

	secretStore.getErr = nil
	deps.HTTP = catalogHTTPClient(func(*http.Request) (*http.Response, error) { return nil, errors.New("network unavailable") })
	out.Reset()
	if err := executeCatalogCommand(t, NewCommand(deps)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Catalog request failed") {
		t.Fatalf("catalog failure output = %s", out.String())
	}
}

func writeUnreadableConfig(path string) error {
	return os.Mkdir(path, 0o700)
}

func TestModelsCommandName(t *testing.T) {
	if got := NewCommand(invocation.Context{}).Name(); got != "models" {
		t.Fatalf("models command = %q", got)
	}
}

func TestModelRowsWithoutObservations(t *testing.T) {
	rows := modelRows(configuredCatalog(), app.CatalogReport{})
	if len(rows) != 2 || rows[0].Route != "claude" || rows[1].Route != "codex" {
		t.Fatalf("route rows = %#v", rows)
	}
}

func TestAmbiguousCatalogueDoesNotReportRoutesAsNotListed(t *testing.T) {
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://responses.test/v1"}}
	cfg.Models["shared"] = domain.Model{Label: "Shared"}
	cfg.Routes["responses"] = domain.Route{
		Label: "Responses", Account: "gateway", Model: "shared", UpstreamModel: "provider-shared",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {domain.CapabilityText}},
	}
	for body, want := range map[string]string{
		`{"data":null}`: "Catalog request failed",
		`{"data":[{"id":"provider-shared","id":"other"}]}`: "Catalog request failed",
		`{"data":[]}`:                         "Not listed",
		`{"data":[{"id":"provider-shared"}]}`: "Listed",
	} {
		client := catalogHTTPClient(func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
		})
		deps, _ := catalogDependencies(t, cfg, map[string]string{"gateway": "token"}, client)
		rows := modelRows(cfg, app.DiscoverCatalog(context.Background(), deps.Secrets, deps.HTTP, cfg))
		if len(rows) != 1 || !strings.HasPrefix(rows[0].Catalog, want) {
			t.Fatalf("body=%s rows=%#v, want %q", body, rows, want)
		}
	}
}
