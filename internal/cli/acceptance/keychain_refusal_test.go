package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/native"
)

// The texts that every surface takes from the refusal of a Keychain item.
const (
	refusalEvidence = "macOS refused AIGW the item while its keychain was unlocked (Keychain OSStatus -25293)."
	refusalImpact   = "The item is unchanged; AIGW neither read nor replaced it. Deleting it also takes it from the program that stored it."
)

// refusalTitle is the title of the refusal of provider's Token item.
func refusalTitle(provider string) string {
	return "Another program stored provider " + provider + "'s Token in the Keychain"
}

// refusalFix is the fix of the refusal of provider's Token item, whose last
// step runs command.
func refusalFix(provider, command string) string {
	return "Have the Token at hand, delete the item with `security delete-generic-password -s AIGW_TOKEN -a " + provider + "`, then run " + command + "."
}

// requireRefusal checks that args, run in text or as JSON, ended with the
// refusal of provider's Token item and showed its title, its evidence, the
// general impact and fix, and no Token.
func requireRefusal(t *testing.T, args []string, err error, out *bytes.Buffer, jsonMode bool, provider, fix string) {
	t.Helper()
	if _, refused := errors.AsType[*secret.KeychainItemRefusedError](err); !refused || !errors.Is(err, native.ErrUnavailable) {
		t.Fatalf("%v = %v\n%s", args, err, out)
	}
	if strings.Contains(out.String(), "fixture-token") {
		t.Fatalf("%v wrote the Token:\n%s", args, out)
	}
	if !jsonMode {
		text := strings.Join(strings.Fields(out.String()), " ")
		for _, want := range []string{refusalTitle(provider), refusalEvidence, refusalImpact, fix} {
			if !strings.Contains(text, want) {
				t.Fatalf("%v output lacks %q:\n%s", args, want, out)
			}
		}
		return
	}
	var document struct {
		OK         bool   `json:"ok"`
		Error      string `json:"error"`
		Evidence   string `json:"evidence"`
		Impact     string `json:"impact"`
		NextAction string `json:"next_action"`
		NextDetail string `json:"next_detail"`
	}
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatalf("%v wrote %s: %v", args, out, err)
	}
	if document.OK || document.Error != refusalTitle(provider) || document.Evidence != refusalEvidence || document.Impact != refusalImpact ||
		document.NextAction != "" || document.NextDetail != fix {
		t.Fatalf("%v wrote %+v\nwant the fix %q", args, document, fix)
	}
}

// Every command that stores a Token gives a Keychain item of another program
// the same problem, in text and as JSON, and stores nothing. The fix has the
// Token at hand, deletes the item and stores the Token again: with `aigw
// provider rotate` for a provider that the configuration has, and with the
// refused command's own line for a command that adds the provider, which
// `aigw provider rotate` would not know yet. A line that would need quoting
// is named as the same command. A team setup observes the item of the provider
// that it names before it reads the Token, so it sends no request.
func TestAKeychainItemOfAnotherProgramNamesItsFixWhereATokenIsStored(t *testing.T) {
	teamFile := gatewayTeamFile(t)
	for _, test := range []struct {
		name     string
		provider string
		args     []string
		// last is the last step of the fix.
		last    string
		prepare func(t *testing.T, app *cli.App)
		// requests are the requests that the command sends before it meets the
		// item: the validation of a Token that it read.
		requests int
	}{
		{name: "provider rotate", provider: "one", args: []string{"provider", "rotate", "one", "--token-stdin"}, last: "`aigw provider rotate one`", requests: 1,
			prepare: func(t *testing.T, app *cli.App) {
				saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
			}},
		{name: "provider add", provider: "team", args: []string{"provider", "add", "team", "--endpoint", "anthropic=https://team.test", "--token-stdin"},
			last: "`aigw provider add team --endpoint anthropic=https://team.test --token-stdin`", requests: 1},
		{name: "provider add that needs quoting", provider: "team", last: "the same command again", requests: 1,
			args: []string{"provider", "add", "team", "--label", "Team Gateway", "--endpoint", "anthropic=https://team.test", "--token-stdin"}},
		{name: "setup", provider: "gateway", args: []string{"setup", "--from", teamFile, "--provider", "gateway", "--token-stdin"},
			last: setupLine(teamFile)},
	} {
		for _, jsonMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%t", test.name, jsonMode), func(t *testing.T) {
				app, out, _, _, requests := testApp(t, "fixture-token\n")
				if test.prepare != nil {
					test.prepare(t, app)
				}
				items := withItemsOfAnotherProgram(t, app, test.provider)
				args := test.args
				if jsonMode {
					args = append(append([]string{}, args...), "--json")
				}
				err := cli.Execute(app, args)
				requireRefusal(t, args, err, out, jsonMode, test.provider, refusalFix(test.provider, test.last))
				if changed := items.changed(t); len(changed) != 0 || requests.calls != test.requests {
					t.Fatalf("%v changed the keychain with %q and sent %d requests, want %d\n%s", args, changed, requests.calls, test.requests, out)
				}
			})
		}
	}
}

// setupLine is the last step that names the team setup of teamFile for the
// gateway provider: its own line, or the same command where the host's
// temporary folder makes the path need quoting.
func setupLine(teamFile string) string {
	if invocation.Command(teamFile) == "" {
		return "the same command again"
	}
	return "`aigw setup --from " + teamFile + " --provider gateway --token-stdin`"
}

