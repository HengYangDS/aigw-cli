package domain

import (
	"context"
	"strings"
	"testing"
)

func TestProbeRequestRejectsUnknownOrAmbiguousProtocol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		client    string
		protocols []EndpointProtocol
		want      string
	}{
		{name: "unknown client", client: "unknown", want: `unsupported credential validation client "unknown"`},
		{name: "ambiguous Hermes protocol", client: ClientHermes, want: `requires an admitted endpoint protocol`},
		{name: "unadmitted Claude protocol", client: ClientClaude, protocols: []EndpointProtocol{ProtocolResponses}, want: `requires an admitted endpoint protocol`},
		{name: "multiple protocols", client: ClientCodex, protocols: []EndpointProtocol{ProtocolResponses, ProtocolAnthropic}, want: `requires one protocol`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := ProbeRequest(context.Background(), testAdmission(), test.client, "https://gateway.example", "secret", test.protocols...); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestModelCatalogRequestKeepsProtocolAuthenticationDistinct(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    string
		protocol    EndpointProtocol
		wantURL     string
		wantBearer  string
		wantAPIKey  string
		wantVersion string
	}{
		{
			name:       "OpenAI Chat Completions",
			endpoint:   "https://chat.test/v1",
			protocol:   ProtocolChat,
			wantURL:    "https://chat.test/v1/models",
			wantBearer: "Bearer token",
		},
		{
			name:       "OpenAI Responses",
			endpoint:   "https://responses.test/v1",
			protocol:   ProtocolResponses,
			wantURL:    "https://responses.test/v1/models",
			wantBearer: "Bearer token",
		},
		{
			name:        "Anthropic Messages",
			endpoint:    "https://anthropic.test",
			protocol:    ProtocolAnthropic,
			wantURL:     "https://anthropic.test/v1/models",
			wantAPIKey:  "token",
			wantVersion: "2023-06-01",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := ModelCatalogRequest(context.Background(), test.endpoint, test.protocol, "token")
			if err != nil {
				t.Fatal(err)
			}
			if request.URL.String() != test.wantURL || request.Header.Get("Authorization") != test.wantBearer ||
				request.Header.Get("X-Api-Key") != test.wantAPIKey || request.Header.Get("Anthropic-Version") != test.wantVersion {
				t.Fatalf("request = %#v headers=%#v", request.URL, request.Header)
			}
		})
	}
}

func TestModelsEndpointKeepsAnExplicitModelsPath(t *testing.T) {
	t.Parallel()

	if got := modelsEndpoint("https://example.test/models", ProtocolResponses); got != "https://example.test/models" {
		t.Fatalf("models endpoint = %q", got)
	}
}

// TestCatalogRequestKeyNamesOneRequest shows that Responses and Chat
// Completions on one URL send one catalog request, which an Anthropic
// endpoint on that URL does not share, because it authenticates otherwise.
func TestCatalogRequestKeyNamesOneRequest(t *testing.T) {
	t.Parallel()

	responses := CatalogRequestKey("https://gateway.test/v1/", ProtocolResponses)
	if chat := CatalogRequestKey("https://gateway.test/v1", ProtocolChat); chat != responses {
		t.Fatalf("one URL gives two keys: %q, %q", responses, chat)
	}
	for _, other := range []string{
		CatalogRequestKey("https://gateway.test/v1", ProtocolAnthropic),
		CatalogRequestKey("https://other.test/v1", ProtocolResponses),
	} {
		if other == responses {
			t.Fatalf("%q names the Responses request", other)
		}
	}
}
