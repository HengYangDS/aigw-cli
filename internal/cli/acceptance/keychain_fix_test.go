package cli_test

import (
	"bytes"
	"io"
	"net/http"
	"path/filepath"
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

// printedFix finds, in what a command printed, the fix of a Keychain item of
// another program: the system tool's deletion, and the steps that follow it,
// each a command line or the same command again.
var printedFix = regexp.MustCompile("delete the item with `([^`]+)`((?:,(?: then)? run (?:`[^`]+`|the same command again))+)\\.")

// printedStep finds one step after the deletion: its command line, or none
// when the step names the same command again.
var printedStep = regexp.MustCompile("run (?:`([^`]+)`|the same command again)")

// fixSteps returns the steps of the fix that a command printed: the deletion,
// and in order each step after it, as the words of its command line, or as no
// words when the step names the same command again. A command that prints its
// fix for a row and as its next step prints one fix.
func fixSteps(t *testing.T, out *bytes.Buffer) (deletion string, steps [][]string) {
	t.Helper()
	found := printedFix.FindAllStringSubmatch(strings.Join(strings.Fields(out.String()), " "), -1)
	if len(found) == 0 {
		t.Fatalf("the output names no fix with a deletion and the steps after it:\n%s", out)
	}
	for _, fix := range found[1:] {
		if fix[0] != found[0][0] {
			t.Fatalf("the output names two fixes, %q and %q", found[0][0], fix[0])
		}
	}
	for _, step := range printedStep.FindAllStringSubmatch(found[0][2], -1) {
		steps = append(steps, strings.Fields(step[1]))
	}
	t.Logf("the printed fix deletes with %q and goes on with%s", found[0][1], found[0][2])
	return found[0][1], steps
}

// followSteps runs the steps that a printed fix names after its deletion, in
// order, through the test's entry point, each with input on standard input:
// the printed aigw command line, word for word, or again, the first command,
// when the step names the same command.
func followSteps(t *testing.T, app *cli.App, out *bytes.Buffer, steps [][]string, again []string, input string) {
	t.Helper()
	for _, step := range steps {
		args := again
		if len(step) > 0 {
			if step[0] != "aigw" {
				t.Fatalf("the step %q is no aigw command line", step)
			}
			args = step[1:]
		}
		out.Reset()
		app.In = strings.NewReader(input)
		if err := cli.Execute(app, args); err != nil {
			t.Fatalf("the fix's step %q failed: %v\n%s", args, err, out)
		}
		t.Logf("the fix's step %q succeeded", args)
	}
}

// fixtureToken is the Token that the tests of a printed fix give a command.
const fixtureToken = "fixture-token"

// The fix of a Keychain item of another program is followed as it is printed,
// once per kind of fix: the test removes the item that the printed deletion
// names, then runs each printed step after it word for word. Every step
// succeeds, the last one stores the credential, and the command that met the
// refusal no longer meets it. `aigw provider add` stops before the provider is
// saved and stores the Token itself, so its last step is itself.
func TestTheFixOfARefusedProviderAddRunsAsPrinted(t *testing.T) {
	app, out, _, _, _ := testApp(t, fixtureToken+"\n")
	items := withItemsOfAnotherProgram(t, app, "team")
	args := []string{"provider", "add", "team", "--endpoint", "anthropic=https://team.test", "--token-stdin"}
	err := cli.Execute(app, args)
	requireRefusal(t, args, err, out, false, "team", refusalFix("team", "`aigw provider add team --endpoint anthropic=https://team.test --token-stdin`"))
	deletion, steps := fixSteps(t, out)
	items.delete(t, deletion)
	followSteps(t, app, out, steps, args, fixtureToken+"\n")
	cfg, err := app.Config.Load()
	if stored, readErr := items.Get("team"); err != nil || cfg.Providers["team"].Endpoints.Anthropic != "https://team.test" || readErr != nil || stored != fixtureToken {
		t.Fatalf("after the fix: provider %+v, %v; Token stored %t, %v", cfg.Providers["team"], err, stored == fixtureToken, readErr)
	}
}

// `aigw setup --from` stops before it saves the provider of the team file, so
// its last step is itself.
func TestTheFixOfARefusedSetupRunsAsPrinted(t *testing.T) {
	app, out, _, _, _ := testApp(t, fixtureToken+"\n")
	items := withItemsOfAnotherProgram(t, app, "gateway")
	teamFile := gatewayTeamFile(t)
	args := []string{"setup", "--from", teamFile, "--provider", "gateway", "--token-stdin"}
	err := cli.Execute(app, args)
	requireRefusal(t, args, err, out, false, "gateway", refusalFix("gateway", setupLine(teamFile)))
	deletion, steps := fixSteps(t, out)
	items.delete(t, deletion)
	followSteps(t, app, out, steps, args, fixtureToken+"\n")
	cfg, err := app.Config.Load()
	if stored, readErr := items.Get("gateway"); err != nil || cfg.Providers["gateway"].Label != "Gateway" || readErr != nil || stored != fixtureToken {
		t.Fatalf("after the fix: provider %+v, %v; Token stored %t, %v", cfg.Providers["gateway"], err, stored == fixtureToken, readErr)
	}
}

// relabelingTeamFile is the gateway team file, which also names the model
// that Codex selects on provider one with another label, so that applying it
// rewrites Codex's file.
func relabelingTeamFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "team.toml")
	writeFile(t, path, append(readFile(t, gatewayTeamFile(t)), []byte(`
[[providers]]
id = "one"
label = "One"
endpoints = {responses = "https://one.test/v1"}

[[providers.models]]
id = "gpt-test"
family = "gpt-test"
label = "Relabeled"
protocols = {responses = []}

[[families]]
id = "gpt-test"
label = "GPT Test"
`)...), 0o600)
	return path
}

