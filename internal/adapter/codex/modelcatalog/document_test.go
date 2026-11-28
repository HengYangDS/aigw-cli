package modelcatalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

type catalogDocument struct {
	Models []map[string]json.RawMessage `json:"models"`
}

// testWindow is a declared context window that differs from the stand-in
// table's own.
const testWindow = 640_000

// testBundledCatalog is a stand-in for the client's own table. It keeps the
// shape that matters here: several bare slugs and entries carrying fields this
// package does not model.
func testBundledCatalog(slugs ...string) []byte {
	entries := make([]string, 0, len(slugs))
	for index, slug := range slugs {
		entries = append(entries, fmt.Sprintf(
			`{"slug":%q,"display_name":"Display %d","context_window":400000,"max_context_window":872000,"unknown_future_field":{"nested":[1,2]}}`,
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
			if projected, base := project(document, model, family, "", testWindow); projected != nil || base != "" {
				t.Fatalf("Project(%q, %q) = %q / %q, want no alias", model, family, projected, base)
			}
		}
	}
	projected, base := project(document, "openai.gpt-5.6-sol", "gpt-5.6-sol", "", testWindow)
	if base != "gpt-5.6-sol" || strings.Join(catalogSlugList(t, projected), ",") != "gpt-5.6-sol,gpt-5.5,codex-auto-review,openai.gpt-5.6-sol" {
		t.Fatalf("Project() through the family = %q / %q, want one alias", projected, base)
	}
}

// TestDocumentProjectionKeepsTheCompleteBundledTable is the incremental-snapshot
// guard. The client replaces its own table with this file, so a catalog holding
// only the alias would push every other model onto fallback metadata.
func TestDocumentProjectionKeepsTheCompleteBundledTable(t *testing.T) {
	bundled := testBundledCatalog("gpt-5.6-sol", "gpt-5.5")
	data, _ := project(mustParseCatalog(t, bundled), "gpt-5.6-sol-cdx", "gpt-5.6-sol", "", testWindow)
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
	data, _ := project(mustParseCatalog(t, bundled), "provider.base", "base", "", testWindow)
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
// an id that the table does not hold, without a family or with one that the
// table does not hold either, keeps the client's own fallback and its warning.
func TestDocumentProjectionWithholdsWhatItCannotProve(t *testing.T) {
	bundled := testBundledCatalog("gpt-5.6-sol", "gpt-5.5")
	for _, model := range []string{
		"openai.no-such-model",
		"no-such-model",
		"openai.",
		".gpt-5.6-sol",
		"",
	} {
		for _, family := range []string{"", model, "no-such-model"} {
			if data, _ := project(mustParseCatalog(t, bundled), model, family, "", testWindow); data != nil {
				t.Fatalf("Document.Project(%q, %q) generated a catalog for an id it cannot prove", model, family)
			}
		}
	}
}

// TestDocumentProjectionUsesDeclaredFamily links a variant by its family: it
// adds only the exact model ID as an alias and copies the family's entry but
// its display name, which TestDocumentProjectionNamesTheAliasApart pins, and
// its window, which TestProjectionGivesAnAliasTheDeclaredWindow pins, without
// editing the client table.
func TestDocumentProjectionUsesDeclaredFamily(t *testing.T) {
	const (
		family = "gpt-6.1-sol"
		wire   = "gpt-6.1-sol-cdx"
	)
	document := mustParseCatalog(t, testBundledCatalog(family, "gpt-6-luna"))
	projected, base := project(document, wire, family, "", testWindow)
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
		if field != "slug" && field != "display_name" && !slices.Contains(windowFields, field) && !bytes.Equal(aliasEntry[field], value) {
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
		projected, base := project(document, wire, family, "", testWindow)
		if projected != nil || base != "" {
			t.Fatalf("Project(%q, %q) = %q / %q, want no projection", wire, family, projected, base)
		}
	}
	nativeWire := mustParseCatalog(t, testBundledCatalog(wire, "gpt-6.1-sol"))
	projected, base := project(nativeWire, wire, "gpt-6.1-sol", "", testWindow)
	if got := strings.Join(catalogSlugList(t, projected), ","); base != wire || got != wire+",gpt-6.1-sol" {
		t.Fatalf("Project duplicated already-native wire model %q: %q / %q", wire, got, base)
	}
}

func TestDocumentProjectionIsDeterministicAndIdempotent(t *testing.T) {
	bundled := testBundledCatalog("gpt-5.5", "gpt-5.6-sol", "codex-auto-review")
	first, _ := project(mustParseCatalog(t, bundled), "openai.gpt-5.6-sol", "gpt-5.6-sol", "", testWindow)
	second, _ := project(mustParseCatalog(t, bundled), "openai.gpt-5.6-sol", "gpt-5.6-sol", "", testWindow)
	if string(first) != string(second) {
		t.Fatal("Document.Project() is not byte-deterministic")
	}
	// Re-running against a table that already carries the alias must add
	// nothing, so a converged target keeps producing the same bytes.
	again, _ := project(mustParseCatalog(t, first), "openai.gpt-5.6-sol", "gpt-5.6-sol", "", testWindow)
	if !bytes.Equal(again, first) {
		t.Fatalf("Document.Project() changed a table that already carries the alias:\n%s", again)
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
	before, base := project(document, "provider.base", "base", "", testWindow)
	if base != "base" {
		t.Fatalf("Project() base = %q", base)
	}
	model := document.Model("base")
	model["instructions"][1] = 'X'
	delete(model, "slug")
	after, _ := project(document, "provider.base", "base", "", testWindow)
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

// TestDocumentProjectionNamesTheAliasApart shows the alias by the name it is
// given, keeps the family's name when it is given none, and never lists two
// entries under one display name: Codex's picker would show both alike.
func TestDocumentProjectionNamesTheAliasApart(t *testing.T) {
	document := mustParseCatalog(t, []byte(`{"models":[{"slug":"gpt-6.2-sol","display_name":"GPT-6.2-Sol"},{"slug":"gpt-6.2-astra","display_name":"GPT-6.2-Astra"}]}`))
	displayName := func(data []byte, slug string) string {
		t.Helper()
		var parsed catalogDocument
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatal(err)
		}
		for _, entry := range parsed.Models {
			var entrySlug, name string
			_ = json.Unmarshal(entry["slug"], &entrySlug)
			if entrySlug == slug {
				_ = json.Unmarshal(entry["display_name"], &name)
				return name
			}
		}
		t.Fatalf("no entry %q", slug)
		return ""
	}
	for _, testCase := range []struct{ name, want string }{
		{"GPT-6.2 Sol · cdx", "GPT-6.2 Sol · cdx"},
		{"", "GPT-6.2-Sol (gpt-6.2-sol-cdx)"},
		{"gpt-6.2-astra", "gpt-6.2-astra (gpt-6.2-sol-cdx)"},
	} {
		data, base := project(document, "gpt-6.2-sol-cdx", "gpt-6.2-sol", testCase.name, testWindow)
		if base != "gpt-6.2-sol" || displayName(data, "gpt-6.2-sol-cdx") != testCase.want {
			t.Fatalf("Project(name %q) shows %q, want %q", testCase.name, displayName(data, "gpt-6.2-sol-cdx"), testCase.want)
		}
		if displayName(data, "gpt-6.2-sol") != "GPT-6.2-Sol" {
			t.Fatal("the family's own entry changed its name")
		}
	}
	// A table that already shows the name with the model ID in it leaves no
	// name to give, so nothing is projected.
	taken := mustParseCatalog(t, []byte(`{"models":[{"slug":"gpt-6.2-sol","display_name":"GPT-6.2-Sol"},{"slug":"other","display_name":"GPT-6.2-Sol (gpt-6.2-sol-cdx)"}]}`))
	if data, base := project(taken, "gpt-6.2-sol-cdx", "gpt-6.2-sol", "", testWindow); data != nil || base != "" {
		t.Fatalf("a name the table already shows twice = %q / %q", data, base)
	}
}
