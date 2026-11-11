// Package diagnostics owns bounded, redacted endpoint and authentication
// health observations. It classifies what one request observed and leaves the
// fix to the command that sent it, which knows how to run it again. It does
// not change the configuration, a credential or a client.
package diagnostics

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret/probe"
)

// Kind classifies a provider endpoint diagnostic outcome.
type Kind string

// Scope states what the authenticated request actually observes.
type Scope string

const (
	// ScopeEndpoint authenticates with a model-free endpoint request.
	ScopeEndpoint Scope = "endpoint"
	// ScopeInference sends a request carrying the selected model's own ID.
	ScopeInference Scope = "inference"
)

const (
	// Healthy identifies a successful authenticated provider probe.
	Healthy Kind = "healthy"
	// InvalidToken identifies credentials rejected as invalid.
	InvalidToken Kind = "invalid_token"
	// QuotaExhausted identifies an otherwise valid Token without available quota.
	QuotaExhausted Kind = "quota_exhausted"
	// TokenDisabled identifies a credential disabled by its provider.
	TokenDisabled Kind = "token_disabled"
	// TokenRestricted identifies a credential that lacks permission for the requested diagnostic.
	TokenRestricted Kind = "token_restricted"
	// RateLimited identifies provider throttling that may succeed later.
	RateLimited Kind = "rate_limited"
	// Timeout identifies a request that the endpoint gave up waiting for,
	// with HTTP 408, which may succeed later.
	Timeout Kind = "timeout"
	// ModelUnavailable identifies a configured model that the provider cannot currently serve.
	ModelUnavailable Kind = "model_unavailable"
	// ModelUnresolved identifies a model without the provider's own ID for inference.
	ModelUnresolved Kind = "model_unresolved"
	// UpstreamFailure identifies a failure returned by the selected endpoint.
	UpstreamFailure Kind = "upstream_failure"
	// EndpointMismatch identifies a configured URL that does not expose the expected protocol path.
	EndpointMismatch Kind = "endpoint_mismatch"
	// NetworkFailure identifies a transport failure before a valid provider response.
	NetworkFailure Kind = "network_failure"
	// Unexpected identifies a response outside the admitted diagnostic classifications.
	Unexpected Kind = "unexpected"
)

// HTTPDoer is the minimal transport required by a provider diagnostic probe.
type HTTPDoer interface {
	Do(request *http.Request) (*http.Response, error)
}

// Result is the stable, non-secret diagnostic classification returned to
// commands and JSON consumers: what kind of outcome one request had, a
// summary of it in the product's words, and the HTTP status that it
// received, if any.
type Result struct {
	Kind       Kind   `json:"kind"`
	Scope      Scope  `json:"scope,omitempty"`
	Summary    string `json:"summary"`
	HTTPStatus int    `json:"http_status,omitempty"`
	Retryable  bool   `json:"retryable"`
	Attempts   int    `json:"attempts,omitempty"`
}

const (
	endpointTimeout  = 5 * time.Second
	inferenceTimeout = 60 * time.Second
)

// ProbeBounded makes one authenticated request within the selected scope's deadline.
func ProbeBounded(ctx context.Context, client HTTPDoer, runtime domain.Runtime, token string, scope Scope) Result {
	timeout := endpointTimeout
	if scope == ScopeInference {
		timeout = inferenceTimeout
	}
	attemptCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return Probe(attemptCtx, client, runtime, token, scope)
}

// Probe performs one authenticated diagnostic request using the caller's context.
func Probe(ctx context.Context, client HTTPDoer, runtime domain.Runtime, token string, scope Scope) Result {
	if scope != ScopeEndpoint && scope != ScopeInference {
		return Result{Kind: Unexpected, Scope: scope, Summary: "the request has no scope that AIGW can send"}
	}
	if strings.TrimSpace(runtime.Endpoint) == "" {
		return Result{Kind: EndpointMismatch, Scope: scope, Summary: "the provider has no endpoint for this protocol"}
	}
	var req *http.Request
	var err error
	if scope == ScopeInference {
		if strings.TrimSpace(runtime.UpstreamModel) == "" {
			return Result{Kind: ModelUnresolved, Scope: scope, Summary: "the model has no provider model ID to send"}
		}
		req, err = domain.ModelInferenceRequest(ctx, runtime.Endpoint, runtime.Protocol, token, runtime.UpstreamModel)
	} else {
		req, err = domain.ModelCatalogRequest(ctx, runtime.Endpoint, runtime.Protocol, token)
	}
	if err != nil {
		return Result{Kind: EndpointMismatch, Scope: scope, Summary: "the endpoint is not a URL that a request can go to"}
	}
	resp, err := probe.Send(client, req)
	if err != nil {
		return Result{Kind: NetworkFailure, Scope: scope, Summary: "the endpoint did not answer", Retryable: true, Attempts: 1}
	}
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if readErr != nil {
		return Result{
			Kind:       NetworkFailure,
			Scope:      scope,
			Summary:    "the endpoint's answer could not be read",
			HTTPStatus: resp.StatusCode,
			Retryable:  true,
			Attempts:   1,
		}
	}
	return classifyResponse(Result{HTTPStatus: resp.StatusCode, Scope: scope, Attempts: 1}, runtime.Protocol, body)
}

