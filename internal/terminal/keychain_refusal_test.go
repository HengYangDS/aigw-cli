package terminal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"aigw-cli/internal/platform"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/native"
)

// A Keychain item that macOS refuses AIGW while its keychain is unlocked is
// presented as that credential's problem with its one fix, whatever wraps it:
// a store read or write, or a synchronization whose reader check met it. A
// locked keychain, which gives the same status, keeps the presentations that
// it had.
func TestKeychainItemRefusalIsPresentedWithItsFix(t *testing.T) {
	token := &secret.KeychainItemRefusedError{Kind: secret.APIToken, Provider: "team", Status: -25293}
	balance := &secret.KeychainItemRefusedError{Kind: secret.ProviderDiagnostic, Provider: "team", Status: -25293}
	const evidence = "macOS refused AIGW the item while its keychain was unlocked (Keychain OSStatus -25293)."
	const impact = "The item is unchanged; AIGW neither read nor replaced it."
	for name, test := range map[string]struct {
		err  error
		want Problem
	}{
		"a Token write": {
			err: fmt.Errorf("store Token for provider %q: %w", "team", fmt.Errorf("write AIGW_TOKEN/team to system keyring: %w", token)),
			want: Problem{Title: "Another program stored provider team's Token in the Keychain", Evidence: evidence, Impact: impact,
				Fix: "Delete the item with `security delete-generic-password -s AIGW_TOKEN -a team`, then run `aigw provider rotate team`."},
		},
		"a reader check": {
			err: errors.Join(fmt.Errorf("%w: copied credential reader cannot read the selected Token: %w", secret.ErrNativeReaderUnverified, token), nil),
			want: Problem{Title: "Another program stored provider team's Token in the Keychain", Evidence: evidence, Impact: impact,
				Fix: "Delete the item with `security delete-generic-password -s AIGW_TOKEN -a team`, then run `aigw provider rotate team`."},
		},
		"a balance credential write": {
			err: fmt.Errorf("write AIGW_TOKEN/diagnostic@team to system keyring: %w", balance),
			want: Problem{Title: "Another program stored provider team's balance credentials in the Keychain", Evidence: evidence, Impact: impact,
				Fix: "Delete the item with `security delete-generic-password -s AIGW_TOKEN -a diagnostic@team`, " +
					"then run `aigw provider edit team --balance-token-stdin --balance-user-id <user-id>`."},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := ProblemFor(test.err); got != test.want {
				t.Fatalf("problem = %+v\nwant      %+v", got, test.want)
			}
			var out bytes.Buffer
			renderer := NewRenderer(&out, false)
			RenderError(renderer, test.err, true)
			var document struct {
				Error      string `json:"error"`
				NextAction string `json:"next_action"`
				NextDetail string `json:"next_detail"`
			}
			if err := json.Unmarshal(out.Bytes(), &document); err != nil || renderer.Err() != nil {
				t.Fatalf("document %s: %v, %v", &out, err, renderer.Err())
			}
			if document.Error != test.want.Title || document.NextAction != "" || document.NextDetail != test.want.Fix {
				t.Fatalf("document = %+v", document)
			}
		})
	}
	// A batch that had stored another provider's Token and could not put the
	// previous one back says so beside the refusal.
	partial := errors.Join(fmt.Errorf("store Token for provider %q: %w", "team", token),
		platform.FailedUndo("restoring the Tokens that this command had stored", errors.New("the credential store is locked")))
	if got := ProblemFor(partial); got.Title != token.Title() || got.Fix != token.Fix() ||
		got.Impact != impact+" Restoring the Tokens that this command had stored failed: the credential store is locked." {
		t.Fatalf("a refusal beside a failed undo = %+v", got)
	}
	locked := &native.KeychainError{Status: -25293}
	write := fmt.Errorf("write AIGW_TOKEN/team to system keyring: %w", locked)
	if got := ProblemFor(write); got != (Problem{Title: write.Error(), Fix: "aigw check"}) {
		t.Fatalf("a locked keychain's write = %+v", got)
	}
	check := fmt.Errorf("%w: copied credential reader cannot read the selected Token: %w", secret.ErrNativeReaderUnverified, locked)
	if got := ProblemFor(check); got.Title != "Cannot verify native Token access for this AIGW version" {
		t.Fatalf("a locked keychain's reader check = %+v", got)
	}
}
