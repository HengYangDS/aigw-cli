package adapters

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func TestDecodeAirTOMLBasicKeyEscapeExtra(t *testing.T) {
	cases := []struct {
		input    string
		expected rune
		width    int
		ok       bool
	}{
		{`\b`, '\b', 2, true},
		{`\f`, '\f', 2, true},
		{`\r`, '\r', 2, true},
		{`\t`, '\t', 2, true},
		{`\n`, '\n', 2, true},
		{`\"`, '"', 2, true},
		{`\\`, '\\', 2, true},
		{`\u0041`, 'A', 6, true},
		{`\U00000041`, 'A', 10, true},
		{`\q`, 0, 0, false},
		{`\`, 0, 0, false},
		{``, 0, 0, false},
		{`x`, 0, 0, false},
		{`\u004`, 0, 0, false},
		{`\u004G`, 0, 0, false},
		{`\uD800`, 0, 0, false}, // Surrogate
	}

	for _, c := range cases {
		r, width, ok := decodeAirTOMLBasicKeyEscape(c.input)
		if r != c.expected || width != c.width || ok != c.ok {
			t.Errorf("decodeAirTOMLBasicKeyEscape(%q) = (%v, %v, %v), want (%v, %v, %v)",
				c.input, r, width, ok, c.expected, c.width, c.ok)
		}
	}
}

func TestParseAirTOMLBasicKeySegmentExtra(t *testing.T) {
	cases := []struct {
		input    string
		expected airTOMLKeySegment
	}{
		{`"valid"`, airTOMLKeySegment{value: "valid", raw: "valid", end: 7, valid: true, basic: true}},
		{`"invalid\q"`, airTOMLKeySegment{value: "invalid", raw: "invalid\\q", end: 11, valid: false, basic: true}},
		{`"unfinished`, airTOMLKeySegment{value: "unfinished", raw: "unfinished", end: 11, valid: false, basic: true}},
		{"\"\x00\"", airTOMLKeySegment{value: "", raw: "\x00", end: 3, valid: false, basic: true}},
		{"\"\xff\"", airTOMLKeySegment{value: "", raw: "\xff", end: 3, valid: false, basic: true}},
	}

	for _, c := range cases {
		got := parseAirTOMLBasicKeySegment(c.input)
		if got.value != c.expected.value || got.raw != c.expected.raw || got.end != c.expected.end || got.valid != c.expected.valid {
			t.Errorf("parseAirTOMLBasicKeySegment(%q) = %+v, want %+v", c.input, got, c.expected)
		}
	}
}

func TestAirAliasOverflow(t *testing.T) {
	long := ""
	for i := 0; i < 70; i++ {
		long += "a"
	}
	key := parseAirTOMLBasicKeySegment(`"` + long + `"`)
	if !key.overflow {
		t.Error("expected overflow for long key")
	}
	if len(key.value) != airAliasDecodedKeyLimit {
		t.Errorf("expected value truncated to %d, got %d", airAliasDecodedKeyLimit, len(key.value))
	}
}

func TestAirLineHasIncompleteProtectedTableAliasExtra(t *testing.T) {
	cases := []struct {
		input    string
		expected bool
	}{
		{"[model_providers", true},
		{"[model_providers.", false}, // It says false in code because position < len(line) && line[position] == '.' { return false }
		{"[other", false},
	}
	for _, c := range cases {
		if got := airLineHasIncompleteProtectedTableAlias(c.input); got != c.expected {
			t.Errorf("airLineHasIncompleteProtectedTableAlias(%q) = %v, want %v", c.input, got, c.expected)
		}
	}
}

func TestAirMalformedBasicKeyResemblesAliasExtra(t *testing.T) {
	if !airMalformedBasicKeyResemblesAlias(airTOMLKeySegment{raw: "model_provider"}, "model") {
		t.Error("expected to resemble")
	}
}

func TestAirLineDeclaresAIGWProviderTableExtra(t *testing.T) {
	cases := []struct {
		input    string
		expected bool
	}{
		{"[[model_providers.aigw]]", true},
		{"[model_providers.aigw_fallback]", true},
		{"[model_providers.other]", false},
		{"model_provider = 1", false},
		{"[model_providers]", false},
		{"[model_providers.aigw", true},
	}
	for _, c := range cases {
		if got := airLineDeclaresAIGWProviderTable(c.input); got != c.expected {
			t.Errorf("airLineDeclaresAIGWProviderTable(%q) = %v, want %v", c.input, got, c.expected)
		}
	}
}

func TestRemoveAirProjectionSpansExtra(t *testing.T) {
	// Line 271: removeAirProjectionSpans (83.3%)

	// Out of bounds
	_, err := removeAirProjectionSpans("abc", []airTextSpan{{start: -1, end: 1}})
	if err == nil || !strings.Contains(err.Error(), "outside the captured preimage") {
		t.Errorf("expected out of bounds error, got %v", err)
	}

	// Overlap
	_, err = removeAirProjectionSpans("abc", []airTextSpan{{start: 0, end: 2}, {start: 1, end: 3}})
	if err == nil || !strings.Contains(err.Error(), "spans overlap") {
		t.Errorf("expected overlap error, got %v", err)
	}
}

func TestPlanAirOrphanRemovalExtra(t *testing.T) {
	// Line 75: PlanAirOrphanRemoval (75.0%)
	root := t.TempDir()
	path := filepath.Join(root, "air.toml")
	writeExtraCodexFile(t, path, "")

	// Not an orphan
	_, err := PlanAirOrphanRemoval(path, "")
	if err == nil || !strings.Contains(err.Error(), "not an exact removable orphan") {
		t.Errorf("expected not an orphan error, got %v", err)
	}
}

func TestAirSnapshotWithDataExtra(t *testing.T) {
	// Line 592: airSnapshotWithData (100% usually but good to call)
	snap := airSnapshotWithData(transaction.FileSnapshot{Mode: 0644}, []byte("data"))
	if snap.SHA256 == "" || snap.Mode != 0644 {
		t.Errorf("invalid snapshot %+v", snap)
	}
}

func TestInspectAirCodexConfigReadBoundaries(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		inspection, err := InspectAirCodexConfig(filepath.Join(t.TempDir(), "missing.toml"), "")
		if err != nil {
			t.Fatal(err)
		}
		if inspection.State != "missing" {
			t.Fatalf("inspection = %#v", inspection)
		}
	})

	t.Run("Air path is directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := PlanAirOrphanRemoval(path, ""); err == nil || !strings.Contains(err.Error(), "read Codex config") {
			t.Fatalf("PlanAirOrphanRemoval() error = %v", err)
		}
	})

	t.Run("standalone path is directory", func(t *testing.T) {
		root := t.TempDir()
		airPath := filepath.Join(root, "air.toml")
		standalonePath := filepath.Join(root, "standalone.toml")
		if err := os.Mkdir(standalonePath, 0o700); err != nil {
			t.Fatal(err)
		}
		runtime := atomicTestRuntime()
		block := codexManagedBlock(runtime, runtime.Endpoint)
		if err := os.WriteFile(airPath, []byte(projectCodex("", block, runtime.Model)), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := InspectAirCodexConfig(airPath, standalonePath); err == nil || !strings.Contains(err.Error(), "inspect standalone Codex config") {
			t.Fatalf("InspectAirCodexConfig() error = %v", err)
		}
	})
}

func TestExactAirManagedProjectionRejectsStructuralAmbiguity(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	for _, test := range []struct {
		name string
		text string
	}{
		{
			name: "provider table precedes ownership marker",
			text: codexSelection + "\n" +
				"[model_providers.aigw]\n" + codexBegin + "\n" + codexEnd + "\n",
		},
		{
			name: "ownership marker embedded in line",
			text: codexSelection + "\nprefix " + codexBegin + "\n" + block,
		},
		{
			name: "content separates marker and block",
			text: codexSelection + "\n" + codexBegin + "\nexternal = true\n" + block,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if projection, ok := exactAirManagedProjection(test.text); ok {
				t.Fatalf("exactAirManagedProjection() admitted %#v", projection)
			}
		})
	}
}

func TestAirIncompleteAliasBoundaries(t *testing.T) {
	if airLineHasIncompleteProtectedAlias("=") {
		t.Fatal("invalid empty key resembles a protected alias")
	}
	if !airLineHasIncompleteProtectedTableAlias("[[model_providers") {
		t.Fatal("incomplete array table alias was not detected")
	}
}
