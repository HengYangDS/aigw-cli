package credential

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	configuration "aigw-cli/internal/configuration"
)

// ModelInferenceRequest constructs one capped, non-streaming request carrying
// the Route's exact upstream model. It does not read credentials or responses.
func ModelInferenceRequest(ctx context.Context, endpoint string, protocol configuration.EndpointProtocol, token, model string) (*http.Request, error) {
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("inference probe requires an upstream model")
	}

	base := strings.TrimRight(endpoint, "/")
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("inference probe requires an absolute endpoint URL")
	}

	var path string
	var body any
	switch protocol {
	case configuration.ProtocolAnthropic:
		path = "/v1/messages"
		if strings.HasSuffix(base, "/v1") {
			path = "/messages"
		}
		body = struct {
			Model     string            `json:"model"`
			MaxTokens int               `json:"max_tokens"`
			Messages  []inferencePrompt `json:"messages"`
		}{Model: model, MaxTokens: 1, Messages: []inferencePrompt{{Role: "user", Content: "ping"}}}
	case configuration.ProtocolOpenAIResponses:
		path = "/responses"
		body = struct {
			Model           string `json:"model"`
			Input           string `json:"input"`
			MaxOutputTokens int    `json:"max_output_tokens"`
			Store           bool   `json:"store"`
		}{Model: model, Input: "ping", MaxOutputTokens: 16, Store: false}
	case configuration.ProtocolOpenAIChatCompletions:
		path = "/chat/completions"
		body = struct {
			Model     string            `json:"model"`
			MaxTokens int               `json:"max_tokens"`
			Messages  []inferencePrompt `json:"messages"`
		}{Model: model, MaxTokens: 1, Messages: []inferencePrompt{{Role: "user", Content: "ping"}}}
	default:
		return nil, fmt.Errorf("unsupported inference protocol %q", protocol)
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode inference probe: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	authenticate(request, protocol, token)
	return request, nil
}

type inferencePrompt struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
