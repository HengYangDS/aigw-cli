package app

import (
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
)

func TestModelCatalogHelpersAndResponseParsing(t *testing.T) {
	cfg := domain.NewConfig(clients.Admission())
	cfg.Routes["other"] = domain.Route{Account: "other", Model: "gpt", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.Routes["matching"] = domain.Route{Account: "one", Model: "canonical", UpstreamModel: "gpt", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	models, missing := catalogDifference(cfg, "one", domain.ProtocolOpenAIResponses, []string{"gpt"})
	if len(models) != 1 || models[0].State != CatalogAdmitted || len(models[0].Routes) != 1 || models[0].Routes[0].ID != "matching" || len(missing) != 0 {
		t.Fatalf("difference models=%#v missing=%#v", models, missing)
	}
	for status, want := range map[CatalogStatus]string{
		CatalogEndpointUnavailable: "No supported",
		CatalogTokenUnavailable:    "Token unavailable",
		CatalogRequestFailed:       "Catalog request failed",
		"future":                   "future",
	} {
		if got := status.Label(); !strings.Contains(got, want) {
			t.Errorf("%q.Label() = %q", status, got)
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
			got, err := parseIDs([]byte(test.body))
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
	if _, _, err := fetchIDs(context.Background(), catalogHTTPClient(nil), endpoint, domain.ProtocolOpenAIResponses, "token"); err == nil {
		t.Fatal("expected URL error")
	}

	want := errors.New("network failed")
	endpoint = "https://one.test/v1"
	client := catalogHTTPClient(func(*http.Request) (*http.Response, error) { return nil, want })
	if _, _, err := fetchIDs(context.Background(), client, endpoint, domain.ProtocolOpenAIResponses, "token"); !errors.Is(err, want) {
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
			ids, source, err := fetchIDs(context.Background(), client, endpoint, domain.ProtocolOpenAIResponses, "token")
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
	if _, _, err := fetchIDs(context.Background(), client, endpoint, domain.ProtocolOpenAIResponses, "token"); err == nil || !strings.Contains(err.Error(), "HTTP 502") {
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
			if ids, _, err := fetchIDs(context.Background(), client, "https://catalog.test/v1", domain.ProtocolOpenAIResponses, "token"); err == nil || len(ids) != 0 {
				t.Fatalf("incomplete catalog established membership: ids=%v error=%v", ids, err)
			}
		})
	}
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
	ids, _, err := fetchIDs(context.Background(), client, "https://gateway.test/v1", domain.ProtocolOpenAIResponses, "token")
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

func TestCatalogObservationsFollowAccountOrder(t *testing.T) {
	cfg := configuredCatalog()
	cfg.Accounts["alpha"] = domain.Account{Label: "Alpha"}
	client := catalogHTTPClient(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":["gpt-codex"]}`)), Request: request}, nil
	})
	observations := DiscoverCatalog(context.Background(), catalogSecrets(t, map[string]string{"gateway": "token"}), client, cfg).Observations
	if len(observations) != 3 || observations[0].Account != "alpha" || observations[1].Account != "gateway" || observations[2].Account != "gateway" {
		t.Fatalf("catalog observations = %#v", observations)
	}
}

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
	result := DiscoverCatalog(context.Background(), catalogSecrets(t, map[string]string{"gateway": "token"}), client, cfg)
	if !reflect.DeepEqual(cfg, before) {
		t.Fatalf("catalog observation mutated configuration:\nbefore=%#v\nafter=%#v", before, cfg)
	}
	assertObservationIdentityAndSources(t, result)
	assertRouteCapabilityIsolation(t, result)
	assertCatalogCandidateAndMissingRoute(t, result)
}

func assertObservationIdentityAndSources(t *testing.T, result CatalogReport) {
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

func assertRouteCapabilityIsolation(t *testing.T, result CatalogReport) {
	t.Helper()
	chat := observationForProtocol(t, result, domain.ProtocolOpenAIChatCompletions)
	responses := observationForProtocol(t, result, domain.ProtocolOpenAIResponses)
	chatShared := observedModel(t, chat, "provider-shared")
	if chatShared.State != CatalogDeprecated || len(chatShared.Routes) != 1 || chatShared.Routes[0].ID != "chat" ||
		chatShared.Routes[0].Lifecycle != domain.RouteDeprecated || chatShared.Routes[0].Evidence != CatalogObserved ||
		!reflect.DeepEqual(chatShared.Routes[0].Capabilities, []domain.Capability{domain.CapabilityText}) {
		t.Fatalf("chat admission = %#v", chatShared.Routes)
	}
	responsesShared := observedModel(t, responses, "provider-shared")
	if responsesShared.State != CatalogAdmitted || len(responsesShared.Routes) != 1 || responsesShared.Routes[0].ID != "responses" ||
		responsesShared.Routes[0].Lifecycle != domain.RouteAdmitted || responsesShared.Routes[0].Evidence != CatalogObserved ||
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

func assertCatalogCandidateAndMissingRoute(t *testing.T, result CatalogReport) {
	t.Helper()
	chat := observationForProtocol(t, result, domain.ProtocolOpenAIChatCompletions)
	responses := observationForProtocol(t, result, domain.ProtocolOpenAIResponses)
	candidate := observedModel(t, responses, "provider-new")
	if candidate.State != CatalogCandidate || len(candidate.Routes) != 0 {
		t.Fatalf("candidate = %#v", candidate)
	}
	if len(responses.MissingRoutes) != 1 || responses.MissingRoutes[0].ID != "missing" ||
		responses.MissingRoutes[0].Lifecycle != domain.RouteAdmitted || responses.MissingRoutes[0].Evidence != CatalogRequalificationRequired {
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
		if ids, err := parseIDs([]byte(body)); err == nil || len(ids) != 0 {
			t.Fatalf("ambiguous catalogue item established identity: body=%s ids=%#v error=%v", body, ids, err)
		}
	}

	ids, err := parseIDs([]byte(`{"data":[{"id":" exact ","model":"exact"},"zeta","exact"]}`))
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
		if ids, err := parseIDs([]byte(body)); err == nil || len(ids) != 0 {
			t.Fatalf("ambiguous catalogue established membership: body=%s ids=%#v error=%v", body, ids, err)
		}
	}
	for body, want := range map[string][]string{
		`{"data":[]}`: {},
		`{"object":"list","data":[{"id":"one","object":"model","owned_by":"x","owned_by":"y"}],"has_more":false,"has_more":true}`: {"one"},
	} {
		if ids, err := parseIDs([]byte(body)); err != nil || !reflect.DeepEqual(ids, want) {
			t.Fatalf("valid catalogue body=%s ids=%#v error=%v", body, ids, err)
		}
	}
}

func observationForProtocol(t *testing.T, output CatalogReport, protocol domain.EndpointProtocol) CatalogObservation {
	t.Helper()
	for _, observation := range output.Observations {
		if observation.Source.Protocol == protocol {
			return observation
		}
	}
	t.Fatalf("protocol %q not observed: %#v", protocol, output.Observations)
	return CatalogObservation{}
}

func observedModel(t *testing.T, observation CatalogObservation, id string) CatalogModel {
	t.Helper()
	for _, model := range observation.Models {
		if model.ID == id {
			return model
		}
	}
	t.Fatalf("model %q not observed: %#v", id, observation.Models)
	return CatalogModel{}
}

// catalogSecrets returns a Token store that holds the given Account Tokens.
func catalogSecrets(t *testing.T, values map[string]string) secret.Store {
	t.Helper()
	store := secret.NewMemoryStore()
	for account, value := range values {
		if err := store.Set(account, value); err != nil {
			t.Fatal(err)
		}
	}
	return store
}
