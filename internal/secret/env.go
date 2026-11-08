package secret

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"aigw-cli/internal/domain"
)

// environmentTokenPrefix starts the name of every API-token variable.
const environmentTokenPrefix = "AIGW_TOKEN_"

// environmentStore reads typed credential slots from an explicit environment
// and never persists changes. environ lists that environment as [os.Environ]
// does, of which a listing uses the names and whether each is empty; without
// it the backend cannot name its Tokens.
type environmentStore struct {
	getenv  func(string) string
	environ func() []string
}

// NewEnvironmentStore constructs a read-only environment credential backend.
func NewEnvironmentStore(getenv func(string) string) Store {
	return scopedView{store: environmentStore{getenv: getenv}}
}

// EnvironmentKey returns the API-token variable name for an Account.
func EnvironmentKey(account string) string {
	return environmentTokenPrefix + environmentProviderID(account)
}

// DiagnosticSystemTokenEnvironmentKey returns the provider-diagnostic system-token variable name for an Account.
func DiagnosticSystemTokenEnvironmentKey(account string) string {
	return "AIGW_DIAGNOSTIC_SYSTEM_TOKEN_" + environmentProviderID(account)
}

// DiagnosticUserIDEnvironmentKey returns the provider-diagnostic user-ID variable name for an Account.
func DiagnosticUserIDEnvironmentKey(account string) string {
	return "AIGW_DIAGNOSTIC_USER_ID_" + environmentProviderID(account)
}

// EnvironmentVariables names the variables of an Account's credentials that
// the read-only environment backend sees set, each checked on its own and in
// a fixed order: the Token's, the diagnostic system token's and the
// diagnostic user ID's. A diagnostic credential needs both of its variables,
// so one of them alone is listed although the credential is unavailable.
// Every other backend has none.
func EnvironmentVariables(store Store, account string) []string {
	environment, ok := backendStore(store).(environmentStore)
	if !ok {
		return nil
	}
	set := []string{}
	for _, key := range []string{EnvironmentKey(account), DiagnosticSystemTokenEnvironmentKey(account), DiagnosticUserIDEnvironmentKey(account)} {
		if environment.getenv(key) != "" {
			set = append(set, key)
		}
	}
	return set
}

func environmentProviderID(account string) string {
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

// decodeEnvironmentProviderID is the inverse of environmentProviderID. A name
// that environmentProviderID would not write decodes to nothing.
func decodeEnvironmentProviderID(encoded string) (string, bool) {
	var decoded strings.Builder
	for index := 0; index < len(encoded); index++ {
		if encoded[index] != '_' {
			decoded.WriteString(strings.ToLower(encoded[index : index+1]))
			continue
		}
		if index+2 >= len(encoded) {
			return "", false
		}
		character, err := strconv.ParseUint(encoded[index+1:index+3], 16, 8)
		if err != nil {
			return "", false
		}
		decoded.WriteByte(byte(character))
		index += 2
	}
	id := decoded.String()
	return id, domain.ValidIdentifier(id) && environmentProviderID(id) == encoded
}

func (s environmentStore) list() ([]string, error) {
	if s.environ == nil {
		return nil, errors.New("the environment's variables are unavailable")
	}
	ids := []string{}
	for _, entry := range s.environ() {
		name, value, _ := strings.Cut(entry, "=")
		encoded, token := strings.CutPrefix(name, environmentTokenPrefix)
		if !token || value == "" {
			continue
		}
		if id, valid := decodeEnvironmentProviderID(encoded); valid {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (s environmentStore) get(kind Kind, account string) (string, error) {
	if kind == ProviderDiagnostic {
		systemToken := s.getenv(DiagnosticSystemTokenEnvironmentKey(account))
		userID := s.getenv(DiagnosticUserIDEnvironmentKey(account))
		if systemToken == "" && userID == "" {
			return "", ErrNotFound
		}
		if systemToken == "" || userID == "" {
			return "", errors.New("provider diagnostic environment is incomplete")
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
	return errReadOnly
}

func (environmentStore) delete(Kind, string) error {
	return errReadOnly
}

func (s environmentStore) exists(kind Kind, account string) (bool, error) {
	if kind == ProviderDiagnostic {
		return s.getenv(DiagnosticSystemTokenEnvironmentKey(account)) != "" &&
			s.getenv(DiagnosticUserIDEnvironmentKey(account)) != "", nil
	}
	return s.getenv(EnvironmentKey(account)) != "", nil
}
