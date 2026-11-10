package check

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	domainreadiness "aigw-cli/internal/app/readiness"
)

// Each answer that a provider gives to the model request names the fix that
// acts on its cause: the provider's console for a Token that it disabled,
// restricted or ran out of quota on, the provider's endpoint for a path that
// it does not serve, another model for one that it refuses, and the provider
// for an answer that AIGW does not expect. Only a provider failure, here an
// HTTP 503 for a model, names the same check again.
func TestCheckNamesTheFixOfEachProviderAnswer(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
		state  domainreadiness.State
		action string
		detail string
	}{
		{name: "disabled Token", status: http.StatusForbidden, body: `{"error":{"message":"this token is disabled"}}`, state: domainreadiness.Invalid,
			detail: "Enable the Token in provider one's console."},
		{name: "restricted Token", status: http.StatusForbidden, body: `{"error":{"message":"not allowed"}}`, state: domainreadiness.Invalid,
			detail: "Allow the Token to use one/gpt-test in provider one's console."},
		{name: "exhausted quota", status: http.StatusForbidden, body: `{"error":{"message":"insufficient quota"}}`, state: domainreadiness.Degraded,
			detail: "Add quota to the Token in provider one's console."},
		{name: "missing path", status: http.StatusNotFound, body: `{}`, state: domainreadiness.Invalid,
			detail: "aigw provider edit one --endpoint responses=<url>"},
		{name: "model the provider cannot serve", status: http.StatusServiceUnavailable, body: `{"error":{"message":"no channel for this model"}}`,
			state: domainreadiness.Degraded, action: "aigw check codex"},
		{name: "model the provider refuses now", status: http.StatusBadRequest, body: `{"error":{"message":"model gpt-test cannot be served at the moment"}}`,
			state: domainreadiness.Degraded, detail: "aigw use codex <provider/model>"},
		{name: "unexpected answer", status: http.StatusTeapot, body: `{}`, state: domainreadiness.Unavailable,
			action: "aigw provider show one"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, cfg, out := configuredRuntime(t)
			enable(t, runtime, &cfg, codex.ID)
			runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method == http.MethodGet {
					return answer(request)
				}
				return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body)), Request: request}, nil
			})
			if err := execute(runtime, "--json"); err == nil {
				t.Fatalf("HTTP %d passed", test.status)
			}
			client := decode(t, out).client(t, codex.ID)
			if client.Failure == nil || client.Failure.Depth != depthModel || client.Failure.HTTPStatus != test.status || client.State != test.state {
				t.Fatalf("HTTP %d = %+v", test.status, client)
			}
			if client.NextAction != test.action || client.NextDetail != test.detail {
				t.Fatalf("HTTP %d fix = %q, %q; want %q, %q", test.status, client.NextAction, client.NextDetail, test.action, test.detail)
			}
		})
	}
}
