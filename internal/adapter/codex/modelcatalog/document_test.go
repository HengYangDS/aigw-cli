package modelcatalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type catalogDocument struct {
	Models []map[string]json.RawMessage `json:"models"`
}

// testBundledCatalog is a stand-in for the client's own table. It keeps the
// shape that matters here: several bare slugs and entries carrying fields this
// package does not model.
func testBundledCatalog(slugs ...string) []byte {
	entries := make([]string, 0, len(slugs))
	for index, slug := range slugs {
		entries = append(entries, fmt.Sprintf(
			`{"slug":%q,"display_name":"Display %d","context_window":400000,"unknown_future_field":{"nested":[1,2]}}`,
			slug, index,
		))
	}
	return []byte(`{"models":[` + strings.Join(entries, ",") + `]}`)
}

func catalogSlugList(t *testing.T, data []byte) []string {
	t.Helper()
	var document catalogDocument
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse generated catalog: %v", err)
	}
	slugs := make([]string, 0, len(document.Models))
	for _, entry := range document.Models {
		var slug string
		if err := json.Unmarshal(entry["slug"], &slug); err != nil {
			t.Fatalf("parse generated slug: %v", err)
		}
		slugs = append(slugs, slug)
	}
	return slugs
}

func mustParseCatalog(t *testing.T, data []byte) Document {
	t.Helper()
	document, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return document
}

// TestDocumentProjectionTakesNoMetadataFromAPrefix pins the retired guess: a
// model ID that ends in a slug of the client's table is not that model. A
// prefixed ID gets an alias only through its family, and one that is its own
// family, as a model that `aigw use` adds is, gets none.
func TestDocumentProjectionTakesNoMetadataFromAPrefix(t *testing.T) {
	document := mustParseCatalog(t, testBundledCatalog("gpt-5.6-sol", "gpt-5.5", "codex-auto-review"))
	for _, model := range []string{"openai.gpt-5.6-sol", "us.openai.gpt-5.5", "anthropic.codex-auto-review"} {
		for _, family := range []string{"", model} {
			if projected, base := document.Project(model, family); projected != nil || base != "" {
				t.Fatalf("Project(%q, %q) = %q / %q, want no alias", model, family, projected, base)
			}
		}
	}
	projected, base := document.Project("openai.gpt-5.6-sol", "gpt-5.6-sol")
	if base != "gpt-5.6-sol" || strings.Join(catalogSlugList(t, projected), ",") != "gpt-5.6-sol,gpt-5.5,codex-auto-review,openai.gpt-5.6-sol" {
		t.Fatalf("Project() through the family = %q / %q, want one alias", projected, base)
	}
}

// TestDocumentProjectionKeepsTheCompleteBundledTable is the incremental-snapshot
// guard. The client replaces its own table with this file, so a catalog holding
// only the alias would push every other model onto fallback metadata.
func TestDocumentProjectionKeepsTheCompleteBundledTable(t *testing.T) {
	bundled := testBundledCatalog("gpt-5.6-sol", "gpt-5.5")
	data, _ := mustParseCatalog(t, bundled).Project("gpt-5.6-sol-cdx", "gpt-5.6-sol")
	if data == nil {
		t.Fatal("Document.Project() returned no catalog")
	}
	var source, generated catalogDocument
	if err := json.Unmarshal(bundled, &source); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &generated); err != nil {
		t.Fatal(err)
	}
	if len(generated.Models) != len(source.Models)+1 {
		t.Fatalf("generated %d entries from %d bundled entries", len(generated.Models), len(source.Models))
	}
	for index, entry := range source.Models {
		for key, value := range entry {
			if string(generated.Models[index][key]) != string(value) {
				t.Fatalf("bundled entry %d field %q = %s, want %s", index, key, generated.Models[index][key], value)
			}
		}
	}
}

func TestDocumentProjectionPreservesDocumentMetadata(t *testing.T) {
	bundled := []byte(`{"revision":"client-owned","capabilities":{"schema":2,"features":["tools","images"]},"models":[{"slug":"base","instructions":"retain"}]}`)
	data, _ := mustParseCatalog(t, bundled).Project("provider.base", "base")
	var source, generated map[string]json.RawMessage
	if err := json.Unmarshal(bundled, &source); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &generated); err != nil {
		t.Fatal(err)
	}
	for field, value := range source {
		if field != "models" && !bytes.Equal(generated[field], value) {
			t.Errorf("document field %q = %s, want %s", field, generated[field], value)
		}
	}
}

// TestDocumentProjectionWithholdsWhatItCannotProve is the anti-silencing guard:
// an id the client already knows needs nothing, and an id without a family, or
// whose family is not in the table, keeps the client's own fallback and its
// warning.
func TestDocumentProjectionWithholdsWhatItCannotProve(t *testing.T) {
	bundled := testBundledCatalog("gpt-5.6-sol", "gpt-5.5")
	for _, model := range []string{
		"gpt-5.6-sol",
		"gpt-5.5",
		"openai.no-such-model",
		"no-such-model",
		"openai.",
		".gpt-5.6-sol",
		"",
	} {
		for _, family := range []string{"", model, "no-such-model"} {
			if data, _ := mustParseCatalog(t, bundled).Project(model, family); data != nil {
				t.Fatalf("Document.Project(%q, %q) generated a catalog for an id it cannot prove", model, family)
			}
		}
	}
}

