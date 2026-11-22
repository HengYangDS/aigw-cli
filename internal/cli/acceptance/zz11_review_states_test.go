package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"

	"github.com/pelletier/go-toml/v2"
)

// This file holds the eleventh review's probes of states that the matrix does
// not build: other ways in which a file's format says a pick, and a file
// whose model, record and stored selection are three different models.
//
// Each probe names the three values: F, the model that a file selects, read
// by the parser of the file's own format; R, the model that AIGW's record of
// the file names; S, the stored selection.

// zz11Codex returns a Codex world whose readers report a file that the TOML
// parser refuses in words, where the matrix's readers end the test. A
// byte-order mark is taken off before the parse, as a TOML reader does that
// accepts one.
func zz11Codex(w selectionWorld) selectionWorld {
	shows := func(t *testing.T, path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			return ""
		}
		var document struct {
			Model string `toml:"model"`
		}
		if err := toml.Unmarshal(bytes.TrimPrefix(data, []byte("\uFEFF")), &document); err != nil {
			return "<not TOML: " + strings.ReplaceAll(err.Error(), "\n", " ") + ">"
		}
		return document.Model
	}
	recordOf := w.recordOf
	w.shows = shows
	w.follows = func(t *testing.T, path string) bool {
		t.Helper()
		record := recordOf(t, path)
		if record == nil {
			return false
		}
		var state struct {
			ProjectedModel string `json:"projected_model"`
		}
		if err := json.Unmarshal(record, &state); err != nil {
			t.Fatal(err)
		}
		return state.ProjectedModel == shows(t, path)
	}
	return w
}

// zz11CodexPick is one way to write a model into Codex's file. write returns
// the file's text from the lines that AIGW wrote, the index of its root model
// line among them, and the model.
type zz11CodexPick struct {
	name  string
	write func(lines []string, line int, model string) string
	// root says that the root key model of the result names the model. A
	// spelling without it leaves the root key as it was, or without a value.
	root bool
	// unread says that AIGW's TOML parser refuses the file as a whole. AIGW
	// then leaves the client as it is stored, as it does for any file that it
	// cannot read: a command about something else succeeds and records
	// nothing, and a command that writes Codex's files is refused.
	unread bool
}

// zz11Line writes line, in which MODEL stands for the model, in place of the
// root model line.
func zz11Line(line string) func(lines []string, at int, model string) string {
	return func(lines []string, at int, model string) string {
		lines = slices.Clone(lines)
		lines[at] = strings.Replace(line, "MODEL", model, 1)
		return strings.Join(lines, "\n")
	}
}

