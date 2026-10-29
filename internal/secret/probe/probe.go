// Package probe proves an Account's Token against the Account's endpoints: it
// chooses the client that probes, sends each request without following a
// redirect, and judges each status.
package probe

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"aigw-cli/internal/domain"
)

const validationTimeout = 12 * time.Second

// HTTPDoer executes one validation request.
type HTTPDoer interface {
	Do(request *http.Request) (*http.Response, error)
}

// Validate proves that token is accepted by each requested client protocol;
// admission resolves the clients. When clients is empty, the account's OpenAI
// Responses endpoint, or else its Anthropic endpoint, selects the probe, and
// the first admitted client that speaks only that protocol names it.
func Validate(ctx context.Context, httpClient HTTPDoer, admission domain.Admission, account domain.Account, token string, clients ...string) error {
	if len(clients) == 0 {
		client, err := defaultValidationClient(admission, account)
		if err != nil {
			return err
		}
		clients = []string{client}
	}
	seen := map[string]bool{}
	for _, client := range clients {
		if seen[client] {
			continue
		}
		seen[client] = true
		spec, ok := admission.Spec(client)
		if !ok {
			return fmt.Errorf("unsupported credential validation client %q", client)
		}
		endpoint, protocol, err := spec.ResolveEndpoint(account, "")
		if err != nil {
			return err
		}
		if err := probeEndpoint(ctx, httpClient, admission, client, endpoint, protocol, token); err != nil {
			return err
		}
	}
	return nil
}

// defaultValidationClient names the client whose protocol probes an Account
// Token when the caller requests none.
func defaultValidationClient(admission domain.Admission, account domain.Account) (string, error) {
	protocol := domain.ProtocolAnthropic
	if account.Endpoints.OpenAIResponses != "" {
		protocol = domain.ProtocolOpenAIResponses
	}
	for _, spec := range admission.Specs() {
		if slices.Equal(spec.EndpointProtocols, []domain.EndpointProtocol{protocol}) {
			return spec.ID, nil
		}
	}
	return "", fmt.Errorf("no admitted client probes an Account Token over %s", domain.ProtocolLabel(protocol))
}

// ValidateRuntime verifies one fully resolved client endpoint without
// re-selecting or guessing its protocol; admission names its client.
func ValidateRuntime(ctx context.Context, httpClient HTTPDoer, admission domain.Admission, runtime domain.Runtime, token string) error {
	return probeEndpoint(ctx, httpClient, admission, runtime.Client, runtime.Endpoint, runtime.Protocol, token)
}

func probeEndpoint(ctx context.Context, httpClient HTTPDoer, admission domain.Admission, client, endpoint string, protocol domain.EndpointProtocol, token string) error {
	status, err := Status(ctx, httpClient, admission, client, endpoint, token, protocol)
	if err != nil {
		return err
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return fmt.Errorf("%s authentication was rejected (HTTP %d)", admission.Label(client), status)
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return fmt.Errorf("%s endpoint returned HTTP %d", admission.Label(client), status)
	}
	return nil
}

// Status executes one bounded authenticated endpoint request and closes
// its response before returning the status. A status is transport evidence;
// callers own its interpretation as credential validation or diagnostics.
func Status(ctx context.Context, httpClient HTTPDoer, admission domain.Admission, client, endpoint, token string, protocols ...domain.EndpointProtocol) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, validationTimeout)
	defer cancel()
	request, err := domain.ProbeRequest(ctx, admission, client, endpoint, token, protocols...)
	if err != nil {
		return 0, err
	}
	response, err := Send(httpClient, request)
	if err != nil {
		return 0, fmt.Errorf("%s endpoint is unreachable: %w", admission.Label(client), err)
	}
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		_ = response.Body.Close()
		return 0, fmt.Errorf("read %s endpoint response: %w", admission.Label(client), err)
	}
	if err := response.Body.Close(); err != nil {
		return 0, fmt.Errorf("close %s endpoint response: %w", admission.Label(client), err)
	}
	return response.StatusCode, nil
}

// Send sends one authenticated request without following redirects when
// using a native [http.Client]. It preserves the caller's client configuration.
// Other injected transports must perform only the supplied request.
func Send(httpClient HTTPDoer, request *http.Request) (*http.Response, error) {
	if client, ok := httpClient.(*http.Client); ok {
		clone := *client
		clone.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		httpClient = &clone
	}
	return httpClient.Do(request)
}
