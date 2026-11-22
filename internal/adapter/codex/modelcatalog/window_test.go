package modelcatalog

import (
	"bytes"
	"encoding/json"
	"slices"
	"strconv"
	"testing"
)

// windowFields are the two fields of an entry that carry the context window.
var windowFields = []string{"context_window", "max_context_window"}

// canonicalEntries returns the entries of a table in order, each with its
// fields sorted, so that two tables compare entry by entry.
func canonicalEntries(t *testing.T, data []byte) []string {
	t.Helper()
	var document catalogDocument
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse catalog: %v", err)
	}
	entries := make([]string, 0, len(document.Models))
	for _, entry := range document.Models {
		canonical, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, string(canonical))
	}
	return entries
}

// requireOnlyTheWindowDiffers fails unless got carries window in both window
// fields and equals want in every other field, apart from the fields in own.
func requireOnlyTheWindowDiffers(t *testing.T, got, want map[string]json.RawMessage, window int, own ...string) {
	t.Helper()
	for _, field := range windowFields {
		if string(got[field]) != strconv.Itoa(window) {
			t.Errorf("%s = %s, want the declared window %d", field, got[field], window)
		}
	}
	for field, value := range want {
		if !slices.Contains(windowFields, field) && !slices.Contains(own, field) && !bytes.Equal(got[field], value) {
			t.Errorf("field %q = %s, want the client's value %s", field, got[field], value)
		}
	}
	for field := range got {
		if _, present := want[field]; !present && !slices.Contains(windowFields, field) {
			t.Errorf("field %q is not in the client's entry", field)
		}
	}
}

// TestProjectionGivesAnAliasTheDeclaredWindow holds the alias to the family's
// entry in every field but its ID, its name and the two window fields, which
// carry the declared window, above or below the family's own, and holds the
// family's entry and every other one to the client's table.
func TestProjectionGivesAnAliasTheDeclaredWindow(t *testing.T) {
	const (
		family = "gpt-6.1-sol"
		model  = "gpt-6.1-sol-cdx"
	)
	bundled := testBundledCatalog(family, "gpt-6-luna")
	document := mustParseCatalog(t, bundled)
	for _, window := range []int{750_000, 300_000} {
		projected, base := document.Project(model, family, "", window)
		if projected == nil || base != family {
			t.Fatalf("Project(window %d) = %q / %q, want an alias of %q", window, projected, base, family)
		}
		requireOnlyTheWindowDiffers(t, mustParseCatalog(t, projected).Model(model), document.Model(family), window, "slug", "display_name")
		entries := canonicalEntries(t, projected)
		if want := canonicalEntries(t, bundled); !slices.Equal(entries[:len(entries)-1], want) {
			t.Fatalf("the client's own entries = %q, want them unchanged and in order: %q", entries[:len(entries)-1], want)
		}
	}
}

// TestProjectionGivesAModelThatIsAnEntryItsDeclaredWindow sets the two window
// fields of the model's own entry and changes nothing else of the table,
// whatever family and name the model has in AIGW.
func TestProjectionGivesAModelThatIsAnEntryItsDeclaredWindow(t *testing.T) {
	const model = "gpt-6.1-sol"
	bundled := testBundledCatalog("gpt-5.5", model, "gpt-6-luna")
	document := mustParseCatalog(t, bundled)
	want := canonicalEntries(t, bundled)
	for _, window := range []int{750_000, 300_000} {
		for _, family := range []string{model, "", "no-such-model", "gpt-5.5"} {
			projected, base := document.Project(model, family, "A label of AIGW's", window)
			if projected == nil || base != model {
				t.Fatalf("Project(family %q, window %d) = %q / %q, want the model's own entry", family, window, projected, base)
			}
			requireOnlyTheWindowDiffers(t, mustParseCatalog(t, projected).Model(model), document.Model(model), window)
			entries := canonicalEntries(t, projected)
			if len(entries) != len(want) || entries[0] != want[0] || entries[2] != want[2] {
				t.Fatalf("the other entries = %q, want them unchanged and in order: %q", entries, want)
			}
		}
	}
}

// TestProjectionWritesTheWindowIntoAnEntryThatHadNone decides the case of an
// entry without the two window fields: it gets them, as an alias and as the
// model's own entry, because the requirement does not depend on what the
// client's table said before.
func TestProjectionWritesTheWindowIntoAnEntryThatHadNone(t *testing.T) {
	document := mustParseCatalog(t, []byte(`{"models":[{"slug":"base","display_name":"Base","instructions":"retain"}]}`))
	for model, own := range map[string][]string{"provider.base": {"slug", "display_name"}, "base": nil} {
		projected, _ := document.Project(model, "base", "", 640_000)
		if projected == nil {
			t.Fatalf("Project(%q) returned no table for an entry without window fields", model)
		}
		requireOnlyTheWindowDiffers(t, mustParseCatalog(t, projected).Model(model), document.Model("base"), 640_000, own...)
	}
}

// TestProjectionRefusesAWindowThatIsNotAboveZero pins the caller's error: no
// resolved selection has such a window, and it is never written into a table.
func TestProjectionRefusesAWindowThatIsNotAboveZero(t *testing.T) {
	document := mustParseCatalog(t, testBundledCatalog("gpt-6.1-sol"))
	for _, window := range []int{0, -1} {
		for _, model := range []string{"gpt-6.1-sol-cdx", "gpt-6.1-sol"} {
			if projected, base := document.Project(model, "gpt-6.1-sol", "", window); projected != nil || base != "" {
				t.Fatalf("Project(%q, window %d) = %q / %q, want nothing", model, window, projected, base)
			}
		}
	}
}

// TestContextWindowReadsTheTwoWindowFields reads what an entry carries, and zero
// for a field that is absent or is not a whole number above zero.
func TestContextWindowReadsTheTwoWindowFields(t *testing.T) {
	document := mustParseCatalog(t, []byte(`{"models":[
		{"slug":"both","context_window":500000,"max_context_window":872000},
		{"slug":"window-only","context_window":272000},
		{"slug":"neither"},
		{"slug":"not-numbers","context_window":"500000","max_context_window":0.5},
		{"slug":"not-above-zero","context_window":0,"max_context_window":-1}]}`))
	for slug, want := range map[string][2]int{
		"both": {500000, 872000}, "window-only": {272000, 0}, "neither": {}, "not-numbers": {}, "not-above-zero": {}, "no-such-entry": {},
	} {
		if window, ceiling := document.ContextWindow(slug); window != want[0] || ceiling != want[1] {
			t.Errorf("ContextWindow(%q) = %d / %d, want %d / %d", slug, window, ceiling, want[0], want[1])
		}
	}
}

// TestTheStandInTableCarriesBothWindowFields fails when the table that these
// tests take for Codex's own stops carrying the two fields: the projection
// tests would then show the fields added, not replaced. No test of this
// package runs a Codex program, so the client's real table is held by the
// tracked verifier, which requires both fields in the entry that the client
// renders.
func TestTheStandInTableCarriesBothWindowFields(t *testing.T) {
	document := mustParseCatalog(t, testBundledCatalog("gpt-6.1-sol", "gpt-6-luna"))
	for _, slug := range []string{"gpt-6.1-sol", "gpt-6-luna"} {
		if window, ceiling := document.ContextWindow(slug); window <= 0 || ceiling <= 0 {
			t.Errorf("the stand-in entry %q carries the windows %d / %d, want both fields", slug, window, ceiling)
		}
	}
}
