package secret

import (
	"slices"
	"testing"
)

func TestEnvironmentVariablesChecksEachVariableOnItsOwn(t *testing.T) {
	for _, test := range []struct {
		name      string
		variables map[string]string
		want      []string
	}{
		{"none", map[string]string{}, []string{}},
		{"Token only", map[string]string{"AIGW_TOKEN_TEAM": "t"}, []string{"AIGW_TOKEN_TEAM"}},
		{"half of the diagnostic pair", map[string]string{"AIGW_DIAGNOSTIC_SYSTEM_TOKEN_TEAM": "s"}, []string{"AIGW_DIAGNOSTIC_SYSTEM_TOKEN_TEAM"}},
		{"every variable", map[string]string{"AIGW_TOKEN_TEAM": "t", "AIGW_DIAGNOSTIC_SYSTEM_TOKEN_TEAM": "s", "AIGW_DIAGNOSTIC_USER_ID_TEAM": "u"},
			[]string{"AIGW_TOKEN_TEAM", "AIGW_DIAGNOSTIC_SYSTEM_TOKEN_TEAM", "AIGW_DIAGNOSTIC_USER_ID_TEAM"}},
	} {
		store := NewEnvironmentStore(func(key string) string { return test.variables[key] })
		if got := EnvironmentVariables(store, "team"); !slices.Equal(got, test.want) {
			t.Errorf("%s: EnvironmentVariables() = %q, want %q", test.name, got, test.want)
		}
	}
	if got := EnvironmentVariables(NewMemoryStore(), "team"); got != nil {
		t.Fatalf("a writable backend names variables: %q", got)
	}
}