// classifyResponse classifies the HTTP answer of one request: a success by
// whether it carries what its scope asks for, and a failure by its status
// and the provider's own message, which no output shows.
func classifyResponse(result Result, protocol domain.EndpointProtocol, body []byte) Result {
	status := result.HTTPStatus
	lower := strings.ToLower(providerErrorMessage(body))
	switch {
	case status >= http.StatusOK && status < http.StatusMultipleChoices:
		result = classifySuccessfulResponse(result, protocol, body)
	case status == http.StatusUnauthorized:
		result.Kind, result.Summary = InvalidToken, "the provider rejected the Token"
	case isHardQuotaFailure(status, lower):
		result.Kind, result.Summary = QuotaExhausted, "the Token's quota is exhausted"
	case status == http.StatusForbidden && containsAny(lower, "disabled", "disable"):
		result.Kind, result.Summary = TokenDisabled, "the provider disabled the Token"
	case status == http.StatusForbidden:
		result.Kind, result.Summary = TokenRestricted, "the provider restricted what the Token may do"
	case status == http.StatusRequestTimeout:
		result.Kind, result.Summary, result.Retryable = Timeout, "the endpoint gave up waiting for the request", true
	case status == http.StatusTooManyRequests:
		result.Kind, result.Summary, result.Retryable = RateLimited, "the provider limited the request rate or concurrency", true
	case status == http.StatusNotFound:
		result.Kind, result.Summary = EndpointMismatch, "the endpoint has no such path for this protocol"
	case status == http.StatusServiceUnavailable && containsAny(lower, "model", "channel", "无可用渠道") ||
		status == http.StatusBadRequest && strings.Contains(lower, "model") && strings.Contains(lower, "cannot be served at the moment"):
		result.Kind, result.Summary, result.Retryable = ModelUnavailable, "the provider cannot serve the model right now", true
	case status >= 500:
		result.Kind, result.Summary, result.Retryable = UpstreamFailure, "the provider failed", true
	default:
		result.Kind, result.Summary = Unexpected, "the endpoint gave an answer that AIGW does not expect"
	}
	return result
}

func providerErrorMessage(body []byte) string {
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil {
		var plain string
		if json.Unmarshal(body, &plain) == nil {
			return plain
		}
		if json.Valid(body) {
			return ""
		}
		return string(body)
	}
	values := make([]string, 0, 7)
	collect := func(source map[string]json.RawMessage) {
		for _, key := range []string{"message", "code", "detail"} {
			var value string
			if json.Unmarshal(source[key], &value) == nil && value != "" {
				values = append(values, value)
			}
		}
	}
	collect(fields)
	var nested map[string]json.RawMessage
	if json.Unmarshal(fields["error"], &nested) == nil {
		collect(nested)
	}
	var plain string
	if json.Unmarshal(fields["error"], &plain) == nil && plain != "" {
		values = append(values, plain)
	}
	return strings.Join(values, " ")
}

func isHardQuotaFailure(status int, message string) bool {
	if status == http.StatusForbidden {
		return containsAny(message, "quota", "balance", "insufficient", "exhaust")
	}
	return status == http.StatusTooManyRequests && containsAny(message,
		"insufficient_quota", "quota_exhausted", "insufficient balance", "balance exhausted", "credits exhausted")
}

func classifySuccessfulResponse(result Result, protocol domain.EndpointProtocol, body []byte) Result {
	if result.Scope == ScopeInference {
		if !hasInferenceOutput(protocol, body) {
			result.Kind, result.Summary = Unexpected, "the model's answer held no text"
			return result
		}
		result.Kind, result.Summary = Healthy, "the model answered"
		return result
	}
	result.Kind, result.Summary = Healthy, "the provider accepted the Token"
	return result
}

func hasInferenceOutput(protocol domain.EndpointProtocol, body []byte) bool {
	var payload struct {
		Type    string `json:"type"`
		Role    string `json:"role"`
		Status  string `json:"status"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Output []struct {
			Type    string `json:"type"`
			Role    string `json:"role"`
			Status  string `json:"status"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return false
	}
	switch protocol {
	case domain.ProtocolAnthropic:
		if payload.Type != "message" || payload.Role != "assistant" {
			return false
		}
		for _, content := range payload.Content {
			if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
				return true
			}
		}
	case domain.ProtocolResponses:
		if payload.Status != "completed" {
			return false
		}
		for _, output := range payload.Output {
			if output.Type != "message" || output.Role != "assistant" || (output.Status != "" && output.Status != "completed") {
				continue
			}
			for _, content := range output.Content {
				if content.Type == "output_text" && strings.TrimSpace(content.Text) != "" {
					return true
				}
			}
		}
	case domain.ProtocolChat:
		for _, choice := range payload.Choices {
			if choice.Message.Role == "assistant" && strings.TrimSpace(choice.Message.Content) != "" {
				return true
			}
		}
	}
	return false
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