func zz11CodexPicks() []zz11CodexPick {
	return []zz11CodexPick{
		{name: "the value alone, AIGW's comment kept", root: true, write: zz11Line(`model = "MODEL" # managed by AIGW`)},
		{name: "a comment that holds quotes", root: true, write: zz11Line(`model = "MODEL" # was "gpt-sol"`)},
		{name: "a literal string and a comment with a sign and a quote", root: true, write: zz11Line(`model = 'MODEL' # it's = mine`)},
		{name: "no space before the comment", root: true, write: zz11Line(`model="MODEL"#mine`)},
		{name: "spaces after the value", root: true, write: zz11Line("model = \"MODEL\"  \t")},
		{name: "spaces before the key", root: true, write: zz11Line(`  model = "MODEL"`)},
		{name: "tabs around the sign", root: true, write: zz11Line("model\t=\t\"MODEL\"")},
		{name: "a quoted key", root: true, write: zz11Line(`"model" = "MODEL"`)},
		{name: "a literal key", root: true, write: zz11Line(`'model' = "MODEL"`)},
		{name: "a multi-line basic string on one line", root: true, write: zz11Line(`model = """MODEL"""`)},
		{name: "a multi-line literal string on one line", root: true, write: zz11Line(`model = '''MODEL'''`)},
		{name: "a multi-line basic string on two lines", root: true, write: zz11Line("model = \"\"\"\nMODEL\"\"\"")},
		{name: "a multi-line basic string with a line-ending backslash", root: true, write: zz11Line("model = \"\"\"\\\n  MODEL\"\"\"")},
		{name: "an escaped character", root: true, write: func(lines []string, at int, model string) string {
			return zz11Line(`model = "MODEL"`)(lines, at, strings.ReplaceAll(model, "-", `\u002D`))
		}},
		{name: "a carriage return on the model line", root: true, write: zz11Line("model = \"MODEL\"\r")},
		{name: "the whole file with carriage returns", root: true, write: func(lines []string, at int, model string) string {
			return strings.ReplaceAll(zz11Line(`model = "MODEL"`)(lines, at, model), "\n", "\r\n")
		}},
		{name: "the whole file with carriage returns, AIGW's comment kept", root: true, write: func(lines []string, at int, model string) string {
			return strings.ReplaceAll(zz11Line(`model = "MODEL" # managed by AIGW`)(lines, at, model), "\n", "\r\n")
		}},
		{name: "a byte-order mark", root: true, unread: true, write: func(lines []string, at int, model string) string {
			return "\uFEFF" + zz11Line(`model = "MODEL"`)(lines, at, model)
		}},
		{name: "the line moved to the top", root: true, write: func(lines []string, at int, model string) string {
			moved := append([]string{`model = "` + model + `"`}, lines[:at]...)
			return strings.Join(append(moved, lines[at+1:]...), "\n")
		}},
		{name: "AIGW's line as a comment, the pick below", root: true, write: func(lines []string, at int, model string) string {
			return zz11Line("# "+lines[at]+"\nmodel = \"MODEL\"")(lines, at, model)
		}},
		{name: "AIGW's line as a comment, the pick above", root: true, write: func(lines []string, at int, model string) string {
			return zz11Line("model = \"MODEL\"\n# "+lines[at])(lines, at, model)
		}},
		{name: "a comment line above the pick", root: true, write: zz11Line("# my pick\nmodel = \"MODEL\"")},
	}
}

// zz11WritePick writes the pick of model into the world's Codex file at
// index, from the file as AIGW wrote it.
func zz11WritePick(t *testing.T, w selectionWorld, index int, pick zz11CodexPick, model string) selectionWorld {
	t.Helper()
	lines, at := recCodexModelLine(t, w.files[index])
	if at < 0 {
		t.Fatalf("no root model in %s", w.files[index])
	}
	writeFile(t, w.files[index], []byte(pick.write(lines, at, model)), 0o600)
	if got := w.shows(t, w.files[index]); pick.root && got != model {
		t.Fatalf("the pick %q of %s reads as %q through the TOML parser:\n%s", pick.name, model, got, readFile(t, w.files[index]))
	}
	return w
}

// zz11Did says what a command that neither selects for the client nor
// withdraws it did, and what it did that no such command may do.
//
// The classes: "refused" (it failed), "recorded" (the stored selection is the
// model that the file chose and every record names what its file selects),
// "left" (the client's binding, files and records are as before), and
// "broken" for anything else. A fault is a refusal that changed a path, or a
// success that changed which model a file selects: only an explicit selection,
// a rollback and a withdrawal are asked to do that.
func zz11Did(run ranIn) (class string, faults []string) {
	if run.err != nil {
		if len(run.changed) != 0 {
			faults = append(faults, fmt.Sprintf("it was refused and changed %v", run.changed))
		}
		return "refused", faults
	}
	choice := ""
	for index := range run.before.shows {
		if run.before.shows[index] != "" && !run.before.follows[index] {
			choice = run.before.shows[index]
		}
	}
	for index := range run.before.shows {
		// A file that showed its recorded model may be brought to the model
		// that another file chose; nothing else may change what a file selects.
		moved := run.after.files[index] != nil && run.after.shows[index] != run.before.shows[index]
		if moved && (!run.before.follows[index] || run.after.shows[index] != choice) {
			faults = append(faults, fmt.Sprintf("file %d selected %q and now selects %q", index, run.before.shows[index], run.after.shows[index]))
		}
	}
	if run.after.clientUntouched(run.before) {
		return "left", faults
	}
	if !slices.Contains(run.after.follows, false) && (choice == "" || run.after.stored == choice) {
		return "recorded", faults
	}
	return "broken", append(faults, "it saved without every record naming what its file selects, or stored another model than the file chose")
}

