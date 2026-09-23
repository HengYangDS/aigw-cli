// Package credential owns provider-neutral credential validation against the
// endpoint protocol declared by AIGW configuration.
package credential

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

const validationTimeout = 12 * time.Second

// TokenRecovery names the executable recovery owned by the selected secret
// backend and reports whether the backend can persist a replacement Token.
func TokenRecovery(store secrets.Store, account string) (instruction string, writable bool) {
	if secrets.IsReadOnly(store) {
		return "set environment variable " + secrets.EnvironmentKey(account), false
	}
	return "run `aigw rotate " + account + "`", true
}

// HTTPDoer executes one validation request.
type HTTPDoer interface {
	Do(request *http.Request) (*http.Response, error)
}

// Validate proves that token is accepted by each requested client protocol.
// When clients is empty, the account's available endpoint selects the probe.
func Validate(ctx context.Context, httpClient HTTPDoer, account configuration.Account, token string, clients ...string) error {
	if len(clients) == 0 {
		if account.Endpoints.OpenAIResponses != "" {
			clients = []string{configuration.ClientCodex}
		} else {
			clients = []string{configuration.ClientClaude}
		}
	}
	seen := map[string]bool{}
	for _, client := range clients {
		if seen[client] {
			continue
		}
		seen[client] = true
		if _, ok := configuration.ClientSpecFor(client); !ok {
			return fmt.Errorf("unsupported credential validation client %q", client)
		}
		spec, _ := configuration.ClientSpecFor(client)
		endpoint, protocol, err := spec.ResolveEndpoint(account, "")
		if err != nil {
			return err
		}
		if err := validateEndpoint(ctx, httpClient, client, endpoint, protocol, token); err != nil {
			return err
		}
	}
	return nil
}

// ValidateRuntime verifies one fully resolved client endpoint without
// re-selecting or guessing its protocol.
func ValidateRuntime(ctx context.Context, httpClient HTTPDoer, runtime configuration.Runtime, token string) error {
	return validateEndpoint(ctx, httpClient, runtime.Client, runtime.Endpoint, runtime.Protocol, token)
}

func validateEndpoint(ctx context.Context, httpClient HTTPDoer, client, endpoint string, protocol configuration.EndpointProtocol, token string) error {
	status, err := ProbeStatus(ctx, httpClient, client, endpoint, token, protocol)
	if err != nil {
		return err
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return fmt.Errorf("%s authentication was rejected (HTTP %d)", title(client), status)
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return fmt.Errorf("%s endpoint returned HTTP %d", title(client), status)
	}
	return nil
}

// ProbeStatus executes one bounded authenticated endpoint request and closes
// its response before returning the status. A status is transport evidence;
// callers own its interpretation as credential validation or diagnostics.
func ProbeStatus(ctx context.Context, httpClient HTTPDoer, client, endpoint, token string, protocols ...configuration.EndpointProtocol) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, validationTimeout)
	defer cancel()
	request, err := ProbeRequest(ctx, client, endpoint, token, protocols...)
	if err != nil {
		return 0, err
	}
	response, err := DoProbe(httpClient, request)
	if err != nil {
		return 0, fmt.Errorf("%s endpoint is unreachable: %w", title(client), err)
	}
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		_ = response.Body.Close()
		return 0, fmt.Errorf("read %s endpoint response: %w", title(client), err)
	}
	if err := response.Body.Close(); err != nil {
		return 0, fmt.Errorf("close %s endpoint response: %w", title(client), err)
	}
	return response.StatusCode, nil
}

// DoProbe sends one authenticated request without following redirects when
// using a native [http.Client]. It preserves the caller's client configuration.
// Other injected transports must perform only the supplied request.
func DoProbe(httpClient HTTPDoer, request *http.Request) (*http.Response, error) {
	if client, ok := httpClient.(*http.Client); ok {
		clone := *client
		clone.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		httpClient = &clone
	}
	return httpClient.Do(request)
}

// ProbeRequest constructs the authentication request declared by one admitted
// client's endpoint protocol.
func ProbeRequest(ctx context.Context, client, endpoint, token string, protocols ...configuration.EndpointProtocol) (*http.Request, error) {
	spec, ok := configuration.ClientSpecFor(client)
	if !ok {
		return nil, fmt.Errorf("unsupported credential validation client %q", client)
	}
	protocol := configuration.EndpointProtocol("")
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

// ModelCatalogRequest constructs the authenticated model-catalogue request for
// one exact wire protocol. It does not infer model or inference capabilities.
func ModelCatalogRequest(ctx context.Context, endpoint string, protocol configuration.EndpointProtocol, token string) (*http.Request, error) {
	switch protocol {
	case configuration.ProtocolAnthropic,
		configuration.ProtocolOpenAIResponses,
		configuration.ProtocolOpenAIChatCompletions:
	default:
		return nil, fmt.Errorf("unsupported model catalogue protocol %q", protocol)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsEndpoint(endpoint, protocol), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	authenticate(req, protocol, token)
	return req, nil
}

func authenticate(request *http.Request, protocol configuration.EndpointProtocol, token string) {
	switch protocol {
	case configuration.ProtocolAnthropic:
		request.Header.Set("X-Api-Key", token)
		request.Header.Set("Anthropic-Version", "2023-06-01")
	case configuration.ProtocolOpenAIResponses, configuration.ProtocolOpenAIChatCompletions:
		request.Header.Set("Authorization", "Bearer "+token)
	}
}

func modelsEndpoint(endpoint string, protocol configuration.EndpointProtocol) string {
	endpoint = strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(endpoint, "/models") {
		return endpoint
	}
	if protocol == configuration.ProtocolAnthropic && !strings.HasSuffix(endpoint, "/v1") {
		return endpoint + "/v1/models"
	}
	return endpoint + "/models"
}

func title(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
