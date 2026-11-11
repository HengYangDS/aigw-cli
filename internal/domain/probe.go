package domain

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// ProbeRequest constructs the authentication request declared by one admitted
// client's endpoint protocol.
func ProbeRequest(ctx context.Context, admission Admission, client, endpoint, token string, protocols ...EndpointProtocol) (*http.Request, error) {
	spec, ok := admission.Spec(client)
	if !ok {
		return nil, fmt.Errorf("unsupported credential validation client %q", client)
	}
	protocol := EndpointProtocol("")
	if len(protocols) > 1 {
		return nil, fmt.Errorf("credential probe requires one protocol")
	}
	if len(protocols) == 1 {
		protocol = protocols[0]
	}
	if protocol == "" && len(spec.EndpointProtocols) == 1 {
		protocol = spec.EndpointProtocols[0]
	}
	if protocol == "" || !slices.Contains(spec.EndpointProtocols, protocol) {
		return nil, fmt.Errorf("client %q requires an admitted endpoint protocol", client)
	}
	return ModelCatalogRequest(ctx, endpoint, protocol, token)
}

// ModelCatalogRequest constructs the authenticated model-catalog request for
// one exact wire protocol. It does not infer model or inference capabilities.
func ModelCatalogRequest(ctx context.Context, endpoint string, protocol EndpointProtocol, token string) (*http.Request, error) {
	switch protocol {
	case ProtocolAnthropic,
		ProtocolResponses,
		ProtocolChat:
	default:
		return nil, fmt.Errorf("unsupported model catalog protocol %q", protocol)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsEndpoint(endpoint, protocol), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	authenticate(req, protocol, token)
	return req, nil
}

func authenticate(request *http.Request, protocol EndpointProtocol, token string) {
	switch protocol {
	case ProtocolAnthropic:
		request.Header.Set("X-Api-Key", token)
		request.Header.Set("Anthropic-Version", "2023-06-01")
	case ProtocolResponses, ProtocolChat:
		request.Header.Set("Authorization", "Bearer "+token)
	}
}

// CatalogRequestKey identifies the request that ModelCatalogRequest builds
// for one endpoint and protocol, apart from its Token: its URL and the
// authentication scheme that authenticate gives it. Two protocols with one
// key read one catalog, such as Responses and Chat Completions on one URL.
func CatalogRequestKey(endpoint string, protocol EndpointProtocol) string {
	scheme := "bearer"
	if protocol == ProtocolAnthropic {
		scheme = "x-api-key"
	}
	return scheme + " " + modelsEndpoint(endpoint, protocol)
}

func modelsEndpoint(endpoint string, protocol EndpointProtocol) string {
	endpoint = strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(endpoint, "/models") {
		return endpoint
	}
	if protocol == ProtocolAnthropic && !strings.HasSuffix(endpoint, "/v1") {
		return endpoint + "/v1/models"
	}
	return endpoint + "/models"
}
