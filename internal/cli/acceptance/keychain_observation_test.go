package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// claudeOverAnItemOfAnotherProgram configures Claude Code on provider one,
// with its client files in place, over a keychain whose item for one another
// program stored.
func claudeOverAnItemOfAnotherProgram(t *testing.T) (*cli.App, *bytes.Buffer, *keychain) {
	t.Helper()
	app, out, _, _, _ := testApp(t, "")
	items := withItemsOfAnotherProgram(t, app, "one")
	installed := executableFixture(t, "claude")
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: installed}}}
	cfg := domain.NewConfig(clients.Admission())
	cfg.SetSelectedModel(claude.ID, addProviderModel(&cfg, "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test"), "")
	cfg.SetClientActivation(claude.ID, true, installed, nil)
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return app, out, items
}

// requireNoTokenRead fails when a command read, wrote or deleted an item of
// the keychain: an observation asks the worker whether an item is there and
// whether it is empty, and names the items, and nothing else.
func requireNoTokenRead(t *testing.T, items *keychain, args []string) {
	t.Helper()
	for _, operation := range items.operations(t) {
		if name, _, _ := strings.Cut(operation, " "); !slices.Contains([]string{"exists", "empty", "list"}, name) {
			t.Fatalf("%v ran %q on the keychain: %q", args, operation, items.operations(t))
		}
	}
}

// storeTokenAsPrinted follows the fix that out shows for a provider's item:
// the fixture removes the item that the printed deletion names, and the
// printed steps run word for word, with the Token at a scripted prompt. The
// Token is then stored for the provider whose item the deletion names.
func storeTokenAsPrinted(t *testing.T, app *cli.App, out *bytes.Buffer, items *keychain) {
	t.Helper()
	deletion, steps := fixSteps(t, out)
	items.delete(t, deletion)
	provider := deletion[strings.LastIndex(deletion, " ")+1:]
	prompt := &scriptedPrompt{secrets: []string{fixtureToken}}
	interactive := app.Interactive
	app.Interactive, app.Prompt = true, prompt
	followSteps(t, app, out, steps, nil, "")
	app.Interactive = interactive
	if stored, err := items.Get(provider); err != nil || stored != fixtureToken || len(prompt.secretCalls) != 1 {
		t.Fatalf("after the fix: Token of provider %s stored %t, %v, after %d prompts", provider, stored == fixtureToken, err, len(prompt.secretCalls))
	}
}

// The local depth of a check observes whether each client's provider has a
// Token. A Keychain item that another program stored is there, and macOS
// refuses AIGW its value, so it is no Token that the client can use: the
// client's Token check fails, unavailable, with the item's title and fix, in
// text and as JSON, and the check reads no Token. Followed as printed, the fix
// lets the same check pass.
func TestCheckNamesTheFixOfAKeychainItemOfAnotherProgramAtTheLocalDepth(t *testing.T) {
	t.Parallel()
	app, out, items := claudeOverAnItemOfAnotherProgram(t)
	title, fix := refusalTitle("one"), refusalFix("one", "`aigw provider rotate one`")

	args := []string{"check", "--depth", "local", "--json"}
	if err := cli.Execute(app, args); err == nil {
		t.Fatalf("%v passed\n%s", args, out)
	}
	requireRefusedTokenCheck(t, args, out, title, fix)

	args = []string{"check", "--depth", "local", "--verbose"}
	out.Reset()
	err := cli.Execute(app, args)
	if text := strings.Join(strings.Fields(out.String()), " "); err == nil || strings.Contains(text, "has a Token") ||
		!strings.Contains(text, "Claude Token "+title+" Fix: "+fix) || !strings.Contains(text, "one/claude-test · local "+title+" Fix: "+fix) {
		t.Fatalf("%v = %v\n%s", args, err, out)
	}
	requireNoTokenRead(t, items, args)

	storeTokenAsPrinted(t, app, out, items)
	args = []string{"check", "--depth", "local", "--verbose"}
	out.Reset()
	if err := cli.Execute(app, args); err != nil || !strings.Contains(out.String(), "Provider one has a Token") || strings.Contains(out.String(), "Another program") {
		t.Fatalf("%v after the fix = %v\n%s", args, err, out)
	}
}

