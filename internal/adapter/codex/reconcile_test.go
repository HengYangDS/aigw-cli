package codex

import (
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReconcileConfigsRestoresRemovedFullSelectionTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	original := []byte("model_provider = \"native\"\nmodel = \"gpt-native\"\nuser_setting = true\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	target := HomeTarget(path)
	if _, err := ReconcileConfigs(nil, []TargetRef{target}, atomicTestRuntime(), atomicTestRuntime(), false); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	withUserEdit := strings.Replace(string(projected), codexBegin, "after_projection = true\n"+codexBegin, 1)
	if err := os.WriteFile(path, []byte(withUserEdit), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileConfigs([]TargetRef{target}, nil, atomicTestRuntime(), atomicTestRuntime(), false); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`model_provider = "native"`,
		`model = "gpt-native"`,
		"user_setting = true",
		"after_projection = true",
	} {
		if !strings.Contains(string(restored), want) {
			t.Fatalf("restored config lacks %q:\n%s", want, restored)
		}
	}
	if strings.Contains(string(restored), "AIGW managed") {
		t.Fatalf("restored config still contains AIGW projection:\n%s", restored)
	}
	if _, err := os.Stat(codexStatePath(path)); !os.IsNotExist(err) {
		t.Fatalf("state remains after restore: %v", err)
	}
}

func TestReconcileConfigsCreatesAndRestoresAnAdmittedDefaultTarget(t *testing.T) {
	for _, test := range []struct {
		name        string
		userContent string
		wantConfig  bool
	}{
		{name: "empty pre-state returns to absence"},
		{name: "later user content survives withdrawal", userContent: "user_setting = true\n", wantConfig: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".codex", "config.toml")
			target := HomeTarget(path)
			target.CreateIfAbsent = true

			if _, err := ReconcileConfigs(nil, []TargetRef{target}, atomicTestRuntime(), atomicTestRuntime(), false); err != nil {
				t.Fatalf("create admitted default target: %v", err)
			}
			if test.userContent != "" {
				projected, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				projected = append([]byte(test.userContent), projected...)
				if err := os.WriteFile(path, projected, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := ReconcileConfigs([]TargetRef{target}, nil, atomicTestRuntime(), atomicTestRuntime(), false); err != nil {
				t.Fatalf("restore absent pre-state: %v", err)
			}
			if test.wantConfig {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if string(data) != test.userContent {
					t.Fatalf("restored config = %q, want %q", data, test.userContent)
				}
			} else if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("AIGW-created config remains at %s: %v", path, err)
			}
			for _, ownedPath := range []string{codexStatePath(path), codexCatalogPath(path)} {
				if _, err := os.Stat(ownedPath); !os.IsNotExist(err) {
					t.Fatalf("owned artifact remains at %s: %v", ownedPath, err)
				}
			}
		})
	}
}

func TestReconcileConfigsRollsBackMixedRestoreAndAdd(t *testing.T) {
	dir := t.TempDir()
	restoredPath := filepath.Join(dir, "restore.toml")
	addedPath := filepath.Join(dir, "add.toml")
	for _, path := range []string{restoredPath, addedPath} {
		if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before := []TargetRef{HomeTarget(restoredPath)}
	after := []TargetRef{HomeTarget(addedPath)}
	if _, err := ReconcileConfigs(nil, before, atomicTestRuntime(), atomicTestRuntime(), false); err != nil {
		t.Fatal(err)
	}
	paths := []string{
		restoredPath,
		codexStatePath(restoredPath),
		addedPath,
		codexStatePath(addedPath),
	}
	snapshots := map[string]platform.FileSnapshot{}
	for _, path := range paths {
		snapshot, err := platform.CaptureFileSnapshot(path)
		if err != nil {
			t.Fatal(err)
		}
		snapshots[path] = snapshot
	}

	originalWrite := writeFileAtomicIfUnchanged
	originalRemove := removeFileIfUnchanged
	defer func() {
		writeFileAtomicIfUnchanged = originalWrite
		removeFileIfUnchanged = originalRemove
	}()
	writes := 0
	failOnFourth := func() error {
		writes++
		if writes == 4 {
			return errors.New("injected fourth artifact failure")
		}
		return nil
	}
	writeFileAtomicIfUnchanged = func(path string, expected platform.FileSnapshot, data []byte, mode os.FileMode) (platform.FileSnapshot, error) {
		if err := failOnFourth(); err != nil {
			return platform.FileSnapshot{}, err
		}
		return originalWrite(path, expected, data, mode)
	}
	removeFileIfUnchanged = func(path string, expected platform.FileSnapshot) (platform.FileSnapshot, error) {
		if err := failOnFourth(); err != nil {
			return platform.FileSnapshot{}, err
		}
		return originalRemove(path, expected)
	}

	_, err := ReconcileConfigs(before, after, atomicTestRuntime(), atomicTestRuntime(), false)
	if err == nil || !strings.Contains(err.Error(), "injected fourth artifact failure") {
		t.Fatalf("ReconcileConfigs() error = %v", err)
	}
	for _, path := range paths {
		got, readErr := platform.CaptureFileSnapshot(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !reflect.DeepEqual(got, snapshots[path]) {
			t.Fatalf("%s after rollback = %#v, want %#v", path, got, snapshots[path])
		}
	}
}

func TestReconcileConfigsPreservesCommitAndCompensationFailures(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.toml")
	second := filepath.Join(dir, "second.toml")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	originalWrite := writeFileAtomicIfUnchanged
	originalRestore := restoreFileAtomicIfPostimage
	commitFailure := errors.New("injected commit failure")
	rollbackFailure := errors.New("injected rollback failure")
	t.Cleanup(func() {
		writeFileAtomicIfUnchanged = originalWrite
		restoreFileAtomicIfPostimage = originalRestore
	})
	writes := 0
	writeFileAtomicIfUnchanged = func(path string, expected platform.FileSnapshot, data []byte, mode os.FileMode) (platform.FileSnapshot, error) {
		writes++
		if writes == 2 {
			return platform.FileSnapshot{}, commitFailure
		}
		return originalWrite(path, expected, data, mode)
	}
	restoreFileAtomicIfPostimage = func(path string, preimage, postimage platform.FileSnapshot) error {
		return rollbackFailure
	}

	_, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(first), HomeTarget(second)}, atomicTestRuntime(), atomicTestRuntime(), false)
	if !errors.Is(err, commitFailure) || !errors.Is(err, rollbackFailure) {
		t.Fatalf("ReconcileConfigs() error = %v", err)
	}
}

