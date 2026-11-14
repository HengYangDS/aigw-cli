package secret

import (
	"errors"
	"strings"
	"testing"

	"aigw-cli/internal/secret/native"
)

func TestTokenRemediationMatchesCredentialAuthority(t *testing.T) {
	t.Parallel()

	if got, writable := TokenRecovery(NewMemoryStore(), "team-gateway"); got != "run `aigw provider rotate team-gateway`" || !writable {
		t.Fatalf("writable recovery = %q, %t", got, writable)
	}
	store := NewEnvironmentStore(func(string) string { return "" })
	if got, writable := TokenRecovery(store, "team-gateway"); got != "set environment variable AIGW_TOKEN_TEAM_2DGATEWAY" || writable {
		t.Fatalf("read-only recovery = %q, %t", got, writable)
	}
}

// The refusal of a Keychain item names the credential, what macOS answered
// and the one fix: delete the item with the system tool, then store the
// credential again with its own command. It keeps the native failure
// reachable and names no command in its error text.
func TestKeychainItemRefusalNamesTheCredentialAndItsFix(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		refusal              KeychainItemRefusedError
		title, evidence, fix string
	}{
		{
			refusal:  KeychainItemRefusedError{Kind: APIToken, Provider: "team-gateway", Status: -25293},
			title:    "Another program stored provider team-gateway's Token in the Keychain",
			evidence: "macOS refused AIGW the item while its keychain was unlocked (Keychain OSStatus -25293).",
			fix:      "Delete the item with `security delete-generic-password -s AIGW_TOKEN -a team-gateway`, then run `aigw provider rotate team-gateway`.",
		},
		{
			refusal:  KeychainItemRefusedError{Kind: ProviderDiagnostic, Provider: "team-gateway", Status: -25293},
			title:    "Another program stored provider team-gateway's balance credentials in the Keychain",
			evidence: "macOS refused AIGW the item while its keychain was unlocked (Keychain OSStatus -25293).",
			fix: "Delete the item with `security delete-generic-password -s AIGW_TOKEN -a diagnostic@team-gateway`, " +
				"then run `aigw provider edit team-gateway --balance-token-stdin --balance-user-id <user-id>`.",
		},
	} {
		if got := test.refusal.Title(); got != test.title {
			t.Errorf("title = %q, want %q", got, test.title)
		}
		if got := test.refusal.Evidence(); got != test.evidence {
			t.Errorf("evidence = %q, want %q", got, test.evidence)
		}
		if got := test.refusal.Fix(); got != test.fix {
			t.Errorf("fix = %q, want %q", got, test.fix)
		}
		var err error = &test.refusal
		failure, typed := errors.AsType[*native.KeychainError](err)
		if !typed || failure.Status != -25293 || !failure.ItemRefused() || !errors.Is(err, native.ErrUnavailable) {
			t.Errorf("the refusal lost its native failure: %v", err)
		}
		if text := err.Error(); !strings.Contains(text, "team-gateway") || !strings.Contains(text, "-25293") || strings.Contains(text, "`") {
			t.Errorf("error text = %q", text)
		}
	}
}
