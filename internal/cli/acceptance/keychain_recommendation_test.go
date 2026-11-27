package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/platform"
)

// spareTeamFile is the gateway team file with a second provider, spare, whose
// model the recommendation for Claude Code offers as its alternative.
func spareTeamFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "team.toml")
	writeFile(t, path, []byte(`version = 9

[[providers]]
id = "gateway"
label = "Gateway"
endpoints = {anthropic = "https://gateway.test"}

[[providers.models]]
id = "claude-team"
family = "claude-team"
label = "Claude Team"
protocols = {anthropic = []}

[[providers]]
id = "spare"
label = "Spare"
endpoints = {anthropic = "https://spare.test"}

[[providers.models]]
id = "claude-team"
family = "claude-team"
label = "Claude Team"
protocols = {anthropic = []}

[[families]]
id = "claude-team"
label = "Claude Team"

[recommendations.claude]
primary = {model = "gateway/claude-team"}
alternatives = [{model = "spare/claude-team"}]
`), 0o600)
	return path
}

// recommendedOverAnItemOfAnotherProgram is a home without a configuration,
// with Claude Code installed, over a keychain whose worker is the program's
// own and whose item for provider gateway another program stored.
func recommendedOverAnItemOfAnotherProgram(t *testing.T) (*cli.App, *bytes.Buffer, *keychain) {
	t.Helper()
	app, out, _, _, _ := testApp(t, "")
	items := withItemsOfAnotherProgram(t, app, "gateway")
	items.bindReader(app)
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: executableFixture(t, "claude")}}}
	return app, out, items
}

// applyTeamFile applies teamFile without naming a provider, which leaves the
// provider of the refused item waiting and enables no client.
func applyTeamFile(t *testing.T, app *cli.App, out *bytes.Buffer, teamFile string) {
	t.Helper()
	out.Reset()
	if err := cli.Execute(app, []string{"setup", "--from", teamFile}); err != nil {
		t.Fatalf("setup --from %s = %v\n%s", teamFile, err, out)
	}
	if cfg, err := app.Config.Load(); err != nil || len(cfg.EnabledClientIDs()) != 0 {
		t.Fatalf("the setup enabled %q, %v; want no client", cfg.EnabledClientIDs(), err)
	}
}

// With no client enabled, every surface names as its next step what gives a
// recommended provider a Token. For a provider whose Keychain item another
// program stored, that is the fix of the item, in text and as JSON, where no
// script can run it as it stands: never the bare `aigw provider rotate`,
// which would ask for a Token, send it to the provider and then end with the
// refusal. Followed as printed, the fix stores the Token, and the surface no
// longer names the item.
func TestTheNextStepWithNoClientEnabledIsTheFixOfARecommendedProvidersItem(t *testing.T) {
	t.Parallel()
	fix := refusalFix("gateway", "`aigw provider rotate gateway`")
	for _, surface := range [][]string{{"setup"}, {"status"}, {"sync"}, {"check", "--depth", "local"}} {
		for _, flag := range []string{"", "--json"} {
			t.Run(strings.TrimSpace(strings.Join(surface, " ")+" "+flag), func(t *testing.T) {
				app, out, items := recommendedOverAnItemOfAnotherProgram(t)
				teamFile := gatewayTeamFile(t)
				args := slices.Clone(surface)
				if surface[0] == "setup" {
					args = append(args, "--from", teamFile)
				} else {
					applyTeamFile(t, app, out, teamFile)
				}
				if flag != "" {
					args = append(args, flag)
				}
				out.Reset()
				// A check without an enabled client fails; the others succeed.
				if err := cli.Execute(app, args); (err != nil) != (surface[0] == "check") {
					t.Fatalf("%v = %v\n%s", args, err, out)
				}
				requireNoTokenRead(t, items, args)
				if flag != "" {
					var document struct {
						NextAction string `json:"next_action"`
						NextDetail string `json:"next_detail"`
					}
					if err := json.Unmarshal(out.Bytes(), &document); err != nil || document.NextAction != "" || document.NextDetail != fix {
						t.Fatalf("%v wrote the next step %q, %q; want the fix as guidance: %v\n%s", args, document.NextAction, document.NextDetail, err, out)
					}
					return
				}
				label := "Next "
				if surface[0] == "check" {
					label = "Fix "
				}
				if text := strings.Join(strings.Fields(out.String()), " "); !strings.Contains(text, label+fix) || strings.Contains(text, label+"aigw provider rotate") {
					t.Fatalf("%v does not name the fix of the item as its next step:\n%s", args, out)
				}
				storeTokenAsPrinted(t, app, out, items)
				out.Reset()
				if err := cli.Execute(app, args); (err != nil) != (surface[0] == "check") || strings.Contains(out.String(), "Another program") ||
					strings.Contains(out.String(), "delete-generic-password") {
					t.Fatalf("%v after the fix = %v\n%s", args, err, out)
				}
			})
		}
	}
}

