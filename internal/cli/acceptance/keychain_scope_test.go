package cli_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// askedTwice returns the facts of a credential's item, whether it is there
// and whether it is empty, that the worker was asked a second time without a
// write or a deletion of that item since the first: a command asks each fact
// once, and again only after it changed the item itself. The item is a
// provider's Token or, as diagnostic@<provider>, its balance credentials. The
// credential reader's operations are a client's.
func askedTwice(operations []string) []string {
	asked := map[string]bool{}
	var twice []string
	for _, operation := range operations {
		name, slot, _ := strings.Cut(operation, " ")
		switch name {
		case "write", "delete":
			delete(asked, "exists "+slot)
			delete(asked, "empty "+slot)
		case "exists", "empty":
			if asked[operation] {
				twice = append(twice, operation)
			}
			asked[operation] = true
		}
	}
	return twice
}

// preparation makes the app, its keychain and the arguments of one command
// line that the table of TestACommandAsksEachFactOfACredentialOnce runs.
type preparation func(t *testing.T) (*cli.App, *bytes.Buffer, *keychain, []string)

// hermesOnOneListingTwo saves a configuration in which Hermes, enabled on
// provider one, lists beside it provider two and its two models, over a
// keychain whose worker is the program's own and holds both Tokens. A change
// of provider two or of its models changes Hermes's file, so the command
// that makes it observes both Tokens.
func hermesOnOneListingTwo(t *testing.T) (*cli.App, *bytes.Buffer, *keychain) {
	t.Helper()
	app, out, _, _, _ := testApp(t, "")
	items := withItemsOfAnotherProgram(t, app)
	items.bindReader(app)
	items.hold(t, "one", fixtureToken)
	items.hold(t, "two", fixtureToken)
	target := filepath.Join(t.TempDir(), "hermes", "config.yaml")
	installed := executableFixture(t, "hermes")
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{hermes.ID: installed},
		Surfaces:    []discovery.Surface{{ID: "hermes-home-default", ConfigPath: target}},
	}}
	cfg := domain.NewConfig(clients.Admission())
	cfg.SetSelectedModel(hermes.ID, addProviderModel(&cfg, "one", "One", domain.Endpoints{Responses: "https://one.test/v1"}, hermes.ID, "gpt-one"), "")
	for _, model := range []string{"gpt-two", "gpt-spare"} {
		addProviderModel(&cfg, "two", "Two", domain.Endpoints{Responses: "https://two.test/v1"}, hermes.ID, model)
	}
	cfg.SetClientActivation(hermes.ID, true, installed, []string{target})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return app, out, items
}

// onHermes runs args over hermesOnOneListingTwo.
func onHermes(args ...string) preparation {
	return func(t *testing.T) (*cli.App, *bytes.Buffer, *keychain, []string) {
		app, out, items := hermesOnOneListingTwo(t)
		return app, out, items, args
	}
}

// balanceCredentials is what the keychain holds for a provider's balance
// credentials.
const balanceCredentials = `{"system_token":"synthetic-balance-token","user_id":"7"}`

// withBalanceCredentials runs args over hermesOnOneListingTwo in which each
// of the two providers has a balance probe, with a keychain that also holds
// the balance credentials of both.
func withBalanceCredentials(args ...string) preparation {
	return func(t *testing.T) (*cli.App, *bytes.Buffer, *keychain, []string) {
		app, out, items := hermesOnOneListingTwo(t)
		cfg, err := app.Config.Load()
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"one", "two"} {
			provider := cfg.Providers[id]
			provider.BalanceProbe = &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://diagnostics.test"}
			cfg.Providers[id] = provider
			items.hold(t, "diagnostic@"+id, balanceCredentials)
		}
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		return app, out, items, args
	}
}

// claudeOnOneWithItsToken is claudeOnOne with Claude Code enabled and its
// file in place, over a keychain that holds provider one's Token.
func claudeOnOneWithItsToken(t *testing.T) (*cli.App, *bytes.Buffer, *keychain) {
	t.Helper()
	app, out, items, _ := claudeOnOne(t, true)
	items.hold(t, "one", fixtureToken)
	return app, out, items
}

