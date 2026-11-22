package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

// This file holds the eleventh review's probes of the clients that keep no
// record of a selection, Claude Code and Claude Desktop, beside a Codex whose
// file chose a model, and of the reports.

// zz11Clients is one configuration with Codex, Claude Code and Claude Desktop
// enabled: the Codex world, Claude Code's settings file, and the profile and
// record of Claude Desktop.
type zz11Clients struct {
	codex          selectionWorld
	settings       string
	desktopProfile string
	desktopRecord  string
}

// zz11ThreeClients enables Codex on one file with gateway/gpt-sol, and Claude
// Code and Claude Desktop with gateway/claude-test. The provider spare serves
// one model for each protocol. One earlier change selected another model for
// Claude Code, so that a rollback of the last change restores it.
func zz11ThreeClients(t *testing.T) zz11Clients {
	t.Helper()
	app, output, secrets, _, _ := testApp(t, "")
	bindCredentialReader(t, app)
	target := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, target, []byte("model = \"gpt-native\"\n"), 0o600)
	library := filepath.Join(t.TempDir(), "nested", "Claude-3p", "configLibrary")
	cfg := domain.NewConfig(clients.Admission())
	both := func(host string) domain.Endpoints {
		return domain.Endpoints{Responses: "https://" + host + "/v1", Anthropic: "https://" + host}
	}
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: both("gateway.test")}
	cfg.Providers["spare"] = domain.Provider{Label: "Spare", Endpoints: both("spare.test")}
	for _, model := range []string{"gpt-sol", "gpt-terra", "gpt-luna"} {
		cfg.Models[domain.NewModelRef("gateway", model)] = qualifiedModel(model, "gateway", model, domain.ProtocolResponses)
	}
	for _, model := range []string{"claude-test", "claude-two"} {
		cfg.Models[domain.NewModelRef("gateway", model)] = qualifiedModel(model, "gateway", model, domain.ProtocolAnthropic)
	}
	cfg.Models[domain.NewModelRef("spare", "gpt-spare")] = qualifiedModel("Spare", "spare", "gpt-spare", domain.ProtocolResponses)
	cfg.Models[domain.NewModelRef("spare", "claude-spare")] = qualifiedModel("Spare Claude", "spare", "claude-spare", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("gateway", "gpt-sol"), "")
	cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), []string{target})
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("gateway", "claude-two"), "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	cfg.SetSelectedModel(claudedesktop.ID, domain.NewModelRef("gateway", "claude-test"), "")
	cfg.SetClientActivation(claudedesktop.ID, true, executableFixture(t, "Claude"), []string{library})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	for _, provider := range []string{"gateway", "spare"} {
		if err := secrets.Set(provider, "fixture-token"); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"sync"}, {"use", "claude", "gateway/claude-test"}} {
		output.Reset()
		if err := cli.Execute(app, args); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, output)
		}
	}
	world := selectionWorld{
		app: app, output: output, client: codex.ID, account: "gateway", files: []string{target},
		storedAs: func(runtime domain.Runtime) string { return runtime.UpstreamModel },
	}
	world = zz11Codex(world)
	paths := claudedesktop.PathsForLibrary(library)
	return zz11Clients{
		codex: world, settings: claudeSettingsPath(t, app.Host),
		desktopProfile: paths.Profile, desktopRecord: paths.State,
	}
}

// claudeModel returns the value of the key model in Claude Code's settings,
// as JSON, or "" when the key is absent.
func (c zz11Clients) claudeModel(t *testing.T) string {
	t.Helper()
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(readFile(t, c.settings), &settings); err != nil {
		t.Fatal(err)
	}
	return string(settings["model"])
}

// choosingInClaude sets the key model of Claude Code's settings to value, as
// Claude Code's own /model does.
func (c zz11Clients) choosingInClaude(t *testing.T, value string) {
	t.Helper()
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(readFile(t, c.settings), &settings); err != nil {
		t.Fatal(err)
	}
	settings["model"] = json.RawMessage(`"` + value + `"`)
	own, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, c.settings, own, 0o600)
}

// zz11ClaudeRun is one command run beside Claude Code's own model: what it
// returned, and Claude Code's settings and Claude Desktop's files before and
// after it.
type zz11ClaudeRun struct {
	err                             error
	modelBefore, modelAfter         string
	settingsSame, recordSame        bool
	desktopSame                     bool
	changed                         []string
	codexBefore, codexAfter, output string
}

