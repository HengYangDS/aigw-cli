package modelcatalog

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"
)

// project lists model alone, as the selection of a provider with one family.
// It returns nothing when the table gives the model no entry, and otherwise
// the table and the slug of the entry that describes the model.
func project(document Document, model, family, name string, window int) ([]byte, string) {
	data := document.Project([]Listing{{Model: model, Family: family, Name: name, Window: window}})
	projected, err := Parse(data)
	if err != nil || projected.Model(model) == nil {
		return nil, ""
	}
	return data, document.DescribedBy(model, family)
}

// codexTable is a stand-in for Codex's table with its visibility field: the
// listed and hidden entries of the scenario of the client-adapters delta.
func codexTable(t *testing.T) Document {
	t.Helper()
	return mustParseCatalog(t, []byte(`{"models":[`+
		`{"slug":"gpt-6.2-sol","display_name":"GPT-6.2-Sol","visibility":"list","context_window":272000},`+
		`{"slug":"gpt-6.1-terra","display_name":"GPT-6.1 Terra","visibility":"list","context_window":272000},`+
		`{"slug":"gpt-5.5","display_name":"GPT-5.5","visibility":"list"},`+
		`{"slug":"codex-auto-review","display_name":"Codex Auto Review","visibility":"hide"},`+
		`{"slug":"gpt-daybreak-blue-latest","display_name":"Daybreak Blue","visibility":"none"}]}`))
}

func visibilities(t *testing.T, data []byte) map[string]string {
	t.Helper()
	var parsed catalogDocument
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	shown := map[string]string{}
	for _, entry := range parsed.Models {
		var slug, value string
		_ = json.Unmarshal(entry["slug"], &slug)
		_ = json.Unmarshal(entry["visibility"], &value)
		shown[slug] = value
	}
	return shown
}

// The table lists the selection's family models that it describes, in their
// order, hides every other entry that the client lists, keeps every entry
// that the client hides as it is, and holds no entry for a model that it
// describes by neither its own entry nor its family's.
func TestProjectionListsTheConfiguredFamilies(t *testing.T) {
	document := codexTable(t)
	data := document.Project([]Listing{
		{Model: "gpt-6.2-sol-cdx", Family: "gpt-6.2-sol", Name: "GPT-6.2 Sol · CDX", Window: 400_000},
		{Model: "gpt-6.1-terra", Family: "gpt-6.1-terra", Name: "GPT-6.1 Terra", Window: 1_000_000},
		{Model: "deepseek-v4.1", Family: "deepseek-v4.1", Name: "deepseek-v4.1", Window: 1_000_000},
	})
	projected := mustParseCatalog(t, data)
	if listed := projected.Listed(); !slices.Equal(listed, []string{"gpt-6.1-terra", "gpt-6.2-sol-cdx"}) {
		t.Fatalf("listed entries = %q, want the terra entry and the alias", listed)
	}
	want := map[string]string{
		"gpt-6.2-sol": "hide", "gpt-6.1-terra": "list", "gpt-5.5": "hide", "codex-auto-review": "hide",
		"gpt-daybreak-blue-latest": "none", "gpt-6.2-sol-cdx": "list",
	}
	if got := visibilities(t, data); !maps.Equal(got, want) {
		t.Fatalf("visibilities = %v, want %v", got, want)
	}
	if projected.Model("deepseek-v4.1") != nil {
		t.Fatal("a model that the table does not describe got an entry")
	}
	for slug, window := range map[string]int{"gpt-6.2-sol-cdx": 400_000, "gpt-6.1-terra": 1_000_000, "gpt-6.2-sol": 272_000} {
		if got, ceiling := projected.ContextWindow(slug); got != window || slug != "gpt-6.2-sol" && ceiling != window {
			t.Errorf("%s carries %d and %d, want %d", slug, got, ceiling, window)
		}
	}
	// A hidden entry keeps every field but its visibility.
	hidden := projected.Model("gpt-5.5")
	delete(hidden, "visibility")
	original := document.Model("gpt-5.5")
	delete(original, "visibility")
	if !slices.Equal(sortedKeys(hidden), sortedKeys(original)) || string(hidden["display_name"]) != string(original["display_name"]) {
		t.Fatalf("hiding changed another field: %v, want %v", hidden, original)
	}
}

// AIGW never shows what Codex hides: a listed model whose own entry, or whose
// family's, the client hides keeps that entry hidden.
func TestProjectionKeepsWhatTheClientHidesHidden(t *testing.T) {
	document := codexTable(t)
	data := document.Project([]Listing{
		{Model: "codex-auto-review", Family: "codex-auto-review", Window: 400_000},
		{Model: "daybreak-channel", Family: "gpt-daybreak-blue-latest", Name: "Daybreak Channel", Window: 400_000},
	})
	got := visibilities(t, data)
	if got["codex-auto-review"] != "hide" || got["daybreak-channel"] != "none" {
		t.Fatalf("visibilities = %v, want the client's hidden entries and their alias hidden", got)
	}
	if listed := mustParseCatalog(t, data).Listed(); len(listed) != 0 {
		t.Fatalf("listed entries = %q, want none", listed)
	}
}

// A table that describes none of the listed models lists no entry and keeps
// every entry of the client's, hidden; no models gives no table.
func TestProjectionOfUndescribedModelsListsNothing(t *testing.T) {
	document := codexTable(t)
	data := document.Project([]Listing{{Model: "deepseek-v4.1", Family: "deepseek-v4.1", Window: 1_000_000}})
	projected := mustParseCatalog(t, data)
	if listed := projected.Listed(); len(listed) != 0 {
		t.Fatalf("listed entries = %q, want none", listed)
	}
	if got := len(visibilities(t, data)); got != 5 {
		t.Fatalf("the table holds %d entries, want the client's five", got)
	}
	if document.Project(nil) != nil {
		t.Fatal("no models gave a table")
	}
}

// Two aliases take names apart from each other as from the client's entries.
func TestProjectionNamesEveryAliasApart(t *testing.T) {
	document := codexTable(t)
	data := document.Project([]Listing{
		{Model: "gpt-6.2-sol-cdx", Family: "gpt-6.2-sol", Name: "GPT-6.2 Sol", Window: 400_000},
		{Model: "gpt-6.2-sol-ssvip", Family: "gpt-6.2-sol", Name: "GPT-6.2 Sol", Window: 400_000},
	})
	projected := mustParseCatalog(t, data)
	names := map[string]string{}
	for _, slug := range []string{"gpt-6.2-sol-cdx", "gpt-6.2-sol-ssvip"} {
		var name string
		_ = json.Unmarshal(projected.Model(slug)["display_name"], &name)
		names[slug] = name
	}
	if names["gpt-6.2-sol-cdx"] != "GPT-6.2 Sol" || names["gpt-6.2-sol-ssvip"] != "GPT-6.2 Sol (gpt-6.2-sol-ssvip)" {
		t.Fatalf("alias names = %v", names)
	}
	if shared := projected.SharingDisplayName("gpt-6.2-sol-ssvip"); len(shared) != 0 {
		t.Fatalf("the second alias shares its name with %q", shared)
	}
}

func sortedKeys(entry map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(entry))
	for key := range entry {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
