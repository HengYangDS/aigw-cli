package catalog

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

// TestDocumentProjectionAdaptsEveryProviderPrefix pins the general adaptation: a
// prefix is learned from the selected id and then applied to the whole table, so
// no model name is hard-coded and no model the account can select is left behind.
func TestDocumentProjectionAdaptsEveryProviderPrefix(t *testing.T) {
	bundled := testBundledCatalog("gpt-5.6-sol", "gpt-5.5", "codex-auto-review")
	cases := []struct {
		model     string
		namespace string
	}{
		{model: "openai.gpt-5.6-sol", namespace: "openai"},
		{model: "openai.gpt-5.5", namespace: "openai"},
		{model: "anthropic.codex-auto-review", namespace: "anthropic"},
		{model: "us.openai.gpt-5.5", namespace: "us.openai"},
		{model: "eu.anthropic.gpt-5.6-sol", namespace: "eu.anthropic"},
	}
	for _, c := range cases {
		data, _ := mustParseCatalog(t, bundled).Project(c.model)
		if data == nil {
			t.Fatalf("Document.Project(%q) generated no catalog", c.model)
		}
		got := catalogSlugList(t, data)
		want := []string{
			"gpt-5.6-sol", "gpt-5.5", "codex-auto-review",
			c.namespace + ".codex-auto-review", c.namespace + ".gpt-5.5", c.namespace + ".gpt-5.6-sol",
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("Document.Project(%q) slugs = %v, want %v", c.model, got, want)
		}
		if !strings.Contains(string(data), c.model) {
			t.Fatalf("Document.Project(%q) does not resolve the selected id", c.model)
		}
	}
}

// TestDocumentProjectionKeepsTheCompleteBundledTable is the incremental-snapshot
// guard. The client replaces its own table with this file, so a catalog holding
// only the alias would push every other model onto fallback metadata.
func TestDocumentProjectionKeepsTheCompleteBundledTable(t *testing.T) {
	bundled := testBundledCatalog("gpt-5.6-sol", "gpt-5.5")
	data, _ := mustParseCatalog(t, bundled).Project("openai.gpt-5.6-sol")
	if data == nil {
		t.Fatal("Document.Project() returned no catalogue")
	}
	var source, generated catalogDocument
	if err := json.Unmarshal(bundled, &source); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &generated); err != nil {
		t.Fatal(err)
	}
	if len(generated.Models) != len(source.Models)*2 {
		t.Fatalf("generated %d entries from %d bundled entries", len(generated.Models), len(source.Models))
	}
	aliases := make(map[string]map[string]json.RawMessage, len(source.Models))
	for _, entry := range generated.Models[len(source.Models):] {
		var slug string
		if err := json.Unmarshal(entry["slug"], &slug); err != nil {
			t.Fatal(err)
		}
		aliases[slug] = entry
	}
	for _, entry := range source.Models {
		var slug string
		if err := json.Unmarshal(entry["slug"], &slug); err != nil {
			t.Fatal(err)
		}
		alias, present := aliases["openai."+slug]
		if !present {
			t.Fatalf("no alias generated for %q", slug)
		}
		if len(alias) != len(entry) {
			t.Fatalf("alias %q has %d fields, source has %d", slug, len(alias), len(entry))
		}
		for key, value := range entry {
			if key == "slug" {
				continue
			}
			if string(alias[key]) != string(value) {
				t.Fatalf("alias %q field %q = %s, want %s", slug, key, alias[key], value)
			}
		}
	}
}

func TestDocumentProjectionPreservesDocumentMetadata(t *testing.T) {
	bundled := []byte(`{"revision":"client-owned","capabilities":{"schema":2,"features":["tools","images"]},"models":[{"slug":"base","instructions":"retain"}]}`)
	data, _ := mustParseCatalog(t, bundled).Project("provider.base")
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
// an id the client already knows needs nothing, and an id whose base slug does
// not exist keeps the client's own fallback and its warning.
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
		data, _ := mustParseCatalog(t, bundled).Project(model)
		if data != nil {
			t.Fatalf("Document.Project(%q) generated a catalog for an id it cannot prove", model)
		}
	}
}

// TestDocumentNamespaceRequiresAUniqueMatch pins the prefix split against a
// first-dot shortcut: every dot-separated suffix is matched exactly, and an
// ambiguous id is refused rather than mapped to a guess.
func TestDocumentNamespaceRequiresAUniqueMatch(t *testing.T) {
	// A slug that itself contains dots can make two different splits of the same
	// id look valid. Both are refused, because either one may be the wrong model.
	ambiguous := mustParseCatalog(t, testBundledCatalog("b.c", "c"))
	if namespace, ok := ambiguous.namespace("a.b.c"); ok {
		t.Fatalf("ambiguous id produced namespace %q", namespace)
	}
	slugs := mustParseCatalog(t, testBundledCatalog("gpt-5.6-sol", "plain"))
	namespace, ok := slugs.namespace("openai.plain")
	if !ok || namespace != "openai" {
		t.Fatalf("codexCatalogNamespace() = %q, %v", namespace, ok)
	}
	// A dotted slug is matched whole rather than cut at the first dot.
	namespace, ok = slugs.namespace("us.openai.gpt-5.6-sol")
	if !ok || namespace != "us.openai" {
		t.Fatalf("multi-level namespace = %q, %v", namespace, ok)
	}
	if _, ok = slugs.namespace("openai.gpt-5"); ok {
		t.Fatal("a partial slug produced a namespace")
	}
}

func TestDocumentProjectionIsDeterministicAndIdempotent(t *testing.T) {
	bundled := testBundledCatalog("gpt-5.5", "gpt-5.6-sol", "codex-auto-review")
	first, _ := mustParseCatalog(t, bundled).Project("openai.gpt-5.6-sol")
	second, _ := mustParseCatalog(t, bundled).Project("openai.gpt-5.6-sol")
	if string(first) != string(second) {
		t.Fatal("Document.Project() is not byte-deterministic")
	}
	// Re-running against a table that already carries the aliases must add
	// nothing, so a converged target keeps producing the same bytes.
	again, _ := mustParseCatalog(t, first).Project("openai.gpt-5.6-sol")
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
	before, base := document.Project("provider.base")
	if base != "base" {
		t.Fatalf("Project() base = %q", base)
	}
	model := document.Model("base")
	model["instructions"][1] = 'X'
	delete(model, "slug")
	after, _ := document.Project("provider.base")
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
