// Package dmxapi implements the explicitly selected DMXAPI diagnostic
// provider. It has no role in AIGW's default setup, routing, or token model.
package dmxapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/providers/diagnostic"
	"aigw-cli/internal/redaction"
	"aigw-cli/internal/secrets"
)

// Kind is the stable manifest identifier for the bundled DMXAPI diagnostic.
const Kind = "dmxapi"

const (
	responseLimit  = 2 << 20
	tokenPageSize  = 100
	tokenPageLimit = 100
)

// Probe obtains and classifies DMXAPI account diagnostics without changing provider or AIGW state.
func Probe(ctx context.Context, client credential.HTTPDoer, providerAccount configuration.Account, apiToken string, auth secrets.DiagnosticCredential) (diagnostic.Report, error) {
	if providerAccount.AccountProbe == nil || providerAccount.AccountProbe.Kind != Kind {
		return diagnostic.Report{}, fmt.Errorf("DMXAPI diagnostic provider is not configured")
	}
	base := strings.TrimRight(providerAccount.AccountProbe.BaseURL, "/")
	var user struct {
		Success bool `json:"success"`
		Data    struct {
			Quota int64 `json:"quota"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := getJSON(ctx, client, base+"/api/user/self", auth, &user); err != nil {
		return diagnostic.Report{}, err
	}
	if !user.Success {
		return diagnostic.Report{}, fmt.Errorf("DMXAPI account query failed: %s", redaction.Text(user.Message, auth.SystemToken, auth.UserID))
	}
	items, err := fetchTokens(ctx, client, base, auth)
	if err != nil {
		return diagnostic.Report{}, err
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
		return diagnostic.Report{
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
	return diagnostic.Report{AccountBalance: float64(user.Data.Quota) / 500000}, fmt.Errorf("current API Token was not found in the DMXAPI account")
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

func fetchTokens(ctx context.Context, client credential.HTTPDoer, base string, auth secrets.DiagnosticCredential) ([]token, error) {
	items := []token{}
	for page := 1; page <= tokenPageLimit; page++ {
		endpoint := fmt.Sprintf("%s/api/token/search?page=%d&page_size=%d", base, page, tokenPageSize)
		var payload struct {
			Success bool `json:"success"`
			Data    struct {
				Items    []token `json:"items"`
				PageSize int     `json:"page_size"`
			} `json:"data"`
			Message string `json:"message"`
		}
		if err := getJSON(ctx, client, endpoint, auth, &payload); err != nil {
			return nil, err
		}
		if !payload.Success {
			return nil, fmt.Errorf("DMXAPI token query failed: %s", redaction.Text(payload.Message, auth.SystemToken, auth.UserID))
		}
		items = append(items, payload.Data.Items...)
		if len(payload.Data.Items) < tokenPageSize {
			return items, nil
		}
	}
	return nil, fmt.Errorf("DMXAPI token search is incomplete after %d pages; token presence is unverified", tokenPageLimit)
}

func getJSON(ctx context.Context, client credential.HTTPDoer, endpoint string, auth secrets.DiagnosticCredential, target any) (resultErr error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+auth.SystemToken)
	req.Header.Set("Rix-Api-User", auth.UserID)
	req.Header.Set("Dmx-Api-User", auth.UserID)
	resp, err := credential.DoProbe(client, req)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, resp.Body.Close()) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return errors.Join(fmt.Errorf("DMXAPI platform API returned HTTP %d: %s", resp.StatusCode, redaction.Text(strings.TrimSpace(string(body)), auth.SystemToken, auth.UserID)), readErr)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, responseLimit+1))
	if err != nil {
		return err
	}
	if len(body) > responseLimit {
		return fmt.Errorf("DMXAPI response exceeds %d bytes", responseLimit)
	}
	return json.Unmarshal(body, target)
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
