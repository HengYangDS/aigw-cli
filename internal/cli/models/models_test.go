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

	emptyDeps, _ := catalogRuntime(t, domain.NewConfig(clients.Admission()), nil, catalogHTTPClient(nil))
	if err := executeCatalogCommand(t, NewCommand(emptyDeps)); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("empty config error = %v", err)
	}

	cfg := configuredCatalog()
	cfg.Providers["anthropic"] = domain.Provider{Label: "Anthropic", Endpoints: domain.Endpoints{Anthropic: "https://anthropic.test"}}
	cfg.Models["anthropic"] = domain.Model{Label: "Anthropic", Provider: "anthropic", Family: "claude-only"}
	client := catalogHTTPClient(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":["gpt-codex"]}`)), Request: request}, nil
	})
	deps, out := catalogRuntime(t, cfg, map[string]string{"gateway": "token", "anthropic": "ignored"}, client)
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

func TestModelRowsGroupAccountsThenProtocolsThenDeclaredFamilies(t *testing.T) {
	cfg := configuredCatalog()
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Order: 10, Endpoints: cfg.Providers["gateway"].Endpoints}
	cfg.Families["first"] = domain.Family{Label: "First", Order: 10}
	cfg.Families["second"] = domain.Family{Label: "Second", Order: 20}
	cfg.Models = map[string]domain.Model{}
	for _, model := range []string{"second", "first"} {
		cfg.Models[model] = domain.Model{Provider: "gateway", Family: model, Protocols: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolAnthropic: {}, domain.ProtocolOpenAIResponses: {}, domain.ProtocolOpenAIChatCompletions: {},
		}}
	}
	rows := modelRows(cfg, app.CatalogReport{})
	index := 0
	for _, protocol := range []domain.EndpointProtocol{domain.ProtocolAnthropic, domain.ProtocolOpenAIResponses, domain.ProtocolOpenAIChatCompletions} {
		for _, model := range []string{"first", "second"} {
			if rows[index].Protocol != protocol || rows[index].UpstreamModel != model {
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
	if len(rows) != 2 || rows[0].Model != "claude" || rows[1].Model != "codex" {
		t.Fatalf("route rows = %#v", rows)
	}
}

func TestAmbiguousCatalogDoesNotReportModelsAsNotListed(t *testing.T) {
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://responses.test/v1"}}
	cfg.Families["shared"] = domain.Family{Label: "Shared"}
	cfg.Models["responses"] = domain.Model{
		Label: "Responses", Provider: "gateway", Family: "shared", UpstreamModel: "provider-shared",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {domain.CapabilityText}},
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
		deps, _ := catalogRuntime(t, cfg, map[string]string{"gateway": "token"}, client)
		rows := modelRows(cfg, app.DiscoverCatalog(context.Background(), deps.Secrets, deps.HTTP, cfg))
		if len(rows) != 1 || !strings.HasPrefix(rows[0].Catalog, want) {
			t.Fatalf("body=%s rows=%#v, want %q", body, rows, want)
		}
	}
}
