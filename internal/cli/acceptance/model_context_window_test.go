package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

// A model declares its context window: a model that declares none has the
// default, a team file and the model commands set one, and the configuration
// holds it only for a model that declares it.

// windowDocument is what a model command's JSON document says of a model's
// context window; a list holds one item for each model.
type windowDocument struct {
	ID            string           `json:"id"`
	ContextWindow *int             `json:"context_window"`
	Models        []windowDocument `json:"models"`
}

// windowTeamFile is a team file whose provider serves small, which declares
// the context window that %s is, and large, which declares none.
const windowTeamFile = `version = 8

[[providers]]
id = "team"
label = "Team"
endpoints = {anthropic = "https://team.test"}

[[providers.models]]
id = "small"
protocols = {anthropic = []}
context_window = %s

[[providers.models]]
id = "large"
protocols = {anthropic = []}
`

// windowApp is a home in which the team file has been applied with small's
// context window at 256000.
func windowApp(t *testing.T) (*cli.App, *bytes.Buffer) {
	t.Helper()
	app, out, _, _, _ := testApp(t, "")
	if err := cli.Execute(app, []string{"setup", "--from", writeConfigurationManifest(t, fmt.Sprintf(windowTeamFile, "256000"))}); err != nil {
		t.Fatalf("setup --from: %v\n%s", err, out)
	}
	return app, out
}

// runWindowJSON runs a command that writes one JSON document and decodes it.
func runWindowJSON(t *testing.T, app *cli.App, out *bytes.Buffer, args ...string) windowDocument {
	t.Helper()
	out.Reset()
	if err := cli.Execute(app, args); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
	var document windowDocument
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatalf("%v wrote no JSON document: %v\n%s", args, err, out)
	}
	return document
}

// runWindowText runs a command and returns its human output with runs of
// spaces joined.
func runWindowText(t *testing.T, app *cli.App, out *bytes.Buffer, args ...string) string {
	t.Helper()
	out.Reset()
	if err := cli.Execute(app, args); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
	return strings.Join(strings.Fields(out.String()), " ")
}

// assertReportedWindow requires `aigw model list --json` and `aigw model show
// --json` to report want as the context window of model.
func assertReportedWindow(t *testing.T, app *cli.App, out *bytes.Buffer, model string, want int) {
	t.Helper()
	listed := runWindowJSON(t, app, out, "model", "list", "--json")
	found := false
	for _, item := range listed.Models {
		if item.ContextWindow == nil {
			t.Fatalf("model list --json reports no context_window for %s", item.ID)
		}
		if item.ID == model {
			found = true
			if *item.ContextWindow != want {
				t.Fatalf("model list --json reports the context window %d for %s, want %d", *item.ContextWindow, model, want)
			}
		}
	}
	if !found {
		t.Fatalf("model list --json lacks %s: %+v", model, listed.Models)
	}
	shown := runWindowJSON(t, app, out, "model", "show", model, "--json")
	if shown.ContextWindow == nil || *shown.ContextWindow != want {
		t.Fatalf("model show %s --json reports the context window %v, want %d", model, shown.ContextWindow, want)
	}
}

// declaredWindow is the context window that the stored configuration declares
// for model, or zero when it declares none.
func declaredWindow(t *testing.T, app *cli.App, model string) int {
	t.Helper()
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ref, err := domain.ParseModelRef(model)
	if err != nil {
		t.Fatal(err)
	}
	declared, found := cfg.Models[ref]
	if !found {
		t.Fatalf("the configuration has no model %s", model)
	}
	return declared.ContextWindow
}

// exportedTeamFile is what `aigw config export` writes.
func exportedTeamFile(t *testing.T, app *cli.App, out *bytes.Buffer) string {
	t.Helper()
	out.Reset()
	if err := cli.Execute(app, []string{"config", "export"}); err != nil {
		t.Fatalf("config export: %v\n%s", err, out)
	}
	return out.String()
}

// Scenario "A model declares no window": the reports give it the default,
// and neither aigw.toml nor `aigw config export` holds a window for it.
func TestAModelThatDeclaresNoWindowHasTheDefault(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, "claude", "plain")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	assertReportedWindow(t, app, out, "one/plain", domain.DefaultContextWindow)
	if stored := string(readFile(t, app.Config.Path())); strings.Contains(stored, "context_window") {
		t.Fatalf("aigw.toml holds a context window that no model declares:\n%s", stored)
	}
	if exported := exportedTeamFile(t, app, out); strings.Contains(exported, "context_window") || !strings.Contains(exported, "id = 'plain'") {
		t.Fatalf("config export holds a context window that no model declares:\n%s", exported)
	}
	// Human output of the list names no window that is the default; show has
	// a row for it.
	if text := runWindowText(t, app, out, "model", "list"); strings.Contains(text, "context window") {
		t.Fatalf("model list names the default window:\n%s", out)
	}
	if text, want := runWindowText(t, app, out, "model", "show", "one/plain"), fmt.Sprintf("Context window %d tokens", domain.DefaultContextWindow); !strings.Contains(text, want) {
		t.Fatalf("model show lacks %q:\n%s", want, out)
	}
}

