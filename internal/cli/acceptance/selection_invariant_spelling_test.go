package cli_test

import (
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v4"
)

// This file holds the matrix's dimension "how a pick is spelled": the ways
// in which a person may write the same model into Codex's TOML and into
// Hermes's YAML, and the readers that read every one of them through the
// format's own parser.

// codexSpelling is one way to write the root model assignment of Codex's
// file: the words that a state's name carries for it, and the line, in which
// MODEL stands for the model.
type codexSpelling struct{ name, line string }

var (
	// basicString is the spelling that AIGW itself writes, without its
	// comment.
	basicString = codexSpelling{"", `model = "MODEL"`}
	// literalString is a TOML literal string, which names the same model.
	literalString = codexSpelling{", as a literal string", `model = 'MODEL'`}
	withoutSpaces = codexSpelling{", without spaces", `model="MODEL"`}
	withAComment  = codexSpelling{", with a comment", `model = "MODEL" # mine`}
)

// codexSpellings is the dimension for Codex.
func codexSpellings() []codexSpelling {
	return []codexSpelling{basicString, literalString, withoutSpaces, withAComment}
}

// codexSelects returns the model that Codex's file at path selects, read as
// Codex reads it: the root key model of the TOML document.
func codexSelects(t *testing.T, path string) string {
	t.Helper()
	var document struct {
		Model string `toml:"model"`
	}
	if err := toml.Unmarshal(readFile(t, path), &document); err != nil {
		t.Fatalf("Codex's file %s is not TOML: %v", path, err)
	}
	return document.Model
}

// picking selects model in the world's Codex file at index, as a person who
// edits the file does, in the given spelling.
func (w selectionWorld) picking(t *testing.T, index int, spelling codexSpelling, model string) selectionWorld {
	t.Helper()
	line := strings.Replace(spelling.line, "MODEL", model, 1)
	recPickAs(t, w.files[index], line)
	if got := w.shows(t, w.files[index]); got != model {
		t.Fatalf("the pick %s reads as the model %q", line, got)
	}
	return w.chose(model)
}

// hermesSpelling is one way to write a pick into Hermes's YAML: the words
// that a state's name carries for it, and the styles of the two scalars that
// a pick sets, the provider entry and the model.
type hermesSpelling struct {
	name            string
	provider, model yaml.Style
}

// plainScalars is the spelling that Hermes and AIGW write.
var plainScalars = hermesSpelling{}

// hermesSpellings is the dimension for Hermes: the model, and separately the
// provider entry, as a plain, a single-quoted and a double-quoted scalar.
func hermesSpellings() []hermesSpelling {
	return []hermesSpelling{
		plainScalars,
		{name: ", the model single-quoted", model: yaml.SingleQuotedStyle},
		{name: ", the model double-quoted", model: yaml.DoubleQuotedStyle},
		{name: ", the provider single-quoted", provider: yaml.SingleQuotedStyle},
		{name: ", the provider double-quoted", provider: yaml.DoubleQuotedStyle},
	}
}

// hermesSelectionScalars parses Hermes's file at path and returns the
// document with the two scalars of its own selection.
func hermesSelectionScalars(t *testing.T, path string) (document, provider, model *yaml.Node) {
	t.Helper()
	document = &yaml.Node{}
	if err := yaml.Unmarshal(readFile(t, path), document); err != nil {
		t.Fatalf("Hermes's file %s is not YAML: %v", path, err)
	}
	value := func(node *yaml.Node, key string) *yaml.Node {
		for index := 0; node != nil && index+1 < len(node.Content); index += 2 {
			if node.Content[index].Value == key {
				return node.Content[index+1]
			}
		}
		t.Fatalf("Hermes's file %s lacks %q", path, key)
		return nil
	}
	selection := value(document.Content[0], "model")
	return document, value(selection, "provider"), value(selection, "default")
}

// choosing selects, in the world's Hermes file, model of the provider entry
// that AIGW keeps for account on protocol, as Hermes's model picker does, and
// writes the two scalars of the selection in the given spelling. The file is
// read again through the YAML parser to prove that it holds that spelling and
// still names the same entry and model.
func (w selectionWorld) choosing(t *testing.T, account, protocol, model string, spelling hermesSpelling) selectionWorld {
	t.Helper()
	path, entry := w.files[0], "aigw-"+account+"-"+protocol
	hermesNativeSelection(t, path, entry, model, nil)
	document, providerNode, modelNode := hermesSelectionScalars(t, path)
	providerNode.Style, modelNode.Style = spelling.provider, spelling.model
	spelled, err := yaml.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, spelled, 0o600)
	_, providerNode, modelNode = hermesSelectionScalars(t, path)
	if providerNode.Style != spelling.provider || modelNode.Style != spelling.model {
		t.Fatalf("Hermes's file holds the styles %v and %v, not the spelling%s", providerNode.Style, modelNode.Style, spelling.name)
	}
	if got := w.shows(t, path); got != entry+"/"+model {
		t.Fatalf("the pick of %s/%s reads as %q", entry, model, got)
	}
	if account != w.account {
		w.chosenAccount = account
	}
	return w.chose(entry + "/" + model)
}
