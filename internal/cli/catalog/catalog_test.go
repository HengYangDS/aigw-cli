package catalog

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
)

type recordingClient struct{ path string }

func (client *recordingClient) Do(request *http.Request) (*http.Response, error) {
	client.path = request.URL.Path
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"zeta"},{"id":"alpha"},{"id":"alpha"}]}`)),
		Request:    request,
	}, nil
}

func TestFetchIDsOwnsCatalogProtocol(t *testing.T) {
	client := &recordingClient{}
	ids, err := FetchIDs(context.Background(), client, configuration.Account{
		Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"},
	}, "token")
	if err != nil {
		t.Fatal(err)
	}
	if client.path != "/v1/models" {
		t.Fatalf("path = %q, want /v1/models", client.path)
	}
	if got := strings.Join(ids, ","); got != "alpha,zeta" {
		t.Fatalf("IDs = %q, want alpha,zeta", got)
	}
}

func TestCommandConstructorsExposeStableNames(t *testing.T) {
	deps := Dependencies{}
	if got := NewModelsCommand(deps).Name(); got != "models" {
		t.Fatalf("models command = %q", got)
	}
	if got := NewCatalogCommand(deps).Name(); got != "catalog" {
		t.Fatalf("catalog command = %q", got)
	}
}
