package onboarding

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
)

func TestVerifyCredentialRequestAndResponseErrors(t *testing.T) {
	account := configuration.Account{ID: "one", Endpoints: configuration.Endpoints{OpenAIResponses: "https://one.test/v1", Anthropic: "https://one.test"}}

	t.Run("unknown client", func(t *testing.T) {
		err := credential.Validate(context.Background(), nil, account, "token", "other")
		if err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing endpoint", func(t *testing.T) {
		err := credential.Validate(context.Background(), nil, configuration.Account{ID: "one", Endpoints: configuration.Endpoints{Anthropic: "https://one.test"}}, "token", configuration.ClientCodex)
		if err == nil || !strings.Contains(err.Error(), "no OpenAI") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("invalid URL", func(t *testing.T) {
		bad := account
		bad.Endpoints.OpenAIResponses = "://bad"
		if err := credential.Validate(context.Background(), nil, bad, "token", configuration.ClientCodex); err == nil {
			t.Fatal("expected request construction failure")
		}
	})

	t.Run("network", func(t *testing.T) {
		want := errors.New("network failed")
		app := invocation.Context{HTTP: setupHTTPClient(func(*http.Request) (*http.Response, error) { return nil, want })}
		if err := credential.Validate(context.Background(), app.HTTP, account, "token", configuration.ClientCodex); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("body read", func(t *testing.T) {
		want := errors.New("read failed")
		app := invocation.Context{HTTP: setupHTTPClient(func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: setupResponseBody{Reader: errorReader{err: want}}, Request: request}, nil
		})}
		if err := credential.Validate(context.Background(), app.HTTP, account, "token", configuration.ClientCodex); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("body close", func(t *testing.T) {
		want := errors.New("close failed")
		app := invocation.Context{HTTP: setupHTTPClient(func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: setupResponseBody{Reader: strings.NewReader("ok"), closeErr: want}, Request: request}, nil
		})}
		if err := credential.Validate(context.Background(), app.HTTP, account, "token", configuration.ClientCodex); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("duplicate client", func(t *testing.T) {
		calls := 0
		app := invocation.Context{HTTP: setupHTTPClient(func(request *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Request: request}, nil
		})}
		if err := credential.Validate(context.Background(), app.HTTP, account, "token", configuration.ClientCodex, configuration.ClientCodex); err != nil || calls != 1 {
			t.Fatalf("calls=%d error=%v", calls, err)
		}
	})
}
