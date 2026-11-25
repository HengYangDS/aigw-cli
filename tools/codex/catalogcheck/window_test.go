package main

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex/modelcatalog"
	"aigw-cli/internal/domain"
)

// loweredWindow is a declared context window below the default and below the
// one that the fake client's table gives the family gpt-6.1-sol.
const loweredWindow = 400_000

// TestVerifyCatalogHoldsAnAliasToItsDeclaredWindow requires the alias that the
// client rendered to carry the declared window in both fields, records the two
// numbers, and leaves them out of the comparison with the family's entry,
// which carries another window and no ceiling.
func TestVerifyCatalogHoldsAnAliasToItsDeclaredWindow(t *testing.T) {
	const (
		family = "gpt-6.1-sol"
		model  = "gpt-6.1-sol-cdx"
	)
	for _, window := range []int{domain.DefaultContextWindow, loweredWindow} {
		verification, err := verifyCatalog(fakeCodexClient(t, true), model, family, "Alias", window)
		if err != nil {
			t.Fatalf("verifyCatalog() error = %v", err)
		}
		if err := verification.Check(); err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		if verification.BaseSlug != family || verification.Unadapted.Present || verification.Adapted.MetadataSHA256 != verification.Reference.MetadataSHA256 {
			t.Fatalf("the alias is not held to its family's entry: %+v", verification)
		}
		if verification.DeclaredWindow != window || verification.ContextWindow != window || verification.MaxContextWindow != window {
			t.Fatalf("the record holds the windows %d, %d and %d, want %d in each", verification.DeclaredWindow, verification.ContextWindow, verification.MaxContextWindow, window)
		}
	}
}

// TestVerifyCatalogHoldsAModelThatIsAnEntryToItsDeclaredWindow verifies a
// model that the client's own table holds: the entry that the client rendered
// carries the declared window in both fields and equals the bundled entry in
// every other field, and no name is asked of it.
func TestVerifyCatalogHoldsAModelThatIsAnEntryToItsDeclaredWindow(t *testing.T) {
	const model = "gpt-6.1-sol"
	for _, window := range []int{domain.DefaultContextWindow, loweredWindow} {
		verification, err := verifyCatalog(fakeCodexClient(t, true), model, model, "A label of AIGW's", window)
		if err != nil {
			t.Fatalf("verifyCatalog() error = %v", err)
		}
		if err := verification.Check(); err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		if verification.BaseSlug != model || !verification.Reference.Present || !verification.Unadapted.Present || verification.Unknown.Present ||
			verification.Adapted.MetadataSHA256 != verification.Reference.MetadataSHA256 || verification.SharedName != nil {
			t.Fatalf("the entry is not held to the client's own: %+v", verification)
		}
		if verification.DeclaredWindow != window || verification.ContextWindow != window || verification.MaxContextWindow != window {
			t.Fatalf("the record holds the windows %d, %d and %d, want %d in each", verification.DeclaredWindow, verification.ContextWindow, verification.MaxContextWindow, window)
		}
	}
}

// TestVerifyCatalogRejectsAnEntryThatTheClientDidNotGiveTheWindow runs a
// client that ignores the generated catalog: the entry is the bundled one,
// with the family's own window and no ceiling, and the verdict names both
// numbers that were read.
func TestVerifyCatalogRejectsAnEntryThatTheClientDidNotGiveTheWindow(t *testing.T) {
	verification, err := verifyCatalog(fakeCodexClient(t, false), "gpt-6.1-sol", "gpt-6.1-sol", "Alias", domain.DefaultContextWindow)
	if err != nil {
		t.Fatalf("verifyCatalog() error = %v", err)
	}
	err = verification.Check()
	if err == nil || !strings.Contains(err.Error(), "context_window 500000 and max_context_window 0") || verification.ContextWindow != 500000 || verification.MaxContextWindow != 0 {
		t.Fatalf("Check() error = %v for %+v", err, verification)
	}
}

// TestCheckHoldsTheWindowAndTheEntry pins the verdicts that the declared
// window and the entry case add, over records of both kinds.
func TestCheckHoldsTheWindowAndTheEntry(t *testing.T) {
	const window = loweredWindow
	base := modelProbe{Model: "gpt-6.1-sol", Present: true, MetadataSHA256: "base"}
	alias := verificationResult{
		Model: "gpt-6.1-sol-cdx", BaseSlug: "gpt-6.1-sol", DeclaredWindow: window, ContextWindow: window, MaxContextWindow: window,
		Reference: base, Unadapted: modelProbe{Model: "gpt-6.1-sol-cdx"},
		Adapted: modelProbe{Model: "gpt-6.1-sol-cdx", Present: true, MetadataSHA256: "base"}, Unknown: modelProbe{Model: unknownProbeModel},
	}
	entry := alias
	entry.Model, entry.Unadapted, entry.Adapted = "gpt-6.1-sol", base, base
	for name, test := range map[string]struct {
		record func(verificationResult) verificationResult
		want   string
	}{
		"the window as declared":               {record: func(v verificationResult) verificationResult { return v }},
		"another context_window":               {record: func(v verificationResult) verificationResult { v.ContextWindow = 500000; return v }, want: "context_window 500000 and max_context_window 400000, want the declared context window 400000 in both"},
		"another max_context_window":           {record: func(v verificationResult) verificationResult { v.MaxContextWindow = 872000; return v }, want: "context_window 400000 and max_context_window 872000"},
		"no window field rendered":             {record: func(v verificationResult) verificationResult { v.ContextWindow, v.MaxContextWindow = 0, 0; return v }, want: "context_window 0 and max_context_window 0"},
		"no declared window":                   {record: func(v verificationResult) verificationResult { v.DeclaredWindow = 0; return v }, want: "the declared context window 0 is not above zero"},
		"other metadata than the base's":       {record: func(v verificationResult) verificationResult { v.Adapted.MetadataSHA256 = "different"; return v }, want: "metadata digest"},
		"absent from the client's table":       {record: func(v verificationResult) verificationResult { v.Reference.Present = false; return v }, want: "absent from the bundled catalog"},
		"absent from what the client rendered": {record: func(v verificationResult) verificationResult { v.Adapted = modelProbe{Model: v.Model}; return v }, want: "absent from the effective catalog"},
	} {
		for kind, record := range map[string]verificationResult{"alias": alias, "entry": entry} {
			err := test.record(record).Check()
			if test.want == "" && err != nil || test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Errorf("%s, %s: Check() error = %v, want %q", name, kind, err, test.want)
			}
		}
	}
}