func TestReconciliationReceiptPreservesForeignEditsAndRestoresOwnedFiles(t *testing.T) {
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "first.toml"), filepath.Join(dir, "second.toml")}
	original := []byte("model_provider = \"native\"\n")
	for _, path := range paths {
		if err := os.WriteFile(path, original, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	receipt, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(paths[0]), HomeTarget(paths[1])}, atomicTestRuntime(), atomicTestRuntime(), false)
	if err != nil {
		t.Fatal(err)
	}
	foreign := []byte("model_provider = \"user-selected\"\n")
	if err := os.WriteFile(paths[1], foreign, 0o600); err != nil {
		t.Fatal(err)
	}
	originalRestore := restoreFileAtomicIfPostimage
	t.Cleanup(func() { restoreFileAtomicIfPostimage = originalRestore })
	var conflict error
	restoreFileAtomicIfPostimage = func(path string, preimage, postimage platform.FileSnapshot) error {
		err := originalRestore(path, preimage, postimage)
		if err != nil {
			conflict = err
		}
		return err
	}
	err = receipt.Rollback()
	for i, path := range paths {
		want := original
		if i == 1 {
			want = foreign
		}
		if got, readErr := os.ReadFile(path); readErr != nil || !bytes.Equal(got, want) {
			t.Errorf("configuration %s = %q, %v; want %q", path, got, readErr, want)
		}
		if _, statErr := os.Stat(codexStatePath(path)); !os.IsNotExist(statErr) {
			t.Errorf("owned sidecar %s remains after compensation: %v", path, statErr)
		}
	}
	if conflict == nil || !errors.Is(err, conflict) {
		t.Fatalf("compensation must retain its actual conflict: %v; cause %v", err, conflict)
	}
}