// A team setup that names a provider that the configuration already holds,
// whose Token it would replace, ends where it observes that provider's item,
// before it reads a Token. `aigw provider rotate` knows such a provider, so
// the fix ends with it and stores the Token, and the setup then runs.
func TestTheFixOfASetupRefusedForAConfiguredProviderRunsAsPrinted(t *testing.T) {
	app, out, _, _, _ := testApp(t, fixtureToken+"\n")
	items := withItemsOfAnotherProgram(t, app, "gateway")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, app.Config.Path())
	args := []string{"setup", "--from", gatewayTeamFile(t), "--provider", "gateway", "--token-stdin"}
	err := cli.Execute(app, args)
	requireRefusal(t, args, err, out, false, "gateway", refusalFix("gateway", "`aigw provider rotate gateway`"))
	if changed := items.changed(t); len(changed) != 0 || !bytes.Equal(readFile(t, app.Config.Path()), before) {
		t.Fatalf("the refused setup changed the keychain with %q or changed the configuration", changed)
	}
	deletion, steps := fixSteps(t, out)
	items.delete(t, deletion)
	const rotated = "rotated-token"
	prompt := &scriptedPrompt{secrets: []string{rotated}}
	app.Interactive, app.Prompt = true, prompt
	followSteps(t, app, out, steps, args, "")
	if stored, err := items.Get("gateway"); err != nil || stored != rotated || len(prompt.secretCalls) != 1 {
		t.Fatalf("after the fix: Token stored %t, %v, after %d prompts", stored == rotated, err, len(prompt.secretCalls))
	}

	// Followed once, the fix lets the same command run.
	out.Reset()
	app.Interactive, app.In = false, strings.NewReader(fixtureToken+"\n")
	if err := cli.Execute(app, args); err != nil {
		t.Fatalf("%v after the fix = %v\n%s", args, err, out)
	}
	after, err := app.Config.Load()
	if stored, readErr := items.Get("gateway"); err != nil || after.Providers["gateway"].Label != "Gateway" || len(after.Models) == len(cfg.Models) ||
		readErr != nil || stored != fixtureToken {
		t.Fatalf("after the setup: provider %+v, %d models, %v; Token stored %t, %v", after.Providers["gateway"], len(after.Models), err, stored == fixtureToken, readErr)
	}
}

