// Package diagnostics owns bounded, redacted endpoint and authentication
// health observations. It does not mutate routes, credentials, or clients.
package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/redaction"
)

// Kind classifies a provider endpoint diagnostic outcome.
type Kind string

// Scope states what the authenticated request actually observes.
type Scope string

const (
	// ScopeEndpoint authenticates with a model-free endpoint request.
	ScopeEndpoint Scope = "endpoint"
	// ScopeInference sends a request carrying the selected Route's wire model.
	ScopeInference Scope = "inference"
)

const (
	// Healthy identifies a successful authenticated provider probe.
	Healthy Kind = "healthy"
	// InvalidToken identifies credentials rejected as invalid.
	InvalidToken Kind = "invalid_token"
	// QuotaExhausted identifies an otherwise valid account without available quota.
	QuotaExhausted Kind = "quota_exhausted"
	// TokenDisabled identifies a credential disabled by its provider.
	TokenDisabled Kind = "token_disabled"
	// TokenRestricted identifies a credential that lacks permission for the requested diagnostic.
	TokenRestricted Kind = "token_restricted"
	// RateLimited identifies provider throttling that may succeed later.
	RateLimited Kind = "rate_limited"
	// ModelUnavailable identifies a configured model that the provider cannot currently serve.
	ModelUnavailable Kind = "model_unavailable"
	// ModelUnresolved identifies a Route without an exact upstream model for inference.
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

// Result is the stable, non-secret diagnostic classification returned to commands and JSON consumers.
type Result struct {
	Kind       Kind   `json:"kind"`
	Scope      Scope  `json:"scope,omitempty"`
	Summary    string `json:"summary"`
	Detail     string `json:"detail,omitempty"`
	Fix        string `json:"fix,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty"`
	Retryable  bool   `json:"retryable"`
	Attempts   int    `json:"attempts,omitempty"`
}

const (
	endpointTimeout  = 5 * time.Second
	inferenceTimeout = 60 * time.Second
)

// ProbeBounded makes one authenticated request within the selected scope's deadline.
func ProbeBounded(ctx context.Context, client HTTPDoer, runtime configuration.Runtime, token string, scope Scope) Result {
	if scope != ScopeEndpoint && scope != ScopeInference {
		return Probe(ctx, client, runtime, token, scope)
	}
	timeout := endpointTimeout
	if scope == ScopeInference {
		timeout = inferenceTimeout
	}
	attemptCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return Probe(attemptCtx, client, runtime, token, scope)
}

// Probe performs one authenticated diagnostic request using the caller's context.
func Probe(ctx context.Context, client HTTPDoer, runtime configuration.Runtime, token string, scope Scope) Result {
	if scope != ScopeEndpoint && scope != ScopeInference {
		return Result{Kind: Unexpected, Scope: scope, Summary: "Unsupported diagnostic scope", Fix: "Run `aigw doctor` for detailed status"}
	}
	if strings.TrimSpace(runtime.Endpoint) == "" {
		return Result{Kind: EndpointMismatch, Scope: scope, Summary: "Invalid API URL", Fix: "Check the protocol endpoint for the current route's account"}
	}
	var req *http.Request
	var err error
	if scope == ScopeInference {
		if strings.TrimSpace(runtime.Model) == "" {
			return Result{Kind: ModelUnresolved, Scope: scope, Summary: "Route has no upstream model for inference", Fix: "Check the selected Route's upstream model"}
		}
		req, err = credential.ModelInferenceRequest(ctx, runtime.Endpoint, runtime.Protocol, token, runtime.Model)
	} else {
		req, err = credential.ProbeRequest(ctx, runtime.Client, runtime.Endpoint, token, runtime.Protocol)
	}
	if err != nil {
		return Result{Kind: EndpointMismatch, Scope: scope, Summary: "Cannot construct the diagnostic request", Detail: err.Error(), Fix: "Check the endpoint and protocol for the active route"}
	}
	resp, err := credential.DoProbe(client, req)
	if err != nil {
		return Result{Kind: NetworkFailure, Scope: scope, Summary: "Cannot reach the endpoint", Detail: redaction.Text(err.Error(), token), Fix: "Check the configured endpoint and network, then try again", Retryable: true, Attempts: 1}
	}
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if readErr != nil {
		return Result{
			Kind:       NetworkFailure,
			Scope:      scope,
			Summary:    "Cannot read the endpoint response",
			Detail:     redaction.Text(readErr.Error(), token),
			Fix:        "Check the configured endpoint and network, then try again",
			HTTPStatus: resp.StatusCode,
			Retryable:  true,
			Attempts:   1,
		}
	}
	message := strings.TrimSpace(string(body))
	lower := strings.ToLower(providerErrorMessage(body))
	result := Result{HTTPStatus: resp.StatusCode, Scope: scope, Detail: compact(message, token), Attempts: 1}
	switch {
	case resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices:
		result = classifySuccessfulResponse(result, runtime.Protocol, body)
	case resp.StatusCode == http.StatusUnauthorized:
		result.Kind, result.Summary = InvalidToken, "Account Token is invalid or does not belong to the configured endpoint"
		result.Fix = "Run `aigw rotate " + runtime.AccountID + "` to enter the token again, and confirm that the configured endpoint belongs to this Account"
	case resp.StatusCode == http.StatusForbidden && containsAny(lower, "quota", "balance", "insufficient", "exhaust"):
		result.Kind, result.Summary = QuotaExhausted, "Token quota is exhausted"
		result.Fix = "Increase the Token quota for Account " + runtime.AccountID + " in the provider console"
	case resp.StatusCode == http.StatusForbidden && containsAny(lower, "disabled", "disable"):
		result.Kind, result.Summary = TokenDisabled, "Token is disabled"
		result.Fix = "Enable the Token for Account " + runtime.AccountID + " in the provider console"
	case resp.StatusCode == http.StatusForbidden:
		result.Kind, result.Summary = TokenRestricted, "Token or Account is restricted"
		result.Fix = "Review access restrictions for Account " + runtime.AccountID + " in the provider console"
	case resp.StatusCode == http.StatusTooManyRequests:
		result.Kind, result.Summary, result.Retryable = RateLimited, "Request rate or concurrency quota is exhausted", true
		result.Fix = "Reduce concurrency and try again later; if it persists, check the provider rate-limit policy"
	case resp.StatusCode == http.StatusNotFound:
		result.Kind, result.Summary = EndpointMismatch, "API URL or path does not match"
		result.Fix = "Check whether the configured protocol endpoint requires /v1 and whether its host is correct"
	case resp.StatusCode == http.StatusServiceUnavailable && containsAny(lower, "model", "channel", "无可用渠道"):
		result.Kind, result.Summary, result.Retryable = ModelUnavailable, "Current model or channel is unavailable", true
		result.Fix = "Confirm the model name and token model restrictions, or try again later"
	case resp.StatusCode >= 500:
		result.Kind, result.Summary, result.Retryable = UpstreamFailure, "Endpoint provider failed", true
		result.Fix = "Try again later; if it persists, contact the endpoint operator with the HTTP status code"
	default:
		result.Kind, result.Summary = Unexpected, fmt.Sprintf("Endpoint returned unexpected HTTP status %d", resp.StatusCode)
		result.Fix = "Run `aigw doctor` for detailed status"
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

func classifySuccessfulResponse(result Result, protocol configuration.EndpointProtocol, body []byte) Result {
	if result.Scope == ScopeInference {
		if !hasInferenceOutput(protocol, body) {
			result.Kind, result.Summary = Unexpected, "Inference response did not contain usable model output"
			result.Fix = "Check the selected Route's model and protocol, then try again"
			return result
		}
		result.Kind, result.Summary = Healthy, "Inference diagnostic returned a successful response"
		return result
	}
	result.Kind, result.Summary = Healthy, "Endpoint diagnostic returned a successful response"
	return result
}

func hasInferenceOutput(protocol configuration.EndpointProtocol, body []byte) bool {
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
	case configuration.ProtocolAnthropic:
		if payload.Type != "message" || payload.Role != "assistant" {
			return false
		}
		for _, content := range payload.Content {
			if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
				return true
			}
		}
	case configuration.ProtocolOpenAIResponses:
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
	case configuration.ProtocolOpenAIChatCompletions:
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

func compact(value string, secrets ...string) string {
	value = redaction.Text(value, secrets...)
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 500 {
		value = value[:500] + "…"
	}
	return redaction.Text(value, secrets...)
}
