package cli_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// itemsOfAnotherProgram are the Keychain items of one test that another
// program stored, by the account under which the system tool names each: a
// provider's ID for its Token, and diagnostic@<provider> for its balance
// credentials.
type itemsOfAnotherProgram map[string]bool

// delete does what the printed deletion step does: it removes the item that
// the command names. A command that is not the system tool's deletion of one
// of the test's items fails the test.
func (items itemsOfAnotherProgram) delete(t *testing.T, command string) {
	t.Helper()
	words := strings.Fields(command)
	if len(words) != 6 || !slices.Equal(words[:5], []string{"security", "delete-generic-password", "-s", secret.Service, "-a"}) || !items[words[5]] {
		t.Fatalf("the printed step %q deletes no item that another program stored: %v", command, items)
	}
	delete(items, words[5])
}

// keychainStore is a credential store of one kind over a keychain that holds
// items of another program. As macOS answers for such an item, AIGW observes
// it and may delete it, and its read and its replacement are refused. Every
// other slot is the backend's.
type keychainStore[T string | secret.DiagnosticCredential] struct {
	backend interface {
		Get(provider string) (T, error)
		Set(provider string, value T) error
		Delete(provider string) error
		Exists(provider string) (bool, error)
	}
	kind  secret.Kind
	items itemsOfAnotherProgram
}

// slot is the account of provider's item of the store's kind.
func (store keychainStore[T]) slot(provider string) string {
	if store.kind == secret.ProviderDiagnostic {
		return "diagnostic@" + provider
	}
	return provider
}

// refusal is the failure of an operation on provider's item, when another
// program stored it, as the system keyring reports it.
func (store keychainStore[T]) refusal(operation, provider string) error {
	if !store.items[store.slot(provider)] {
		return nil
	}
	return fmt.Errorf("%s %s/%s in the system keyring: %w", operation, secret.Service, store.slot(provider),
		&secret.KeychainItemRefusedError{Kind: store.kind, Provider: provider, Status: -25293})
}

func (store keychainStore[T]) Get(provider string) (T, error) {
	if err := store.refusal("read", provider); err != nil {
		var none T
		return none, err
	}
	return store.backend.Get(provider)
}

func (store keychainStore[T]) Set(provider string, value T) error {
	if err := store.refusal("write", provider); err != nil {
		return err
	}
	return store.backend.Set(provider, value)
}

func (store keychainStore[T]) Delete(provider string) error {
	if store.items[store.slot(provider)] {
		delete(store.items, store.slot(provider))
		return nil
	}
	return store.backend.Delete(provider)
}

func (store keychainStore[T]) Exists(provider string) (bool, error) {
	if store.items[store.slot(provider)] {
		return true, nil
	}
	return store.backend.Exists(provider)
}

// withItemsOfAnotherProgram puts app's credential stores over a keychain
// that holds an item of another program at each of slots.
func withItemsOfAnotherProgram(app *cli.App, tokens secret.Store, slots ...string) itemsOfAnotherProgram {
	items := itemsOfAnotherProgram{}
	for _, slot := range slots {
		items[slot] = true
	}
	app.Secrets = keychainStore[string]{backend: tokens, kind: secret.APIToken, items: items}
	app.Providers = keychainStore[secret.DiagnosticCredential]{backend: app.Providers, kind: secret.ProviderDiagnostic, items: items}
	return items
}

// printedFix finds, in what a command printed, the fix of a Keychain item of
// another program: the system tool's deletion, and the last step, which is a
// command line or the same command again.
var printedFix = regexp.MustCompile("delete the item with `([^`]+)`, then run (`[^`]+`|the same command again)\\.")

