package secrets

import (
	"encoding/json"
	"fmt"
	"strings"
)

// environmentStore reads typed credential slots from an explicit environment and never persists changes.
type environmentStore struct{ getenv func(string) string }

// NewEnvironmentStore constructs a read-only environment credential backend.
func NewEnvironmentStore(getenv func(string) string) Store {
	return scopedView{store: environmentStore{getenv: getenv}}
}

// EnvironmentKey returns the API-token variable name for an Account.
func EnvironmentKey(account string) string {
	return "AIGW_TOKEN_" + environmentAccountID(account)
}

// DiagnosticSystemTokenEnvironmentKey returns the provider-diagnostic system-token variable name for an Account.
func DiagnosticSystemTokenEnvironmentKey(account string) string {
	return "AIGW_DIAGNOSTIC_SYSTEM_TOKEN_" + environmentAccountID(account)
}

// DiagnosticUserIDEnvironmentKey returns the provider-diagnostic user-ID variable name for an Account.
func DiagnosticUserIDEnvironmentKey(account string) string {
	return "AIGW_DIAGNOSTIC_USER_ID_" + environmentAccountID(account)
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

func (s environmentStore) get(kind Kind, account string) (string, error) {
	if kind == ProviderDiagnostic {
		systemToken := s.getenv(DiagnosticSystemTokenEnvironmentKey(account))
		userID := s.getenv(DiagnosticUserIDEnvironmentKey(account))
		if systemToken == "" || userID == "" {
			return "", ErrNotFound
		}
		value, _ := json.Marshal(DiagnosticCredential{SystemToken: systemToken, UserID: userID})
		return string(value), nil
	}
	value := s.getenv(EnvironmentKey(account))
	if value == "" {
		return "", ErrNotFound
	}
	return value, nil
}

func (environmentStore) set(Kind, string, string) error {
	return ErrReadOnly
}

func (environmentStore) delete(Kind, string) error {
	return ErrReadOnly
}

func (s environmentStore) exists(kind Kind, account string) (bool, error) {
	if kind == ProviderDiagnostic {
		return s.getenv(DiagnosticSystemTokenEnvironmentKey(account)) != "" &&
			s.getenv(DiagnosticUserIDEnvironmentKey(account)) != "", nil
	}
	return s.getenv(EnvironmentKey(account)) != "", nil
}
