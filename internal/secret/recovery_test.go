package secret

import "testing"

func TestTokenRemediationMatchesCredentialAuthority(t *testing.T) {
	t.Parallel()

	if got, writable := TokenRecovery(NewMemoryStore(), "team-gateway"); got != "run `aigw rotate team-gateway`" || !writable {
		t.Fatalf("writable recovery = %q, %t", got, writable)
	}
	store := NewEnvironmentStore(func(string) string { return "" })
	if got, writable := TokenRecovery(store, "team-gateway"); got != "set environment variable AIGW_TOKEN_TEAM_2DGATEWAY" || writable {
		t.Fatalf("read-only recovery = %q, %t", got, writable)
	}
}
