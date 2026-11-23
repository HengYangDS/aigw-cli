package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

// claudeOnOne saves a configuration in which Claude Code selects
// one/claude-test of provider one, which also has claude-next added, over a
// keychain whose worker is the program's own, as the product's is: a command
// that writes Claude Code's file installs the credential reader and checks it
// against the same items. With enabled, Claude Code is enabled and its file
// is in place. slots name the items that another program stored. It returns
// the path of the installed client.
func claudeOnOne(t *testing.T, enabled bool, slots ...string) (app *cli.App, out *bytes.Buffer, items *keychain, installed string) {
	t.Helper()
	app, out, _, _, _ = testApp(t, "")
	items = withItemsOfAnotherProgram(t, app, slots...)
	items.bindReader(app)
	installed = executableFixture(t, "claude")
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: installed}}}
	cfg := domain.NewConfig(clients.Admission())
	endpoints := domain.Endpoints{Anthropic: "https://one.test"}
	addProviderModel(&cfg, "one", "One", endpoints, claude.ID, "claude-next")
	cfg.SetSelectedModel(claude.ID, addProviderModel(&cfg, "one", "One", endpoints, claude.ID, "claude-test"), "")
	if enabled {
		cfg.SetClientActivation(claude.ID, true, installed, nil)
		synchronizeClaudeProjection(t, app, cfg)
	}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return app, out, items, installed
}

// claudeFile returns Claude Code's settings file, or nothing when there is
// none.
func claudeFile(t *testing.T, app *cli.App) []byte {
	t.Helper()
	data, err := os.ReadFile(claudeSettingsPath(t, app.Host))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return data
}

// requireObservedOnly requires that a command asked the worker whether
// provider one's item is there and whether it is empty, once each, and ran
// nothing else: no read, no write and no reader.
func requireObservedOnly(t *testing.T, items *keychain, args []string) {
	t.Helper()
	if operations := items.operations(t); !slices.Equal(operations, []string{"exists one", "empty one"}) {
		t.Fatalf("%v ran %q on the keychain; want one observation and one emptiness answer", args, operations)
	}
}

// `aigw use` and `aigw client enable` each point one client at its provider's
// Token, so each needs that Token for its own purpose. A Keychain item that
// another program stored is no Token that the client could read: the command
// ends with the item's refusal, in text, as JSON and as a preview, before it
// changes the configuration or a client file and without a read of the Token.
// The provider is configured, so the fix ends with `aigw provider rotate`.
// Followed as printed, it lets the same command run: it asks once whether
// the item is there and whether it is empty, and the credential reader that
// it installs then reads the Token.
func TestACommandForOneClientEndsWithTheRefusalOfItsProvidersItem(t *testing.T) {
	for _, command := range []struct {
		name    string
		enabled bool
		args    func(installed string) []string
		// selected is what Claude Code's file names once the command ran.
		selected string
	}{
		{name: "use", enabled: true, selected: "claude-next",
			args: func(string) []string { return []string{"use", claude.ID, "one/claude-next"} }},
		{name: "client enable", selected: "claude-test",
			args: func(installed string) []string {
				return []string{"client", "enable", claude.ID, "--executable", installed}
			}},
	} {
		for _, flag := range []string{"", "--json", "--dry-run"} {
			t.Run(strings.TrimSpace(command.name+" "+flag), func(t *testing.T) {
				app, out, items, installed := claudeOnOne(t, command.enabled, "one")
				configuration, file := readFile(t, app.Config.Path()), claudeFile(t, app)
				args := command.args(installed)
				if flag != "" {
					args = append(args, flag)
				}
				err := cli.Execute(app, args)
				requireRefusal(t, args, err, out, flag == "--json", "one", refusalFix("one", "`aigw provider rotate one`"))
				requireObservedOnly(t, items, args)
				if !bytes.Equal(readFile(t, app.Config.Path()), configuration) || !bytes.Equal(claudeFile(t, app), file) {
					t.Fatalf("%v changed the configuration or Claude Code's file", args)
				}
				if flag != "" {
					return
				}
				storeTokenAsPrinted(t, app, out, items)
				fixed := len(items.operations(t))
				out.Reset()
				if err := cli.Execute(app, args); err != nil {
					t.Fatalf("%v after the fix = %v\n%s", args, err, out)
				}
				cfg, err := app.Config.Load()
				if err != nil || !cfg.Clients[claude.ID].Enabled || cfg.SelectedModel(claude.ID).ID() != command.selected ||
					!bytes.Contains(claudeFile(t, app), []byte(command.selected)) {
					t.Fatalf("%v after the fix: Claude Code enabled %t on %v, %v; its file:\n%s", args, cfg.Clients[claude.ID].Enabled, cfg.SelectedModel(claude.ID), err, claudeFile(t, app))
				}
				if operations := items.operations(t)[fixed:]; !slices.Equal(operations, []string{"exists one", "empty one", "reader read one"}) {
					t.Fatalf("%v after the fix ran %q; want one observation, one emptiness answer and the reader's read of the Token", args, operations)
				}
			})
		}
	}
}

