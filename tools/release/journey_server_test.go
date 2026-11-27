package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newNativeJourneyServer(t *testing.T, observe ...func(*http.Request)) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		for _, observer := range observe {
			observer(request)
		}
		response.Header().Set("Content-Type", "application/json")
		switch request.Method + " " + request.URL.Path {
		case "POST /v1/messages":
			_, _ = response.Write([]byte(`{"type":"message","role":"assistant","content":[{"type":"text","text":"pong"}],"stop_reason":"end_turn"}`))
		case "POST /v1/responses":
			_, _ = response.Write([]byte(`{"status":"completed","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"pong"}]}]}`))
		case "POST /v1/chat/completions":
			_, _ = response.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}]}`))
		default:
			_, _ = response.Write([]byte(`{"data":[]}`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// newForwardingService is a forwarding service in front of upstream, as
// openai-responses-proxy 8.0.0 names an upstream: it accepts
// `<root>/http/<upstream's authority>/<path>`, gives observe each request
// with the path that it relays, and relays the request to upstream at
// `/<path>`. Any other path is not found.
func newForwardingService(t *testing.T, upstream *httptest.Server, observe func(request *http.Request, relayed string)) *httptest.Server {
	t.Helper()
	prefix := "/http/" + strings.TrimPrefix(upstream.URL, "http://")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		relayed, named := strings.CutPrefix(request.URL.Path, prefix)
		if !named || !strings.HasPrefix(relayed, "/") {
			http.NotFound(response, request)
			return
		}
		if observe != nil {
			observe(request, relayed)
		}
		forwarded := request.Clone(request.Context())
		forwarded.URL.Path, forwarded.URL.RawPath = relayed, ""
		upstream.Config.Handler.ServeHTTP(response, forwarded)
	}))
	t.Cleanup(server.Close)
	return server
}