// Guided setup of a provider that the configuration holds meets that
// provider's item before it asks for a model. `aigw provider rotate` knows the
// provider, so the fix ends with it; guided setup then uses the stored Token
// without a prompt.
func TestTheFixOfAGuidedSetupOfAConfiguredProviderRunsAsPrinted(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	app.Interactive = true
	items := withItemsOfAnotherProgram(t, app, "gateway")
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: executableFixture(t, "claude")}}}
	cfg := domain.NewConfig(clients.Admission())
	cfg.SetSelectedModel(claude.ID, addProviderModel(&cfg, "gateway", "Gateway", domain.Endpoints{Anthropic: "https://gateway.test"}, claude.ID, "claude-test"), "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	chosen := &scriptedPrompt{selections: []string{"gateway"}}
	app.Prompt = chosen
	args := []string{"setup"}
	err := cli.Execute(app, args)
	requireRefusal(t, args, err, out, false, "gateway", refusalFix("gateway", "`aigw provider rotate gateway`"))
	if chosen.callCount() != 1 {
		t.Fatalf("setup asked %d questions before the refusal; want the provider alone", chosen.callCount())
	}
	deletion, steps := fixSteps(t, out)
	items.delete(t, deletion)
	token := &scriptedPrompt{secrets: []string{fixtureToken}}
	app.Prompt = token
	followSteps(t, app, out, steps, args, "")
	if stored, err := items.Get("gateway"); err != nil || stored != fixtureToken || len(token.secretCalls) != 1 {
		t.Fatalf("after the fix: Token stored %t, %v, after %d prompts", stored == fixtureToken, err, len(token.secretCalls))
	}

	again := &scriptedPrompt{selections: []string{"gateway"}, texts: []string{"claude-test"}}
	app.Prompt = again
	out.Reset()
	if err := cli.Execute(app, args); err != nil || len(again.secretCalls) != 0 || !strings.Contains(out.String(), "Setup saved") {
		t.Fatalf("%v after the fix = %v, after %d Token prompts\n%s", args, err, len(again.secretCalls), out)
	}
}

// A command that reads the Token of a configured provider names `aigw
// provider rotate`, which asks for the Token at a terminal. Afterwards the
// command reads the provider's catalog.
func TestTheFixOfARefusedTokenRunsAsPrinted(t *testing.T) {
	app, out, _, _, requests := testApp(t, "")
	saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
	requests.body = `{"data":[{"id":"claude-test"}]}`
	items := withItemsOfAnotherProgram(t, app, "one")
	args := []string{"model", "search", "--provider", "one"}
	err := cli.Execute(app, args)
	if text := strings.Join(strings.Fields(out.String()), " "); err == nil ||
		!strings.Contains(text, "Anthropic Messages: "+refusalTitle("one")) || !strings.Contains(text, "Fix: "+refusalFix("one", "`aigw provider rotate one`")) {
		t.Fatalf("%v = %v\n%s", args, err, out)
	}
	deletion, steps := fixSteps(t, out)
	items.delete(t, deletion)
	prompt := &scriptedPrompt{secrets: []string{fixtureToken}}
	app.Interactive, app.Prompt = true, prompt
	followSteps(t, app, out, steps, nil, "")
	if stored, err := items.Get("one"); err != nil || stored != fixtureToken || len(prompt.secretCalls) != 1 {
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
	app, out, _, _, requests := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["one"] = domain.Provider{Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://one.test"},
		BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://diagnostics.test"}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	items := withItemsOfAnotherProgram(t, app, "diagnostic@one")
	if err := items.Set("one", "sk-team-token-0001"); err != nil {
		t.Fatal(err)
	}
	requests.handler = balanceProbeAnswers
	args := []string{"provider", "balance", "one"}
	err := cli.Execute(app, args)
	fix := "Have the balance credentials at hand, delete the item with `security delete-generic-password -s AIGW_TOKEN -a diagnostic@one`, " +
		"then run `aigw provider edit one --balance-token-stdin --balance-user-id <user-id>`."
	if text := strings.Join(strings.Fields(out.String()), " "); err == nil ||
		!strings.Contains(text, "Another program stored provider one's balance credentials in the Keychain") || !strings.Contains(text, "Fix: "+fix) {
		t.Fatalf("%v = %v\n%s", args, err, out)
	}
	deletion, steps := fixSteps(t, out)
	items.delete(t, deletion)
	last := steps[len(steps)-1]
	placeholder := slices.Index(last, "<user-id>")
	if placeholder < 0 {
		t.Fatalf("the last step %q names no user ID to fill in", last)
	}
	last[placeholder] = "7"
	followSteps(t, app, out, steps, nil, "synthetic-balance-token\n")
	if stored, err := app.Providers.Get("one"); err != nil || stored != (secret.DiagnosticCredential{SystemToken: "synthetic-balance-token", UserID: "7"}) {
		t.Fatalf("after the fix: balance credentials stored %t, %v", err == nil && stored.UserID == "7", err)
	}
	out.Reset()
	if err := cli.Execute(app, args); err != nil || strings.Contains(out.String(), "Another program") || !strings.Contains(out.String(), "Remaining quota") {
		t.Fatalf("%v after the fix = %v\n%s", args, err, out)
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