// chosenCommand finds the command of the one provider without a Token in a
// next step that offers a choice.
var chosenCommand = regexp.MustCompile(`Choose one compatible provider: (aigw provider rotate [a-z]+)\. `)

// A recommendation that offers a provider without a Token beside one whose
// Keychain item another program stored names both: the command that stores
// the first one's Token, then the second one's item and its fix as a sentence
// of its own. Each named step works as printed: either gives one compatible
// provider a Token, after which the status names the team file again and no
// item.
func TestTheNextStepNamesAMissingTokenAndARefusedItemEachWithItsStep(t *testing.T) {
	t.Parallel()
	want := "Choose one compatible provider: aigw provider rotate spare. " + refusalTitle("gateway") + ": " +
		refusalFix("gateway", "`aigw provider rotate gateway`")
	for _, followed := range []string{"the command of the provider without a Token", "the fix of the refused item"} {
		t.Run(followed, func(t *testing.T) {
			app, out, items := recommendedOverAnItemOfAnotherProgram(t)
			applyTeamFile(t, app, out, spareTeamFile(t))
			if text := strings.Join(strings.Fields(out.String()), " "); !strings.Contains(text, "Next "+want) {
				t.Fatalf("the setup does not name both steps:\n%s", out)
			}
			status := []string{"status", "--json"}
			out.Reset()
			var document struct {
				NextAction string `json:"next_action"`
				NextDetail string `json:"next_detail"`
			}
			if err := cli.Execute(app, status); err != nil || json.Unmarshal(out.Bytes(), &document) != nil || document.NextAction != "" || document.NextDetail != want {
				t.Fatalf("%v = %v, next %q, %q\n%s", status, err, document.NextAction, document.NextDetail, out)
			}
			status = []string{"status"}
			out.Reset()
			if err := cli.Execute(app, status); err != nil {
				t.Fatalf("%v = %v\n%s", status, err, out)
			}
			provider := "gateway"
			if followed == "the fix of the refused item" {
				storeTokenAsPrinted(t, app, out, items)
			} else {
				provider = "spare"
				command := chosenCommand.FindStringSubmatch(strings.Join(strings.Fields(out.String()), " "))
				if command == nil {
					t.Fatalf("%v names no command of a provider without a Token:\n%s", status, out)
				}
				prompt := &scriptedPrompt{secrets: []string{fixtureToken}}
				app.Interactive, app.Prompt = true, prompt
				followSteps(t, app, out, [][]string{strings.Fields(command[1])}, nil, "")
				app.Interactive = false
			}
			if stored, err := items.Get(provider); err != nil || stored != fixtureToken {
				t.Fatalf("after %s: Token of provider %s stored %t, %v", followed, provider, stored == fixtureToken, err)
			}
			out.Reset()
			if err := cli.Execute(app, status); err != nil || strings.Contains(out.String(), "Another program") ||
				!strings.Contains(strings.Join(strings.Fields(out.String()), " "), "Next aigw setup --from <file>") {
				t.Fatalf("%v after %s = %v\n%s", status, followed, err, out)
			}
		})
	}
}

