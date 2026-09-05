package codex

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/surface"
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
		"max_threads = 16 # managed by AIGW",
		"max_depth = 1 # managed by AIGW",
		"[features.multi_agent_v2]",
		"max_concurrent_threads_per_session = 16 # managed by AIGW",
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
	if err := validateCodexSchedulerState(map[string]*int{"invalid": nil}); err == nil {
		t.Fatal("scheduler state validation accepted an invalid key")
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
	withoutTrailingNewline := "[agents]\nmax_depth = 4\n[next]"
	_, end, present = codexTableBounds(withoutTrailingNewline, "agents")
	if !present || withoutTrailingNewline[end:] != "[next]" {
		t.Fatalf("table without trailing newline ends at %d: %q", end, withoutTrailingNewline[end:])
	}
	if value, present, err := codexIntegerKey(text, "agents", "max_depth"); err != nil || !present || value != 4 {
		t.Fatalf("integer key = %d, %v, %v", value, present, err)
	}
	if _, present, err := codexIntegerKey(text, "agents", "unknown"); err != nil || present {
		t.Fatalf("missing integer key = %v, %v", present, err)
	}

	oneLine := "[agents]"
	updated := setCodexIntegerKey(oneLine, "agents", "max_depth", 1)
	if updated != "[agents]\nmax_depth = 1 # managed by AIGW\n" {
		t.Fatalf("one-line table update = %q", updated)
	}
	if updated := setCodexIntegerKey("external = true", "agents", "max_depth", 1); updated != "external = true\n\n[agents]\nmax_depth = 1 # managed by AIGW\n" {
		t.Fatalf("append table without trailing newline = %q", updated)
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
	if err := validateCodexScheduler("[agents]\nmax_depth = 999999999999999999999999999999999999999999999999999999999999\n"); err == nil || !strings.Contains(err.Error(), "parse Codex scheduler key") {
		t.Fatalf("scheduler integer overflow error = %v", err)
	}
	if _, err := captureCodexScheduler("[agents]\nmax_depth = 999999999999999999999999999999999999999999999999999999999999\n"); err == nil || !strings.Contains(err.Error(), "decimal number is too large") {
		t.Fatalf("scheduler capture overflow error = %v", err)
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

func TestCodexSchedulerRejectsMalformedKeyInspection(t *testing.T) {
	if _, err := codexKeyPresent("[agents\n", "agents", "max_threads"); err == nil {
		t.Fatal("codexKeyPresent accepted malformed TOML")
	}
}

func projectedSchedulerFixture() string {
	return "[agents]\nmax_threads = 16\nmax_depth = 1\n\n[features.multi_agent_v2]\nmax_concurrent_threads_per_session = 16\n"
}

// TestCodexSchedulerBindsOneKeyOfTheAliasPairPerTable pins the reason the
// projected key set changed: Codex reads [agents].max_threads as the session
// concurrency field and max_concurrent_threads_per_session as its retired alias,
// so a table declaring both is one field declared twice and Codex refuses to
// start. AIGW must therefore clear the alias it once projected.
func TestCodexSchedulerBindsOneKeyOfTheAliasPairPerTable(t *testing.T) {
	legacy := "[agents]\nmax_concurrent_threads_per_session = 7\nmax_depth = 3\n"
	projected, err := projectCodexScheduler(legacy)
	if err != nil {
		t.Fatal(err)
	}
	start, end, present := codexTableBounds(projected, "agents")
	if !present {
		t.Fatalf("projection lost the agents table:\n%s", projected)
	}
	agents := projected[start:end]
	if !strings.Contains(agents, "max_threads = 16 # managed by AIGW") {
		t.Fatalf("agents table does not bind max_threads:\n%s", agents)
	}
	if strings.Contains(agents, "max_concurrent_threads_per_session") {
		t.Fatalf("agents table still declares the retired alias beside max_threads:\n%s", agents)
	}
	if value, present, err := codexIntegerKey(projected, "features.multi_agent_v2", "max_concurrent_threads_per_session"); err != nil || !present || value != codexSessionConcurrency {
		t.Fatalf("feature-gated key = %d, %v, %v", value, present, err)
	}
	if err := validateCodexTOML(projected); err != nil {
		t.Fatalf("projection is not parseable TOML: %v\n%s", err, projected)
	}
	if err := validateCodexScheduler(projected); err != nil {
		t.Fatal(err)
	}
	// The alias the projection cleared is still recorded, so a restore returns the
	// user's own value rather than dropping it.
	captured, err := captureCodexScheduler(legacy)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := restoreCodexScheduler(projected, captured)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimRight(restored, "\n") != strings.TrimRight(legacy, "\n") {
		t.Fatalf("restore mismatch\nwant:\n%s\ngot:\n%s", legacy, restored)
	}
	// Validation must no longer demand the retired alias: the old projected shape
	// is now a scheduler mismatch, not a match.
	if err := validateCodexScheduler("[agents]\nmax_concurrent_threads_per_session = 16\nmax_depth = 1\n\n[features.multi_agent_v2]\nmax_concurrent_threads_per_session = 16\n"); err == nil {
		t.Fatal("validation accepted the retired [agents] alias as the projected key")
	}
}

func TestCodexSchedulerHashAdmitsOnlyCurrentProjection(t *testing.T) {
	current := projectedSchedulerFixture()
	if !codexSchedulerHashMatches(codexSchedulerHash(current), current) {
		t.Fatal("the current projection hash was rejected")
	}
	if codexSchedulerHashMatches("", current) {
		t.Fatal("an absent projection hash proved ownership")
	}
	if codexSchedulerHashMatches("foreign", current) {
		t.Fatal("a foreign hash was accepted")
	}
}

func TestCodexReconciliationRecognizesOwnedState(t *testing.T) {
	state := codexState{
		ProjectionMode: ProjectionFullSelection,
		WriterID:       ProjectionWriterID,
		TransactionID:  "transaction",
	}
	data := encodeCodexState(state)
	parsed, err := codexStateForTarget(transaction.FileSnapshot{Exists: true, Data: data})
	if err != nil || parsed.TransactionID != state.TransactionID {
		t.Fatalf("state = %#v, err=%v", parsed, err)
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
	if transactionID := newCodexTransactionID(); len(transactionID) != 32 {
		t.Fatalf("transaction ID = %q", transactionID)
	}
	if absent, err := codexStateForTarget(transaction.FileSnapshot{}); err != nil || !reflect.DeepEqual(absent, codexState{}) {
		t.Fatalf("absent state = %#v, err %v", absent, err)
	}
	if _, err := codexStateForTarget(transaction.FileSnapshot{Exists: true, Data: []byte("{")}); err == nil {
		t.Fatal("invalid state JSON was accepted")
	}
	if _, err := normalizeCodexTargets([]TargetRef{{}}); err == nil {
		t.Fatal("incomplete target was accepted")
	}
	if _, err := canonicalCodexTargetPath(t.TempDir() + "/missing/config.toml"); err != nil {
		t.Fatal(err)
	}
	if _, err := normalizeCodexTargets([]TargetRef{{
		SurfaceID: string(surface.CodexHomeDefault), Authority: string(surface.AuthorityAIGW),
		ProjectionMode: ProjectionFullSelection, Path: config,
	}, {
		SurfaceID: string(surface.CodexHomeDefault), Authority: string(surface.AuthorityAIGW),
		ProjectionMode: ProjectionFullSelection, Path: config,
	}}); err == nil {
		t.Fatal("duplicate target was accepted")
	}
	if _, err := codexStateForTarget(transaction.FileSnapshot{Exists: true, Data: []byte(`{"projection_mode":"full_selection"}`)}); err == nil {
		t.Fatal("incomplete state attribution was accepted")
	}
	if _, err := codexStateForTarget(transaction.FileSnapshot{Exists: true, Data: []byte(`{"projection_mode":"other","writer_id":"aigw-cli","transaction_id":"x"}`)}); err == nil {
		t.Fatal("unsupported projection mode was accepted")
	}
	if _, err := codexStateForTarget(transaction.FileSnapshot{Exists: true, Data: []byte(`{"projection_mode":"full_selection","writer_id":"foreign","transaction_id":"x"}`)}); err == nil {
		t.Fatal("foreign writer was accepted")
	}

}

func TestCodexReconciliationReportsManagedDrift(t *testing.T) {
	driftPath := t.TempDir() + "/drift.toml"
	driftRuntime := configuration.Runtime{ProfileID: "p", ProfileLabel: "P", Endpoint: "https://example.test", Model: "m"}
	if err := os.WriteFile(driftPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SyncConfig(driftPath, driftRuntime); err != nil {
		t.Fatal(err)
	}
	driftData, err := os.ReadFile(driftPath)
	if err != nil {
		t.Fatal(err)
	}
	driftText := strings.Replace(string(driftData), `name = "AIGW: P"`, `name = "AIGW: changed"`, 1)
	if err := os.WriteFile(driftPath, []byte(driftText), 0o600); err != nil {
		t.Fatal(err)
	}
	inspection, err := InspectConfig(driftPath)
	if err != nil || inspection.State != "aigw-drift" {
		t.Fatalf("drift inspection = %#v, %v", inspection, err)
	}
	managedData, err := os.ReadFile(driftPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := SyncConfig(driftPath, driftRuntime); err == nil {
		t.Fatal("sync unexpectedly repaired drift")
	}
	if err := os.WriteFile(driftPath, managedData, 0o600); err != nil {
		t.Fatal(err)
	}
	inspection, err = InspectConfig(driftPath)
	if err != nil || inspection.State != "aigw-drift" {
		t.Fatalf("restored drift inspection = %#v, %v", inspection, err)
	}

}

func TestCodexReconciliationRejectsConflictsAndMalformedHelpers(t *testing.T) {
	state := codexState{
		ProjectionMode: ProjectionFullSelection,
		WriterID:       ProjectionWriterID,
		TransactionID:  "transaction",
	}
	driftRuntime := configuration.Runtime{ProfileID: "p", ProfileLabel: "P", Endpoint: "https://example.test", Model: "m"}

	conflictPath := t.TempDir() + "/conflict.toml"
	if err := os.WriteFile(conflictPath, []byte("model_provider = \"aigw\" # managed by AIGW\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	conflictState := state
	conflictState.ProjectionMode = "unsupported"
	conflictData := encodeCodexState(conflictState)
	if err := os.WriteFile(codexStatePath(conflictPath), conflictData, 0o600); err != nil {
		t.Fatal(err)
	}
	inspection, err := InspectConfig(conflictPath)
	if err != nil || inspection.State != "ownership-conflict" {
		t.Fatalf("conflict inspection = %#v, %v", inspection, err)
	}

	unattributedPath := t.TempDir() + "/unattributed.toml"
	unattributedBlock := codexManagedBlock(driftRuntime, driftRuntime.Endpoint)
	unattributedConfig := "model_provider = \"aigw\" # managed by AIGW\n\n" + codexBegin + "\n" + unattributedBlock
	if err := os.WriteFile(unattributedPath, []byte(unattributedConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	unattributedState := codexState{ManagedBlockHash: hashText(unattributedBlock)}
	unattributedData := encodeCodexState(unattributedState)
	if err := os.WriteFile(codexStatePath(unattributedPath), unattributedData, 0o600); err != nil {
		t.Fatal(err)
	}
	inspection, err = InspectConfig(unattributedPath)
	if err != nil || inspection.State != "ownership-conflict" {
		t.Fatalf("unattributed inspection = %#v, %v", inspection, err)
	}
	if _, err := codexManagedBlockIn(codexBegin + "\n" + codexEnd); err == nil {
		t.Fatal("managed block without provider table was accepted")
	}
	if _, err := codexManagedBlockIn(codexBegin + "\n[model_providers.aigw]\n"); err == nil {
		t.Fatal("incomplete managed block was accepted")
	}
	if got := removeCodexBeginMarker("plain"); got != "plain" {
		t.Fatalf("plain text changed: %q", got)
	}
	if got := classifyCodexDiskSelection("model_provider ="); got != "external-or-host-owned" {
		t.Fatalf("empty selection = %q", got)
	}
	if got := restoreModelSelection("plain", `model = "native"`); !strings.HasPrefix(got, `model = "native"`) {
		t.Fatalf("restored model selection = %q", got)
	}
	if got := removeEnvironment([]string{"KEEP=1", "DROP=2", "AIGW_TOKEN_TEST=secret"}, "DROP"); len(got) != 1 || got[0] != "KEEP=1" {
		t.Fatalf("filtered environment = %v", got)
	}
}

// TestCodexRejectsAReappearingRetiredAliasAfterProjection pins the invariant that
// AIGW owns the *absence* of [agents].max_concurrent_threads_per_session, not only
// the value of the key it projects. Codex rejects a table carrying both members of
// the pair, so an alias that reappears after projection recreates exactly the shape
// this change exists to prevent. The invariant is that the key does not exist, so
// the table walks every TOML value type a user could reintroduce it with: detecting
// only an integer would let a string, boolean, or array value pass as absent.
// For each shape, validation must report the drift, the ownership hash must read it
// as drift, sync and disable must refuse instead of overwriting, and nothing may
// clean the key on the user's behalf.
func TestCodexRejectsAReappearingRetiredAliasAfterProjection(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		value string
	}{
		{name: "integer", value: "16"},
		{name: "string", value: "\"16\""},
		{name: "boolean", value: "true"},
		{name: "array", value: "[16]"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			path := t.TempDir() + "/configuration.toml"
			original := "model_provider = \"native\"\n\n[agents]\nmax_threads = 6\n"
			if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			runtime := atomicTestRuntime()
			if err := SyncConfig(path, runtime); err != nil {
				t.Fatal(err)
			}
			projected, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// Positive control: without it, every assertion below could pass on a
			// fixture that was never a valid current projection in the first place.
			if err := ValidateConfig(path, runtime); err != nil {
				t.Fatalf("the fixture is not a valid current projection: %v", err)
			}

			injected := strings.Replace(string(projected), "max_threads = 16 # managed by AIGW", "max_threads = 16 # managed by AIGW\nmax_concurrent_threads_per_session = "+testCase.value, 1)
			if injected == string(projected) {
				t.Fatalf("the projection no longer carries the key this test injects beside:\n%s", projected)
			}
			if err := validateCodexTOML(injected); err != nil {
				t.Fatalf("the alias pair must be a semantic duplicate Codex parses, not a TOML error: %v", err)
			}
			if err := os.WriteFile(path, []byte(injected), 0o600); err != nil {
				t.Fatal(err)
			}

			err = ValidateConfig(path, runtime)
			if err == nil || !strings.Contains(err.Error(), "max_concurrent_threads_per_session") {
				t.Fatalf("validation accepted the reappearing retired alias: %v", err)
			}
			if codexSchedulerHashMatches(codexSchedulerHash(string(projected)), injected) {
				t.Fatal("the ownership hash read the reappearing alias as AIGW's own projection")
			}
			if err := SyncConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "scheduler keys changed") {
				t.Fatalf("sync overwrote the drifted table instead of refusing: %v", err)
			}
			if err := DisableConfig(path); err == nil || !strings.Contains(err.Error(), "scheduler keys changed") {
				t.Fatalf("disable overwrote the drifted table instead of refusing: %v", err)
			}
			// Every path above must report the drift, never clean it: the alias may
			// be the user's own line, and validation only reads configuration.
			unchanged, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(unchanged) != injected {
				t.Fatalf("a refusing path still edited the file:\n%s", unchanged)
			}
		})
	}
}