// TestDocumentProjectionUsesDeclaredFamily links a variant by its family: it
// adds only the exact model ID as an alias and copies the family's entry
// without editing the client table.
func TestDocumentProjectionUsesDeclaredFamily(t *testing.T) {
	const (
		family = "gpt-6.1-sol"
		wire   = "gpt-6.1-sol-cdx"
	)
	document := mustParseCatalog(t, testBundledCatalog(family, "gpt-6-luna"))
	projected, base := document.Project(wire, family)
	if projected == nil || base != family {
		t.Fatalf("Project(%q, %q) = %q / %q, want an explicit alias", wire, family, projected, base)
	}
	if got, want := strings.Join(catalogSlugList(t, projected), ","), family+",gpt-6-luna,"+wire; got != want {
		t.Fatalf("projected slugs = %q, want %q", got, want)
	}
	baseEntry := document.Model(family)
	aliasEntry := mustParseCatalog(t, projected).Model(wire)
	if len(aliasEntry) != len(baseEntry) {
		t.Fatalf("alias fields = %v, family fields = %v", aliasEntry, baseEntry)
	}
	for field, value := range baseEntry {
		if field != "slug" && !bytes.Equal(aliasEntry[field], value) {
			t.Errorf("alias metadata %q = %s, want the family's value %s", field, aliasEntry[field], value)
		}
	}
	if document.Model(wire) != nil {
		t.Fatalf("Project mutated the bundled document with %q", wire)
	}
}

// TestDocumentProjectionRejectsUnprovedFamily forbids guessing from a part of
// the ID, even when a similar native model exists: a model that is its own
// family gets no alias.
func TestDocumentProjectionRejectsUnprovedFamily(t *testing.T) {
	const wire = "gpt-6.1-sol-cdx"
	document := mustParseCatalog(t, testBundledCatalog("gpt-6.1-sol", "gpt-6-luna"))
	for _, family := range []string{"", "gpt-6.1", "missing-model", wire} {
		projected, base := document.Project(wire, family)
		if projected != nil || base != "" {
			t.Fatalf("Project(%q, %q) = %q / %q, want no projection", wire, family, projected, base)
		}
	}
	nativeWire := mustParseCatalog(t, testBundledCatalog(wire, "gpt-6.1-sol"))
	if projected, _ := nativeWire.Project(wire, "gpt-6.1-sol"); projected != nil {
		t.Fatalf("Project duplicated already-native wire model %q", wire)
	}
}

func TestDocumentProjectionIsDeterministicAndIdempotent(t *testing.T) {
	bundled := testBundledCatalog("gpt-5.5", "gpt-5.6-sol", "codex-auto-review")
	first, _ := mustParseCatalog(t, bundled).Project("openai.gpt-5.6-sol", "gpt-5.6-sol")
	second, _ := mustParseCatalog(t, bundled).Project("openai.gpt-5.6-sol", "gpt-5.6-sol")
	if string(first) != string(second) {
		t.Fatal("Document.Project() is not byte-deterministic")
	}
	// Re-running against a table that already carries the alias must add
	// nothing, so a converged target keeps producing the same bytes.
	again, _ := mustParseCatalog(t, first).Project("openai.gpt-5.6-sol", "gpt-5.6-sol")
	if again != nil {
		t.Fatalf("Document.Project() duplicated existing aliases:\n%s", again)
	}
}

func TestParseRejectsUnusableBundledTables(t *testing.T) {
	cases := []struct {
		name    string
		bundled string
		message string
	}{
		{name: "not json", bundled: "{", message: "parse Codex model catalog"},
		{name: "wrong entries shape", bundled: `{"models":{}}`, message: "entries"},
		{name: "empty", bundled: `{"models":[]}`, message: "empty"},
		{name: "no slug", bundled: `{"models":[{"display_name":"x"}]}`, message: "has no slug"},
		{name: "empty slug", bundled: `{"models":[{"slug":""}]}`, message: "empty slug"},
		{name: "slug not a string", bundled: `{"models":[{"slug":7}]}`, message: "slug"},
		{name: "duplicate slug", bundled: `{"models":[{"slug":"a"},{"slug":"a"}]}`, message: "twice"},
	}
	for _, c := range cases {
		_, err := Parse([]byte(c.bundled))
		if err == nil || !strings.Contains(err.Error(), c.message) {
			t.Fatalf("%s: Parse() error = %v, want %q", c.name, err, c.message)
		}
	}
}

func TestDocumentModelObservationCannotChangeProjection(t *testing.T) {
	document := mustParseCatalog(t, []byte(`{"models":[{"slug":"base","instructions":"original"}]}`))
	before, base := document.Project("provider.base", "base")
	if base != "base" {
		t.Fatalf("Project() base = %q", base)
	}
	model := document.Model("base")
	model["instructions"][1] = 'X'
	delete(model, "slug")
	after, _ := document.Project("provider.base", "base")
	if !bytes.Equal(before, after) {
		t.Fatalf("model observation changed projection: %s", after)
	}
}

func TestDocumentModelReportsAbsence(t *testing.T) {
	document := mustParseCatalog(t, testBundledCatalog("base"))
	if model := document.Model("unknown"); model != nil {
		t.Fatalf("unknown model = %v", model)
	}
}