// A preview of a team setup saves nothing, so `aigw provider rotate` does not
// know a provider that only the team file holds once the preview ends. For
// such a provider whose Keychain item another program stored, and which the
// preview does not name, the fix on its Token row therefore ends with the
// setup that stores that provider's Token: this command's line without
// --dry-run, naming the provider, and with the Token on standard input where
// the rerun could not ask for it. A team file whose path would need quoting
// is named as the file to fill in. Followed as printed, the fix saves the
// provider and stores its Token, and the same preview no longer meets the
// item.
func TestTheFixOfASetupPreviewForAProviderThatOnlyTheTeamFileHoldsRunsAsPrinted(t *testing.T) {
	t.Parallel()
	const asked, piped = "`aigw setup --from <file> --provider gateway`", "`aigw setup --from <file> --provider gateway --token-stdin`"
	for _, test := range []struct {
		name string
		json bool
		// terminal says that the preview runs at a terminal, where its apply
		// can ask for the Token unless it writes a document.
		terminal bool
		last     string
	}{
		{name: "without a terminal", last: piped},
		{name: "without a terminal as json", json: true, last: piped},
		{name: "at a terminal", terminal: true, last: asked},
		{name: "at a terminal as json", json: true, terminal: true, last: piped},
	} {
		for _, spaced := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/spaced=%t", test.name, spaced), func(t *testing.T) {
				app, out, items := recommendedOverAnItemOfAnotherProgram(t)
				app.Interactive = test.terminal
				requests, counted := app.HTTP.(*fakeHTTP)
				if !counted {
					t.Fatal("the fixture's HTTP client counts no requests")
				}
				teamFile := gatewayTeamFile(t)
				if spaced {
					// A folder whose name has a space makes the path need quoting.
					teamFile = filepath.Join(t.TempDir(), "team files", "team.toml")
					writeFile(t, teamFile, readFile(t, gatewayTeamFile(t)), 0o600)
				}
				last := test.last
				if platform.AIGWCommand(teamFile) != "" {
					last = strings.Replace(last, "<file>", teamFile, 1)
				}
				fix := refusalFix("gateway", last)
				args := []string{"setup", "--from", teamFile, "--dry-run"}
				if test.json {
					args = append(args, "--json")
				}
				if err := cli.Execute(app, args); err != nil {
					t.Fatalf("%v = %v\n%s", args, err, out)
				}
				requireNoTokenRead(t, items, args)
				if requests.calls != 0 {
					t.Fatalf("%v sent %d requests", args, requests.calls)
				}
				requireWaitingTokenRow(t, args, out, test.json, fix)

				printed := out
				if test.json {
					// The document carries the fix as its reader decodes it.
					printed = bytes.NewBufferString(fix)
				}
				if prompted := storeGatewayTokenAsPrinted(t, app, out, printed, items, teamFile, args); prompted != (test.terminal && !test.json) {
					t.Fatalf("the fix asked for the Token at a prompt %t", prompted)
				}
				out.Reset()
				if err := cli.Execute(app, args); err != nil || strings.Contains(out.String(), "Another program") || strings.Contains(out.String(), "delete-generic-password") {
					t.Fatalf("%v after the fix = %v\n%s", args, err, out)
				}
			})
		}
	}
}

// storeGatewayTokenAsPrinted follows the fix that printed shows for provider
// gateway's item, whose last step is a setup: the fixture removes the item
// that the printed deletion names, the team file takes the place of a file to
// fill in, and the printed step runs word for word with the Token on standard
// input and at a scripted prompt, writing to out. The setup then saved the
// provider and stored its Token. It reports whether the step asked at the
// prompt.
func storeGatewayTokenAsPrinted(t *testing.T, app *cli.App, out, printed *bytes.Buffer, items *keychain, teamFile string, again []string) bool {
	t.Helper()
	deletion, steps := fixSteps(t, printed)
	items.delete(t, deletion)
	if file := slices.Index(steps[0], "<file>"); file >= 0 {
		steps[0][file] = teamFile
	}
	prompt := &scriptedPrompt{secrets: []string{fixtureToken}}
	app.Prompt = prompt
	followSteps(t, app, out, steps, again, fixtureToken+"\n")
	cfg, err := app.Config.Load()
	if stored, readErr := items.Get("gateway"); err != nil || cfg.Providers["gateway"].Label != "Gateway" || readErr != nil || stored != fixtureToken {
		t.Fatalf("after the fix: provider %+v, %v; Token stored %t, %v", cfg.Providers["gateway"], err, stored == fixtureToken, readErr)
	}
	return len(prompt.secretCalls) == 1
}

// requireWaitingTokenRow requires that a team setup's report shows provider
// gateway's Token waiting with fix: in the document as an unobserved Token
// with the fix as guidance, and in text with the item's title and the fix
// beneath the row.
func requireWaitingTokenRow(t *testing.T, args []string, out *bytes.Buffer, document bool, fix string) {
	t.Helper()
	if !document {
		if text := strings.Join(strings.Fields(out.String()), " "); !strings.Contains(text, "gateway "+refusalTitle("gateway")+" Fix: "+fix) {
			t.Fatalf("%v does not show the fix %q in the row of provider gateway's Token:\n%s", args, fix, out)
		}
		return
	}
	var report struct {
		Tokens []stepRow `json:"tokens"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("%v wrote %s: %v", args, out, err)
	}
	if token, observed := named(report.Tokens, "gateway"); !observed || token.State != "unobserved" || token.NextAction != "" || token.NextDetail != fix {
		t.Fatalf("%v wrote the Tokens %+v; want the fix %q", args, report.Tokens, fix)
	}
}
