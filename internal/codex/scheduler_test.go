package codex

import (
	"os"
	"strings"
	"testing"

	"aigw-cli/internal/transaction"
)

func TestCodexSchedulerHelpersCoverAbsentAndMalformedShapes(t *testing.T) {
	original := "model_provider = \"native\"\n\n[agents]\nmax_threads = 9\n"
	projected, err := projectCodexScheduler(original)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"[agents]",
		"max_threads = 9",
		"max_concurrent_threads_per_session = 16 # managed by AIGW",
		"max_depth = 1 # managed by AIGW",
		"[features.multi_agent_v2]",
	} {
		if !strings.Contains(projected, want) {
			t.Fatalf("projection lacks %q:\n%s", want, projected)
		}
	}
	captured, err := captureCodexScheduler(original)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := restoreCodexScheduler(projected, captured)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimRight(restored, "\n") != strings.TrimRight(original, "\n") {
		t.Fatalf("restore mismatch\nwant:\n%s\ngot:\n%s", original, restored)
	}

	if _, err := captureCodexScheduler("[agents\n"); err == nil {
		t.Fatal("capture accepted malformed TOML")
	}
	if _, err := projectCodexScheduler("[agents\n"); err == nil {
		t.Fatal("projection accepted malformed TOML")
	}
	if _, err := restoreCodexScheduler("", map[string]*int{"invalid": nil}); err == nil {
		t.Fatal("restore accepted an invalid scheduler state key")
	}
}

func TestCodexSchedulerHelpersCoverTableBoundariesAndHashing(t *testing.T) {
	text := "[agents]\nmax_depth = 4 # note\nother = 1\n\n[next]\nvalue = 2\n"
	start, end, present := codexTableBounds(text, "agents")
	if !present || text[start:end] != "[agents]\nmax_depth = 4 # note\nother = 1\n\n" {
		t.Fatalf("table bounds = %d, %d, %v: %q", start, end, present, text[start:end])
	}
	if _, _, present := codexTableBounds(text, "missing"); present {
		t.Fatal("missing table reported present")
	}
	if value, present := codexIntegerKey(text, "agents", "max_depth"); !present || value != 4 {
		t.Fatalf("integer key = %d, %v", value, present)
	}
	if _, present := codexIntegerKey(text, "agents", "unknown"); present {
		t.Fatal("missing integer key reported present")
	}

	oneLine := "[agents]"
	updated := setCodexIntegerKey(oneLine, "agents", "max_depth", 1)
	if updated != "[agents]\nmax_depth = 1 # managed by AIGW\n" {
		t.Fatalf("one-line table update = %q", updated)
	}
	if removed := removeCodexIntegerKey(text, "missing", "max_depth"); removed != text {
		t.Fatalf("remove from absent table = %q", removed)
	}
	if got := removeEmptyCodexTable("[agents]\n# comment\n\n[next]\nvalue = 2\n", "agents"); got != "[next]\nvalue = 2\n" {
		t.Fatalf("empty table removal = %q", got)
	}
	if got := removeEmptyCodexTable(text, "agents"); got != text {
		t.Fatal("non-empty table was removed")
	}

	if err := validateCodexScheduler(projectedSchedulerFixture()); err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(projectedSchedulerFixture(), "max_depth = 1", "max_depth = 2", 1)
	if err := validateCodexScheduler(drifted); err == nil {
		t.Fatal("scheduler validation accepted drift")
	}
	if codexSchedulerHash(projectedSchedulerFixture()) == codexSchedulerHash("") {
		t.Fatal("scheduler hash did not distinguish missing keys")
	}
}

func TestCodexSchedulerHelpersCoverRemainingErrorPaths(t *testing.T) {
	originalKeys := codexSchedulerKeys
	defer func() { codexSchedulerKeys = originalKeys }()

	codexSchedulerKeys = map[string]map[string]int{"agents": {"max_depth": 1}}
	if err := validateCodexScheduler("[agents]\nmax_depth = 1\n"); err != nil {
		t.Fatal(err)
	}
	if err := validateCodexScheduler("[agents]\nmax_depth = 2\n"); err == nil {
		t.Fatal("validation accepted a different scheduler value")
	}

	codexSchedulerKeys = map[string]map[string]int{"bad[": {"max_depth": 1}}
	if _, err := projectCodexScheduler(""); err == nil {
		t.Fatal("projection accepted an invalid table name")
	}
	value := 1
	if _, err := restoreCodexScheduler("", map[string]*int{"bad[.max_depth": &value}); err == nil {
		t.Fatal("restore accepted an invalid table name")
	}
}

func projectedSchedulerFixture() string {
	return "[agents]\nmax_concurrent_threads_per_session = 16\nmax_depth = 1\n\n[features.multi_agent_v2]\nmax_concurrent_threads_per_session = 16\n"
}

func TestCodexReconciliationHelpersCoverRecognizedState(t *testing.T) {
	state := codexState{
		ProjectionMode: ProjectionFullSelection,
		WriterID:       ProjectionWriterID,
		TransactionID:  "transaction",
	}
	data, err := encodeCodexState(state)
	if err != nil {
		t.Fatal(err)
	}
	parsed, legacy, err := codexStateForTarget(transaction.FileSnapshot{Exists: true, Data: data}, ProjectionFullSelection)
	if err != nil || legacy || parsed.TransactionID != state.TransactionID {
		t.Fatalf("state = %#v, legacy=%v, err=%v", parsed, legacy, err)
	}

	config := t.TempDir() + "/config.toml"
	if err := os.WriteFile(codexStatePath(config), data, 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := ReadProjectionIdentity(config)
	if err != nil || !identity.Present || identity.AttributionState != "recognized" {
		t.Fatalf("identity = %#v, err=%v", identity, err)
	}
	if _, err := canonicalCodexTargetPath(config); err != nil {
		t.Fatal(err)
	}
	if _, err := newCodexTransactionID(); err != nil {
		t.Fatal(err)
	}
}
