package store

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

// declareWindow gives the model of cfg that ref names a context window.
func declareWindow(cfg *domain.Config, ref domain.ModelRef, window int) {
	model := cfg.Models[ref]
	model.ContextWindow = window
	cfg.Models[ref] = model
}

// holdingWindow returns text, the aigw.toml of schemaEightConfig, in which
// the model dmxapi/claude-opus-5-5 holds `context_window = value`.
func holdingWindow(t *testing.T, text, value string) []byte {
	t.Helper()
	const protocols = "protocols = { anthropic = ['text', 'tool_search'] }\n"
	held := strings.Replace(text, protocols, protocols+"context_window = "+value+"\n", 1)
	if held == text {
		t.Fatalf("the fixture has no %q", protocols)
	}
	return []byte(held)
}

// A declared context window is written under its model, after the model's
// protocols, reads back as declared, and encodes as the same bytes again.
func TestADeclaredContextWindowRoundTrips(t *testing.T) {
	cfg, text := schemaEightConfig(t)
	sol := dmxapiModel("gpt-6.1-sol-cdx")
	declareWindow(&cfg, sol, 256000)
	const protocols = "protocols = { responses = ['text', 'reasoning'] }\n"
	want := strings.Replace(text, protocols, protocols+"context_window = 256000\n", 1)
	if want == text {
		t.Fatalf("the fixture has no %q", protocols)
	}
	data, err := encodeConfig(cfg)
	if err != nil || string(data) != want {
		t.Fatalf("aigw.toml =\n%s\n%v\nwant\n%s", data, err, want)
	}
	loaded, err := decodeTOMLConfig(data, testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range loaded.ModelRefs() {
		want := 0
		if ref == sol {
			want = 256000
		}
		if got := loaded.Models[ref].ContextWindow; got != want {
			t.Errorf("%s reads back with the declared window %d, want %d", ref, got, want)
		}
	}
	if again, err := encodeConfig(loaded); err != nil || string(again) != want {
		t.Fatalf("the loaded configuration encodes as\n%s\n%v", again, err)
	}
}

// A configuration in which no model declares a context window encodes as the
// bytes that it had before the field existed, and neither aigw.toml nor its
// export as a team file holds the key. A model that stores the default, which
// no file and no command gives it, is written as one that declares none.
func TestAModelWithoutAContextWindowEncodesNoSuchKey(t *testing.T) {
	cfg, want := schemaEightConfig(t)
	stored := cfg.Clone()
	declareWindow(&stored, dmxapiModel("gpt-6.1-sol-cdx"), domain.DefaultContextWindow)
	for name, cfg := range map[string]domain.Config{"no model declares a window": cfg, "a model stores the default": stored} {
		data, err := encodeConfig(cfg)
		if err != nil || string(data) != want || strings.Contains(string(data), "context_window") {
			t.Errorf("%s: aigw.toml =\n%s\n%v\nwant\n%s", name, data, err, want)
		}
		exported, err := ExportTeamFile(cfg)
		if err != nil || strings.Contains(string(exported), "context_window") {
			t.Errorf("%s: the exported team file holds a context window:\n%s\n%v", name, exported, err)
		}
	}
}

// A file that gives a model the default context window gives it no
// declaration. aigw.toml reads as the configuration that the file without the
// key holds and is written back without it; a team file's model reads as the
// one beside it that declares none, and the configuration that the file is
// applied to is stored and exported without the key.
func TestAFileThatDeclaresTheDefaultContextWindowDeclaresNone(t *testing.T) {
	window := strconv.Itoa(domain.DefaultContextWindow)
	_, plain := schemaEightConfig(t)
	want, err := decodeTOMLConfig([]byte(plain), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := decodeTOMLConfig(holdingWindow(t, plain, window), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, want) {
		t.Errorf("aigw.toml with context_window = %s does not read as the file without the key does", window)
	}
	if data, err := encodeConfig(loaded); err != nil || string(data) != plain {
		t.Errorf("aigw.toml with context_window = %s is written back as\n%s\n%v", window, data, err)
	}
	file, err := ParseTeamFile(fmt.Appendf(nil, windowTeamFile, window), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	small, large := file.Models[domain.NewModelRef("team", "small")], file.Models[domain.NewModelRef("team", "large")]
	if small.ContextWindow != large.ContextWindow || small.Window() != domain.DefaultContextWindow {
		t.Errorf("the team file's model stores %d and has the window %d; the model that declares none stores %d", small.ContextWindow, small.Window(), large.ContextWindow)
	}
	cfg, _, err := domain.MergeTeamFile(domain.NewConfig(testAdmission()), file, false)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := encodeConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	exported, err := ExportTeamFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"aigw.toml": stored, "the exported team file": exported} {
		if strings.Contains(string(data), "context_window") {
			t.Errorf("%s holds a context window for a model that has the default:\n%s", name, data)
		}
	}
}

// windowTeamFile is a team file whose provider serves two models: small,
// which declares the context window that %s is, and large, which declares
// none.
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

// A team file's context window reaches the configuration that the file is
// applied to, and aigw.toml and the configuration's export hold it for the
// model that declares it and for no other.
func TestATeamFilesContextWindowReachesTheConfiguration(t *testing.T) {
	file, err := ParseTeamFile(fmt.Appendf(nil, windowTeamFile, "256000"), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := domain.MergeTeamFile(domain.NewConfig(testAdmission()), file, false)
	if err != nil {
		t.Fatal(err)
	}
	small, large := cfg.Models[domain.NewModelRef("team", "small")], cfg.Models[domain.NewModelRef("team", "large")]
	if small.ContextWindow != 256000 || small.Window() != 256000 || large.ContextWindow != 0 || large.Window() != domain.DefaultContextWindow {
		t.Fatalf("the applied models declare %d and %d, with the windows %d and %d", small.ContextWindow, large.ContextWindow, small.Window(), large.Window())
	}
	stored, err := encodeConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	exported, err := ExportTeamFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"aigw.toml": stored, "the exported team file": exported} {
		if text := string(data); strings.Count(text, "context_window") != 1 || !strings.Contains(text, "id = 'small'\nprotocols = { anthropic = [] }\ncontext_window = 256000\n") {
			t.Errorf("%s does not hold the window for small alone:\n%s", name, text)
		}
	}
}

// A context window that is zero or below it is refused when the file is read,
// in aigw.toml and in a team file, by the entry of the model that declares
// it; a value that is no whole number is refused as a value of the wrong type.
func TestAContextWindowThatIsNotAboveZeroIsRefusedByItsModel(t *testing.T) {
	_, valid := schemaEightConfig(t)
	declared := func(value string) []byte { return holdingWindow(t, valid, value) }
	for _, value := range []string{"0", "-1"} {
		refusal := "a context window is a whole number of tokens above zero, not " + value
		_, err := decodeTOMLConfig(declared(value), testAdmission())
		loadErr, isLoad := errors.AsType[*LoadError](err)
		if entry, named := domain.EntryOf(err); !isLoad || loadErr.Phase != LoadPhaseValidate || !named || entry != "model `dmxapi/claude-opus-5-5`" ||
			!strings.Contains(err.Error(), "model dmxapi/claude-opus-5-5: "+refusal) {
			t.Errorf("aigw.toml with context_window = %s: %v, entry %q", value, err, entry)
		}
		_, err = ParseTeamFile(fmt.Appendf(nil, windowTeamFile, value), testAdmission())
		if entry, named := domain.EntryOf(err); !named || entry != "model `team/small`" || !strings.Contains(err.Error(), "model team/small: "+refusal) {
			t.Errorf("a team file with context_window = %s: %v, entry %q", value, err, entry)
		}
	}
	for _, value := range []string{"'many'", "1.5"} {
		_, err := decodeTOMLConfig(declared(value), testAdmission())
		if loadErr, isLoad := errors.AsType[*LoadError](err); !isLoad || loadErr.Phase != LoadPhaseParse {
			t.Errorf("aigw.toml with context_window = %s: %v", value, err)
		}
		if _, err := ParseTeamFile(fmt.Appendf(nil, windowTeamFile, value), testAdmission()); err == nil {
			t.Errorf("a team file with context_window = %s was accepted", value)
		}
	}
}