// fixSteps returns the two steps of the fix that a command printed: the
// deletion, and the words of the last step's command line, or none when the
// fix names the same command again. A command that prints its fix for a row
// and as its next step prints one fix.
func fixSteps(t *testing.T, out *bytes.Buffer) (deletion string, last []string) {
	t.Helper()
	found := printedFix.FindAllStringSubmatch(strings.Join(strings.Fields(out.String()), " "), -1)
	if len(found) == 0 {
		t.Fatalf("the output names no fix with a deletion and a last step:\n%s", out)
	}
	for _, fix := range found[1:] {
		if fix[0] != found[0][0] {
			t.Fatalf("the output names two fixes, %q and %q", found[0][0], fix[0])
		}
	}
	if line, printed := strings.CutPrefix(found[0][2], "`"); printed {
		last = strings.Fields(strings.TrimSuffix(line, "`"))
	}
	t.Logf("the printed fix deletes with %q and ends with %q", found[0][1], found[0][2])
	return found[0][1], last
}

// followLastStep runs the last step of a printed fix through the test's entry
// point, with input on standard input: the printed aigw command line, word
// for word, or again, the first command, when the fix names the same command.
func followLastStep(t *testing.T, app *cli.App, out *bytes.Buffer, last, again []string, input string) {
	t.Helper()
	args := again
	if last != nil {
		if last[0] != "aigw" {
			t.Fatalf("the last step %q is no aigw command line", last)
		}
		args = last[1:]
	}
	out.Reset()
	app.In = strings.NewReader(input)
	if err := cli.Execute(app, args); err != nil {
		t.Fatalf("the fix's last step %q failed: %v\n%s", args, err, out)
	}
	t.Logf("the fix's last step %q succeeded", args)
}

// fixtureToken is the Token that the tests of a printed fix give a command.
const fixtureToken = "fixture-token"

// The fix of a Keychain item of another program is followed as it is printed,
// once per kind of last step: the test removes the item that the printed
// deletion names, then runs the printed last step word for word. The step
// succeeds and stores the credential, and the command that met the refusal no
// longer meets it. `aigw provider add` stops before the provider is saved, so
// its last step is itself.
func TestTheFixOfARefusedProviderAddRunsAsPrinted(t *testing.T) {
	app, out, tokens, _, _ := testApp(t, fixtureToken+"\n")
	items := withItemsOfAnotherProgram(app, tokens, "team")
	args := []string{"provider", "add", "team", "--endpoint", "anthropic=https://team.test", "--token-stdin"}
	err := cli.Execute(app, args)
	requireRefusal(t, args, err, out, false, "team", refusalFix("team", "`aigw provider add team --endpoint anthropic=https://team.test --token-stdin`"))
	deletion, last := fixSteps(t, out)
	items.delete(t, deletion)
	followLastStep(t, app, out, last, args, fixtureToken+"\n")
	cfg, err := app.Config.Load()
	if stored, readErr := tokens.Get("team"); err != nil || cfg.Providers["team"].Endpoints.Anthropic != "https://team.test" || readErr != nil || stored != fixtureToken {
		t.Fatalf("after the fix: provider %+v, %v; Token stored %t, %v", cfg.Providers["team"], err, stored == fixtureToken, readErr)
	}
}

// `aigw setup --from` stops before it saves the provider of the team file, so
// its last step is itself.
func TestTheFixOfARefusedSetupRunsAsPrinted(t *testing.T) {
	app, out, tokens, _, _ := testApp(t, fixtureToken+"\n")
	items := withItemsOfAnotherProgram(app, tokens, "gateway")
	teamFile := gatewayTeamFile(t)
	args := []string{"setup", "--from", teamFile, "--provider", "gateway", "--token-stdin"}
	err := cli.Execute(app, args)
	requireRefusal(t, args, err, out, false, "gateway", refusalFix("gateway", setupLine(teamFile)))
	deletion, last := fixSteps(t, out)
	items.delete(t, deletion)
	followLastStep(t, app, out, last, args, fixtureToken+"\n")
	cfg, err := app.Config.Load()
	if stored, readErr := tokens.Get("gateway"); err != nil || cfg.Providers["gateway"].Label != "Gateway" || readErr != nil || stored != fixtureToken {
		t.Fatalf("after the fix: provider %+v, %v; Token stored %t, %v", cfg.Providers["gateway"], err, stored == fixtureToken, readErr)
	}
}

