package secret

import (
	"errors"
	"fmt"
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
// and the one fix: have the credential at hand, delete the item with the
// system tool, then store the credential again with its own command. It keeps
// the native failure reachable and names no command in its error text.
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
			fix:      "Have the Token at hand, delete the item with `security delete-generic-password -s AIGW_TOKEN -a team-gateway`, then run `aigw provider rotate team-gateway`.",
		},
		{
			refusal:  KeychainItemRefusedError{Kind: ProviderDiagnostic, Provider: "team-gateway", Status: -25293},
			title:    "Another program stored provider team-gateway's balance credentials in the Keychain",
			evidence: "macOS refused AIGW the item while its keychain was unlocked (Keychain OSStatus -25293).",
			fix: "Have the balance credentials at hand, delete the item with `security delete-generic-password -s AIGW_TOKEN -a diagnostic@team-gateway`, " +
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

// A command that adds the provider stores the Token itself, and the
// credential's own command would not know that provider yet. The refusal that
// such a command met therefore names the command's own line as the last step
// of its fix, or the same command when the line would need quoting, wherever
// the command's error wraps it. Any other error returns as it is.
func TestTheRefusedCommandIsTheLastStepOfItsFix(t *testing.T) {
	t.Parallel()

	const item = "Have the Token at hand, delete the item with `security delete-generic-password -s AIGW_TOKEN -a team`, then run "
	refusal := &KeychainItemRefusedError{Kind: APIToken, Provider: "team", Status: -25293}
	wrapped := fmt.Errorf("store Token for provider %q: %w", "team", fmt.Errorf("write AIGW_TOKEN/team to system keyring: %w", refusal))
	if got := refusal.Fix(); got != item+"`aigw provider rotate team`." {
		t.Fatalf("fix before a command claimed the last step = %q", got)
	}
	for command, last := range map[string]string{
		"aigw provider add team --endpoint anthropic=https://team.test --token-stdin": "`aigw provider add team --endpoint anthropic=https://team.test --token-stdin`.",
		"": "the same command again.",
	} {
		if got := AfterDeletionRun(wrapped, command); !errors.Is(got, wrapped) || refusal.Fix() != item+last {
			t.Errorf("after %q: error %v, fix %q, want the last step %q", command, got, refusal.Fix(), last)
		}
	}
	balance := &KeychainItemRefusedError{Kind: ProviderDiagnostic, Provider: "team", Status: -25293}
	if _ = AfterDeletionRun(balance, "aigw setup"); balance.Fix() !=
		"Have the balance credentials at hand, delete the item with `security delete-generic-password -s AIGW_TOKEN -a diagnostic@team`, then run `aigw setup`." {
		t.Errorf("a balance item's fix = %q", balance.Fix())
	}
	other := errors.New("backend failed")
	if got := AfterDeletionRun(other, "aigw setup"); !errors.Is(got, other) || got.Error() != other.Error() {
		t.Errorf("another error = %v", got)
	}
	if got := AfterDeletionRun(nil, "aigw setup"); got != nil {
		t.Errorf("no error = %v", got)
	}
}