// Scenario "A team file declares a lower window": after `aigw setup --from`
// the configuration and every report give the model that window, the model
// beside it keeps the default, and human output of the list shows the window
// that differs.
func TestATeamFileDeclaresALowerWindow(t *testing.T) {
	app, out := windowApp(t)
	if got := declaredWindow(t, app, "team/small"); got != 256000 {
		t.Fatalf("the configuration declares %d for team/small, want 256000", got)
	}
	if got := declaredWindow(t, app, "team/large"); got != 0 {
		t.Fatalf("the configuration declares %d for team/large, which the team file gives no window", got)
	}
	assertReportedWindow(t, app, out, "team/small", 256000)
	assertReportedWindow(t, app, out, "team/large", domain.DefaultContextWindow)
	const declaration = "id = 'small'\nprotocols = { anthropic = [] }\ncontext_window = 256000\n"
	for name, text := range map[string]string{"aigw.toml": string(readFile(t, app.Config.Path())), "config export": exportedTeamFile(t, app, out)} {
		if strings.Count(text, "context_window") != 1 || !strings.Contains(text, declaration) {
			t.Fatalf("%s does not hold the window for team/small alone:\n%s", name, text)
		}
	}
	list := runWindowText(t, app, out, "model", "list")
	if !strings.Contains(list, "team/small Team · small · Anthropic Messages · context window 256000") || strings.Count(list, "context window") != 1 {
		t.Fatalf("model list does not show the window of team/small alone:\n%s", out)
	}
	if text := runWindowText(t, app, out, "model", "show", "team/small"); !strings.Contains(text, "Context window 256000 tokens") {
		t.Fatalf("model show lacks the window:\n%s", out)
	}
}

// Scenario "An operator sets a window and returns to the default": an edit
// declares a window, another removes the declaration, and an addition
// declares one, each in the configuration, in its own document and in text.
func TestAnOperatorSetsAWindowAndReturnsToTheDefault(t *testing.T) {
	app, out := windowApp(t)
	edited := runWindowJSON(t, app, out, "model", "edit", "team/large", "--context-window", "512000", "--json")
	if edited.ContextWindow == nil || *edited.ContextWindow != 512000 || declaredWindow(t, app, "team/large") != 512000 {
		t.Fatalf("model edit --context-window 512000 reports %v and stores %d", edited.ContextWindow, declaredWindow(t, app, "team/large"))
	}
	assertReportedWindow(t, app, out, "team/large", 512000)
	// A preview names the window and writes nothing.
	before := readFile(t, app.Config.Path())
	if text := runWindowText(t, app, out, "model", "edit", "team/large", "--default-context-window", "--dry-run"); !strings.Contains(text, fmt.Sprintf("Context window %d tokens", domain.DefaultContextWindow)) ||
		!strings.Contains(text, "aigw model edit team/large --default-context-window") || !bytes.Equal(readFile(t, app.Config.Path()), before) {
		t.Fatalf("the preview of --default-context-window:\n%s", out)
	}
	restored := runWindowJSON(t, app, out, "model", "edit", "team/large", "--default-context-window", "--json")
	if restored.ContextWindow == nil || *restored.ContextWindow != domain.DefaultContextWindow || declaredWindow(t, app, "team/large") != 0 {
		t.Fatalf("model edit --default-context-window reports %v and stores %d", restored.ContextWindow, declaredWindow(t, app, "team/large"))
	}
	assertReportedWindow(t, app, out, "team/large", domain.DefaultContextWindow)
	if stored := string(readFile(t, app.Config.Path())); strings.Count(stored, "context_window") != 1 {
		t.Fatalf("aigw.toml still holds the removed declaration:\n%s", stored)
	}
	// Another edit leaves a declared window as it is.
	if text := runWindowText(t, app, out, "model", "edit", "team/small", "--label", "Small"); !strings.Contains(text, "Context window 256000 tokens") || declaredWindow(t, app, "team/small") != 256000 {
		t.Fatalf("an edit of the label changed the window:\n%s", out)
	}
	added := runWindowJSON(t, app, out, "model", "add", "team/extra", "--unlisted", "--context-window", "512000", "--json")
	if added.ContextWindow == nil || *added.ContextWindow != 512000 || declaredWindow(t, app, "team/extra") != 512000 {
		t.Fatalf("model add --context-window 512000 reports %v and stores %d", added.ContextWindow, declaredWindow(t, app, "team/extra"))
	}
	assertReportedWindow(t, app, out, "team/extra", 512000)
	plain := runWindowJSON(t, app, out, "model", "add", "team/plain", "--unlisted", "--json")
	if plain.ContextWindow == nil || *plain.ContextWindow != domain.DefaultContextWindow || declaredWindow(t, app, "team/plain") != 0 {
		t.Fatalf("model add without --context-window reports %v and stores %d", plain.ContextWindow, declaredWindow(t, app, "team/plain"))
	}
}

