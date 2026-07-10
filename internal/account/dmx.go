package account

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Report struct {
	AccountBalance      float64
	TokenName           string
	TokenStatus         string
	TokenUsed           float64
	TokenRemaining      float64
	TokenUnlimitedQuota bool
	TokenRemainingCount int64
	TokenUnlimitedCount bool
	TokenExpiredAt      int64
}

func Probe(ctx context.Context, client HTTPDoer, profile domain.Profile, apiToken string, credential Credential) (Report, error) {
	if profile.AccountProbe == nil {
		return Report{}, fmt.Errorf("profile %q has no account probe", profile.ID)
	}
	if profile.AccountProbe.Kind != "dmxapi" {
		return Report{}, fmt.Errorf("unsupported account probe %q", profile.AccountProbe.Kind)
	}
	base := strings.TrimRight(profile.AccountProbe.BaseURL, "/")
	var user struct {
		Success bool `json:"success"`
		Data    struct {
			Quota int64 `json:"quota"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := getJSON(ctx, client, base+"/api/user/self", credential, &user); err != nil {
		return Report{}, err
	}
	if !user.Success {
		return Report{}, fmt.Errorf("DMXAPI account query failed: %s", user.Message)
	}
	items, err := fetchDMXTokens(ctx, client, base, credential)
	if err != nil {
		return Report{}, err
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
		return Report{
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
	return Report{AccountBalance: float64(user.Data.Quota) / 500000}, fmt.Errorf("current API Token was not found in the DMXAPI account")
}

type dmxToken struct {
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

func fetchDMXTokens(ctx context.Context, client HTTPDoer, base string, credential Credential) ([]dmxToken, error) {
	items := []dmxToken{}
	for page := 1; page <= 100; page++ {
		endpoint := fmt.Sprintf("%s/api/token/search?page=%d&page_size=100", base, page)
		var payload struct {
			Success bool `json:"success"`
			Data    struct {
				Items    []dmxToken `json:"items"`
				PageSize int        `json:"page_size"`
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

func getJSON(ctx context.Context, client HTTPDoer, endpoint string, credential Credential, target any) error {
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

func maskedToken(token string) string {
	token = strings.TrimPrefix(strings.TrimSpace(token), "sk-")
	if decoded, err := url.QueryUnescape(token); err == nil {
		token = decoded
	}
	if len(token) < 8 {
		return token
	}
	return token[:4] + strings.Repeat("*", 10) + token[len(token)-4:]
}
