package diagnostics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/redaction"
)

type Kind string

const (
	Healthy                Kind = "healthy"
	InvalidToken           Kind = "invalid_token"
	AuthenticationUnstable Kind = "authentication_unstable"
	QuotaExhausted         Kind = "quota_exhausted"
	TokenDisabled          Kind = "token_disabled"
	TokenRestricted        Kind = "token_restricted"
	RateLimited            Kind = "rate_limited"
	ModelUnavailable       Kind = "model_unavailable"
	GatewayFailure         Kind = "gateway_failure"
	EndpointMismatch       Kind = "endpoint_mismatch"
	NetworkFailure         Kind = "network_failure"
	Unexpected             Kind = "unexpected"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

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

type StabilityPolicy struct {
	RecoveryDelays []time.Duration
	AttemptTimeout time.Duration
}

func DefaultStabilityPolicy() StabilityPolicy {
	return StabilityPolicy{
		RecoveryDelays: []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, time.Second},
		AttemptTimeout: 5 * time.Second,
	}
}

func ProbeStable(ctx context.Context, client HTTPDoer, runtime domain.Runtime, token string, policy StabilityPolicy) Result {
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

func probeWithTimeout(ctx context.Context, client HTTPDoer, runtime domain.Runtime, token string, timeout time.Duration) Result {
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

func Probe(ctx context.Context, client HTTPDoer, runtime domain.Runtime, token string) Result {
	endpoint := strings.TrimRight(runtime.Endpoint, "/")
	if endpoint == "" {
		return Result{Kind: EndpointMismatch, Summary: "Invalid API URL", Fix: "Check the protocol endpoint for the current profile's account"}
	}
	if strings.HasSuffix(endpoint, "/v1") {
		endpoint += "/models"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{Kind: EndpointMismatch, Summary: "Invalid API URL", Detail: err.Error(), Fix: "Check the gateway URL for the team profile"}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return Result{Kind: NetworkFailure, Summary: "Cannot reach the gateway", Detail: sanitize(err.Error(), token), Fix: "Check the network, proxy, and gateway URL, then try again", Retryable: true}
	}
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if readErr != nil {
		return Result{
			Kind:       NetworkFailure,
			Summary:    "Cannot read the gateway response",
			Detail:     sanitize(readErr.Error(), token),
			Fix:        "Check the network, proxy, and gateway URL, then try again",
			HTTPStatus: resp.StatusCode,
			Retryable:  true,
		}
	}
	message := strings.TrimSpace(string(body))
	lower := strings.ToLower(message)
	result := Result{HTTPStatus: resp.StatusCode, Detail: compact(message, token)}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 400:
		result.Kind, result.Summary = Healthy, "API token and gateway are healthy"
	case resp.StatusCode == http.StatusUnauthorized:
		result.Kind, result.Summary = InvalidToken, "API token is invalid or belongs to a different gateway"
		result.Fix = "Run `aigw rotate` to enter the token again, and confirm that its URL and gateway match"
	case resp.StatusCode == http.StatusForbidden && containsAny(lower, "quota", "quota", "balance", "insufficient", "exhaust"):
		result.Kind, result.Summary = QuotaExhausted, "Token quota is exhausted"
		result.Fix = "Increase the token quota in the provider console, make it unlimited, or run `aigw rotate` to switch tokens"
	case resp.StatusCode == http.StatusForbidden && containsAny(lower, "disabled", "disable", "disabled", "disabled"):
		result.Kind, result.Summary = TokenDisabled, "Token is disabled"
		result.Fix = "Enable the token in the provider console, or run `aigw rotate` to replace it"
	case resp.StatusCode == http.StatusForbidden:
		result.Kind, result.Summary = TokenRestricted, "Token or account is restricted"
		result.Fix = "Check the token group, IP allowlist, model restrictions, and account status; run `aigw balance` for precise details"
	case resp.StatusCode == http.StatusTooManyRequests:
		result.Kind, result.Summary, result.Retryable = RateLimited, "Request rate or concurrency quota is exhausted", true
		result.Fix = "Reduce concurrency and try again later; if it persists, check the provider rate-limit policy"
	case resp.StatusCode == http.StatusNotFound:
		result.Kind, result.Summary = EndpointMismatch, "API URL or path does not match"
		result.Fix = "Check whether the gateway URL needs /v1 and whether the .cn/.com site matches"
	case resp.StatusCode == http.StatusServiceUnavailable && containsAny(lower, "model", "channel", "model", "channel"):
		result.Kind, result.Summary, result.Retryable = ModelUnavailable, "Current model or channel is unavailable", true
		result.Fix = "Confirm the model name and token model restrictions, or try again later"
	case resp.StatusCode >= 500:
		result.Kind, result.Summary, result.Retryable = GatewayFailure, "Gateway or upstream service failure", true
		result.Fix = "Try again later; if it persists, contact the gateway administrator with the HTTP status code"
	default:
		result.Kind, result.Summary = Unexpected, fmt.Sprintf("Gateway returned unexpected HTTP status %d", resp.StatusCode)
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
	return sanitize(value, secrets...)
}

func sanitize(value string, secrets ...string) string {
	return redaction.Text(value, secrets...)
}