// onClaude runs args over claudeOnOneWithItsToken.
func onClaude(args ...string) preparation {
	return func(t *testing.T) (*cli.App, *bytes.Buffer, *keychain, []string) {
		app, out, items := claudeOnOneWithItsToken(t)
		return app, out, items, args
	}
}

// credentialCommand is one command line that the table of
// TestACommandAsksEachFactOfACredentialOnce runs.
type credentialCommand struct {
	name string
	// flags are the forms of the command beside its plain one that reach the
	// credential store by another path: --json where the command prompts or
	// reports differently, and --dry-run where the preview is not a part of
	// the change.
	flags []string
	// stdin is what the command reads from standard input: the Token or the
	// balance token that it stores.
	stdin string
	// asks are questions that the command asks the worker, each `exists` and
	// the item: a provider's ID for its Token and diagnostic@<provider> for
	// its balance credentials. None is a command that asks no fact at all.
	asks    []string
	prepare preparation
}

// run runs the command with flag, or plainly without one, and requires that
// the worker was asked the command's questions and no fact a second time.
func (command credentialCommand) run(t *testing.T, flag string) {
	app, out, items, args := command.prepare(t)
	if flag != "" {
		args = append(slices.Clone(args), flag)
	}
	app.In = strings.NewReader(command.stdin)
	if err := cli.Execute(app, args); err != nil {
		t.Fatalf("%v = %v\n%s", args, err, out)
	}
	operations := items.operations(t)
	t.Logf("%v ran %q on the keychain", args, operations)
	if twice := askedTwice(operations); len(twice) != 0 {
		t.Fatalf("%v asked %q a second time; the keychain ran %q", args, twice, operations)
	}
	// A row that asked nothing would prove nothing, unless its command is
	// one that asks nothing.
	if len(command.asks) == 0 && slices.ContainsFunc(operations, func(operation string) bool {
		return strings.HasPrefix(operation, "exists ") || strings.HasPrefix(operation, "empty ")
	}) {
		t.Fatalf("%v ran %q on the keychain; want no fact of a credential asked", args, operations)
	}
	for _, question := range command.asks {
		if !slices.Contains(operations, question) {
			t.Fatalf("%v ran %q on the keychain; want it to ask %q", args, operations, question)
		}
	}
}

