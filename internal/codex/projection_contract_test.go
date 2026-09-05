package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/transaction"
)

func TestCodexEndpointRequiresConfiguration(t *testing.T) {
	cases := []struct {
		name     string
		runtime  configuration.Runtime
		expected string
		wantErr  bool
	}{
		{"missing", configuration.Runtime{ProfileID: "p"}, "", true},
		{"valid", configuration.Runtime{ProfileID: "p", Endpoint: "https://example.com/"}, "https://example.com/", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := codexEndpoint(c.runtime)
			if (err != nil) != c.wantErr {
				t.Fatalf("got err = %v, wantErr %v", err, c.wantErr)
			}
			if got != c.expected {
				t.Errorf("got %q, want %q", got, c.expected)
			}
		})
	}
}

func TestRestoreModelSelectionPreservesOriginalState(t *testing.T) {
	cases := []struct {
		name          string
		base          string
		originalModel string
		expected      string
	}{
		{
			name:          "empty original",
			base:          "model = \"a\" # managed by AIGW\nother = 1",
			originalModel: "",
			expected:      "other = 1",
		},
		{
			name:          "with original, matching line",
			base:          "model = \"a\" # managed by AIGW\nother = 1",
			originalModel: "model = \"b\"",
			expected:      "model = \"b\"\nother = 1",
		},
		{
			name:          "with original, no matching line",
			base:          "other = 1",
			originalModel: "model = \"b\"",
			expected:      "model = \"b\"\nother = 1",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := restoreModelSelection(c.base, c.originalModel)
			if got != c.expected {
				t.Errorf("got %q, want %q", got, c.expected)
			}
		})
	}
}

func TestProjectCodexRejectsMalformedUserConfiguration(t *testing.T) {
	if _, err := projectCodex("[broken", "", "", ""); err == nil || !strings.Contains(err.Error(), "parse Codex config") {
		t.Fatalf("projectCodex() error = %v", err)
	}
}

func TestRemoveCodexProjectionRejectsInvalidCapturedSchedulerState(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	current, err := projectCodex("external = true\n", block, runtime.Model, "")
	if err != nil {
		t.Fatal(err)
	}
	state := codexState{
		ManagedBlockHash:       hashText(block),
		OriginalScheduler:      map[string]*int{"invalid": nil},
		ProjectedSchedulerHash: codexSchedulerHash(current),
	}
	if _, err := removeCodexProjection(current, state); err == nil || !strings.Contains(err.Error(), "invalid Codex scheduler state key") {
		t.Fatalf("removeCodexProjection() error = %v", err)
	}
}

func TestManagedBlockAcceptsCRLFMarkerBoundary(t *testing.T) {
	text := codexBegin + "\r\n[model_providers.aigw]\r\n" + codexEnd + "\r\n"
	block, err := codexManagedBlockIn(text)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(block, "\r\n") {
		t.Fatalf("managed block = %q", block)
	}
}

func TestValidateConfigRejectsIncompleteOrMismatchedProjection(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "configuration.toml")

	err := ValidateConfig(path, configuration.Runtime{ProfileID: "p"})
	if err == nil || !strings.Contains(err.Error(), "no Codex endpoint") {
		t.Errorf("expected endpoint error, got %v", err)
	}

	runtime := configuration.Runtime{ProfileID: "p", Endpoint: "https://e.t"}
	err = ValidateConfig(path, runtime)
	if err == nil || !strings.Contains(err.Error(), "read Codex config") {
		t.Errorf("expected read error, got %v", err)
	}

	writeCodexFixture(t, path, "")
	err = ValidateConfig(path, runtime)
	if err == nil || !strings.Contains(err.Error(), "AIGW state is missing") {
		t.Errorf("expected missing state error, got %v", err)
	}

	state := codexState{
		ProjectionMode:   ProjectionFullSelection,
		WriterID:         ProjectionWriterID,
		TransactionID:    "t",
		ManagedBlockHash: "mismatch",
	}
	writeCodexStateFixture(t, path, state)

	block := codexManagedBlock(runtime, runtime.Endpoint)
	content := "model_provider = \"aigw\" # managed by AIGW\n" + codexBegin + "\n" + block
	writeCodexFixture(t, path, content)

	err = ValidateConfig(path, runtime)
	if err == nil || !strings.Contains(err.Error(), "state does not match") {
		t.Errorf("expected hash mismatch error, got %v", err)
	}
}

