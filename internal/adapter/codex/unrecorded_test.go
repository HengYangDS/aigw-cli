package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/terminal"

	"github.com/pelletier/go-toml/v2"
)

// hasProviderTable reports whether text, as TOML, holds a provider named aigw.
func hasProviderTable(text string) bool {
	var document struct {
		Providers map[string]any `toml:"model_providers"`
	}
	if toml.Unmarshal([]byte(text), &document) != nil {
		return false
	}
	_, present := document.Providers["aigw"]
	return present
}

// parses reports whether text is TOML that the parser accepts.
func parses(text string) error {
	var document map[string]any
	return toml.Unmarshal([]byte(text), &document)
}

// A projected file whose record is deleted keeps AIGW's provider table. The
// projection refuses before any write, naming the record to restore and the
// table to take out, and leaves the file as it was, so it still parses.
func TestAProjectionRefusesAnUnrecordedProviderTable(t *testing.T) {
	path := projectedCodexTarget(t)
	if err := os.Remove(codexStatePath(path)); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ReconcileConfigs(nil, []TargetRef{HomeTarget(path)}, atomicTestRuntime(), atomicTestRuntime(), false)
	problem := terminal.ProblemFor(err)
	if !strings.Contains(problem.Title, "[model_providers.aigw]") || !strings.Contains(problem.Title, "config.toml.aigw-state.json") ||
		problem.Fix != "Restore config.toml.aigw-state.json in its folder from a copy of it, or take the [model_providers.aigw] table and the lines marked \"# managed by AIGW\" out of config.toml, then run `aigw sync`" {
		t.Fatalf("the refusal = %+v", problem)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("the refusal changed the file: %v", err)
	}
	if err := parses(string(after)); err != nil {
		t.Fatalf("the file no longer parses: %v", err)
	}
	if _, err := os.Stat(codexStatePath(path)); !os.IsNotExist(err) {
		t.Fatalf("the refusal wrote a record: %v", err)
	}
}

// Rendering a projection over an existing configuration either refuses or
// produces TOML that parses, whatever the file already holds: AIGW's own table
// with or without its record, a foreign provider of the same name, comments
// with the markers, and dotted or inline spellings.
func TestEveryCodexProjectionParsesOrRefuses(t *testing.T) {
	marked := "model_provider = \"aigw\" # managed by AIGW\nmodel = \"gpt-5.6-terra\" # managed by AIGW\n\n" + codexBegin + "\n[model_providers.aigw]\nname = \"AIGW\"\nbase_url = \"http://127.0.0.1:48721/v1\"\nwire_api = \"responses\"\n" + codexEnd + "\n"
	recorded := projectedCodexTarget(t)
	complete, err := os.ReadFile(recorded)
	if err != nil {
		t.Fatal(err)
	}
	for name, original := range map[string]string{
		"AIGW's own projection without its record": string(complete),
		"an empty file":                         "",
		"settings and comments":                 "# my settings\napproval_policy = \"never\" # keep\n\n[profiles.fast]\nmodel = \"other\"\n",
		"AIGW's table without its record":       marked,
		"a foreign provider of the same name":   "[model_providers.aigw]\nname = \"Mine\"\nbase_url = \"https://mine.test/v1\"\n",
		"a dotted provider of the same name":    "model_providers.aigw.name = \"Mine\"\n",
		"an inline provider of the same name":   "[model_providers]\naigw = { name = \"Mine\" }\n",
		"a subtable of the same provider":       "[model_providers.aigw.auth]\ncommand = \"mine\"\n",
		"the markers in a comment only":         "# " + codexBegin + "\n# [model_providers.aigw]\n# " + codexEnd + "\napproval_policy = \"never\"\n",
		"another provider and a managed marker": "[model_providers.other]\nname = \"Other\" # managed by AIGW\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "home", "config.toml")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			writeCodexFixture(t, path, original)
			_, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(path)}, atomicTestRuntime(), atomicTestRuntime(), false)
			written, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if err != nil {
				if string(written) != original {
					t.Fatalf("a refusal changed the file: %v", err)
				}
				return
			}
			if parseErr := parses(string(written)); parseErr != nil {
				t.Fatalf("the projection does not parse: %v\n%s", parseErr, written)
			}
			// No record proves that a table of AIGW's name is AIGW's, so a
			// projection that wrote over one adopted it.
			if hasProviderTable(original) {
				t.Fatalf("a table of AIGW's name without its record was adopted:\n%s", written)
			}
		})
	}
}

// The parse check refuses rendered settings that the parser rejects.
func TestAnUnparsableRenderingIsRefused(t *testing.T) {
	if err := requireParsableCodexConfig("[a]\nx = 1\n[a]\nx = 2\n"); err == nil || !strings.Contains(err.Error(), "does not parse") {
		t.Fatalf("a duplicated table = %v", err)
	}
	if err := requireParsableCodexConfig("[a]\nx = 1\n"); err != nil {
		t.Fatalf("valid settings = %v", err)
	}
	if _, refused := adapter.ConflictStatus(refuseUnrecordedProvider(platform.FileSnapshot{Exists: true, Data: []byte("x = 1\n")}, platform.FileSnapshot{}, "aigw", "config.toml", "config.toml.aigw-state.json")); refused {
		t.Fatal("a file without AIGW's table was refused")
	}
}

// A file that already declares AIGW's provider table twice, as 0.6.1 left it,
// is refused with the one fix that lets Codex read it again, and is not
// changed.
func TestAFileWithARepeatedProviderTableIsRefused(t *testing.T) {
	path := projectedCodexTarget(t)
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(projected), "[model_providers.aigw]")
	end := strings.Index(string(projected), codexEnd)
	repeated := string(projected) + "\n" + string(projected)[start:end]
	writeCodexFixture(t, path, repeated)
	_, err = ReconcileConfigs(nil, []TargetRef{HomeTarget(path)}, atomicTestRuntime(), atomicTestRuntime(), false)
	problem := terminal.ProblemFor(err)
	if !strings.Contains(problem.Title, "declares [model_providers.aigw] 2 times") ||
		problem.Fix != "Keep one [model_providers.aigw] table in config.toml, the one between AIGW's markers, and remove the others, then run `aigw sync`" {
		t.Fatalf("the refusal = %+v", problem)
	}
	if after, _ := os.ReadFile(path); string(after) != repeated {
		t.Fatal("the refusal changed the file")
	}
	if err := ValidateConfig(path, atomicTestRuntime()); err == nil || !strings.Contains(err.Error(), "2 times") {
		t.Fatalf("inspection of the file = %v", err)
	}
}
