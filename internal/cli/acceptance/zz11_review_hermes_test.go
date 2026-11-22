package cli_test

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/hermes"

	"go.yaml.in/yaml/v4"
)

// This file holds the eleventh review's probes of Hermes's own selection
// written in each further way that YAML allows, and of a pick of another model
// of the same entry.

// zz11HermesBlock writes block in place of the top-level key model of
// Hermes's file, as text, so that the spelling is the one given and no YAML
// writer chooses it. block may hold further top-level keys ahead of model.
func zz11HermesBlock(t *testing.T, path, block string) {
	t.Helper()
	text := strings.ReplaceAll(string(readFile(t, path)), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	// cut removes the top-level key and its block, and returns where it stood.
	cut := func(key string) int {
		start := slices.IndexFunc(lines, func(line string) bool { return strings.HasPrefix(line, key+":") })
		if start < 0 {
			return -1
		}
		end := start + 1
		for end < len(lines) && (lines[end] == "" || lines[end][0] == ' ' || lines[end][0] == '#') {
			end++
		}
		lines = slices.Delete(lines, start, end)
		return start
	}
	// An earlier spelling's own key, which holds its anchor, goes with it.
	cut("mine")
	start := cut("model")
	if start < 0 {
		t.Fatalf("no top-level model in %s:\n%s", path, text)
	}
	lines = slices.Insert(lines, start, strings.Split(strings.TrimSuffix(block, "\n"), "\n")...)
	writeFile(t, path, []byte(strings.Join(lines, "\n")), 0o600)
}

// zz11Hermes returns a Hermes world whose reader of the file reports a file
// that the YAML parser refuses in words. The reader decodes the document into
// a structure, so aliases and merge keys are resolved as YAML defines them.
func zz11Hermes(w selectionWorld) selectionWorld {
	w.shows = func(t *testing.T, path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			return ""
		}
		var projected struct {
			Model struct {
				Provider string `yaml:"provider"`
				Default  string `yaml:"default"`
			} `yaml:"model"`
		}
		if err := yaml.Unmarshal(data, &projected); err != nil {
			return "<not YAML: " + strings.ReplaceAll(err.Error(), "\n", " ") + ">"
		}
		return projected.Model.Provider + "/" + projected.Model.Default
	}
	return w
}

// zz11HermesPick is one way to write Hermes's own selection. block is the
// text of the top-level key model, in which ENTRY, MODEL, ADDRESS and MODE
// stand for the provider entry, the model, and the entry's address and mode.
type zz11HermesPick struct{ name, block string }