// A Token slot that is there and holds nothing is a missing Token to both
// commands. Without a terminal `aigw use` names the step that stores one, and
// in a terminal it asks for one and stores it with the selection. `aigw
// client enable` says that the provider is missing a Token. Neither reads a
// Token or installs a reader for one that is not there.
func TestACommandForOneClientCountsAnEmptyTokenAsMissing(t *testing.T) {
	use := []string{"use", claude.ID, "one/claude-next"}
	for _, test := range []struct {
		name    string
		enabled bool
		args    func(installed string) []string
		problem string
		fix     string
	}{
		{name: "use", enabled: true, args: func(string) []string { return use },
			problem: "provider one has no Token, and aigw use asks for one only in a terminal",
			fix:     "Pipe the Token into `aigw provider rotate one --token-stdin`, then run `aigw use claude one/claude-next`."},
		{name: "use as json", enabled: true, args: func(string) []string { return append(slices.Clone(use), "--json") },
			problem: "provider one has no Token, and aigw use --json never prompts for one",
			fix:     "Pipe the Token into `aigw provider rotate one --token-stdin`, then run `aigw use claude one/claude-next`."},
		{name: "client enable",
			args: func(installed string) []string {
				return []string{"client", "enable", claude.ID, "--executable", installed}
			},
			problem: `provider "one" is missing a Token; run ` + "`aigw provider rotate one`"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, out, items, installed := claudeOnOne(t, test.enabled)
			items.hold(t, "one", "")
			configuration, file := readFile(t, app.Config.Path()), claudeFile(t, app)
			args := test.args(installed)
			err := cli.Execute(app, args)
			if text := strings.Join(strings.Fields(out.String()), " "); err == nil || !strings.Contains(text, test.problem) || !strings.Contains(text, test.fix) {
				t.Fatalf("%v = %v\n%s", args, err, out)
			}
			requireObservedOnly(t, items, args)
			if !bytes.Equal(readFile(t, app.Config.Path()), configuration) || !bytes.Equal(claudeFile(t, app), file) {
				t.Fatalf("%v changed the configuration or Claude Code's file", args)
			}
		})
	}

	t.Run("use in a terminal", func(t *testing.T) {
		app, out, items, _ := claudeOnOne(t, true)
		items.hold(t, "one", "")
		prompt := &scriptedPrompt{secrets: []string{fixtureToken}}
		app.Interactive, app.Prompt = true, prompt
		if err := cli.Execute(app, use); err != nil {
			t.Fatalf("%v = %v\n%s", use, err, out)
		}
		operations := items.operations(t)
		cfg, err := app.Config.Load()
		stored, storeErr := items.Get("one")
		if err != nil || storeErr != nil || stored != fixtureToken || len(prompt.secretCalls) != 1 || cfg.SelectedModel(claude.ID).ID() != "claude-next" ||
			!bytes.Contains(claudeFile(t, app), []byte("claude-next")) {
			t.Fatalf("%v: Token stored %t, %v, after %d prompts; Claude Code selects %v, %v\n%s",
				use, stored == fixtureToken, storeErr, len(prompt.secretCalls), cfg.SelectedModel(claude.ID), err, out)
		}
		if !slices.Contains(operations, "write one") || !slices.Contains(operations, "reader read one") {
			t.Fatalf("%v ran %q; want the Token stored and read by the reader", use, operations)
		}
	})
}