// requireRefusedTokenCheck requires that the document of a check stopped
// Claude Code at the local depth, unavailable, at its Token check, with title
// as the problem and fix as the one next step, which no script can run as it
// stands.
func requireRefusedTokenCheck(t *testing.T, args []string, out *bytes.Buffer, title, fix string) {
	t.Helper()
	var report struct {
		checkReport
		Clients []struct {
			checkedClient
			Checks []stepRow `json:"checks"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil || len(report.Clients) != 1 {
		t.Fatalf("%v wrote %s: %v", args, out, err)
	}
	client := report.Clients[0]
	if report.OK || report.NextAction != "" || report.NextDetail != fix || client.ID != claude.ID || client.State != "unavailable" ||
		client.NextAction != "" || client.NextDetail != fix {
		t.Fatalf("%v wrote %+v\n%s", args, client, out)
	}
	if failure := client.Failure; failure == nil || failure.Depth != "local" || failure.Problem != title {
		t.Fatalf("%v wrote the failure %+v", args, failure)
	}
	if token, checked := named(client.Checks, "claude:token"); !checked || token.State != "failed" || token.Detail != title || token.NextAction != "" || token.NextDetail != fix {
		t.Fatalf("%v wrote the Token check %+v", args, client.Checks)
	}
}

// `aigw status`, which bare `aigw` runs, shows each client's readiness from
// the same observation. A client whose provider's Keychain item another
// program stored is unavailable with the item's title, and the status names
// the item's fix as its next step, in text and as JSON; it reads no Token.
// Followed as printed, the fix leaves the client configured.
func TestStatusNamesTheFixOfAKeychainItemOfAnotherProgram(t *testing.T) {
	t.Parallel()
	app, out, items := claudeOverAnItemOfAnotherProgram(t)
	title, fix := refusalTitle("one"), refusalFix("one", "`aigw provider rotate one`")

	args := []string{"status", "--json"}
	if err := cli.Execute(app, args); err != nil {
		t.Fatalf("%v = %v\n%s", args, err, out)
	}
	var document struct {
		Clients    clientStates `json:"clients"`
		NextAction string       `json:"next_action"`
		NextDetail string       `json:"next_detail"`
	}
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatalf("%v wrote %s: %v", args, out, err)
	}
	if client := document.Clients[claude.ID]; client.State != "unavailable" || client.Detail != title || client.NextAction != "" || client.NextDetail != fix ||
		document.NextAction != "" || document.NextDetail != fix {
		t.Fatalf("%v wrote %+v, next %q, %q", args, client, document.NextAction, document.NextDetail)
	}

	for _, args := range [][]string{nil, {"status"}} {
		out.Reset()
		err := cli.Execute(app, args)
		if text := strings.Join(strings.Fields(out.String()), " "); err != nil || strings.Contains(text, "Configured") ||
			!strings.Contains(text, "one/claude-test · Unavailable · "+title) || !strings.Contains(text, "Next "+fix) {
			t.Fatalf("%v = %v\n%s", args, err, out)
		}
	}
	requireNoTokenRead(t, items, []string{"status"})

	storeTokenAsPrinted(t, app, out, items)
	out.Reset()
	if err := cli.Execute(app, []string{"status"}); err != nil || !strings.Contains(out.String(), "one/claude-test · Configured") || strings.Contains(out.String(), "Another program") {
		t.Fatalf("status after the fix = %v\n%s", err, out)
	}
}

// stepRow decodes one row of a document that names its subject, its state and
// its next step: a local check of `aigw check`, and a Token or a client of a
// team setup.
type stepRow struct {
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	ID         string `json:"id"`
	State      string `json:"state"`
	Detail     string `json:"detail"`
	NextAction string `json:"next_action"`
	NextDetail string `json:"next_detail"`
}

// named returns the row of rows whose name, provider or ID is subject.
func named(rows []stepRow, subject string) (stepRow, bool) {
	index := slices.IndexFunc(rows, func(row stepRow) bool { return subject == row.Name+row.Provider+row.ID })
	if index < 0 {
		return stepRow{}, false
	}
	return rows[index], true
}

// requireWaitingRows requires that a team setup's report shows provider one's
// Token and client, which selects its model, waiting with the title and the
// fix of provider one's item: in the document as an unobserved Token and a
// client that waits for it, each with the fix as its next step and no title,
// and in text with the title and the fix beneath each of the two rows, of
// which row is the client's.
func requireWaitingRows(t *testing.T, args []string, out *bytes.Buffer, document bool, client, row string) {
	t.Helper()
	title, fix := refusalTitle("one"), refusalFix("one", "`aigw provider rotate one`")
	if !document {
		if text := strings.Join(strings.Fields(out.String()), " "); strings.Count(text, title+" Fix: "+fix) != 2 ||
			!strings.Contains(text, "one "+title) || !strings.Contains(text, row+" "+title) || strings.Contains(text, "cannot tell") {
			t.Fatalf("%v shows the item of provider one in the rows of its Token and of its client:\n%s", args, out)
		}
		return
	}
	var report struct {
		Tokens  []stepRow `json:"tokens"`
		Clients []stepRow `json:"clients"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil || strings.Contains(out.String(), "Another program") {
		t.Fatalf("%v wrote %s: %v", args, out, err)
	}
	if token, observed := named(report.Tokens, "one"); !observed || token.State != "unobserved" || token.NextAction != "" || token.NextDetail != fix {
		t.Fatalf("%v wrote the Tokens %+v", args, report.Tokens)
	}
	if waiting, selected := named(report.Clients, client); !selected || waiting.State != "waits_for_token" || waiting.NextAction != "" || waiting.NextDetail != fix {
		t.Fatalf("%v wrote the clients %+v", args, report.Clients)
	}
}

// A team setup observes the Token of each provider that a client's selection
// or a recommendation needs. One that it does not name does not stop it: a
// provider whose Keychain item another program stored waits, as a provider
// without a Token waits, with the item's title and fix in its row and in the
// row of the client that needs it, in text and as JSON, while the setup stores
// the Token of the provider that it names. A preview shows the same and reads
// no Token. Followed as printed, the fix stores the Token, and the client's
// local check passes.
func TestASetupLeavesTheItemOfAProviderThatItDoesNotNameWaiting(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		flags []string
	}{
		{name: "text"},
		{name: "json", flags: []string{"--json"}},
		{name: "a preview", flags: []string{"--dry-run"}},
		{name: "a preview as json", flags: []string{"--dry-run", "--json"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, out, items := claudeOverAnItemOfAnotherProgram(t)
			app.In = strings.NewReader(fixtureToken + "\n")
			before := readFile(t, app.Config.Path())
			preview, document := slices.Contains(test.flags, "--dry-run"), slices.Contains(test.flags, "--json")
			args := append([]string{"setup", "--from", gatewayTeamFile(t), "--provider", "gateway", "--token-stdin"}, test.flags...)
			if err := cli.Execute(app, args); err != nil {
				t.Fatalf("%v = %v\n%s", args, err, out)
			}
			if preview {
				requireNoTokenRead(t, items, args)
			}

			requireWaitingRows(t, args, out, document, claude.ID, "Claude one/claude-test")

			stored, storeErr := items.Get("gateway")
			cfg, err := app.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			if preview {
				if storeErr == nil || !bytes.Equal(readFile(t, app.Config.Path()), before) {
					t.Fatalf("%v stored a Token for gateway (%v) or changed the configuration", args, storeErr)
				}
				return
			}
			if storeErr != nil || stored != fixtureToken || cfg.Providers["gateway"].Label != "Gateway" || cfg.SelectedModel(claude.ID).Provider() != "one" {
				t.Fatalf("%v stored gateway's Token %t, %v; provider %+v; Claude Code selects %v", args, stored == fixtureToken, storeErr, cfg.Providers["gateway"], cfg.SelectedModel(claude.ID))
			}
			if document {
				return
			}
			storeTokenAsPrinted(t, app, out, items)
			check := []string{"check", claude.ID, "--depth", "local"}
			out.Reset()
			if err := cli.Execute(app, check); err != nil {
				t.Fatalf("%v after the fix = %v\n%s", check, err, out)
			}
		})
	}
}

// A team setup ends at the item of the provider that it names, before it reads
// a Token. For a provider that the configuration does not hold, the last step
// of the fix is the setup that stores that provider's Token: the command as it
// was given where it reads the Token itself; without --dry-run for a preview,
// which stores nothing; and with the Token on standard input where the refused
// command could not prompt for it, since the same command would then end
// without a Token. A line that differs from the one given names its team file
// to fill in when the path would need quoting, and one that does not differ is
// the same command again. Followed as printed, the fix saves the provider and
// stores its Token.
func TestTheFixOfASetupForTheProviderThatItNamesRunsAsPrinted(t *testing.T) {
	t.Parallel()
	const piped = "`aigw setup --from <file> --provider gateway --token-stdin`"
	for _, test := range []struct {
		name  string
		flags []string
		// terminal says that the setup can prompt for the Token.
		terminal bool
		// last is the last step with the team file as <file>, and quoted the
		// one for a team file whose path would need quoting.
		last, quoted string
	}{
		{name: "without a terminal or standard input", last: piped, quoted: piped},
		{name: "as json", flags: []string{"--json"}, last: piped, quoted: piped},
		{name: "a preview", flags: []string{"--dry-run"}, last: piped, quoted: piped},
		{name: "a preview with the Token on standard input", flags: []string{"--token-stdin", "--dry-run"},
			last: piped, quoted: "`aigw setup --provider gateway --token-stdin --from <file>`"},
		{name: "at a terminal", terminal: true, last: "`aigw setup --from <file> --provider gateway`", quoted: "the same command again"},
		{name: "a preview at a terminal", flags: []string{"--dry-run"}, terminal: true,
			last: "`aigw setup --from <file> --provider gateway`", quoted: "`aigw setup --provider gateway --from <file>`"},
	} {
		for _, spaced := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/spaced=%t", test.name, spaced), func(t *testing.T) {
				app, out, _, _, requests := testApp(t, "")
				app.Interactive = test.terminal
				items := withItemsOfAnotherProgram(t, app, "gateway")
				teamFile := gatewayTeamFile(t)
				if spaced {
					// A folder whose name has a space makes the path need quoting.
					teamFile = filepath.Join(t.TempDir(), "team files", "team.toml")
					writeFile(t, teamFile, readFile(t, gatewayTeamFile(t)), 0o600)
				}
				last := strings.Replace(test.last, "<file>", teamFile, 1)
				if platform.AIGWCommand(teamFile) == "" {
					last = test.quoted
				}
				args := append([]string{"setup", "--from", teamFile, "--provider", "gateway"}, test.flags...)
				err := cli.Execute(app, args)
				requireRefusal(t, args, err, out, slices.Contains(test.flags, "--json"), "gateway", refusalFix("gateway", last))
				if operations := items.operations(t); !slices.Equal(operations, []string{"exists gateway", "empty gateway"}) || requests.calls != 0 {
					t.Fatalf("%v ran %q on the keychain and sent %d requests; want one observation and one emptiness answer", args, operations, requests.calls)
				}
				if slices.Contains(test.flags, "--json") {
					return
				}
				deletion, steps := fixSteps(t, out)
				items.delete(t, deletion)
				if file := slices.Index(steps[0], "<file>"); file >= 0 {
					steps[0][file] = teamFile
				}
				prompt := &scriptedPrompt{secrets: []string{fixtureToken}}
				app.Prompt = prompt
				followSteps(t, app, out, steps, args, fixtureToken+"\n")
				cfg, err := app.Config.Load()
				if stored, readErr := items.Get("gateway"); err != nil || cfg.Providers["gateway"].Label != "Gateway" || readErr != nil || stored != fixtureToken {
					t.Fatalf("after the fix: provider %+v, %v; Token stored %t, %v", cfg.Providers["gateway"], err, stored == fixtureToken, readErr)
				}
				if asked := len(prompt.secretCalls) == 1; asked != test.terminal {
					t.Fatalf("the fix asked for the Token at a prompt %t, want %t", asked, test.terminal)
				}
			})
		}
	}
}