func zz11HermesPicks() []zz11HermesPick {
	return []zz11HermesPick{
		{"plain, as AIGW writes it", "model:\n  provider: ENTRY\n  default: MODEL\n  base_url: ADDRESS\n  api_mode: MODE\n"},
		{"a flow mapping", "model: {provider: ENTRY, default: MODEL, base_url: \"ADDRESS\", api_mode: MODE}\n"},
		{"a flow mapping with quoted keys", "model: {\"provider\": \"ENTRY\", \"default\": \"MODEL\", \"base_url\": \"ADDRESS\", \"api_mode\": \"MODE\"}\n"},
		{"the model as a literal block scalar", "model:\n  provider: ENTRY\n  default: |-\n    MODEL\n  base_url: ADDRESS\n  api_mode: MODE\n"},
		{"the model as a folded block scalar", "model:\n  provider: ENTRY\n  default: >-\n    MODEL\n  base_url: ADDRESS\n  api_mode: MODE\n"},
		{"the model with an anchor", "model:\n  provider: ENTRY\n  default: &mine MODEL\n  base_url: ADDRESS\n  api_mode: MODE\n"},
		{"the model as an alias", "mine: &mine MODEL\nmodel:\n  provider: ENTRY\n  default: *mine\n  base_url: ADDRESS\n  api_mode: MODE\n"},
		{"the provider as an alias", "mine: &mine ENTRY\nmodel:\n  provider: *mine\n  default: MODEL\n  base_url: ADDRESS\n  api_mode: MODE\n"},
		{"the selection through a merge key", "mine: &mine\n  provider: ENTRY\n  default: MODEL\n  base_url: ADDRESS\n  api_mode: MODE\nmodel:\n  <<: *mine\n"},
		{"the selection as an alias", "mine: &mine\n  provider: ENTRY\n  default: MODEL\n  base_url: ADDRESS\n  api_mode: MODE\nmodel: *mine\n"},
		{"a comment on the model's line", "model:\n  provider: ENTRY\n  default: MODEL # mine\n  base_url: ADDRESS\n  api_mode: MODE\n"},
		{"a comment line inside the selection", "model:\n  # my pick\n  provider: ENTRY\n  default: MODEL\n  base_url: ADDRESS\n  api_mode: MODE\n"},
		{"the keys in another order", "model:\n  api_mode: MODE\n  base_url: ADDRESS\n  default: MODEL\n  provider: ENTRY\n"},
		{"quoted keys", "model:\n  \"provider\": ENTRY\n  'default': MODEL\n  base_url: ADDRESS\n  api_mode: MODE\n"},
		{"the model with a tag", "model:\n  provider: ENTRY\n  default: !!str MODEL\n  base_url: ADDRESS\n  api_mode: MODE\n"},
		{"four spaces of indentation", "model:\n    provider: ENTRY\n    default: MODEL\n    base_url: ADDRESS\n    api_mode: MODE\n"},
		{"the model named twice, the pick last", "model:\n  provider: ENTRY\n  default: gpt-6.2-sol\n  default: MODEL\n  base_url: ADDRESS\n  api_mode: MODE\n"},
	}
}

// zz11HermesEntries are the entries that the probes pick, with the address
// and mode that AIGW writes for them.
var zz11HermesEntries = map[string][2]string{
	"aigw-dmxapi-responses": {"https://www.dmxapi.cn/v1", "codex_responses"},
	"aigw-dmxapi-anthropic": {"https://www.dmxapi.cn", "anthropic_messages"},
}

// zz11HermesWrite writes the pick of shown (<entry>/<model>) into the world's
// Hermes file in the spelling.
func zz11HermesWrite(t *testing.T, w selectionWorld, pick zz11HermesPick, shown string, crlf bool) {
	t.Helper()
	entry, model, _ := strings.Cut(shown, "/")
	block := strings.NewReplacer("ENTRY", entry, "MODEL", model, "ADDRESS", zz11HermesEntries[entry][0], "MODE", zz11HermesEntries[entry][1]).Replace(pick.block)
	zz11HermesBlock(t, w.files[0], block)
	if crlf {
		writeFile(t, w.files[0], []byte(strings.ReplaceAll(string(readFile(t, w.files[0])), "\n", "\r\n")), 0o600)
	}
}

// zz11HermesCommands are the commands that commit without selecting for
// Hermes.
func zz11HermesCommands() map[string][]string {
	return map[string][]string{
		"another provider": {"provider", "edit", "ucloud", "--label", "Other", "--json"},
		"sync":             {"sync", "--json"},
		"own setting":      {"client", "edit", "hermes", "--via", hermesForwardedTo, "--json"},
	}
}