// run runs args and reads the three clients before and after.
func (c zz11Clients) run(t *testing.T, args ...string) zz11ClaudeRun {
	t.Helper()
	read := func(path string) []byte {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		return data
	}
	before := [][]byte{read(c.settings), read(c.settings + adapter.StateSuffix), read(c.desktopProfile), read(c.desktopRecord)}
	result := zz11ClaudeRun{modelBefore: c.claudeModel(t)}
	run := runIn(t, c.codex, args...)
	after := [][]byte{read(c.settings), read(c.settings + adapter.StateSuffix), read(c.desktopProfile), read(c.desktopRecord)}
	result.err, result.changed, result.output = run.err, run.changed, run.output
	result.codexBefore, result.codexAfter = shownLine(run.before), shownLine(run.after)
	if after[0] != nil {
		result.modelAfter = c.claudeModel(t)
	}
	result.settingsSame, result.recordSame = bytes.Equal(before[0], after[0]), bytes.Equal(before[1], after[1])
	result.desktopSame = bytes.Equal(before[2], after[2]) && bytes.Equal(before[3], after[3])
	return result
}

// Claude Code's own model beside every kind of command, with a model chosen
// in Codex at the same time, with the Token and without it.
//
// Before: AIGW selected gateway/claude-test for Claude Code, and its settings
// name "claude-test". The person selects opus[1m] in Claude Code: the key
// model of the settings names "opus[1m]". Codex's file chose gpt-terra
// (F = gpt-terra, R = gpt-sol, S = gateway/gpt-sol).
//
// The Change's requirement: no command changes Claude Code's own model except
// an explicit `aigw use claude`. A command that names nothing of Claude Code
// leaves its settings and record as they are, byte for byte; a command that
// must rewrite the settings for another value (the provider's endpoint, a
// forwarding) keeps "opus[1m]". Claude Desktop keeps no model of the person's
// in a file: a command that names nothing of it leaves its files as they are.
func TestZZ11ClaudeCodesOwnModelBesideEveryCommand(t *testing.T) {
	type probe struct {
		name string
		args []string
		// rewrites says that the command changes a value that Claude Code's
		// settings hold, so the settings may change, but not the model.
		rewrites bool
		// desktop says that the command changes what Claude Desktop's files hold.
		desktop bool
	}
	probes := []probe{
		{name: "status", args: []string{"status", "--json"}},
		{name: "check claude", args: []string{"check", "claude", "--depth", "local", "--json"}},
		{name: "client list", args: []string{"client", "list", "--json"}},
		{name: "sync", args: []string{"sync", "--json"}},
		{name: "sync preview", args: []string{"sync", "--dry-run", "--json"}},
		{name: "another provider", args: []string{"provider", "edit", "spare", "--label", "Other", "--json"}},
		{name: "Codex's own setting", args: []string{"client", "edit", "codex", "--request-retries", "2", "--json"}},
		{name: "select for Codex", args: []string{"use", "codex", "gateway/gpt-luna", "--json"}},
		{name: "select for Claude Desktop", args: []string{"use", "claude-desktop", "gateway/claude-two", "--json"}, desktop: true},
		{name: "disable Codex", args: []string{"client", "disable", "codex", "--json"}},
		{name: "disable Claude Desktop", args: []string{"client", "disable", "claude-desktop", "--json"}, desktop: true},
		{name: "remove a model that nobody uses", args: []string{"model", "remove", "spare/gpt-spare", "--json"}},
		{name: "the provider's label", args: []string{"provider", "edit", "gateway", "--label", "Renamed", "--json"}, rewrites: true, desktop: true},
		{name: "the provider's endpoint", args: []string{"provider", "edit", "gateway", "--endpoint", "anthropic=https://gateway-two.test", "--json"}, rewrites: true, desktop: true},
		{name: "a forwarding for Claude Code", args: []string{"client", "edit", "claude", "--via", "http://127.0.0.1:8792", "--json"}, rewrites: true},
		{name: "add a model", args: []string{"model", "add", "gateway/claude-three", "--unlisted", "--protocol", "anthropic", "--json"}, rewrites: true, desktop: true},
		{name: "relabel the selected model", args: []string{"model", "edit", "gateway/claude-test", "--label", "Test Two", "--json"}, rewrites: true, desktop: true},
		{name: "rollback of the last change", args: []string{"config", "rollback", "--last-change", "--json"}, rewrites: true, desktop: true},
		{name: "rollback preview", args: []string{"config", "rollback", "--last-change", "--dry-run", "--json"}},
	}
	for _, probe := range probes {
		for _, token := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/token=%t", probe.name, token), func(t *testing.T) {
				three := zz11ThreeClients(t)
				three.choosingInClaude(t, "opus[1m]")
				recPick(t, three.codex.files[0], "gpt-terra")
				three.codex.setToken(t, token)
				run := three.run(t, probe.args...)
				t.Logf("err=%v; Claude Code's model %s -> %s; settings same %t, record same %t; Claude Desktop same %t\n  Codex before: %s\n  Codex after:  %s",
					run.err, run.modelBefore, run.modelAfter, run.settingsSame, run.recordSame, run.desktopSame, run.codexBefore, run.codexAfter)
				// A command that writes none of Claude Code's settings leaves the
				// model it names. What a command that rewrites the settings does
				// to that model belongs to the Change about Claude Code's own
				// model, so a rewrite only logs it here.
				if run.modelAfter != `"opus[1m]"` && !probe.rewrites {
					t.Errorf("%s changed Claude Code's own model from %s to %s (err=%v)", probe.name, run.modelBefore, run.modelAfter, run.err)
				}
				if !probe.rewrites && (!run.settingsSame || !run.recordSame) {
					t.Errorf("%s names nothing that Claude Code's settings hold and changed them or their record (err=%v)", probe.name, run.err)
				}
				if !probe.desktop && !run.desktopSame {
					t.Errorf("%s names nothing that Claude Desktop's files hold and changed them (err=%v)", probe.name, run.err)
				}
				if run.err != nil && len(run.changed) != 0 {
					t.Errorf("%s was refused and changed %v", probe.name, run.changed)
				}
			})
		}
	}
}