// TestCatalogProbeComparesWhatTheEntryDoesNotOwn digests an entry without the
// two window fields, and an alias also without its slug and its display name:
// a model's own entry keeps the name that the client gives it.
func TestCatalogProbeComparesWhatTheEntryDoesNotOwn(t *testing.T) {
	parse := func(entry string) modelcatalog.Document {
		t.Helper()
		document, err := modelcatalog.Parse([]byte(`{"models":[` + entry + `]}`))
		if err != nil {
			t.Fatal(err)
		}
		return document
	}
	client := parse(`{"slug":"base","display_name":"Base","context_window":500000,"max_context_window":872000,"priority":1}`)
	for name, test := range map[string]struct {
		entry        string
		alias, equal bool
	}{
		"an alias with its own window":    {entry: `{"slug":"base","display_name":"Alias","context_window":1,"max_context_window":1,"priority":1}`, alias: true, equal: true},
		"an entry with its own window":    {entry: `{"slug":"base","display_name":"Base","context_window":1,"max_context_window":1,"priority":1}`, equal: true},
		"an entry under another name":     {entry: `{"slug":"base","display_name":"Alias","context_window":1,"max_context_window":1,"priority":1}`},
		"an alias with other metadata":    {entry: `{"slug":"base","display_name":"Alias","context_window":1,"max_context_window":1,"priority":2}`, alias: true},
		"an entry with one more field":    {entry: `{"slug":"base","display_name":"Base","context_window":1,"max_context_window":1,"priority":1,"more":true}`},
		"an alias without window fields":  {entry: `{"slug":"base","display_name":"Alias","priority":1}`, alias: true, equal: true},
		"an entry without a window field": {entry: `{"slug":"base","display_name":"Base","context_window":500000,"priority":1}`, equal: true},
	} {
		own := modelcatalog.ProjectedFields(test.alias)
		reference, rendered := catalogProbe(client, "base", own), catalogProbe(parse(test.entry), "base", own)
		if !reference.Present || !rendered.Present || (reference.MetadataSHA256 == rendered.MetadataSHA256) != test.equal {
			t.Errorf("%s: digests %s and %s, want equal %t", name, reference.MetadataSHA256, rendered.MetadataSHA256, test.equal)
		}
	}
}

// TestRunTakesTheDeclaredWindow passes -context-window to the verifier, the
// default window without it, and refuses one that is not above zero before it
// looks at the client's table.
func TestRunTakesTheDeclaredWindow(t *testing.T) {
	original := verifyModelCatalog
	t.Cleanup(func() { verifyModelCatalog = original })
	var got []int
	verifyModelCatalog = func(_, _, _, _ string, window int) (verificationResult, error) {
		got = append(got, window)
		verification := sampleVerification()
		verification.DeclaredWindow, verification.ContextWindow, verification.MaxContextWindow = window, window, window
		return verification, nil
	}
	arguments := []string{"-model", "gpt-6.1-sol-cdx", "-family", "gpt-6.1-sol", "-name", "GPT-6.1 Sol · cdx", "-codex", writeExecutableForTest(t)}
	for _, extra := range [][]string{nil, {"-context-window", "400000"}} {
		if code, err := run(append(arguments, extra...), io.Discard); code != 0 || err != nil {
			t.Fatalf("run(%q) = %d, %v", extra, code, err)
		}
	}
	if len(got) != 2 || got[0] != domain.DefaultContextWindow || got[1] != loweredWindow {
		t.Fatalf("the verifier took the windows %v, want the default %d and then %d", got, domain.DefaultContextWindow, loweredWindow)
	}
	for _, refused := range []string{"0", "-1"} {
		code, err := run(append(arguments, "-context-window", refused), io.Discard)
		if code != exitPrerequisiteMissing || err == nil || !strings.Contains(err.Error(), "-context-window") || len(got) != 2 {
			t.Fatalf("run(-context-window %s) = %d, %v after %d verifications", refused, code, err, len(got))
		}
	}
}

// TestReportShowsTheWindows prints the declared window and the two numbers
// read, and carries them in the JSON result under their own names.
func TestReportShowsTheWindows(t *testing.T) {
	verification := sampleVerification()
	verification.DeclaredWindow, verification.ContextWindow, verification.MaxContextWindow = loweredWindow, loweredWindow, 872000
	out := &bytes.Buffer{}
	if err := report(out, verification, false); err != nil || !strings.Contains(out.String(), "window   declared 400000; rendered context_window 400000, max_context_window 872000\n") {
		t.Fatalf("report() = %v:\n%s", err, out)
	}
	out.Reset()
	if err := report(out, verification, true); err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &fields); err != nil {
		t.Fatal(err)
	}
	for field, want := range map[string]string{"declared_context_window": "400000", "context_window": "400000", "max_context_window": "872000"} {
		if string(fields[field]) != want {
			t.Errorf("JSON field %s = %s, want %s", field, fields[field], want)
		}
	}
}