// tokenCommands are the commands that observe a provider's Token.
// `aigw provider rotate` reads and replaces the Token and asks no fact of it.
// A model command observes a Token only where a client's file lists the
// model's provider, as Hermes's does, and `aigw model add` without --unlisted
// reads the provider's catalog first, which the scripted provider answers.
func tokenCommands() []credentialCommand {
	return []credentialCommand{
		{name: "provider edit", asks: []string{"exists one"},
			prepare: onClaude("provider", "edit", "one", "--endpoint", "anthropic=https://other.test")},
		{name: "provider remove", asks: []string{"exists one"}, prepare: onHermes("provider", "remove", "two")},
		{name: "provider rotate", stdin: fixtureToken + "\n", prepare: onHermes("provider", "rotate", "two", "--token-stdin")},
		{name: "provider add", stdin: fixtureToken + "\n", asks: []string{"exists team"},
			prepare: onHermes("provider", "add", "team", "--endpoint", "responses=https://team.test/v1", "--token-stdin")},
		{name: "model add", asks: []string{"exists two"}, prepare: onHermes("model", "add", "two/gpt-new", "--unlisted")},
		{name: "model add of a model that the catalog lists", asks: []string{"exists two"},
			prepare: func(t *testing.T) (*cli.App, *bytes.Buffer, *keychain, []string) {
				app, out, items := hermesOnOneListingTwo(t)
				app.HTTP = &fakeHTTP{status: 200, body: `{"data":[{"id":"gpt-new"}]}`}
				return app, out, items, []string{"model", "add", "two/gpt-new"}
			}},
		{name: "model remove", asks: []string{"exists two"}, prepare: onHermes("model", "remove", "two/gpt-spare")},
		{name: "use", flags: []string{"--json", "--dry-run"}, asks: []string{"exists two"}, prepare: onHermes("use", hermes.ID, "two/gpt-two")},
		{name: "sync", flags: []string{"--dry-run"}, asks: []string{"exists one"}, prepare: onHermes("sync")},
		{name: "status", flags: []string{"--json"}, asks: []string{"exists one"}, prepare: onHermes("status")},
		{name: "bare aigw", asks: []string{"exists one"}, prepare: onHermes()},
		{name: "check", flags: []string{"--json"}, asks: []string{"exists one"}, prepare: onClaude("check", "--depth", "local")},
		{name: "client enable", flags: []string{"--dry-run"}, asks: []string{"exists one"},
			prepare: func(t *testing.T) (*cli.App, *bytes.Buffer, *keychain, []string) {
				app, out, items, installed := claudeOnOne(t, false)
				items.hold(t, "one", fixtureToken)
				return app, out, items, []string{"client", "enable", claude.ID, "--program", installed}
			}},
		{name: "client edit", asks: []string{"exists one"},
			prepare: func(t *testing.T) (*cli.App, *bytes.Buffer, *keychain, []string) {
				app, out, items, _, moved := codexOnOne(t)
				items.hold(t, "one", fixtureToken)
				return app, out, items, []string{"client", "edit", codex.ID, "--target", moved}
			}},
		{name: "config rollback", flags: []string{"--dry-run"}, asks: []string{"exists one"},
			prepare: func(t *testing.T) (*cli.App, *bytes.Buffer, *keychain, []string) {
				app, out, items := hermesOnOneListingTwo(t)
				cfg, err := app.Config.Load()
				if err != nil {
					t.Fatal(err)
				}
				cfg.SetSelectedModel(hermes.ID, domain.NewModelRef("two", "gpt-two"), "")
				if err := app.Config.Save(cfg); err != nil {
					t.Fatal(err)
				}
				return app, out, items, []string{"config", "rollback", "--last-change"}
			}},
		{name: "setup", flags: []string{"--json", "--dry-run"}, stdin: fixtureToken + "\n", asks: []string{"exists gateway"},
			prepare: func(t *testing.T) (*cli.App, *bytes.Buffer, *keychain, []string) {
				app, out, items := hermesOnOneListingTwo(t)
				return app, out, items, []string{"setup", "--from", gatewayTeamFile(t), "--provider", "gateway", "--token-stdin"}
			}},
		// Guided setup stores the Token that it asks for, observes it again
		// for the client's file, and ends with the client's check, which is
		// a part of the same command.
		{name: "guided setup", asks: []string{"exists team"},
			prepare: func(t *testing.T) (*cli.App, *bytes.Buffer, *keychain, []string) {
				app, out, _, _, _ := testApp(t, "")
				items := withItemsOfAnotherProgram(t, app)
				items.bindReader(app)
				app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: executableFixture(t, "claude")}}}
				app.Interactive, app.Prompt = true, &scriptedPrompt{
					selections: []string{string(domain.ProtocolAnthropic)},
					texts:      []string{"team", "Team", "https://team.test", "claude-test"},
					secrets:    []string{fixtureToken},
				}
				return app, out, items, []string{"setup"}
			}},
	}
}

