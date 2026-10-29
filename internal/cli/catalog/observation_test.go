package catalog

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

func TestCatalogObservationSeparatesProtocolCapabilitiesAndDifferences(t *testing.T) {
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["gateway"] = domain.Account{
		Label: "Gateway",
		Endpoints: domain.Endpoints{
			OpenAIChatCompletions: "https://chat.test/v1",
			OpenAIResponses:       "https://responses.test/v1",
		},
	}
	cfg.Models["shared"] = domain.Model{Label: "Shared"}
	cfg.Models["missing"] = domain.Model{Label: "Missing"}
	cfg.Routes["chat"] = domain.Route{
		Label:         "Chat",
		Account:       "gateway",
		Model:         "shared",
		UpstreamModel: "provider-shared",
		Lifecycle:     domain.RouteDeprecated,
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolOpenAIChatCompletions: {domain.CapabilityText},
		},
	}
	cfg.Routes["responses"] = domain.Route{
		Label:         "Responses",
		Account:       "gateway",
		Model:         "shared",
		UpstreamModel: "provider-shared",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolOpenAIResponses: {
				domain.CapabilityText,
				domain.CapabilityReasoning,
			},
		},
	}
	cfg.Routes["missing"] = domain.Route{
		Label:         "Missing",
		Account:       "gateway",
		Model:         "missing",
		UpstreamModel: "provider-missing",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolOpenAIResponses: {domain.CapabilityText},
		},
	}

	before := cfg.Clone()
	client := catalogHTTPClient(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"data":[{"id":"provider-shared"},{"id":"provider-new"}]}`,
			)),
			Request: request,
		}, nil
	})
	deps, _ := catalogDependencies(t, cfg, map[string]string{"gateway": "token"}, client)

	result := discoverCatalog(context.Background(), deps, cfg)
	if !reflect.DeepEqual(cfg, before) {
		t.Fatalf("catalog observation mutated configuration:\nbefore=%#v\nafter=%#v", before, cfg)
	}
	assertObservationIdentityAndSources(t, result)
	assertRouteCapabilityIsolation(t, result)
	assertCatalogCandidateAndMissingRoute(t, result)
}

func assertObservationIdentityAndSources(t *testing.T, result catalogOutput) {
	t.Helper()
	if len(result.Observations) != 2 {
		t.Fatalf("observations = %#v", result.Observations)
	}

	chat := observationForProtocol(t, result, domain.ProtocolOpenAIChatCompletions)
	responses := observationForProtocol(t, result, domain.ProtocolOpenAIResponses)
	if chat.ObservationID == "" || responses.ObservationID == "" || chat.ObservationID == responses.ObservationID {
		t.Fatalf("observation identities do not bind protocol: chat=%q responses=%q", chat.ObservationID, responses.ObservationID)
	}
	if got := catalogObservationID("gateway", domain.ProtocolOpenAIResponses, responses.Source.Endpoint, []string{"provider-new", "provider-shared"}); got != responses.ObservationID {
		t.Fatalf("observation identity depends on provider order: got %q want %q", got, responses.ObservationID)
	}
	if chat.Source.Endpoint != "https://chat.test/v1/models" || responses.Source.Endpoint != "https://responses.test/v1/models" {
		t.Fatalf("sources = chat %#v responses %#v", chat.Source, responses.Source)
	}
}

func assertRouteCapabilityIsolation(t *testing.T, result catalogOutput) {
	t.Helper()
	chat := observationForProtocol(t, result, domain.ProtocolOpenAIChatCompletions)
	responses := observationForProtocol(t, result, domain.ProtocolOpenAIResponses)
	chatShared := observedModel(t, chat, "provider-shared")
	if chatShared.State != catalogDeprecated || len(chatShared.Routes) != 1 || chatShared.Routes[0].ID != "chat" ||
		chatShared.Routes[0].Lifecycle != domain.RouteDeprecated || chatShared.Routes[0].Evidence != catalogObserved ||
		!reflect.DeepEqual(chatShared.Routes[0].Capabilities, []domain.Capability{domain.CapabilityText}) {
		t.Fatalf("chat admission = %#v", chatShared.Routes)
	}
	responsesShared := observedModel(t, responses, "provider-shared")
	if responsesShared.State != catalogAdmitted || len(responsesShared.Routes) != 1 || responsesShared.Routes[0].ID != "responses" ||
		responsesShared.Routes[0].Lifecycle != domain.RouteAdmitted || responsesShared.Routes[0].Evidence != catalogObserved ||
		!reflect.DeepEqual(responsesShared.Routes[0].Capabilities, []domain.Capability{
			domain.CapabilityReasoning,
			domain.CapabilityText,
		}) {
		t.Fatalf("Responses admission = %#v", responsesShared.Routes)
	}
	if slices.Contains(chatShared.Routes[0].Capabilities, domain.CapabilityReasoning) {
		t.Fatalf("Responses reasoning leaked into Chat Completions: %#v", chatShared.Routes[0])
	}
}

func assertCatalogCandidateAndMissingRoute(t *testing.T, result catalogOutput) {
	t.Helper()
	chat := observationForProtocol(t, result, domain.ProtocolOpenAIChatCompletions)
	responses := observationForProtocol(t, result, domain.ProtocolOpenAIResponses)
	candidate := observedModel(t, responses, "provider-new")
	if candidate.State != catalogCandidate || len(candidate.Routes) != 0 {
		t.Fatalf("candidate = %#v", candidate)
	}
	if len(responses.MissingRoutes) != 1 || responses.MissingRoutes[0].ID != "missing" ||
		responses.MissingRoutes[0].Lifecycle != domain.RouteAdmitted || responses.MissingRoutes[0].Evidence != catalogRequalificationRequired {
		t.Fatalf("missing Routes = %#v", responses.MissingRoutes)
	}
	if len(chat.MissingRoutes) != 0 {
		t.Fatalf("Chat Completions missing Routes = %#v", chat.MissingRoutes)
	}
}

func TestCatalogParsingRejectsAmbiguousProviderIdentifiers(t *testing.T) {
	for _, body := range []string{
		`{"data":[{"id":"one","model":"two"}]}`,
		`{"data":[{"id":3}]}`,
		`{"data":[{}]}`,
		`{"data":[null]}`,
		`{"data":[" "]}`,
	} {
		if ids, err := ParseIDs([]byte(body)); err == nil || len(ids) != 0 {
			t.Fatalf("ambiguous catalogue item established identity: body=%s ids=%#v error=%v", body, ids, err)
		}
	}

	ids, err := ParseIDs([]byte(`{"data":[{"id":" exact ","model":"exact"},"zeta","exact"]}`))
	if err != nil || !reflect.DeepEqual(ids, []string{"exact", "zeta"}) {
		t.Fatalf("normalized IDs = %#v, error=%v", ids, err)
	}
}

// encoding/json reads null as an empty list and keeps the last of repeated
// members, so these responses would otherwise become confident observations.
// Other provider members, repeated or not, are not evidence and stay ignored.
func TestCatalogParsingRejectsAmbiguousContainers(t *testing.T) {
	for _, body := range []string{
		`{"data":null}`,
		`{"data":[],"data":["shadowed"]}`,
		`{"data":["shadowed"],"data":[]}`,
		`{"data":[{"id":"one","id":"two"}]}`,
		`{"data":[{"id":"same","id":"same"}]}`,
		`{"data":[{"model":"one","model":"two"}]}`,
		`{"data":[{"name":"one","name":"two"}]}`,
	} {
		if ids, err := ParseIDs([]byte(body)); err == nil || len(ids) != 0 {
			t.Fatalf("ambiguous catalogue established membership: body=%s ids=%#v error=%v", body, ids, err)
		}
	}
	for body, want := range map[string][]string{
		`{"data":[]}`: {},
		`{"object":"list","data":[{"id":"one","object":"model","owned_by":"x","owned_by":"y"}],"has_more":false,"has_more":true}`: {"one"},
	} {
		if ids, err := ParseIDs([]byte(body)); err != nil || !reflect.DeepEqual(ids, want) {
			t.Fatalf("valid catalogue body=%s ids=%#v error=%v", body, ids, err)
		}
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
		rows := modelRows(cfg, discoverCatalog(context.Background(), deps, cfg))
		if len(rows) != 1 || !strings.HasPrefix(rows[0].Catalog, want) {
			t.Fatalf("body=%s rows=%#v, want %q", body, rows, want)
		}
	}
}

func TestDeprecatedCatalogEntryDoesNotPresentAsAdmitted(t *testing.T) {
	state, detail := catalogModelDisplay(catalogModel{
		ID:    "legacy",
		State: catalogDeprecated,
		Routes: []catalogRoute{{
			ID: "legacy-route", Lifecycle: domain.RouteDeprecated, Evidence: catalogObserved,
		}},
	})
	if state == 0 || detail != "Deprecated Routes: legacy-route" {
		t.Fatalf("deprecated display state=%v detail=%q", state, detail)
	}
}

func TestDefaultCatalogRenderingKeepsDeprecatedRoutesSeparateFromCandidates(t *testing.T) {
	out := new(bytes.Buffer)
	renderCatalogObservation(renderer(Dependencies{Out: out, Width: 120}), nil, catalogObservation{
		Models: []catalogModel{
			{ID: "current", State: catalogAdmitted, Routes: []catalogRoute{{ID: "current-route"}}},
			{ID: "legacy", State: catalogDeprecated, Routes: []catalogRoute{{ID: "legacy-route"}}},
			{ID: "future", State: catalogCandidate},
		},
	}, false)

	text := out.String()
	for _, want := range []string{"1 admitted", "1 deprecated", "legacy", "Deprecated Routes: legacy-route", "1 candidate models require qualification"} {
		if !strings.Contains(text, want) {
			t.Fatalf("default catalogue output lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "future") {
		t.Fatalf("default catalogue output exposed candidate detail:\n%s", text)
	}
}

func observationForProtocol(t *testing.T, output catalogOutput, protocol domain.EndpointProtocol) catalogObservation {
	t.Helper()
	for _, observation := range output.Observations {
		if observation.Source.Protocol == protocol {
			return observation
		}
	}
	t.Fatalf("protocol %q not observed: %#v", protocol, output.Observations)
	return catalogObservation{}
}

func observedModel(t *testing.T, observation catalogObservation, id string) catalogModel {
	t.Helper()
	for _, model := range observation.Models {
		if model.ID == id {
			return model
		}
	}
	t.Fatalf("model %q not observed: %#v", id, observation.Models)
	return catalogModel{}
}
