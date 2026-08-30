package credential

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestTokenRemediationMatchesCredentialAuthority(t *testing.T) {
	t.Parallel()

	if got, writable := TokenRecovery(secrets.NewMemoryStore(), "team-gateway"); got != "run `aigw rotate team-gateway`" || !writable {
		t.Fatalf("writable recovery = %q, %t", got, writable)
	}
	store := secrets.NewEnvironmentStore(func(string) string { return "" })
	if got, writable := TokenRecovery(store, "team-gateway"); got != "set environment variable AIGW_TOKEN_TEAM_2DGATEWAY" || writable {
		t.Fatalf("read-only recovery = %q, %t", got, writable)
	}
}

type doerFunc func(*http.Request) (*http.Response, error)

func (do doerFunc) Do(request *http.Request) (*http.Response, error) {
	return do(request)
}

type responseBody struct {
	io.Reader
	closeErr error
}

func (body responseBody) Close() error { return body.closeErr }

type failingReader struct{ err error }

func (reader failingReader) Read([]byte) (int, error) { return 0, reader.err }

func TestValidateProtocolsAndDefaultSelection(t *testing.T) {
	t.Parallel()

	account := configuration.Account{
		ID: "team",
		Endpoints: configuration.Endpoints{
			Anthropic:       "https://anthropic.example/api",
			OpenAIResponses: "https://responses.example/v1/",
		},
	}
	requests := make([]*http.Request, 0, 2)
	doer := doerFunc(func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request.Clone(request.Context()))
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})

	if err := Validate(context.Background(), doer, account, "secret", configuration.ClientClaude, configuration.ClientCodex, configuration.ClientCodex); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
	if got := requests[0].URL.String(); got != "https://anthropic.example/api/v1/models" {
		t.Fatalf("Anthropic URL = %q", got)
	}
	if got := requests[0].Header.Get("X-Api-Key"); got != "secret" {
		t.Fatalf("X-Api-Key = %q", got)
	}
	if got := requests[0].Header.Get("Anthropic-Version"); got != "2023-06-01" {
		t.Fatalf("Anthropic-Version = %q", got)
	}
	if got := requests[1].URL.String(); got != "https://responses.example/v1/models" {
		t.Fatalf("Responses URL = %q", got)
	}
	if got := requests[1].Header.Get("Authorization"); got != "Bearer secret" {
		t.Fatalf("Authorization = %q", got)
	}

	requests = requests[:0]
	if err := Validate(context.Background(), doer, account, "secret"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0].URL.Host != "responses.example" {
		t.Fatalf("default requests = %#v", requests)
	}

	requests = requests[:0]
	account.Endpoints.OpenAIResponses = ""
	if err := Validate(context.Background(), doer, account, "secret"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0].URL.Host != "anthropic.example" {
		t.Fatalf("Anthropic default requests = %#v", requests)
	}
}

func TestValidateRejectsInvalidRequestsAndResponses(t *testing.T) {
	t.Parallel()

	account := configuration.Account{
		ID: "team",
		Endpoints: configuration.Endpoints{
			OpenAIResponses: "https://responses.example/v1",
		},
	}
	tests := []struct {
		name    string
		account configuration.Account
		client  string
		doer    HTTPDoer
		want    string
	}{
		{name: "unknown client", account: account, client: "unknown", want: `unsupported credential validation client "unknown"`},
		{name: "missing endpoint", account: account, client: configuration.ClientClaude, want: "no Anthropic endpoint"},
		{name: "invalid URL", account: configuration.Account{ID: "team", Endpoints: configuration.Endpoints{OpenAIResponses: "://bad"}}, client: configuration.ClientCodex, want: "missing protocol scheme"},
		{name: "unreachable", account: account, client: configuration.ClientCodex, doer: doerFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") }), want: "Codex endpoint is unreachable: offline"},
		{name: "read body", account: account, client: configuration.ClientCodex, doer: staticResponse(http.StatusOK, responseBody{Reader: failingReader{err: errors.New("broken body")}}), want: "read Codex endpoint response: broken body"},
		{name: "close body", account: account, client: configuration.ClientCodex, doer: staticResponse(http.StatusOK, responseBody{Reader: strings.NewReader("ok"), closeErr: errors.New("broken close")}), want: "close Codex endpoint response: broken close"},
		{name: "unauthorized", account: account, client: configuration.ClientCodex, doer: staticResponse(http.StatusUnauthorized, http.NoBody), want: "Codex authentication was rejected (HTTP 401)"},
		{name: "forbidden", account: account, client: configuration.ClientCodex, doer: staticResponse(http.StatusForbidden, http.NoBody), want: "Codex authentication was rejected (HTTP 403)"},
		{name: "unexpected status", account: account, client: configuration.ClientCodex, doer: staticResponse(http.StatusBadGateway, http.NoBody), want: "Codex endpoint returned HTTP 502"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(context.Background(), test.doer, test.account, "secret", test.client)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestProbeRequestRejectsUnknownClient(t *testing.T) {
	t.Parallel()

	if _, err := ProbeRequest(context.Background(), "unknown", "https://gateway.example", "secret"); err == nil || !strings.Contains(err.Error(), `unsupported credential validation client "unknown"`) {
		t.Fatalf("error = %v, want unsupported client", err)
	}
}

func TestValidateHTTPClientDoesNotFollowRedirects(t *testing.T) {
	t.Parallel()

	followed := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/v1/models" {
			http.Redirect(writer, request, "/followed", http.StatusFound)
			return
		}
		followed = true
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	account := configuration.Account{ID: "team", Endpoints: configuration.Endpoints{OpenAIResponses: server.URL + "/v1"}}

	err := Validate(context.Background(), server.Client(), account, "secret", configuration.ClientCodex)
	if err == nil || !strings.Contains(err.Error(), "HTTP 302") {
		t.Fatalf("error = %v, want HTTP 302", err)
	}
	if followed {
		t.Fatal("validation followed a redirect")
	}
}

func TestCredentialHelpers(t *testing.T) {
	t.Parallel()

	request, err := http.NewRequest(http.MethodGet, "https://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := modelsEndpoint("https://example.test/models", configuration.ProtocolOpenAIResponses); got != "https://example.test/models" {
		t.Fatalf("models endpoint = %q", got)
	}
	called := false
	doer := doerFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, nil
	})
	if _, err := withoutRedirects(doer).Do(request); err != nil || !called {
		t.Fatalf("non-HTTP doer call: called=%v error=%v", called, err)
	}
	if got := title(""); got != "" {
		t.Fatalf("empty title = %q", got)
	}
}

func staticResponse(status int, body io.ReadCloser) HTTPDoer {
	return doerFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Body: body, Request: request}, nil
	})
}
