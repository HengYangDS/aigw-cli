package secrets

import (
	"encoding/json"
	"fmt"
	"strings"
)

type EnvironmentStore struct{ getenv func(string) string }

func NewEnvironmentStore(getenv func(string) string) EnvironmentStore {
	return EnvironmentStore{getenv: getenv}
}

func EnvironmentKey(profile string) string {
	return "AIGW_TOKEN_" + environmentAccountID(profile)
}

func DiagnosticSystemTokenEnvironmentKey(profile string) string {
	return "AIGW_DIAGNOSTIC_SYSTEM_TOKEN_" + environmentAccountID(profile)
}

func DiagnosticUserIDEnvironmentKey(profile string) string {
	return "AIGW_DIAGNOSTIC_USER_ID_" + environmentAccountID(profile)
}

func environmentAccountID(account string) string {
	var encoded strings.Builder
	for _, character := range strings.ToUpper(account) {
		if character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' {
			encoded.WriteRune(character)
			continue
		}
		_, _ = fmt.Fprintf(&encoded, "_%02X", character)
	}
	return encoded.String()
}

func (s EnvironmentStore) Get(profile string) (string, error) {
	return s.get(APIToken, profile)
}

func (s EnvironmentStore) get(kind Kind, profile string) (string, error) {
	if err := validate(profile, "", false); err != nil {
		return "", err
	}
	if kind == ProviderDiagnostic {
		systemToken := s.getenv(DiagnosticSystemTokenEnvironmentKey(profile))
		userID := s.getenv(DiagnosticUserIDEnvironmentKey(profile))
		if systemToken == "" || userID == "" {
			return "", ErrNotFound
		}
		value, _ := json.Marshal(struct {
			SystemToken string `json:"system_token"`
			UserID      string `json:"user_id"`
		}{SystemToken: systemToken, UserID: userID})
		return string(value), nil
	}
	value := s.getenv(EnvironmentKey(profile))
	if value == "" {
		return "", ErrNotFound
	}
	return value, nil
}

func (EnvironmentStore) Set(profile, value string) error {
	return EnvironmentStore{}.set(APIToken, profile, value)
}

func (EnvironmentStore) set(_ Kind, profile, value string) error {
	if err := validate(profile, value, true); err != nil {
		return err
	}
	return ErrReadOnly
}

func (EnvironmentStore) Delete(profile string) error {
	return EnvironmentStore{}.delete(APIToken, profile)
}

func (EnvironmentStore) delete(_ Kind, profile string) error {
	if err := validate(profile, "", false); err != nil {
		return err
	}
	return ErrReadOnly
}

func (s EnvironmentStore) Has(profile string) bool {
	_, err := s.Get(profile)
	return err == nil
}

func (EnvironmentStore) ReadOnly() bool { return true }