func TestReconcileConfigsRejectsChangedPreimageWithoutOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalWrite := writeFileAtomicIfUnchanged
	defer func() { writeFileAtomicIfUnchanged = originalWrite }()
	mutated := false
	writeFileAtomicIfUnchanged = func(path string, expected platform.FileSnapshot, data []byte, mode os.FileMode) (platform.FileSnapshot, error) {
		if !mutated {
			mutated = true
			if err := os.WriteFile(path, []byte("newer external state\n"), 0o600); err != nil {
				return platform.FileSnapshot{}, err
			}
		}
		return originalWrite(path, expected, data, mode)
	}
	_, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(path)}, atomicTestRuntime(), atomicTestRuntime(), false)
	if err == nil || !strings.Contains(err.Error(), "preimage changed") {
		t.Fatalf("ReconcileConfigs() error = %v, want preimage changed", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || string(got) != "newer external state\n" {
		t.Fatalf("config after rejected commit = %q, %v", got, readErr)
	}
}

func TestRollbackCodexArtifactsPreservesExternalEdit(t *testing.T) {
	if err := rollbackCodexArtifacts(nil); err != nil {
		t.Fatalf("rollbackCodexArtifacts(nil) error = %v", err)
	}

	path := filepath.Join(t.TempDir(), "configuration.toml")
	writeCodexFixture(t, path, "original\n")
	before, err := platform.CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	post, err := platform.WriteFileAtomicIfUnchanged(path, before, []byte("transaction postimage\n"), 0o600)
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
	_, err := prepareCodexReconciliationTransition([]TargetRef{{Path: ""}}, nil, domain.Runtime{}, domain.Runtime{}, false)
	if err == nil {
		t.Error("expected error for invalid target")
	}
}

func TestPrepareCodexRestoreHandlesRestoredAndUnsupportedState(t *testing.T) {
	target := TargetRef{Path: "p"}
	configSnap := platform.FileSnapshot{Exists: true, Data: []byte("")}
	stateSnap := platform.FileSnapshot{Exists: false}

	plan, err := prepareCodexRestoreTransition(target, configSnap, stateSnap, platform.FileSnapshot{}, domain.Runtime{}, false)
	if err != nil || plan.plan.Action != ProjectionActionAlreadyRestored {
		t.Errorf("expected already-restored, got %+v, err %v", plan, err)
	}

	state := codexState{ProjectionMode: "invalid", WriterID: ProjectionWriterID, TransactionID: "t"}
	data, _ := json.Marshal(state)
	stateSnap = platform.FileSnapshot{Exists: true, Data: data}
	_, err = prepareCodexRestoreTransition(target, configSnap, stateSnap, platform.FileSnapshot{}, domain.Runtime{}, false)
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected unsupported error, got %v", err)
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

func attributedCodexStateFixture(block string) codexState {
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
		ProjectionMode:         ProjectionFullSelection,
		WriterID:               ProjectionWriterID,
		TransactionID:          "test-transaction",
	}
}

func TestCodexReconciliationPreflightErrors(t *testing.T) {
	t.Run("invalid after target", func(t *testing.T) {
		if _, err := planCodexReconciliationTransition(nil, []TargetRef{{}}, atomicTestRuntime(), atomicTestRuntime(), false); err == nil || !strings.Contains(err.Error(), "requires surface_id") {
			t.Fatalf("planCodexReconciliationTransition() error = %v", err)
		}
	})

	t.Run("endpoint missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeCodexFixture(t, path, "external = true\n")
		if _, err := planCodexReconciliationTransition(nil, []TargetRef{HomeTarget(path)}, domain.Runtime{RouteID: "missing-endpoint"}, domain.Runtime{RouteID: "missing-endpoint"}, false); err == nil || !strings.Contains(err.Error(), "no Codex endpoint") {
			t.Fatalf("planCodexReconciliationTransition() error = %v", err)
		}
	})

	t.Run("config missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.toml")
		if _, err := planCodexReconciliationTransition(nil, []TargetRef{HomeTarget(path)}, atomicTestRuntime(), atomicTestRuntime(), false); err == nil || !strings.Contains(err.Error(), "config does not exist") || !strings.Contains(err.Error(), "prepare Codex target") {
			t.Fatalf("planCodexReconciliationTransition() error = %v", err)
		}
	})

	t.Run("config is directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := planCodexReconciliationTransition(nil, []TargetRef{HomeTarget(path)}, atomicTestRuntime(), atomicTestRuntime(), false); err == nil || !strings.Contains(err.Error(), "read") {
			t.Fatalf("planCodexReconciliationTransition() error = %v", err)
		}
	})

	t.Run("sidecar is directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeCodexFixture(t, path, "external = true\n")
		if err := os.Mkdir(codexStatePath(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := planCodexReconciliationTransition(nil, []TargetRef{HomeTarget(path)}, atomicTestRuntime(), atomicTestRuntime(), false); err == nil || !strings.Contains(err.Error(), "prepare Codex target") {
			t.Fatalf("planCodexReconciliationTransition() error = %v", err)
		}
	})

	t.Run("desired surface must be validated before preparation", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeCodexFixture(t, path, "external = true\n")
		target := HomeTarget(path)
		target.SurfaceID = "foreign"
		if _, err := planCodexReconciliationTransition(nil, []TargetRef{target}, atomicTestRuntime(), atomicTestRuntime(), false); err == nil || !strings.Contains(err.Error(), "cannot use authority") {
			t.Fatalf("planCodexReconciliationTransition() error = %v", err)
		}
	})

	t.Run("symlink loop", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "loop.toml")
		if err := os.Symlink(path, path); err != nil {
			t.Fatal(err)
		}
		if _, err := normalizeCodexTargets([]TargetRef{HomeTarget(path)}); err == nil || !strings.Contains(err.Error(), "resolve Codex target symlinks") {
			t.Fatalf("normalizeCodexTargets() error = %v", err)
		}
	})
}