// balanceCommands are the commands that observe, read or change a provider's
// balance credentials. `aigw provider edit` reads and replaces or removes
// them, and `aigw provider balance` reads them and the Token for the balance
// probe, which the scripted provider answers; neither asks a fact. `aigw
// status` shows them in text alone.
func balanceCommands() []credentialCommand {
	return []credentialCommand{
		{name: "provider remove of a provider with balance credentials", asks: []string{"exists two", "exists diagnostic@two"},
			prepare: withBalanceCredentials("provider", "remove", "two")},
		{name: "provider remove of credentials without a provider", flags: []string{"--dry-run"},
			asks: []string{"exists leftover", "exists diagnostic@leftover"},
			prepare: func(t *testing.T) (*cli.App, *bytes.Buffer, *keychain, []string) {
				app, out, items := hermesOnOneListingTwo(t)
				items.hold(t, "leftover", fixtureToken)
				items.hold(t, "diagnostic@leftover", balanceCredentials)
				return app, out, items, []string{"provider", "remove", "leftover"}
			}},
		{name: "provider edit that stores balance credentials", stdin: "balance-token\n",
			prepare: withBalanceCredentials("provider", "edit", "two", "--balance-token-stdin", "--balance-user-id", "7")},
		{name: "provider edit that removes balance credentials",
			prepare: withBalanceCredentials("provider", "edit", "two", "--no-balance-token")},
		{name: "provider balance",
			prepare: func(t *testing.T) (*cli.App, *bytes.Buffer, *keychain, []string) {
				app, out, items, args := withBalanceCredentials("provider", "balance", "two")(t)
				// The scripted probe reports the quota of this Token.
				items.hold(t, "two", "sk-team-token-0001")
				app.HTTP = &fakeHTTP{status: 200, handler: balanceProbeAnswers}
				return app, out, items, args
			}},
		{name: "provider show", flags: []string{"--json"}, asks: []string{"exists two", "exists diagnostic@two"},
			prepare: withBalanceCredentials("provider", "show", "two")},
		{name: "status of a provider with a balance probe", asks: []string{"exists one", "exists diagnostic@one"},
			prepare: withBalanceCredentials("status")},
	}
}

// Each command asks the credential store each fact of a provider's Token and
// of its balance credentials once: its admission, its preview, its change and
// its next step share one reading, whichever of them the command has. The
// worker records what it was asked, so the rule is a count and not an
// estimate.
func TestACommandAsksEachFactOfACredentialOnce(t *testing.T) {
	t.Parallel()
	for _, command := range slices.Concat(tokenCommands(), balanceCommands()) {
		for _, flag := range append([]string{""}, command.flags...) {
			t.Run(strings.TrimSpace(command.name+" "+flag), func(t *testing.T) { command.run(t, flag) })
		}
	}
}

// useInATerminal is claudeOnOne with Claude Code enabled, at a terminal whose
// prompt answers with the fixture Token, and the selection that moves Claude
// Code to the provider's other model. The keychain holds no Token.
func useInATerminal(t *testing.T) (*cli.App, *bytes.Buffer, *keychain, []string) {
	t.Helper()
	app, out, items, _ := claudeOnOne(t, true)
	app.Interactive, app.Prompt = true, &scriptedPrompt{secrets: []string{fixtureToken}}
	return app, out, items, []string{"use", claude.ID, "one/claude-next"}
}

// A reading holds until the command changes the Token itself. `aigw use` in a
// terminal finds no Token, asks for one and stores it with the selection: it
// then asks the worker again, finds the Token that it stored, and writes
// Claude Code's file, whose reader reads that Token.
func TestACommandAsksAgainAboutTheTokenThatItStored(t *testing.T) {
	t.Parallel()
	app, out, items, args := useInATerminal(t)
	if err := cli.Execute(app, args); err != nil {
		t.Fatalf("%v = %v\n%s", args, err, out)
	}
	want := []string{"exists one", "read one", "write one", "exists one", "empty one", "reader read one"}
	if operations := items.operations(t); !slices.Equal(operations, want) {
		t.Fatalf("%v ran %q on the keychain; want %q", args, operations, want)
	}
	if !bytes.Contains(claudeFile(t, app), []byte("claude-next")) || !strings.Contains(out.String(), "Validated and stored") {
		t.Fatalf("%v did not write Claude Code's file for the stored Token:\n%s", args, out)
	}
}

