// Package dmxapi implements the explicitly selected DMXAPI diagnostic
// provider. It has no role in AIGW's default setup, routing, or token model.
package dmxapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/account"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

func Probe(ctx context.Context, client HTTPDoer, providerAccount domain.Account, apiToken string, credential account.Credential) (account.Report, error) {
	if providerAccount.AccountProbe == nil || providerAccount.AccountProbe.Kind != "dmxapi" {
		return account.Report{}, fmt.Errorf("DMXAPI diagnostic provider is not configured")
	}
	base := strings.TrimRight(providerAccount.AccountProbe.BaseURL, "/")
	var user struct {
		Success bool `json:"success"`
		Data    struct {
			Quota int64 `json:"quota"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := getJSON(ctx, client, base+"/api/user/self", credential, &user); err != nil {
		return account.Report{}, err
	}
	if !user.Success {
		return account.Report{}, fmt.Errorf("DMXAPI account query failed: %s", user.Message)
	}
	items, err := fetchTokens(ctx, client, base, credential)
	if err != nil {
		return account.Report{}, err
	}
	masked := maskedToken(apiToken)
	for _, token := range items {
		if token.Key != masked {
			continue
		}
		status := "disabled"
		if token.Status == 1 {
			status = "enabled"
		}
		return account.Report{
			AccountBalance: float64(user.Data.Quota) / 500000,
			TokenName:      token.Name, TokenStatus: status,
			TokenUsed:           float64(token.UsedQuota) / 500000,
			TokenRemaining:      float64(token.RemainQuota) / 500000,
			TokenUnlimitedQuota: token.UnlimitedQuota,
			TokenRemainingCount: token.RemainCount,
			TokenUnlimitedCount: token.UnlimitedCount,
			TokenExpiredAt:      token.ExpiredTime,
		}, nil
	}
	return account.Report{AccountBalance: float64(user.Data.Quota) / 500000}, fmt.Errorf("current API Token was not found in the DMXAPI account")
}

type token struct {
	Name           string `json:"name"`
	Key            string `json:"key"`
	Status         int    `json:"status"`
	UsedQuota      int64  `json:"used_quota"`
	RemainQuota    int64  `json:"remain_quota"`
	UnlimitedQuota bool   `json:"unlimited_quota"`
	RemainCount    int64  `json:"remain_count"`
	UnlimitedCount bool   `json:"unlimited_count"`
	ExpiredTime    int64  `json:"expired_time"`
}

func fetchTokens(ctx context.Context, client HTTPDoer, base string, credential account.Credential) ([]token, error) {
	items := []token{}
	for page := 1; page <= 100; page++ {
		endpoint := fmt.Sprintf("%s/api/token/search?page=%d&page_size=100", base, page)
		var payload struct {
			Success bool `json:"success"`
			Data    struct {
				Items    []token `json:"items"`
				PageSize int     `json:"page_size"`
			} `json:"data"`
			Message string `json:"message"`
		}
		if err := getJSON(ctx, client, endpoint, credential, &payload); err != nil {
			return nil, err
		}
		if !payload.Success {
			return nil, fmt.Errorf("DMXAPI token query failed: %s", payload.Message)
		}
		items = append(items, payload.Data.Items...)
		if len(payload.Data.Items) < 100 {
			break
		}
	}
	return items, nil
}

func getJSON(ctx context.Context, client HTTPDoer, endpoint string, credential account.Credential, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+credential.SystemToken)
	req.Header.Set("Rix-Api-User", credential.UserID)
	req.Header.Set("Dmx-Api-User", credential.UserID)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("DMXAPI platform API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(target)
}

func maskedToken(value string) string {
	value = strings.TrimPrefix(strings.TrimSpace(value), "sk-")
	if decoded, err := url.QueryUnescape(value); err == nil {
		value = decoded
	}
	if len(value) < 8 {
		return value
	}
	return value[:4] + strings.Repeat("*", 10) + value[len(value)-4:]
}