func TestExactTruncationRejectsShortInput(t *testing.T) {
	if isExactTruncatedCodexProjection("too short", nil, configuration.Runtime{}, "") {
		t.Error("expected false for short input")
	}
}

func TestInspectConfigClassifiesInvalidAndOrphanedState(t *testing.T) {
	root := t.TempDir()

	t.Run("invalid sidecar json", func(t *testing.T) {
		path := filepath.Join(root, "invalid-sidecar.toml")
		writeCodexFixture(t, path, "model_provider = \"native\"")
		writeCodexFixture(t, codexStatePath(path), "invalid json")

		ins, err := InspectConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if ins.State != "invalid-sidecar" {
			t.Errorf("got state %q, want invalid-sidecar", ins.State)
		}
	})

	t.Run("orphaned marker", func(t *testing.T) {
		path := filepath.Join(root, "orphaned.toml")
		writeCodexFixture(t, path, codexBegin)

		ins, err := InspectConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if ins.State != "orphaned-aigw-marker" {
			t.Errorf("got state %q, want orphaned-aigw-marker", ins.State)
		}
	})

	t.Run("ownership conflict", func(t *testing.T) {
		path := filepath.Join(root, "conflict.toml")
		writeCodexFixture(t, path, "")
		state := codexState{WriterID: "foreign"}
		writeCodexStateFixture(t, path, state)

		ins, err := InspectConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if ins.State != "ownership-conflict" {
			t.Errorf("got state %q, want ownership-conflict", ins.State)
		}
	})

	t.Run("stale sidecar", func(t *testing.T) {
		path := filepath.Join(root, "stale.toml")
		writeCodexFixture(t, path, "")
		state := codexState{
			ProjectionMode: ProjectionFullSelection,
			WriterID:       ProjectionWriterID,
			TransactionID:  "some-tx",
		}
		writeCodexStateFixture(t, path, state)

		ins, err := InspectConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if ins.State != "stale-sidecar" {
			t.Errorf("got state %q, want stale-sidecar", ins.State)
		}
	})
}

func TestReadProjectionIdentityDistinguishesMissingAndIncompleteState(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "identity.toml")

	id, err := ReadProjectionIdentity(path)
	if err != nil || id.Present {
		t.Errorf("expected not present, got %+v, err %v", id, err)
	}

	// Unattributed sidecars fail closed.
	writeCodexFixture(t, path, "")
	state := codexState{}
	writeCodexStateFixture(t, path, state)
	id, err = ReadProjectionIdentity(path)
	if err == nil || !strings.Contains(err.Error(), "attribution is incomplete") || id.Present {
		t.Errorf("expected incomplete attribution rejection, got %+v, err %v", id, err)
	}
}

func TestCanonicalCodexTargetPathResolvesAbsoluteAndSymlinkPaths(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "target.toml")

	got, err := canonicalCodexTargetPath(path)
	if err != nil || !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q, err %v", got, err)
	}

	link := filepath.Join(root, "link.toml")
	writeCodexFixture(t, path, "")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}

	path, _ = canonicalCodexTargetPath(path)
	got, err = canonicalCodexTargetPath(link)
	if err != nil || got != path {
		t.Errorf("expected resolved symlink %q, got %q, err %v", path, got, err)
	}
}

func TestValidateDesiredCodexTargetRejectsUnknownSurface(t *testing.T) {
	err := validateDesiredCodexTarget(TargetRef{SurfaceID: "invalid"})
	if err == nil {
		t.Error("expected error for invalid surface")
	}
}

func TestTargetCodexStatePathPrefersExplicitPath(t *testing.T) {
	ref := TargetRef{Path: "p"}
	if got := targetCodexStatePath(ref); got != "p.aigw-state.json" {
		t.Errorf("got %q", got)
	}
	ref.statePath = "s"
	if got := targetCodexStatePath(ref); got != "s" {
		t.Errorf("got %q", got)
	}
}

func TestPreferredCodexStatePathUsesExistingCanonicalSidecar(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "alias.toml")
	canonical := filepath.Join(root, "configuration.toml")
	want := codexStatePath(canonical)
	if err := os.WriteFile(want, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := preferredCodexStatePath(source, canonical); got != want {
		t.Fatalf("preferredCodexStatePath() = %q, want %q", got, want)
	}
}