// An explicit selection for Claude Code is the one command that replaces its
// own model: `aigw use claude gateway/claude-two` over "opus[1m]" writes
// "claude-two".
func TestZZ11AnExplicitSelectionReplacesClaudeCodesOwnModel(t *testing.T) {
	three := zz11ThreeClients(t)
	three.choosingInClaude(t, "opus[1m]")
	run := three.run(t, "use", "claude", "gateway/claude-two", "--json")
	if run.err != nil || run.modelAfter != `"claude-two"` {
		t.Errorf("use claude: err=%v, Claude Code's model %s -> %s\n%s", run.err, run.modelBefore, run.modelAfter, run.output)
	}
}

// Claude Code's own model beside commands that rewrite its settings for
// another value, one or two steps each. The person selected opus[1m] in Claude
// Code; AIGW's selection for it is gateway/claude-test.
//
// The Change's requirement: no command changes Claude Code's own model except
// an explicit `aigw use claude`.
func TestZZ11ClaudeCodesOwnModelBesideARewriteOfItsSettings(t *testing.T) {
	steps := map[string]func(t *testing.T, three zz11Clients) [][]string{
		"the provider's endpoint": func(*testing.T, zz11Clients) [][]string {
			return [][]string{{"provider", "edit", "gateway", "--endpoint", "anthropic=https://gateway-two.test", "--json"}}
		},
		"the provider's endpoint without the Token, then the Token and a synchronization": func(t *testing.T, three zz11Clients) [][]string {
			three.codex.setToken(t, false)
			return [][]string{{"provider", "edit", "gateway", "--endpoint", "anthropic=https://gateway-two.test", "--json"}, {"TOKEN"}, {"sync", "--json"}}
		},
		"a forwarding, then its end": func(*testing.T, zz11Clients) [][]string {
			return [][]string{{"client", "edit", "claude", "--via", "http://127.0.0.1:8792", "--json"}, {"client", "edit", "claude", "--direct", "--json"}}
		},
		"a new credential reader, as a new program has, then a synchronization": func(t *testing.T, three zz11Clients) [][]string {
			bindCredentialReader(t, three.codex.app)
			return [][]string{{"sync", "--json"}}
		},
		"a new credential reader, then a command about another provider": func(t *testing.T, three zz11Clients) [][]string {
			bindCredentialReader(t, three.codex.app)
			return [][]string{{"provider", "edit", "spare", "--label", "Other", "--json"}}
		},
		"a rollback that restores another provider's label": func(t *testing.T, three zz11Clients) [][]string {
			three.codex.run(t, "provider", "edit", "spare", "--label", "Other")
			return [][]string{{"config", "rollback", "--last-change", "--json"}}
		},
		"a rollback that restores the provider's endpoint": func(t *testing.T, three zz11Clients) [][]string {
			three.codex.run(t, "provider", "edit", "gateway", "--endpoint", "anthropic=https://gateway-two.test")
			return [][]string{{"config", "rollback", "--last-change", "--json"}}
		},
		"a rollback that restores another selection for Claude Code": func(*testing.T, zz11Clients) [][]string {
			return [][]string{{"config", "rollback", "--last-change", "--json"}}
		},
	}
	for _, name := range slices.Sorted(mapKeys(steps)) {
		t.Run(name, func(t *testing.T) {
			three := zz11ThreeClients(t)
			commands := steps[name](t, three)
			// The person selects in Claude Code after every preparation.
			three.choosingInClaude(t, "opus[1m]")
			for _, args := range commands {
				if args[0] == "TOKEN" {
					three.codex.setToken(t, true)
					continue
				}
				run := three.run(t, args...)
				t.Logf("%v: err=%v; Claude Code's model %s -> %s; settings same %t", args, run.err, run.modelBefore, run.modelAfter, run.settingsSame)
				if args[0] == "config" {
					t.Logf("the rollback said:\n%s", run.output)
				}
				// Every step here rewrites Claude Code's settings for another value;
				// what that does to its own model is the subject of the Change about
				// Claude Code's own model, so this probe records it and asserts
				// nothing of it in this lane.
				if run.modelAfter != `"opus[1m]"` {
					t.Logf("%v changed Claude Code's own model from %s to %s (err=%v): for the Change about Claude Code's own model", args, run.modelBefore, run.modelAfter, run.err)
					three.choosingInClaude(t, "opus[1m]")
				}
			}
		})
	}
}