// zz11Commands are the three commands that commit without selecting for
// Codex: one about another provider, a synchronization, and an edit of
// Codex's own setting.
func zz11Commands() map[string][]string {
	return map[string][]string{
		"another provider": {"provider", "edit", "spare", "--label", "Other", "--json"},
		"sync":             {"sync", "--json"},
		"own setting":      {"client", "edit", "codex", "--request-retries", "2", "--json"},
	}
}

// zz11Status reads how `aigw status --json` reports client, and requires that
// the report changes no path.
func zz11Status(t *testing.T, w selectionWorld) string {
	t.Helper()
	status := runIn(t, w, "status", "--json")
	if len(status.changed) != 0 {
		t.Errorf("status changed %v", status.changed)
	}
	var report struct {
		Followed []followed `json:"followed_selections"`
		Clients  map[string]struct {
			State  string `json:"state"`
			Model  string `json:"model"`
			Detail string `json:"detail"`
			Next   string `json:"next_action"`
		} `json:"clients"`
	}
	if err := json.Unmarshal([]byte(status.output), &report); err != nil {
		return fmt.Sprintf("status is not one JSON document (err=%v)", status.err)
	}
	entry := report.Clients[w.client]
	follows := "follows nothing"
	if chosen, ok := followedOf(report.Followed, w.client); ok {
		follows = "follows " + chosen.Model + " over " + chosen.Earlier
	}
	return fmt.Sprintf("status: state=%s model=%s detail=%q next=%q, %s", entry.State, entry.Model, entry.Detail, entry.Next, follows)
}

// A pick of another added model in Codex's one file, written in each further
// way that TOML allows, with the Token and without it.
//
// Before: F = R = gpt-sol and S = gateway/gpt-sol. The person selects
// gpt-terra in the spelling: F = gpt-terra by the TOML parser, R = gpt-sol,
// S = gateway/gpt-sol. Nothing in the state prevents the record.
//
// Then one of three commands runs that do not select for Codex. The rule: the
// command records the pick (S = gateway/gpt-terra, R = gpt-terra, F kept), or
// is refused and changes nothing. It never changes which model the file
// selects, it never succeeds with the record left behind the file, and the
// Token decides none of this.
func TestZZ11ACodexPickInFurtherSpellingsIsRecordedOrRefused(t *testing.T) {
	for _, pick := range zz11CodexPicks() {
		t.Run(pick.name, func(t *testing.T) {
			t.Log(zz11Status(t, zz11WritePick(t, zz11Codex(codexWorld(t, 1)), 0, pick, "gpt-terra")))
			for _, name := range slices.Sorted(mapKeys(zz11Commands())) {
				classes := map[bool]string{}
				for _, token := range []bool{true, false} {
					world := zz11Codex(codexWorld(t, 1))
					world.setToken(t, token)
					world = zz11WritePick(t, world, 0, pick, "gpt-terra")
					run := runIn(t, world, zz11Commands()[name]...)
					class, faults := zz11Did(run)
					classes[token] = class
					t.Logf("%s, token=%t: %s (err=%v)\n  before: %s\n  after:  %s", name, token, class, run.err, shownLine(run.before), shownLine(run.after))
					for _, fault := range faults {
						t.Errorf("%s, token=%t: %s", name, token, fault)
					}
					if class == "broken" || (class == "left" && !pick.unread) {
						t.Errorf("%s, token=%t succeeded and left the record behind the pick, which nothing in the state prevents: %s", name, token, shownLine(run.after))
					}
				}
				if classes[true] != classes[false] {
					t.Errorf("%s: the Token decides the outcome: %q with it, %q without it", name, classes[true], classes[false])
				}
			}
		})
	}
}

