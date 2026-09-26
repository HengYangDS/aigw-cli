package onboarding

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
)

func TestSetupValidatesSelectedHermesProtocol(t *testing.T) {
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	credentials := secrets.NewMemoryStore()
	if err := credentials.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	requests := 0
	runtime := invocation.Context{
		Config: store, Secrets: credentials, Discovery: setupDiscovery{result: discovery.Result{}},
		HTTP: setupHTTPClient(func(request *http.Request) (*http.Response, error) {
			requests++
			if request.URL.Host != "messages.test" || request.Header.Get("X-Api-Key") != "token" || request.Header.Get("Authorization") != "" {
				t.Fatal("setup Token validation did not use the selected Anthropic endpoint")
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Request: request}, nil
		}),
		Out: io.Discard, RenderOut: io.Discard,
	}
	request := Request{
		Account: "team", Route: "shared", Client: configuration.ClientHermes, Model: "shared-model",
		OpenAIURL: "https://responses.test/v1", AnthropicURL: "https://messages.test",
		Protocol: string(configuration.ProtocolAnthropic),
	}
	if err := runSetup(context.Background(), runtime, request); err != nil || requests != 1 {
		t.Fatalf("selected-protocol setup validation: requests=%d error=%v", requests, err)
	}
}
