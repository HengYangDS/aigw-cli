// Package credential owns provider-neutral credential validation against the
// endpoint protocol declared by AIGW configuration.
package credential

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
	Do(*http.Request) (*http.Response, error)
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
		endpoint, err := account.EndpointFor(client)
		if err != nil {
			return err
		}
		checkCtx, cancel := context.WithTimeout(ctx, validationTimeout)
		req, err := ProbeRequest(checkCtx, client, endpoint, token)
		if err != nil {
			cancel()
			return err
		}
		clientHTTP := withoutRedirects(httpClient)
		resp, err := clientHTTP.Do(req)
		if err != nil {
			cancel()
			return fmt.Errorf("%s endpoint is unreachable: %w", title(client), err)
		}
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			_ = resp.Body.Close()
			cancel()
			return fmt.Errorf("read %s endpoint response: %w", title(client), err)
		}
		if err := resp.Body.Close(); err != nil {
			cancel()
			return fmt.Errorf("close %s endpoint response: %w", title(client), err)
		}
		cancel()
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("%s authentication was rejected (HTTP %d)", title(client), resp.StatusCode)
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return fmt.Errorf("%s endpoint returned HTTP %d", title(client), resp.StatusCode)
		}
	}
	return nil
}

// ProbeRequest constructs the authentication request declared by one admitted
// client's endpoint protocol.
func ProbeRequest(ctx context.Context, client, endpoint, token string) (*http.Request, error) {
	spec, ok := configuration.ClientSpecFor(client)
	if !ok {
		return nil, fmt.Errorf("unsupported credential validation client %q", client)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsEndpoint(endpoint, spec.EndpointProtocol), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	authenticate(req, spec, token)
	return req, nil
}

func authenticate(request *http.Request, spec configuration.ClientSpec, token string) {
	switch spec.EndpointProtocol {
	case configuration.ProtocolAnthropic:
		request.Header.Set("X-Api-Key", token)
		request.Header.Set("Anthropic-Version", "2023-06-01")
	case configuration.ProtocolOpenAIResponses:
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

func withoutRedirects(doer HTTPDoer) HTTPDoer {
	client, ok := doer.(*http.Client)
	if !ok {
		return doer
	}
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &copy
}

func title(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
