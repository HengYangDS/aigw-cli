package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newNativeJourneyServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.Method + " " + request.URL.Path {
		case "POST /v1/messages":
			_, _ = response.Write([]byte(`{"type":"message","role":"assistant","content":[{"type":"text","text":"pong"}],"stop_reason":"end_turn"}`))
		case "POST /v1/responses":
			_, _ = response.Write([]byte(`{"status":"completed","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"pong"}]}]}`))
		default:
			_, _ = response.Write([]byte(`{"data":[]}`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}