// zz11HermesSpellingProbe runs one spelling of a Hermes pick through the
// commands that do not select for Hermes, with the Token and without it, and
// then the person's return to the earlier model with the Token stored.
func zz11HermesSpellingProbe(t *testing.T, pick zz11HermesPick, chosen, earlier string, crlf bool) {
	t.Helper()
	build := func(token bool) selectionWorld {
		world := zz11Hermes(hermesPlain(t))
		world.setToken(t, token)
		zz11HermesWrite(t, world, pick, chosen, crlf)
		return world
	}
	first := build(true)
	if got := first.shows(t, first.files[0]); got != chosen {
		t.Logf("the YAML parser reads this spelling as %q; the probe only watches that nothing is lost", got)
	}
	t.Log(zz11Status(t, first))
	for _, command := range slices.Sorted(mapKeys(zz11HermesCommands())) {
		classes := map[bool]string{}
		for _, token := range []bool{true, false} {
			world := build(token)
			run := runIn(t, world, zz11HermesCommands()[command]...)
			class, faults := zz11Did(run)
			classes[token] = class
			t.Logf("%s, token=%t: %s (err=%v)\n  before: %s\n  after:  %s", command, token, class, run.err, shownLine(run.before), shownLine(run.after))
			for _, fault := range faults {
				t.Errorf("%s, token=%t: %s", command, token, fault)
			}
			if class == "left" || class == "broken" {
				t.Errorf("%s, token=%t succeeded and left the record behind the pick, which nothing in the state prevents: %s", command, token, shownLine(run.after))
			}
			if class != "recorded" || token {
				continue
			}
			zz11HermesReturn(t, world, pick, earlier, crlf, command)
		}
		if classes[true] != classes[false] {
			t.Errorf("%s: the Token decides the outcome: %q with it, %q without it", command, classes[true], classes[false])
		}
	}
}

// zz11HermesReturn selects the earlier model again in the same spelling with
// the Token stored, and requires the synchronization to leave that last pick
// stored and in the file, or to be refused and change nothing.
func zz11HermesReturn(t *testing.T, world selectionWorld, pick zz11HermesPick, earlier string, crlf bool, command string) {
	t.Helper()
	zz11HermesWrite(t, world, pick, earlier, crlf)
	world.setToken(t, true)
	sync := runIn(t, world, "sync", "--json")
	switch {
	case sync.err != nil:
		t.Logf("%s, then the earlier model picked again, the Token, sync: refused: %v", command, sync.err)
		if len(sync.changed) != 0 {
			t.Errorf("the refused synchronization changed %v", sync.changed)
		}
	case sync.after.stored != earlier || sync.after.shows[0] != earlier:
		t.Errorf("%s recorded the pick; the person then selected %s again, and the synchronization left: %s", command, earlier, shownLine(sync.after))
	}
}

// A pick of another model of the same entry in Hermes: the provider and the
// protocol stay, the model alone changes. The matrix's choices all change the
// entry.
//
// Before: F = R = aigw-dmxapi-responses/gpt-6.2-sol, S = dmxapi/gpt-6.2-sol.
// The person selects gpt-6.1-terra of the same entry.
//
// A command that does not select for Hermes records it, with the Token and
// without it: S = dmxapi/gpt-6.1-terra on the Responses protocol.
func TestZZ11AHermesPickOfAnotherModelOfTheSameEntryIsRecorded(t *testing.T) {
	const chosen = "aigw-dmxapi-responses/gpt-6.1-terra"
	for _, command := range slices.Sorted(mapKeys(zz11HermesCommands())) {
		for _, token := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/token=%t", command, token), func(t *testing.T) {
				world := zz11Hermes(hermesPlain(t))
				world.setToken(t, token)
				if !pickShown(t, world, chosen) {
					t.Fatal("Hermes's file does not list the entry")
				}
				run := runIn(t, world, zz11HermesCommands()[command]...)
				class, faults := zz11Did(run)
				t.Logf("%s (err=%v)\n  before: %s\n  after:  %s", class, run.err, shownLine(run.before), shownLine(run.after))
				for _, fault := range faults {
					t.Error(fault)
				}
				if class != "recorded" {
					t.Errorf("%s did not record the pick: %s", command, shownLine(run.after))
				}
				if got := selection(t, world.app, hermes.ID); got.Model.String() != "dmxapi/gpt-6.1-terra" {
					t.Errorf("the stored selection is %s", got.Model)
				}
			})
		}
	}
}