// A command that reads the Token of a configured provider names `aigw
// provider rotate`, which asks for the Token at a terminal. Afterwards the
// command reads the provider's catalog.
func TestTheFixOfARefusedTokenRunsAsPrinted(t *testing.T) {
	app, out, tokens, _, requests := testApp(t, "")
	saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
	requests.body = `{"data":[{"id":"claude-test"}]}`
	items := withItemsOfAnotherProgram(app, tokens, "one")
	args := []string{"model", "search", "--provider", "one"}
	err := cli.Execute(app, args)
	if text := strings.Join(strings.Fields(out.String()), " "); err == nil ||
		!strings.Contains(text, "Anthropic Messages: "+refusalTitle("one")) || !strings.Contains(text, "Fix: "+refusalFix("one", "`aigw provider rotate one`")) {
		t.Fatalf("%v = %v\n%s", args, err, out)
	}
	deletion, last := fixSteps(t, out)
	items.delete(t, deletion)
	prompt := &scriptedPrompt{secrets: []string{fixtureToken}}
	app.Interactive, app.Prompt = true, prompt
	followLastStep(t, app, out, last, nil, "")
	if stored, err := tokens.Get("one"); err != nil || stored != fixtureToken || len(prompt.secretCalls) != 1 {
		t.Fatalf("after the fix: Token stored %t, %v, after %d prompts", stored == fixtureToken, err, len(prompt.secretCalls))
	}
	out.Reset()
	if err := cli.Execute(app, args); err != nil || strings.Contains(out.String(), "Another program") || !strings.Contains(out.String(), "one/claude-test") {
		t.Fatalf("%v after the fix = %v\n%s", args, err, out)
	}
}

// A command that reads a provider's balance credentials names `aigw provider
// edit` with a user ID to fill in, which reads the balance token from
// standard input. Afterwards the command reports the balance.
func TestTheFixOfRefusedBalanceCredentialsRunsAsPrinted(t *testing.T) {
	app, out, tokens, _, requests := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["one"] = domain.Provider{Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://one.test"},
		BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://diagnostics.test"}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := tokens.Set("one", "sk-team-token-0001"); err != nil {
		t.Fatal(err)
	}
	requests.handler = balanceProbeAnswers
	items := withItemsOfAnotherProgram(app, tokens, "diagnostic@one")
	args := []string{"provider", "balance", "one"}
	err := cli.Execute(app, args)
	fix := "Have the balance credentials at hand, delete the item with `security delete-generic-password -s AIGW_TOKEN -a diagnostic@one`, " +
		"then run `aigw provider edit one --balance-token-stdin --balance-user-id <user-id>`."
	if text := strings.Join(strings.Fields(out.String()), " "); err == nil ||
		!strings.Contains(text, "Another program stored provider one's balance credentials in the Keychain") || !strings.Contains(text, "Fix: "+fix) {
		t.Fatalf("%v = %v\n%s", args, err, out)
	}
	deletion, last := fixSteps(t, out)
	items.delete(t, deletion)
	placeholder := slices.Index(last, "<user-id>")
	if placeholder < 0 {
		t.Fatalf("the last step %q names no user ID to fill in", last)
	}
	last[placeholder] = "7"
	followLastStep(t, app, out, last, nil, "synthetic-balance-token\n")
	if stored, err := app.Providers.Get("one"); err != nil || stored != (secret.DiagnosticCredential{SystemToken: "synthetic-balance-token", UserID: "7"}) {
		t.Fatalf("after the fix: balance credentials stored %t, %v", err == nil && stored.UserID == "7", err)
	}
	out.Reset()
	if err := cli.Execute(app, args); err != nil || strings.Contains(out.String(), "Another program") || !strings.Contains(out.String(), "Remaining quota") {
		t.Fatalf("%v after the fix = %v\n%s", args, err, out)
	}
}

