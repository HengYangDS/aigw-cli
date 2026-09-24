package readiness

import (
	"io"
	"net/http"
	"strings"
)

func successfulReadinessResponse(request *http.Request) (*http.Response, error) {
	body := `{"data":[]}`
	switch request.Method + " " + request.URL.Path {
	case "POST /v1/messages":
		body = `{"type":"message","role":"assistant","content":[{"type":"text","text":"pong"}],"stop_reason":"end_turn"}`
	case "POST /v1/responses":
		body = `{"status":"completed","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"pong"}]}]}`
	case "POST /v1/chat/completions":
		body = `{"choices":[{"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}]}`
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
}