// A pick of the stored model beside an earlier record, in each further
// spelling: the state that the eighth review found, in the spellings that the
// matrix does not have.
//
// Before: S = gateway/gpt-sol waits for its Token, F = R = gpt-terra. The
// person selects gpt-sol in the spelling: F = gpt-sol = S, R = gpt-terra.
//
// A command commits without the Token. The rule: R = gpt-sol afterwards, or
// the command is refused. Then the person selects gpt-terra again in the same
// spelling, the Token arrives and `aigw sync` runs: gpt-terra, picked last, is
// stored, selected by the file and named by the record, or the
// synchronization is refused and changes nothing. It never writes gpt-sol
// over the pick.
func TestZZ11ACodexPickOfTheStoredModelInFurtherSpellingsIsRecorded(t *testing.T) {
	for _, pick := range zz11CodexPicks() {
		t.Run(pick.name, func(t *testing.T) {
			for _, name := range []string{"another provider", "sync"} {
				world := zz11Codex(codexPending(t, 1))
				lines, at := recCodexModelLine(t, world.files[0])
				writeFile(t, world.files[0], []byte(pick.write(slices.Clone(lines), at, "gpt-sol")), 0o600)
				commit := runIn(t, world, zz11Commands()[name]...)
				class, faults := zz11Did(commit)
				t.Logf("%s without the Token: %s (err=%v)\n  before: %s\n  after:  %s", name, class, commit.err, shownLine(commit.before), shownLine(commit.after))
				for _, fault := range faults {
					t.Errorf("%s: %s", name, fault)
				}
				if commit.err == nil && slices.Contains(commit.after.follows, false) && !pick.unread {
					t.Errorf("%s committed and left the record behind the file: %s", name, shownLine(commit.after))
				}
				// The person returns to gpt-terra in the same spelling.
				writeFile(t, world.files[0], []byte(pick.write(slices.Clone(lines), at, "gpt-terra")), 0o600)
				world.setToken(t, true)
				sync := runIn(t, world, "sync", "--json")
				t.Logf("then gpt-terra picked again, the Token, sync: err=%v\n  before: %s\n  after:  %s", sync.err, shownLine(sync.before), shownLine(sync.after))
				switch {
				case sync.err != nil:
					if len(sync.changed) != 0 {
						t.Errorf("the refused synchronization changed %v", sync.changed)
					}
				case !settledOn(sync.after, "gpt-terra"):
					t.Errorf("the person selected gpt-terra last; the synchronization left: %s", shownLine(sync.after))
				}
			}
		})
	}
}

// Three different models: a stored selection that waits for its Token, a
// record that names the model that AIGW wrote before, and a file in which the
// person picked a third model. The matrix has every pair of the three values
// equal in some state, and never all three apart.
//
// Before: S = gateway/gpt-sol waits, F = R = gpt-terra. The person selects
// gpt-luna: F = gpt-luna, R = gpt-terra, S = gateway/gpt-sol.
//
// A command that does not select for Codex commits, with the Token and
// without it. The rule: S = gateway/gpt-luna and R = gpt-luna afterwards, F
// kept; the Token decides neither.
func TestZZ11AThirdModelPickedWhileTheStoredSelectionWaits(t *testing.T) {
	for _, files := range []int{1, 2} {
		for _, name := range slices.Sorted(mapKeys(zz11Commands())) {
			classes := map[bool]string{}
			for _, token := range []bool{true, false} {
				t.Run(fmt.Sprintf("files=%d/%s/token=%t", files, name, token), func(t *testing.T) {
					world := zz11Codex(codexPending(t, files))
					recPick(t, world.files[0], "gpt-luna")
					world.setToken(t, token)
					run := runIn(t, world, zz11Commands()[name]...)
					class, faults := zz11Did(run)
					classes[token] = class
					t.Logf("%s (err=%v)\n  before: %s\n  after:  %s", class, run.err, shownLine(run.before), shownLine(run.after))
					for _, fault := range faults {
						t.Error(fault)
					}
					if class != "recorded" {
						t.Errorf("%s did not record the pick of gpt-luna: %s", name, shownLine(run.after))
					}
				})
			}
			if classes[true] != classes[false] {
				t.Errorf("files=%d, %s: the Token decides the outcome: %q with it, %q without it", files, name, classes[true], classes[false])
			}
		}
	}
}