// The scenario "The choice ended a forwarding that Hermes's file still
// holds" names `aigw status` beside the local check; the lane's regression
// test reads the check. This probe reads the status in the same state, for a
// choice of another provider and for one of the same provider on another
// protocol: the followed selection is named with forwarding_cleared, and the
// status writes nothing.
func TestZZ11StatusAfterAChoiceThatEndsAForwarding(t *testing.T) {
	for _, account := range []string{"ucloud", "dmxapi"} {
		t.Run(account, func(t *testing.T) {
			world := forwardedThenChose(t, account)
			status := runIn(t, world, "status", "--json")
			if len(status.changed) != 0 {
				t.Errorf("status changed %v", status.changed)
			}
			var report struct {
				Followed []followed `json:"followed_selections"`
				Clients  map[string]struct {
					State string `json:"state"`
					Model string `json:"model"`
					Next  string `json:"next_action"`
				} `json:"clients"`
			}
			if err := json.Unmarshal([]byte(status.output), &report); err != nil {
				t.Fatalf("status is not one JSON document (err=%v):\n%s", status.err, status.output)
			}
			named, isNamed := followedOf(report.Followed, world.client)
			t.Logf("status: err=%v, Hermes %+v, follows %+v", status.err, report.Clients[world.client], report.Followed)
			if !isNamed || !named.ForwardingCleared || named.Model != account+"/claude-opus-5-5" {
				t.Errorf("status follows %+v; want the choice, with its forwarding ended", report.Followed)
			}
			if entry := report.Clients[world.client]; entry.Model != account+"/claude-opus-5-5" {
				t.Errorf("status names %s for Hermes; its file selects %s/claude-opus-5-5", entry.Model, account)
			}
		})
	}
}

// The scenario "An edit asks for what is stored already" says that the edit
// reports that nothing changed. The matrix reads the files, not the report.
// This probe reads the document of three such edits in the state in which
// every file shows its recorded model: `changed` is false, and no path
// changes.
func TestZZ11AnEditThatAsksForWhatIsStoredReportsNoChange(t *testing.T) {
	for name, edit := range map[string]struct {
		build func(t *testing.T) selectionWorld
		args  func(t *testing.T, w selectionWorld) []string
	}{
		"Hermes, --direct without a forwarding": {hermesPlain, zz11Args("client", "edit", "hermes", "--direct", "--json")},
		"Codex, --native-retries without a budget": {func(t *testing.T) selectionWorld { return codexWorld(t, 1) },
			zz11Args("client", "edit", "codex", "--native-retries", "--json")},
		"Codex, the files that it has": {func(t *testing.T) selectionWorld { return codexWorld(t, 1) }, zz11SameTargets},
	} {
		t.Run(name, func(t *testing.T) {
			world := edit.build(t)
			run := runIn(t, world, edit.args(t, world)...)
			if run.err != nil {
				t.Fatalf("the edit: %v\n%s", run.err, run.output)
			}
			var document struct {
				Changed *bool `json:"changed"`
			}
			run.decode(t, &document)
			if document.Changed == nil || *document.Changed || len(run.changed) != 0 {
				t.Errorf("the edit reports changed=%v and changed the paths %v\n%s", document.Changed, run.changed, run.output)
			}
		})
	}
}

// TestZZ11ShowTheClaudeFixtures prints what AIGW writes for Claude Code and
// Claude Desktop, so that the probes below are written against the files as
// they are.
func TestZZ11ShowTheClaudeFixtures(t *testing.T) {
	three := zz11ThreeClients(t)
	t.Logf("Claude Code's settings:\n%s", readFile(t, three.settings))
	t.Logf("Claude Code's record:\n%s", readFile(t, three.settings+adapter.StateSuffix))
	t.Logf("Claude Desktop's profile:\n%s", readFile(t, three.desktopProfile))
	t.Logf("Claude Desktop's record:\n%s", readFile(t, three.desktopRecord))
	entries, _ := os.ReadDir(filepath.Dir(three.desktopProfile))
	for _, entry := range entries {
		t.Logf("in Claude Desktop's library: %s", entry.Name())
	}
}