// codexOnOne saves a configuration in which Codex, on provider one, is enabled
// with its file, over a keychain whose worker is the program's own. slots name
// the items that another program stored. It returns Codex's file and a second
// file, to which an edit can move Codex.
func codexOnOne(t *testing.T, slots ...string) (app *cli.App, out *bytes.Buffer, items *keychain, target, moved string) {
	t.Helper()
	app, out, _, _, _ = testApp(t, "")
	items = withItemsOfAnotherProgram(t, app, slots...)
	items.bindReader(app)
	target = guidedCodexTarget(t, app)
	cfg := domain.NewConfig(clients.Admission())
	cfg.SetSelectedModel(codex.ID, addProviderModel(&cfg, "one", "One", domain.Endpoints{Responses: "https://one.test/v1"}, codex.ID, "gpt-test"), "")
	cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), []string{target})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	moved = filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, moved, []byte("approval_policy = \"never\"\n"), 0o600)
	return app, out, items, target, moved
}

// `aigw client edit <client> --target` moves the configuration files of one
// enabled client, whose provider's Token the client needs. Without one that
// AIGW can use, a Keychain item that another program stored or an empty
// Token, the edit is saved all the same: AIGW withdraws its settings and
// record from the file that the edit drops, which needs no Token, the file
// that Codex moves to waits for the Token, and the edit's next step is the
// Token's own fix, in text and as JSON: the item's, or the step that stores a
// missing Token. The edit observes the Token and reads none. Followed as
// printed, the item's fix lets `aigw sync` write the file that Codex moved to.
func TestAClientEditWithdrawsTheDroppedFileWhileAIGWCannotUseTheToken(t *testing.T) {
	for _, state := range []string{"an item of another program", "an empty Token"} {
		for _, flag := range []string{"", "--json"} {
			t.Run(strings.TrimSpace(state+" "+flag), func(t *testing.T) {
				editCodexTargetWithoutAUsableToken(t, state == "an item of another program", flag)
			})
		}
	}
}

func editCodexTargetWithoutAUsableToken(t *testing.T, refused bool, flag string) {
	t.Helper()
	var slots []string
	if refused {
		slots = []string{"one"}
	}
	app, out, items, target, moved := codexOnOne(t, slots...)
	if !refused {
		items.hold(t, "one", "")
	}
	native, other := readFile(t, target), readFile(t, moved)
	args := []string{"client", "edit", codex.ID, "--target", moved}
	if flag != "" {
		args = append(args, flag)
	}
	if err := cli.Execute(app, args); err != nil {
		t.Fatalf("%v = %v\n%s", args, err, out)
	}
	fix := "aigw provider rotate one"
	if refused {
		fix = refusalFix("one", "`aigw provider rotate one`")
	}
	if text := strings.Join(strings.Fields(out.String()), " "); !strings.Contains(text, fix) || strings.Contains(text, fixtureToken) {
		t.Fatalf("%v names no next step %q, or shows the Token:\n%s", args, fix, out)
	}
	requireObservedOnly(t, items, args)
	cfg, err := app.Config.Load()
	if err != nil || !slices.Equal(cfg.Clients[codex.ID].Targets, []string{moved}) {
		t.Fatalf("%v saved Codex's files %q, %v; want the file that it moves to", args, cfg.Clients[codex.ID].Targets, err)
	}
	if bytes.Contains(readFile(t, target), []byte("one.test")) || !bytes.Equal(readFile(t, target), native) || !bytes.Equal(readFile(t, moved), other) {
		t.Fatalf("%v wrote a file of Codex without a Token that AIGW can use", args)
	}
	if !refused || flag != "" {
		return
	}
	storeTokenAsPrinted(t, app, out, items)
	out.Reset()
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync after the fix = %v\n%s", err, out)
	}
	if !bytes.Contains(readFile(t, moved), []byte("https://one.test/v1")) {
		t.Fatalf("sync after the fix left the file that Codex moved to:\n%s", readFile(t, moved))
	}
}
