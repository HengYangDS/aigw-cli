package probe_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/claudedesktop"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/client/hermes"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret/probe"
)

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

	account := domain.Account{
		ID: "team",
		Endpoints: domain.Endpoints{
			Anthropic:       "https://anthropic.example/api",
			OpenAIResponses: "https://responses.example/v1/",
		},
	}
	requests := make([]*http.Request, 0, 2)
	doer := doerFunc(func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request.Clone(request.Context()))
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})

	if err := probe.Validate(context.Background(), doer, clients.Admission(), account, "secret", claude.ID, codex.ID, codex.ID); err != nil {
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
	if err := probe.Validate(context.Background(), doer, clients.Admission(), account, "secret"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0].URL.Host != "responses.example" {
		t.Fatalf("default requests = %#v", requests)
	}

	requests = requests[:0]
	account.Endpoints.OpenAIResponses = ""
	if err := probe.Validate(context.Background(), doer, clients.Admission(), account, "secret"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0].URL.Host != "anthropic.example" {
		t.Fatalf("Anthropic default requests = %#v", requests)
	}
}

// TestValidateNamesTheDefaultProbeClient shows that a default probe's error
// names the first admitted client that speaks only the account's protocol, and
// that an admission without one refuses instead of probing.
func TestValidateNamesTheDefaultProbeClient(t *testing.T) {
	t.Parallel()

	anthropic := domain.Endpoints{Anthropic: "https://anthropic.test"}
	for name, test := range map[string]struct {
		endpoints domain.Endpoints
		admission domain.Admission
		want      string
	}{
		"Anthropic Messages": {endpoints: anthropic, admission: clients.Admission(), want: "Claude authentication was rejected (HTTP 401)"},
		"OpenAI Responses": {
			endpoints: domain.Endpoints{Anthropic: anthropic.Anthropic, OpenAIResponses: "https://responses.test/v1"},
			admission: clients.Admission(), want: "Codex authentication was rejected (HTTP 401)",
		},
		"no probing client": {endpoints: anthropic, admission: domain.NewAdmission(), want: "no admitted client probes an Account Token over Anthropic Messages"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			account := domain.Account{ID: "team", Endpoints: test.endpoints}
			err := probe.Validate(context.Background(), staticResponse(http.StatusUnauthorized, http.NoBody), test.admission, account, "secret")
			if err == nil || err.Error() != test.want {
				t.Fatalf("Validate() = %v, want %q", err, test.want)
			}
		})
	}
}

func TestValidateRuntimeUsesTheSelectedEndpointProtocol(t *testing.T) {
	t.Parallel()

	selected := domain.Runtime{
		Client:   hermes.ID,
		Endpoint: "https://chat.example/v1/",
		Protocol: domain.ProtocolOpenAIChatCompletions,
	}
	called := false
	doer := doerFunc(func(request *http.Request) (*http.Response, error) {
		called = true
		if request.Method != http.MethodGet || request.URL.String() != "https://chat.example/v1/models" ||
			request.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("selected Chat Completions probe = %s %s, bearer_matches=%t", request.Method, request.URL, request.Header.Get("Authorization") == "Bearer token")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})
	if err := probe.ValidateRuntime(context.Background(), doer, clients.Admission(), selected, "token"); err != nil || !called {
		t.Fatalf("selected runtime validation: called=%t error=%v", called, err)
	}
}

func TestValidateRejectsInvalidRequestsAndResponses(t *testing.T) {
	t.Parallel()

	account := domain.Account{
		ID: "team",
		Endpoints: domain.Endpoints{
			OpenAIResponses: "https://responses.example/v1",
		},
	}
	tests := []struct {
		name    string
		account domain.Account
		client  string
		doer    probe.HTTPDoer
		want    string
	}{
		{name: "unknown client", account: account, client: "unknown", want: `unsupported credential validation client "unknown"`},
		{name: "missing endpoint", account: account, client: claude.ID, want: "no Anthropic Messages endpoint"},
		{name: "invalid URL", account: domain.Account{ID: "team", Endpoints: domain.Endpoints{OpenAIResponses: "://bad"}}, client: codex.ID, want: "missing protocol scheme"},
		{name: "unreachable", account: account, client: codex.ID, doer: doerFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") }), want: "Codex endpoint is unreachable: offline"},
		{name: "read body", account: account, client: codex.ID, doer: staticResponse(http.StatusOK, responseBody{Reader: failingReader{err: errors.New("broken body")}}), want: "read Codex endpoint response: broken body"},
		{name: "close body", account: account, client: codex.ID, doer: staticResponse(http.StatusOK, responseBody{Reader: strings.NewReader("ok"), closeErr: errors.New("broken close")}), want: "close Codex endpoint response: broken close"},
		{name: "unauthorized", account: account, client: codex.ID, doer: staticResponse(http.StatusUnauthorized, http.NoBody), want: "Codex authentication was rejected (HTTP 401)"},
		{name: "forbidden", account: account, client: codex.ID, doer: staticResponse(http.StatusForbidden, http.NoBody), want: "Codex authentication was rejected (HTTP 403)"},
		{name: "unexpected status", account: account, client: codex.ID, doer: staticResponse(http.StatusBadGateway, http.NoBody), want: "Codex endpoint returned HTTP 502"},
		{name: "multi-word client label", account: domain.Account{ID: "team", Endpoints: domain.Endpoints{Anthropic: "https://anthropic.test"}}, client: claudedesktop.ID, doer: staticResponse(http.StatusUnauthorized, http.NoBody), want: "Claude Desktop authentication was rejected (HTTP 401)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := probe.Validate(context.Background(), test.doer, clients.Admission(), test.account, "secret", test.client)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
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
	account := domain.Account{ID: "team", Endpoints: domain.Endpoints{OpenAIResponses: server.URL + "/v1"}}

	httpClient := server.Client()
	err := probe.Validate(context.Background(), httpClient, clients.Admission(), account, "secret", codex.ID)
	if err == nil || !strings.Contains(err.Error(), "HTTP 302") {
		t.Fatalf("error = %v, want HTTP 302", err)
	}
	if followed {
		t.Fatal("validation followed a redirect")
	}
	if httpClient.CheckRedirect != nil {
		t.Fatal("validation changed its caller's HTTP client")
	}
}

func TestSendCallsAnInjectedDoer(t *testing.T) {
	t.Parallel()

	request, err := http.NewRequest(http.MethodGet, "https://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	doer := doerFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, nil
	})
	response, err := probe.Send(doer, request)
	if response != nil {
		_ = response.Body.Close()
	}
	if err != nil || !called {
		t.Fatalf("non-HTTP doer call: called=%v error=%v", called, err)
	}
}

func staticResponse(status int, body io.ReadCloser) probe.HTTPDoer {
	return doerFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Body: body, Request: request}, nil
	})
}