// savingDiscovery is a discovery that saves a configuration when a command
// discovers its clients, as another command would while this one runs.
type savingDiscovery struct {
	discovery.Discoverer
	save func()
}

func (d savingDiscovery) Discover(clients ...string) discovery.Result {
	d.save()
	return d.Discoverer.Discover(clients...)
}

// A selection whose commit fails after it stored the Token removes the Token
// again, and then asks the credential store once more: the answer that it had
// after it stored the Token no longer holds, so the failure says that the
// Token was not kept. That holds over a system keyring that is named, whose
// worker records the questions, and over the backend that AIGW chooses when
// none is named, whose removal goes to the chosen backend itself: the system
// keyring where its probe passes, which is the product's default, and the
// file backend where it fails.
func TestACommandAsksAgainAboutTheTokenThatItRemoved(t *testing.T) {
	t.Parallel()
	const named, chosenKeyring, chosenFile = "the system keyring", "the system keyring that AIGW chooses", "the file backend that AIGW chooses"
	for _, backend := range []string{named, chosenKeyring, chosenFile} {
		t.Run(backend, func(t *testing.T) {
			app, out, items, args := useInATerminal(t)
			if backend != named {
				selection := secret.Selection{
					GOOS: runtime.GOOS, Root: filepath.Join(t.TempDir(), "secrets"), Executable: items.worker(),
					KeyringProbe: func(secret.Store) error { return nil },
				}
				if backend == chosenFile {
					selection.KeyringProbe = func(secret.Store) error { return errors.New("no system keyring") }
				}
				chosen, err := secret.Select(selection)
				if err != nil {
					t.Fatal(err)
				}
				app.Secrets = chosen
			}
			newer, err := app.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			provider := newer.Providers["one"]
			provider.Label = "One (renamed elsewhere)"
			newer.Providers["one"] = provider
			app.Discovery = savingDiscovery{Discoverer: app.Discovery, save: func() {
				if err := app.Config.Save(newer); err != nil {
					t.Fatal(err)
				}
			}}
			err = cli.Execute(app, args)
			if text := strings.Join(strings.Fields(out.String()), " "); err == nil || !strings.Contains(text, "The Token that you entered was not kept.") {
				t.Fatalf("%v = %v\n%s", args, err, out)
			}
			operations := items.operations(t)
			if secretExists(t, app.Secrets, "one") {
				t.Fatalf("%v kept the Token of a selection that failed", args)
			}
			if backend == chosenFile {
				return
			}
			want := []string{"exists one", "read one", "write one", "exists one", "empty one", "read one", "delete one", "exists one"}
			if !slices.Equal(operations, want) {
				t.Fatalf("%v ran %q on the keychain; want %q: the Token observed after it was stored, and again after it was removed", args, operations, want)
			}
		})
	}
}

// A reading ends with its command. Two commands that run one after the other
// in one process each ask the worker, and the second reports what the
// keychain holds then: an item that lost its Token between them.
func TestTheNextCommandAsksTheWorkerAgain(t *testing.T) {
	t.Parallel()
	app, out, items := claudeOnOneWithItsToken(t)
	check := []string{"check", "--depth", "local"}
	if err := cli.Execute(app, check); err != nil {
		t.Fatalf("%v = %v\n%s", check, err, out)
	}
	first := items.operations(t)
	items.hold(t, "one", "")
	out.Reset()
	err := cli.Execute(app, check)
	if text := strings.Join(strings.Fields(out.String()), " "); err == nil || !strings.Contains(text, "aigw provider rotate one") {
		t.Fatalf("%v over an item that lost its Token = %v\n%s", check, err, out)
	}
	second := items.operations(t)[len(first):]
	for _, operations := range [][]string{first, second} {
		if twice := askedTwice(operations); len(twice) != 0 || !slices.Contains(operations, "exists one") || !slices.Contains(operations, "empty one") {
			t.Fatalf("the first %v ran %q on the keychain and the second %q; want each to ask each fact of the Token once", check, first, second)
		}
	}
}
