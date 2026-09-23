package catalog

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
)

func TestCatalogObservationSeparatesProtocolCapabilitiesAndDifferences(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{
		Label: "Gateway",
		Endpoints: configuration.Endpoints{
			OpenAIChatCompletions: "https://chat.test/v1",
			OpenAIResponses:       "https://responses.test/v1",
		},
	}
	cfg.Models["shared"] = configuration.Model{Label: "Shared"}
	cfg.Models["missing"] = configuration.Model{Label: "Missing"}
	cfg.Routes["chat"] = configuration.Route{
		Label:         "Chat",
		Account:       "gateway",
		Model:         "shared",
		UpstreamModel: "provider-shared",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{
			configuration.ProtocolOpenAIChatCompletions: {configuration.CapabilityText},
		},
	}
	cfg.Routes["responses"] = configuration.Route{
		Label:         "Responses",
		Account:       "gateway",
		Model:         "shared",
		UpstreamModel: "provider-shared",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{
			configuration.ProtocolOpenAIResponses: {
				configuration.CapabilityText,
				configuration.CapabilityReasoning,
			},
		},
	}
	cfg.Routes["missing"] = configuration.Route{
		Label:         "Missing",
		Account:       "gateway",
		Model:         "missing",
		UpstreamModel: "provider-missing",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{
			configuration.ProtocolOpenAIResponses: {configuration.CapabilityText},
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
	if len(result.Observations) != 2 {
		t.Fatalf("observations = %#v", result.Observations)
	}

	chat := observationForProtocol(t, result, configuration.ProtocolOpenAIChatCompletions)
	responses := observationForProtocol(t, result, configuration.ProtocolOpenAIResponses)
	if chat.ObservationID == "" || responses.ObservationID == "" || chat.ObservationID == responses.ObservationID {
		t.Fatalf("observation identities do not bind protocol: chat=%q responses=%q", chat.ObservationID, responses.ObservationID)
	}
	if got := catalogObservationID("gateway", configuration.ProtocolOpenAIResponses, responses.Source.Endpoint, []string{"provider-new", "provider-shared"}); got != responses.ObservationID {
		t.Fatalf("observation identity depends on provider order: got %q want %q", got, responses.ObservationID)
	}
	if chat.Source.Endpoint != "https://chat.test/v1/models" || responses.Source.Endpoint != "https://responses.test/v1/models" {
		t.Fatalf("sources = chat %#v responses %#v", chat.Source, responses.Source)
	}

	chatShared := observedModel(t, chat, "provider-shared")
	if len(chatShared.Routes) != 1 || chatShared.Routes[0].ID != "chat" ||
		!reflect.DeepEqual(chatShared.Routes[0].Capabilities, []configuration.Capability{configuration.CapabilityText}) {
		t.Fatalf("chat admission = %#v", chatShared.Routes)
	}
	responsesShared := observedModel(t, responses, "provider-shared")
	if len(responsesShared.Routes) != 1 || responsesShared.Routes[0].ID != "responses" ||
		!reflect.DeepEqual(responsesShared.Routes[0].Capabilities, []configuration.Capability{
			configuration.CapabilityReasoning,
			configuration.CapabilityText,
		}) {
		t.Fatalf("Responses admission = %#v", responsesShared.Routes)
	}
	if slices.Contains(chatShared.Routes[0].Capabilities, configuration.CapabilityReasoning) {
		t.Fatalf("Responses reasoning leaked into Chat Completions: %#v", chatShared.Routes[0])
	}

	candidate := observedModel(t, responses, "provider-new")
	if candidate.State != catalogCandidate || len(candidate.Routes) != 0 {
		t.Fatalf("candidate = %#v", candidate)
	}
	if len(responses.MissingRoutes) != 1 || responses.MissingRoutes[0].ID != "missing" ||
		responses.MissingRoutes[0].State != catalogRequalificationRequired {
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

func observationForProtocol(t *testing.T, output catalogOutput, protocol configuration.EndpointProtocol) catalogObservation {
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
