// Package diagnostics owns bounded, redacted endpoint and authentication
// health observations. It does not mutate routes, credentials, or clients.
package diagnostics

import (
	"context"
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

const (
	// Healthy identifies a successful authenticated provider probe.
	Healthy Kind = "healthy"
	// InvalidToken identifies credentials rejected as invalid.
	InvalidToken Kind = "invalid_token"
	// AuthenticationUnstable identifies a transient authentication boundary that did not stabilize.
	AuthenticationUnstable Kind = "authentication_unstable"
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
	// UpstreamFailure identifies a failure returned by the selected service endpoint.
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
	Do(*http.Request) (*http.Response, error)
}

// Result is the stable, non-secret diagnostic classification returned to commands and JSON consumers.
type Result struct {
	Kind               Kind   `json:"kind"`
	Summary            string `json:"summary"`
	Detail             string `json:"detail,omitempty"`
	Fix                string `json:"fix,omitempty"`
	HTTPStatus         int    `json:"http_status,omitempty"`
	Retryable          bool   `json:"retryable"`
	Attempts           int    `json:"attempts,omitempty"`
	RecoveredTransient bool   `json:"recovered_transient,omitempty"`
}

// StabilityPolicy bounds retry delays and each diagnostic attempt.
type StabilityPolicy struct {
	RecoveryDelays []time.Duration
	AttemptTimeout time.Duration
}

// DefaultStabilityPolicy returns the production retry and timeout bounds for a live diagnostic.
func DefaultStabilityPolicy() StabilityPolicy {
	return StabilityPolicy{
		RecoveryDelays: []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, time.Second},
		AttemptTimeout: 5 * time.Second,
	}
}

// ProbeStable retries only admitted transient outcomes and returns the final classified result.
func ProbeStable(ctx context.Context, client HTTPDoer, runtime configuration.Runtime, token string, policy StabilityPolicy) Result {
	result := probeWithTimeout(ctx, client, runtime, token, policy.AttemptTimeout)
	result.Attempts = 1
	if result.Kind != InvalidToken {
		return result
	}

	recovery := make([]Result, 0, len(policy.RecoveryDelays))
	for _, delay := range policy.RecoveryDelays {
		if err := waitForRecovery(ctx, delay); err != nil {
			return unstableAuthentication(result.Attempts, err.Error())
		}
		observation := probeWithTimeout(ctx, client, runtime, token, policy.AttemptTimeout)
		recovery = append(recovery, observation)
		result.Attempts++
	}

	if allKind(recovery, Healthy) {
		recovered := recovery[len(recovery)-1]
		recovered.Attempts = result.Attempts
		recovered.RecoveredTransient = true
		return recovered
	}
	if allKind(recovery, InvalidToken) {
		persistent := recovery[len(recovery)-1]
		persistent.Attempts = result.Attempts
		return persistent
	}
	return unstableAuthentication(result.Attempts, "Authentication responses were inconsistent across bounded recovery attempts")
}

func probeWithTimeout(ctx context.Context, client HTTPDoer, runtime configuration.Runtime, token string, timeout time.Duration) Result {
	if timeout <= 0 {
		return Probe(ctx, client, runtime, token)
	}
	attemptCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return Probe(attemptCtx, client, runtime, token)
}

func waitForRecovery(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func allKind(results []Result, kind Kind) bool {
	if len(results) == 0 {
		return false
	}
	for _, result := range results {
		if result.Kind != kind {
			return false
		}
	}
	return true
}

func unstableAuthentication(attempts int, detail string) Result {
	return Result{
		Kind:      AuthenticationUnstable,
		Summary:   "Authentication could not be confirmed consistently",
		Detail:    detail,
		Fix:       "Run `aigw check` again later",
		Retryable: true,
		Attempts:  attempts,
	}
}

// Probe performs one bounded authenticated diagnostic request against the selected profile endpoint.
func Probe(ctx context.Context, client HTTPDoer, runtime configuration.Runtime, token string) Result {
	if strings.TrimSpace(runtime.Endpoint) == "" {
		return Result{Kind: EndpointMismatch, Summary: "Invalid API URL", Fix: "Check the protocol endpoint for the current profile's account"}
	}
	req, err := credential.ProbeRequest(ctx, runtime.Client, runtime.Endpoint, token)
	if err != nil {
		return Result{Kind: EndpointMismatch, Summary: "Invalid API URL", Detail: err.Error(), Fix: "Check the endpoint for the active profile"}
	}
	resp, err := credential.DoProbe(client, req)
	if err != nil {
		return Result{Kind: NetworkFailure, Summary: "Cannot reach the endpoint", Detail: redaction.Text(err.Error(), token), Fix: "Check the configured endpoint and network, then try again", Retryable: true}
	}
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if readErr != nil {
		return Result{
			Kind:       NetworkFailure,
			Summary:    "Cannot read the service endpoint response",
			Detail:     redaction.Text(readErr.Error(), token),
			Fix:        "Check the configured endpoint and network, then try again",
			HTTPStatus: resp.StatusCode,
			Retryable:  true,
		}
	}
	message := strings.TrimSpace(string(body))
	lower := strings.ToLower(message)
	result := Result{HTTPStatus: resp.StatusCode, Detail: compact(message, token)}
	switch {
	case resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices:
		result.Kind, result.Summary = Healthy, "Endpoint diagnostic returned a successful response"
	case resp.StatusCode == http.StatusUnauthorized:
		result.Kind, result.Summary = InvalidToken, "Account Token is invalid or belongs to a different service"
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
	case resp.StatusCode == http.StatusServiceUnavailable && containsAny(lower, "model", "channel"):
		result.Kind, result.Summary, result.Retryable = ModelUnavailable, "Current model or channel is unavailable", true
		result.Fix = "Confirm the model name and token model restrictions, or try again later"
	case resp.StatusCode >= 500:
		result.Kind, result.Summary, result.Retryable = UpstreamFailure, "Upstream service failed", true
		result.Fix = "Try again later; if it persists, contact the service operator with the HTTP status code"
	default:
		result.Kind, result.Summary = Unexpected, fmt.Sprintf("Service endpoint returned unexpected HTTP status %d", resp.StatusCode)
		result.Fix = "Run `aigw doctor` for detailed status"
	}
	return result
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
