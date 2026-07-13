package diagnostics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/redaction"
)

type Kind string

const (
	Healthy          Kind = "healthy"
	InvalidToken     Kind = "invalid_token"
	QuotaExhausted   Kind = "quota_exhausted"
	TokenDisabled    Kind = "token_disabled"
	TokenRestricted  Kind = "token_restricted"
	RateLimited      Kind = "rate_limited"
	ModelUnavailable Kind = "model_unavailable"
	GatewayFailure   Kind = "gateway_failure"
	EndpointMismatch Kind = "endpoint_mismatch"
	NetworkFailure   Kind = "network_failure"
	Unexpected       Kind = "unexpected"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Result struct {
	Kind       Kind   `json:"kind"`
	Summary    string `json:"summary"`
	Detail     string `json:"detail,omitempty"`
	Fix        string `json:"fix,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty"`
	Retryable  bool   `json:"retryable"`
}

func Probe(ctx context.Context, client HTTPDoer, runtime domain.Runtime, token string) Result {
	endpoint := strings.TrimRight(runtime.Endpoint, "/")
	if endpoint == "" {
		return Result{Kind: EndpointMismatch, Summary: "API 地址无效", Fix: "检查当前 Profile 对应 Account 的协议端点"}
	}
	if strings.HasSuffix(endpoint, "/v1") {
		endpoint += "/models"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{Kind: EndpointMismatch, Summary: "API 地址无效", Detail: err.Error(), Fix: "检查团队 Profile 的网关 URL"}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return Result{Kind: NetworkFailure, Summary: "无法连接网关", Detail: sanitize(err.Error(), token), Fix: "检查网络、代理和网关地址后重试", Retryable: true}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	message := strings.TrimSpace(string(body))
	lower := strings.ToLower(message)
	result := Result{HTTPStatus: resp.StatusCode, Detail: compact(message, token)}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 400:
		result.Kind, result.Summary = Healthy, "API Token 与网关正常"
	case resp.StatusCode == http.StatusUnauthorized:
		result.Kind, result.Summary = InvalidToken, "API Token 无效或与网关站点不匹配"
		result.Fix = "运行 `aigw rotate` 重新录入 Token，并确认 URL 与 Token 所属站点一致"
	case resp.StatusCode == http.StatusForbidden && containsAny(lower, "quota", "额度", "余额", "insufficient", "exhaust"):
		result.Kind, result.Summary = QuotaExhausted, "Token 额度已耗尽"
		result.Fix = "在服务商后台增加 Token 额度或设为无限额度；也可运行 `aigw rotate` 切换 Token"
	case resp.StatusCode == http.StatusForbidden && containsAny(lower, "disabled", "disable", "禁用", "停用"):
		result.Kind, result.Summary = TokenDisabled, "Token 已被禁用"
		result.Fix = "在服务商后台启用 Token，或运行 `aigw rotate` 更换 Token"
	case resp.StatusCode == http.StatusForbidden:
		result.Kind, result.Summary = TokenRestricted, "Token 或账户受到限制"
		result.Fix = "检查 Token 分组、IP 白名单、模型限制和账户状态；运行 `aigw balance` 获取精确信息"
	case resp.StatusCode == http.StatusTooManyRequests:
		result.Kind, result.Summary, result.Retryable = RateLimited, "请求过快或并发额度已满", true
		result.Fix = "降低并发并稍后重试；持续出现时检查服务商限速策略"
	case resp.StatusCode == http.StatusNotFound:
		result.Kind, result.Summary = EndpointMismatch, "API 地址或路径不匹配"
		result.Fix = "检查网关 URL 是否需要 /v1，以及 .cn/.com 站点是否匹配"
	case resp.StatusCode == http.StatusServiceUnavailable && containsAny(lower, "model", "channel", "模型", "渠道"):
		result.Kind, result.Summary, result.Retryable = ModelUnavailable, "当前模型或渠道不可用", true
		result.Fix = "确认模型名称与 Token 模型限制，或稍后重试"
	case resp.StatusCode >= 500:
		result.Kind, result.Summary, result.Retryable = GatewayFailure, "网关或上游服务故障", true
		result.Fix = "稍后重试；持续失败时联系网关管理员并提供 HTTP 状态码"
	default:
		result.Kind, result.Summary = Unexpected, fmt.Sprintf("网关返回未预期状态 HTTP %d", resp.StatusCode)
		result.Fix = "运行 `aigw doctor` 查看详细状态"
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
