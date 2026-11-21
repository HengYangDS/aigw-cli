package store

import (
	"errors"
	"fmt"
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
// export as a team file holds the key.
func TestAModelWithoutAContextWindowEncodesNoSuchKey(t *testing.T) {
	cfg, want := schemaEightConfig(t)
	data, err := encodeConfig(cfg)
	if err != nil || string(data) != want || strings.Contains(string(data), "context_window") {
		t.Fatalf("aigw.toml =\n%s\n%v\nwant\n%s", data, err, want)
	}
	exported, err := ExportTeamFile(cfg)
	if err != nil || strings.Contains(string(exported), "context_window") {
		t.Fatalf("the exported team file holds a context window that no model declares:\n%s\n%v", exported, err)
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
	const protocols = "protocols = { anthropic = ['text', 'tool_search'] }\n"
	declared := func(value string) []byte {
		text := strings.Replace(valid, protocols, protocols+"context_window = "+value+"\n", 1)
		if text == valid {
			t.Fatalf("the fixture has no %q", protocols)
		}
		return []byte(text)
	}
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
