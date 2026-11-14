package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/native"
)

// refusedTokenStore is a credential store that holds, for provider, a Keychain
// item that macOS refuses AIGW while its keychain is unlocked: a read of the
// Token fails as the system keyring's read of such an item does.
func refusedTokenStore(backend secret.Store, provider string) *recordingCredentialStore[string] {
	refusal := &secret.KeychainItemRefusedError{Kind: secret.APIToken, Provider: provider, Status: -25293}
	return &recordingCredentialStore[string]{backend: backend,
		getErr: fmt.Errorf("read %s/%s from system keyring: %w", secret.Service, provider, refusal)}
}

// Every command that stores a Token gives a Keychain item of another program
// the same problem and the same one fix, in text and as JSON, and stores
// nothing: `aigw provider rotate`, `aigw provider add` and `aigw setup`.
func TestAKeychainItemOfAnotherProgramNamesItsFixWhereATokenIsStored(t *testing.T) {
	for _, test := range []struct {
		name     string
		provider string
		args     []string
		prepare  func(t *testing.T, app *cli.App)
	}{
		{name: "provider rotate", provider: "one", args: []string{"provider", "rotate", "one", "--token-stdin"},
			prepare: func(t *testing.T, app *cli.App) {
				saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
			}},
		{name: "provider add", provider: "team", args: []string{"provider", "add", "team", "--endpoint", "anthropic=https://team.test", "--token-stdin"}},
		{name: "setup", provider: "gateway", args: []string{"setup", "--from", gatewayTeamFile(t), "--provider", "gateway", "--token-stdin"}},
	} {
		title := "Another program stored provider " + test.provider + "'s Token in the Keychain"
		fix := "Delete the item with `security delete-generic-password -s AIGW_TOKEN -a " + test.provider + "`, then run `aigw provider rotate " + test.provider + "`."
		for _, jsonMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%t", test.name, jsonMode), func(t *testing.T) {
				app, out, secretStore, _, _ := testApp(t, "fixture-token\n")
				if test.prepare != nil {
					test.prepare(t, app)
				}
				store := refusedTokenStore(secretStore, test.provider)
				app.Secrets = store
				args := test.args
				if jsonMode {
					args = append(append([]string{}, args...), "--json")
				}
				err := cli.Execute(app, args)
				if _, refused := errors.AsType[*secret.KeychainItemRefusedError](err); !refused || !errors.Is(err, native.ErrUnavailable) {
					t.Fatalf("%v = %v\n%s", args, err, out)
				}
				if len(store.setCalls)+len(store.deleteCalls) != 0 || strings.Contains(out.String(), "fixture-token") {
					t.Fatalf("%v wrote %q, deleted %q\n%s", args, store.setCalls, store.deleteCalls, out)
				}
				if jsonMode {
					var document struct {
						OK         bool   `json:"ok"`
						Error      string `json:"error"`
						Evidence   string `json:"evidence"`
						NextAction string `json:"next_action"`
						NextDetail string `json:"next_detail"`
					}
					if err := json.Unmarshal(out.Bytes(), &document); err != nil {
						t.Fatalf("%v wrote %s: %v", args, out, err)
					}
					if document.OK || document.Error != title || document.NextAction != "" || document.NextDetail != fix ||
						document.Evidence != "macOS refused AIGW the item while its keychain was unlocked (Keychain OSStatus -25293)." {
						t.Fatalf("%v wrote %+v", args, document)
					}
					return
				}
				text := strings.Join(strings.Fields(out.String()), " ")
				for _, want := range []string{title, "The item is unchanged; AIGW neither read nor replaced it.", fix} {
					if !strings.Contains(text, want) {
						t.Fatalf("%v output lacks %q:\n%s", args, want, out)
					}
				}
			})
		}
	}
}

// A locked keychain answers with the same status, and keeps the failure that
// it had: no command tells the operator to delete an item that only needs its
// keychain unlocked.
func TestALockedKeychainDoesNotNameTheDeletionOfAnItem(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "fixture-token\n")
	saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
	locked := fmt.Errorf("read %s/one from system keyring: %w", secret.Service, &native.KeychainError{Status: -25293})
	app.Secrets = &recordingCredentialStore[string]{backend: secretStore, getErr: locked}
	err := cli.Execute(app, []string{"provider", "rotate", "one", "--token-stdin"})
	if !errors.Is(err, locked) || strings.Contains(out.String(), "delete-generic-password") || strings.Contains(out.String(), "Another program") ||
		!bytes.Contains(out.Bytes(), []byte("OSStatus -25293")) {
		t.Fatalf("rotation with a locked keychain = %v\n%s", err, out)
	}
}
