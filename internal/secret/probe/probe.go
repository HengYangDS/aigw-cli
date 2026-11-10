// Package probe proves an Account's Token against the Account's endpoints. It
// chooses the client that probes, sends each request without following a
// redirect, and turns a probe's status into a verdict on the Token. Sender sends
// a Provider diagnostic's requests the same way, so that no credential reaches
// another origin.
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
// admission resolves the clients. When clients is empty, the provider's OpenAI
// Responses endpoint, or else its Anthropic endpoint, or else its Chat
// Completions endpoint selects the probe, and the first admitted client that
// speaks only that protocol, or else the first that speaks it, names it.
func Validate(ctx context.Context, httpClient HTTPDoer, admission domain.Admission, account domain.Provider, token string, clients ...string) error {
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

// defaultValidationClient names the client whose protocol probes a provider's
// Token when the caller requests none. A client that speaks only that
// protocol is preferred; one that speaks several resolves the provider's only
// endpoint among them, because a provider without the preferred endpoints has
// just the Chat Completions one.
func defaultValidationClient(admission domain.Admission, account domain.Provider) (string, error) {
	protocol := domain.ProtocolAnthropic
	switch {
	case account.Endpoints.OpenAIResponses != "":
		protocol = domain.ProtocolOpenAIResponses
	case account.Endpoints.Anthropic == "" && account.Endpoints.OpenAIChatCompletions != "":
		protocol = domain.ProtocolOpenAIChatCompletions
	}
	speaker := ""
	for _, spec := range admission.Specs() {
		if slices.Equal(spec.EndpointProtocols, []domain.EndpointProtocol{protocol}) {
			return spec.ID, nil
		}
		if speaker == "" && slices.Contains(spec.EndpointProtocols, protocol) {
			speaker = spec.ID
		}
	}
	if speaker != "" {
		return speaker, nil
	}
	return "", fmt.Errorf("no admitted client probes an Account Token over %s", domain.ProtocolLabel(protocol))
}

// ValidateRuntime verifies one fully resolved client endpoint without
// re-selecting or guessing its protocol; admission names its client.
func ValidateRuntime(ctx context.Context, httpClient HTTPDoer, admission domain.Admission, runtime domain.Runtime, token string) error {
	return probeEndpoint(ctx, httpClient, admission, runtime.Client, runtime.Endpoint, runtime.Protocol, token)
}

func probeEndpoint(ctx context.Context, httpClient HTTPDoer, admission domain.Admission, client, endpoint string, protocol domain.EndpointProtocol, token string) error {
	status, err := endpointStatus(ctx, httpClient, admission, client, endpoint, token, protocol)
	if err != nil {
		return err
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return &TokenRejectedError{Label: admission.Label(client), Status: status}
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return &EndpointStatusError{Label: admission.Label(client), Protocol: protocol, Status: status}
	}
	return nil
}

// TokenRejectedError reports that the provider refused the Token with HTTP
// 401 or 403.
type TokenRejectedError struct {
	Label  string
	Status int
}

// Error names the client whose request the provider refused, and the status.
func (e *TokenRejectedError) Error() string {
	return fmt.Sprintf("%s authentication was rejected (HTTP %d)", e.Label, e.Status)
}

// EndpointStatusError reports a status of the endpoint that neither accepts
// nor rejects the Token, such as 404 from a wrong endpoint URL.
type EndpointStatusError struct {
	Label    string
	Protocol domain.EndpointProtocol
	Status   int
}

// Error names the client whose endpoint answered, and the status.
func (e *EndpointStatusError) Error() string {
	return fmt.Sprintf("%s endpoint returned HTTP %d", e.Label, e.Status)
}

// UnreachableError reports an endpoint that sent no response.
type UnreachableError struct {
	Label string
	Err   error
}

// Error names the client whose endpoint did not answer, and why.
func (e *UnreachableError) Error() string {
	return fmt.Sprintf("%s endpoint is unreachable: %v", e.Label, e.Err)
}

// Unwrap returns the transport failure.
func (e *UnreachableError) Unwrap() error { return e.Err }

// endpointStatus executes one bounded authenticated endpoint request and
// closes its response before returning the status. A status is transport
// evidence; its callers interpret it as credential validation.
func endpointStatus(ctx context.Context, httpClient HTTPDoer, admission domain.Admission, client, endpoint, token string, protocols ...domain.EndpointProtocol) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, validationTimeout)
	defer cancel()
	request, err := domain.ProbeRequest(ctx, admission, client, endpoint, token, protocols...)
	if err != nil {
		return 0, err
	}
	response, err := Send(httpClient, request)
	if err != nil {
		return 0, &UnreachableError{Label: admission.Label(client), Err: err}
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

// Sender sends through Client with Send, for a caller that receives a sender
// instead of a client.
type Sender struct {
	Client HTTPDoer
}

// Send sends request through the sender's client without following a redirect.
func (sender Sender) Send(request *http.Request) (*http.Response, error) {
	return Send(sender.Client, request)
}