// readerCheckedKeychain is a keychain whose item of another program fails
// where a rollback observes a Token, as the reader check of the system keyring
// fails for it; an in-memory store has no reader to check.
type readerCheckedKeychain struct{ keychainStore[string] }

func (store readerCheckedKeychain) Exists(provider string) (bool, error) {
	if err := store.refusal("read", provider); err != nil {
		return false, err
	}
	return store.keychainStore.Exists(provider)
}

// A configuration rollback checks the Tokens of the configuration that it
// restores. `aigw provider rotate` knows the providers of the current
// configuration alone, so it is the last step only for a provider that this
// configuration holds; for a provider that only the restored configuration
// holds, the last step is the rollback again, which then restores the
// configuration and leaves that provider's client for its Token.
func TestTheFixOfARefusedConfigRollbackRunsAsPrinted(t *testing.T) {
	endpoints := domain.Endpoints{Anthropic: "https://one.test"}
	for _, test := range []struct {
		name string
		// change is the last change, whose backup the rollback restores.
		change func(cfg *domain.Config, installed string)
		last   string
		// again says that the last step is another command, after which the
		// rollback itself runs.
		again bool
	}{
		{name: "a provider that the current configuration holds", last: "`aigw provider rotate one`", again: true,
			change: func(cfg *domain.Config, installed string) {
				cfg.SetSelectedModel(claude.ID, addProviderModel(cfg, "one", "One", endpoints, claude.ID, "claude-next"), "")
				cfg.SetClientActivation(claude.ID, true, installed, nil)
			}},
		{name: "a provider that only the restored configuration holds", last: "`aigw config rollback --last-change`",
			change: func(cfg *domain.Config, _ string) { *cfg = domain.NewConfig(clients.Admission()) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, out, tokens, _, _ := testApp(t, "")
			bindCredentialReader(t, app)
			installed := executableFixture(t, "claude")
			app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: installed}}}
			cfg := domain.NewConfig(clients.Admission())
			first := addProviderModel(&cfg, "one", "One", endpoints, claude.ID, "claude-test")
			cfg.SetSelectedModel(claude.ID, first, "")
			cfg.SetClientActivation(claude.ID, true, installed, nil)
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			test.change(&cfg, installed)
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			items := itemsOfAnotherProgram{"one": true}
			app.Secrets = readerCheckedKeychain{keychainStore[string]{backend: tokens, kind: secret.APIToken, items: items}}

			args := []string{"config", "rollback", "--last-change"}
			err := cli.Execute(app, args)
			requireRefusal(t, args, err, out, false, "one", refusalFix("one", test.last))
			deletion, last := fixSteps(t, out)
			items.delete(t, deletion)
			prompt := &scriptedPrompt{secrets: []string{fixtureToken}}
			app.Interactive, app.Prompt = true, prompt
			followLastStep(t, app, out, last, args, "")

			if test.again {
				// Followed once, the fix lets the same command run.
				out.Reset()
				if err := cli.Execute(app, args); err != nil {
					t.Fatalf("%v after the fix = %v\n%s", args, err, out)
				}
			}
			restored, err := app.Config.Load()
			if err != nil || restored.SelectedModel(claude.ID) != first {
				t.Fatalf("after the fix the configuration selects %v, %v; want %v", restored.SelectedModel(claude.ID), err, first)
			}
		})
	}
}

// balanceProbeAnswers answers the DMXAPI balance probe's two requests for an
// account whose one Token is the test provider's.
func balanceProbeAnswers(request *http.Request) (*http.Response, error) {
	body := `{"success":true,"data":{"quota":5000000}}`
	if request.URL.Path == "/api/token/search" {
		body = `{"success":true,"data":{"page_size":100,"items":[{"name":"team","key":"team**********0001","status":1,"used_quota":500000,"remain_quota":2500000,"remain_count":42}]}}`
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
}