// Guided setup uses a stored Token without a prompt. A Token that an earlier
// release stored for the provider that setup adds, in an item that macOS
// refuses AIGW, stops setup at the Token step with the item's refusal: before
// it asks for a model, with nothing saved, and without reading the Token,
// because the observation of the item already tells. The provider is not
// configured yet, so the last step of the fix is setup again.
func TestGuidedSetupStopsAtAStoredTokenThatMacOSRefuses(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	app.Interactive = true
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: executableFixture(t, "claude")}}}
	items := withItemsOfAnotherProgram(t, app, "team")
	prompt := &scriptedPrompt{
		selections: []string{string(domain.ProtocolAnthropic)},
		texts:      []string{"team", "Team", "https://team.test", "claude-test"},
	}
	app.Prompt = prompt
	args := []string{"setup"}
	err := cli.Execute(app, args)
	if prompt.textCalls != 3 || prompt.selectCalls != 1 || len(prompt.secretCalls) != 0 {
		t.Fatalf("setup asked %d texts, %d selections and %q; want the provider's ID, name, protocol and URL alone\n%s",
			prompt.textCalls, prompt.selectCalls, prompt.secretCalls, out)
	}
	if _, statErr := os.Stat(app.Config.Path()); !os.IsNotExist(statErr) || len(items.changed(t)) != 0 {
		t.Fatalf("a refused setup saved configuration (%v) or changed the keychain with %q", statErr, items.changed(t))
	}
	if operations := items.operations(t); !slices.Equal(operations, []string{"exists team", "empty team"}) {
		t.Fatalf("setup ran %q on the keychain; want one observation and one emptiness answer, and no read of the Token", operations)
	}
	requireRefusal(t, args, err, out, false, "team", refusalFix("team", "`aigw setup`"))
}

// A command that would read a Token to read the provider's catalog shows the
// refusal of the Token's Keychain item as the commands that store one do, in
// text and as JSON: adding a model, which meets the item at that read, and
// selecting a model that is not added yet, which ends where it observes the
// item of the provider that its client would use. Neither names a catalog
// that cannot be read or a rerun with --unlisted, which would fail on the
// same item, and neither sends a request or changes the configuration.
func TestAKeychainItemOfAnotherProgramNamesItsFixWhereACatalogIsRead(t *testing.T) {
	for _, test := range [][]string{
		{"model", "add", "one/claude-next"},
		{"use", claude.ID, "one/claude-next"},
	} {
		for _, jsonMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%t", strings.Join(test[:2], " "), jsonMode), func(t *testing.T) {
				app, out, _, _, requests := testApp(t, "")
				saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
				before := readFile(t, app.Config.Path())
				items := withItemsOfAnotherProgram(t, app, "one")
				args := test
				if jsonMode {
					args = append(append([]string{}, args...), "--json")
				}
				err := cli.Execute(app, args)
				requireRefusal(t, args, err, out, jsonMode, "one", refusalFix("one", "`aigw provider rotate one`"))
				if strings.Contains(out.String(), "--unlisted") || strings.Contains(out.String(), "catalog") {
					t.Fatalf("%v named the catalog or --unlisted:\n%s", args, out)
				}
				if requests.calls != 0 || len(items.changed(t)) != 0 || !bytes.Equal(readFile(t, app.Config.Path()), before) {
					t.Fatalf("%v sent %d requests, changed the keychain with %q, or changed the configuration", args, requests.calls, items.changed(t))
				}
			})
		}
	}
}

// `aigw model search` reports each provider's catalog in one document, so a
// Token whose Keychain item macOS refuses AIGW is that provider's problem
// there, with the item's title and its fix, in text and as JSON, never a
// credential backend that could not tell.
func TestModelSearchNamesTheFixOfAKeychainItemThatMacOSRefuses(t *testing.T) {
	fix := refusalFix("one", "`aigw provider rotate one`")
	for _, jsonMode := range []bool{false, true} {
		app, out, _, _, requests := testApp(t, "")
		saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
		withItemsOfAnotherProgram(t, app, "one")
		args := []string{"model", "search", "--provider", "one"}
		if jsonMode {
			args = append(args, "--json")
		}
		if err := cli.Execute(app, args); err == nil || requests.calls != 0 {
			t.Fatalf("%v = %v after %d requests\n%s", args, err, requests.calls, out)
		}
		if strings.Contains(out.String(), "could not tell") || strings.Contains(out.String(), "aigw provider show") || strings.Contains(out.String(), "fixture-token") {
			t.Fatalf("%v named the credential backend:\n%s", args, out)
		}
		if !jsonMode {
			text := strings.Join(strings.Fields(out.String()), " ")
			for _, want := range []string{"Anthropic Messages: " + refusalTitle("one"), "Fix: " + fix} {
				if !strings.Contains(text, want) {
					t.Fatalf("%v output lacks %q:\n%s", args, want, out)
				}
			}
			continue
		}
		var document struct {
			OK         bool   `json:"ok"`
			NextAction string `json:"next_action"`
			NextDetail string `json:"next_detail"`
			Providers  []struct {
				Problem    string `json:"problem"`
				NextAction string `json:"next_action"`
				NextDetail string `json:"next_detail"`
			} `json:"providers"`
		}
		if err := json.Unmarshal(out.Bytes(), &document); err != nil || len(document.Providers) != 1 {
			t.Fatalf("%v wrote %s: %v", args, out, err)
		}
		if provider := document.Providers[0]; document.OK || document.NextAction != "" || document.NextDetail != fix ||
			provider.Problem != "Anthropic Messages: "+refusalTitle("one") || provider.NextAction != "" || provider.NextDetail != fix {
			t.Fatalf("%v wrote %+v", args, document)
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