func TestClassifyCodexDiskSelection(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"", "unset"},
		{"model_provider = \"aigw\" # managed by AIGW", "aigw-managed"},
		{"model_provider = \"aigw\"", "aigw-user-selected"},
		{"model_provider = \"aigw_fallback\"", "aigw-user-selected"},
		{"model_provider = \"native\"", "external-or-host-owned"},
		{"model_provider", "unset"},
	}
	for _, c := range cases {
		if got := classifyCodexDiskSelection(c.input); got != c.expected {
			t.Errorf("classifyCodexDiskSelection(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestRollbackCodexArtifactsPreservesExternalEdit(t *testing.T) {
	if err := rollbackCodexArtifacts(nil); err != nil {
		t.Fatalf("rollbackCodexArtifacts(nil) error = %v", err)
	}

	path := filepath.Join(t.TempDir(), "configuration.toml")
	writeCodexFixture(t, path, "original\n")
	before, err := transaction.CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	post, err := transaction.WriteFileAtomicIfUnchanged(path, before, []byte("transaction postimage\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	writeCodexFixture(t, path, "newer external edit\n")

	committed := []committedCodexArtifact{{
		prepared: codexPreparedArtifact{path: path, before: before},
		post:     post,
	}}
	if err := rollbackCodexArtifacts(committed); err == nil || !strings.Contains(err.Error(), "postimage changed") {
		t.Fatalf("rollbackCodexArtifacts() error = %v", err)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != "newer external edit\n" {
		t.Fatalf("rollback overwrote external edit: %q", current)
	}
}

func TestPrepareCodexReconciliationRejectsInvalidTarget(t *testing.T) {
	_, _, err := prepareCodexReconciliation([]TargetRef{{Path: ""}}, nil, configuration.Runtime{})
	if err == nil {
		t.Error("expected error for invalid target")
	}
}

func TestValidateCodexStateAttributionRejectsIncompleteOrForeignState(t *testing.T) {
	cases := []struct {
		state codexState
		mode  string
	}{
		{codexState{ProjectionMode: "m"}, ""}, // incomplete
		{codexState{ProjectionMode: "invalid", WriterID: "w", TransactionID: "t"}, ""},
		{codexState{ProjectionMode: ProjectionFullSelection, WriterID: "other", TransactionID: "t"}, ""},
	}
	for _, c := range cases {
		err := validateCodexStateAttribution(c.state)
		if err == nil {
			t.Errorf("expected error for state %+v, mode %q", c.state, c.mode)
		}
	}
}

func TestPrepareCodexRestoreHandlesRestoredAndUnsupportedState(t *testing.T) {
	target := TargetRef{Path: "p", ProjectionMode: ProjectionFullSelection}
	configSnap := transaction.FileSnapshot{Exists: true, Data: []byte("")}
	stateSnap := transaction.FileSnapshot{Exists: false}

	plan, err := prepareCodexRestore(target, configSnap, stateSnap, transaction.FileSnapshot{})
	if err != nil || plan.plan.Action != "already-restored" {
		t.Errorf("expected already-restored, got %+v, err %v", plan, err)
	}

	state := codexState{ProjectionMode: "invalid", WriterID: ProjectionWriterID, TransactionID: "t"}
	data, _ := json.Marshal(state)
	stateSnap = transaction.FileSnapshot{Exists: true, Data: data}
	_, err = prepareCodexRestore(target, configSnap, stateSnap, transaction.FileSnapshot{})
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected unsupported error, got %v", err)
	}
}

func TestNormalizeCodexTargetsRejectsIncompleteAndDuplicateTargets(t *testing.T) {
	_, err := normalizeCodexTargets([]TargetRef{{Path: "p"}})
	if err == nil {
		t.Error("expected error for missing fields")
	}

	target := TargetRef{Path: "p", SurfaceID: "s", Authority: "a", ProjectionMode: "m"}
	_, err = normalizeCodexTargets([]TargetRef{target, target})
	if err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Errorf("expected duplicate error, got %v", err)
	}
}

func writeCodexFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeCodexStateFixture(t *testing.T, path string, state codexState) {
	t.Helper()
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	writeCodexFixture(t, codexStatePath(path), string(data)+"\n")
}

func attributedCodexStateFixture(mode, block string) codexState {
	originalScheduler, err := captureCodexScheduler("")
	if err != nil {
		panic(err)
	}
	projectedScheduler, err := projectCodexScheduler("")
	if err != nil {
		panic(err)
	}
	return codexState{
		ManagedBlockHash:       hashText(block),
		OriginalScheduler:      originalScheduler,
		ProjectedSchedulerHash: codexSchedulerHash(projectedScheduler),
		ProjectionMode:         mode,
		WriterID:               ProjectionWriterID,
		TransactionID:          "test-transaction",
	}
}