// Scenario "A window is not a whole number above zero": the flag with zero, a
// negative number or text, both flags together, and a team file with such a
// value are refused, and the configuration and the file of the client that
// uses the model stay as they were.
func TestAWindowThatIsNotAWholeNumberAboveZeroIsRefused(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "team", "Team", domain.Endpoints{Responses: "https://team.test/v1", Anthropic: "https://team.test"}, codex.ID, "small")
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("team", "small"), "")
	cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{target})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	configuration, clientFile := readFile(t, app.Config.Path()), readFile(t, target)
	refused := func(want string, args ...string) {
		t.Helper()
		out.Reset()
		err := cli.Execute(app, args)
		if err == nil || !strings.Contains(out.String(), want) {
			t.Fatalf("%v = %v, want a refusal with %q:\n%s", args, err, want, out)
		}
		if !bytes.Equal(readFile(t, app.Config.Path()), configuration) || !bytes.Equal(readFile(t, target), clientFile) {
			t.Fatalf("the refused %v changed the configuration or Codex's file", args)
		}
	}
	// Each refused edit also changes the label, which reaches Codex's file, so
	// an edit that ran would have written the configuration and that file.
	edit := func(flags ...string) []string {
		return append([]string{"model", "edit", "team/small", "--label", "Renamed"}, flags...)
	}
	const wholeNumber = "expected a whole number of tokens above zero"
	for _, value := range []string{"0", "-1", "many", "1.5", ""} {
		flag := `invalid argument "` + value + `" for "--context-window" flag: ` + wholeNumber
		refused(flag+"; run `aigw model edit --help`", edit("--context-window", value)...)
		refused(flag+"; run `aigw model add --help`", "model", "add", "team/extra", "--unlisted", "--context-window", value)
	}
	refused("--context-window and --default-context-window are mutually exclusive; run `aigw model edit --help`",
		edit("--context-window", "512000", "--default-context-window")...)
	refused("this flag takes one value", edit("--context-window", "512000", "--context-window", "256000")...)
	refused("unknown option --default-context-window; run `aigw model add --help`", "model", "add", "team/extra", "--unlisted", "--default-context-window")
	// A refused flag fails with the JSON document when --json was given.
	out.Reset()
	if err := cli.Execute(app, edit("--json", "--context-window", "0")); err == nil {
		t.Fatal("--context-window 0 succeeded")
	}
	var failure struct {
		OK         bool   `json:"ok"`
		Error      string `json:"error"`
		NextAction string `json:"next_action"`
	}
	if err := json.Unmarshal(out.Bytes(), &failure); err != nil || failure.OK || failure.NextAction != "aigw model edit --help" || !strings.Contains(failure.Error, wholeNumber) {
		t.Fatalf("the JSON refusal = %s, %v", out, err)
	}
	if !bytes.Equal(readFile(t, app.Config.Path()), configuration) || !bytes.Equal(readFile(t, target), clientFile) {
		t.Fatal("the refused --json edit changed the configuration or Codex's file")
	}
	// A team file that declares zero or a negative number is refused by its
	// model's entry, and one that declares no whole number where it is read.
	for _, value := range []string{"0", "-1"} {
		refused("invalid team file: model team/small: a context window is a whole number of tokens above zero, not "+value,
			"setup", "--from", writeConfigurationManifest(t, fmt.Sprintf(windowTeamFile, value)))
		if !strings.Contains(out.String(), "Correct model `team/small` in the team file") {
			t.Fatalf("the refusal of context_window = %s does not name the entry to correct:\n%s", value, out)
		}
	}
	for _, value := range []string{"'many'", "1.5"} {
		refused("Cannot decode TOML document at line 11", "setup", "--from", writeConfigurationManifest(t, fmt.Sprintf(windowTeamFile, value)))
	}
	// The same edit with a window above zero writes both, so the refusals
	// above stopped a command that would have.
	out.Reset()
	if err := cli.Execute(app, edit("--context-window", "512000")); err != nil {
		t.Fatalf("model edit --context-window 512000: %v\n%s", err, out)
	}
	if declaredWindow(t, app, "team/small") != 512000 || bytes.Equal(readFile(t, target), clientFile) {
		t.Fatalf("an accepted edit did not reach the configuration and Codex's file:\n%s", out)
	}
}

// With only one of the two flags, `aigw model edit` has an edit to make, and
// without any flag its refusal names both.
func TestModelEditNamesItsContextWindowFlags(t *testing.T) {
	app, out := windowApp(t)
	out.Reset()
	if err := cli.Execute(app, []string{"model", "edit", "team/small"}); err == nil ||
		!strings.Contains(out.String(), "model edit needs --protocol, --family, --capability, --label, --context-window or --default-context-window; run `aigw model edit --help`") {
		t.Fatalf("an edit without a flag = %v:\n%s", err, out)
	}
	for _, args := range [][]string{{"--context-window", "128000"}, {"--default-context-window"}} {
		out.Reset()
		if err := cli.Execute(app, append([]string{"model", "edit", "team/small"}, args...)); err != nil {
			t.Fatalf("model edit %v: %v\n%s", args, err, out)
		}
	}
	if got := declaredWindow(t, app, "team/small"); got != 0 {
		t.Fatalf("after --default-context-window the configuration declares %d", got)
	}
}