// Codex's file without a root model, or with one that is no model: the
// person took AIGW's model line out, emptied it, or moved it below a table,
// where it is no longer the root key. Codex then selects its own default.
// AIGW reads a file without a root model as one that shows the recorded model
// (readSelection), which was the tenth review's cause in another shape.
//
// Before: F = R = gpt-sol, S = gateway/gpt-sol. After the edit the file
// selects no model, and R = gpt-sol.
//
// The rule for a setting of AIGW's that changed outside: no command follows
// it, a command that writes Codex's files is refused and changes nothing, and
// no command puts gpt-sol back without being asked.
func TestZZ11ACodexFileWithoutARootModel(t *testing.T) {
	shapes := map[string]func(lines []string, at int) string{
		"the line removed": func(lines []string, at int) string {
			return strings.Join(append(slices.Clone(lines[:at]), lines[at+1:]...), "\n")
		},
		"an empty model": func(lines []string, at int) string { return zz11Line(`model = ""`)(lines, at, "") },
		"the model below a table": func(lines []string, at int) string {
			text := strings.Join(append(slices.Clone(lines[:at]), lines[at+1:]...), "\n")
			return strings.Replace(text, "[agents]\n", "[agents]\nmodel = \"gpt-terra\"\n", 1)
		},
	}
	for _, name := range slices.Sorted(mapKeys(shapes)) {
		t.Run(name, func(t *testing.T) {
			build := func(token bool) selectionWorld {
				world := zz11Codex(codexWorld(t, 1))
				world.setToken(t, token)
				lines, at := recCodexModelLine(t, world.files[0])
				writeFile(t, world.files[0], []byte(shapes[name](lines, at)), 0o600)
				if got := world.shows(t, world.files[0]); got != "" {
					t.Fatalf("the file still selects %q", got)
				}
				return world
			}
			t.Log(zz11Status(t, build(true)))
			for _, command := range slices.Sorted(mapKeys(zz11Commands())) {
				outcomes := map[bool]string{}
				for _, token := range []bool{true, false} {
					world := build(token)
					run := runIn(t, world, zz11Commands()[command]...)
					outcomes[token] = fmt.Sprintf("refused=%t stored=%s the file selects %q", run.err != nil, run.after.stored, run.after.shows[0])
					t.Logf("%s, token=%t: err=%v\n  before: %s\n  after:  %s", command, token, run.err, shownLine(run.before), shownLine(run.after))
					if run.err != nil && len(run.changed) != 0 {
						t.Errorf("%s, token=%t was refused and changed %v", command, token, run.changed)
					}
					if run.err == nil && run.after.shows[0] != run.before.shows[0] {
						t.Errorf("%s, token=%t succeeded and wrote the model %q into a file from which the person had taken it", command, token, run.after.shows[0])
					}
				}
				if outcomes[true] != outcomes[false] {
					t.Errorf("%s: the Token decides the outcome: with it %s; without it %s", command, outcomes[true], outcomes[false])
				}
			}
		})
	}
}

// Codex's file selects through a profile: the root key model stays as AIGW
// wrote it, and a profile that the person adds names another model. Codex
// then runs the profile's model; AIGW reads the root key. The probe only
// watches what AIGW reports and that no command removes the profile; whether
// the profile's model is F is a question for the specification.
func TestZZ11ACodexProfileThatNamesAnotherModel(t *testing.T) {
	world := zz11Codex(codexWorld(t, 1))
	lines, at := recCodexModelLine(t, world.files[0])
	lines[at] += "\nprofile = \"mine\""
	text := strings.Join(lines, "\n") + "\n[profiles.mine]\nmodel = \"gpt-terra\"\n"
	writeFile(t, world.files[0], []byte(text), 0o600)
	t.Log(zz11Status(t, world))
	for _, name := range slices.Sorted(mapKeys(zz11Commands())) {
		run := runIn(t, world, zz11Commands()[name]...)
		class, _ := zz11Did(run)
		t.Logf("%s: %s (err=%v); the file still holds the profile: %t", name, class, run.err,
			bytes.Contains(readFile(t, world.files[0]), []byte("[profiles.mine]")))
	}
	if got := selection(t, world.app, codex.ID).Model.String(); got != "gateway/gpt-sol" {
		t.Logf("the stored selection is now %s", got)
	}
}
