package codex

import (
	configuration "aigw-cli/internal/configuration"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReconcileConfigsRejectsUnattributedSidecar(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := codexHomeTarget(path)
	if _, err := ReconcileConfigs(nil, []TargetRef{target}, atomicTestRuntime()); err != nil {
		t.Fatal(err)
	}
	statePath := codexStatePath(path)
	state := readCodexSidecar(t, path)

	state.ProjectionMode = ""
	state.WriterID = ""
	state.TransactionID = ""
	writeCodexStateFixture(t, path, state)
	beforeConfig, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	beforeState, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ReconcileConfigs(nil, []TargetRef{target}, atomicTestRuntime())
	if err == nil || !strings.Contains(err.Error(), "attribution is incomplete") {
		t.Fatalf("ReconcileConfigs() error = %v, want incomplete attribution", err)
	}
	afterConfig, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(afterConfig, beforeConfig) {
		t.Fatalf("config changed after unattributed sidecar rejection: %q, %v", afterConfig, readErr)
	}
	afterState, readErr := os.ReadFile(statePath)
	if readErr != nil || !bytes.Equal(afterState, beforeState) {
		t.Fatalf("sidecar changed after unattributed sidecar rejection: %q, %v", afterState, readErr)
	}

	for _, mutate := range []func(*codexState){
		func(state *codexState) { state.TransactionID = "" },
		func(state *codexState) { state.WriterID = "foreign-projector" },
	} {
		beforeConfig, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		beforeState, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(beforeState, &state); err != nil {
			t.Fatal(err)
		}
		mutate(&state)
		writeCodexStateFixture(t, path, state)
		currentState, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatal(err)
		}
		_, err = ReconcileConfigs(nil, []TargetRef{target}, atomicTestRuntime())
		if err == nil {
			t.Fatal("ReconcileConfigs() succeeded for invalid attribution")
		}
		afterConfig, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(afterConfig, beforeConfig) {
			t.Fatalf("config changed after attribution rejection: %q, %v", afterConfig, readErr)
		}
		afterState, readErr := os.ReadFile(statePath)
		if readErr != nil || !bytes.Equal(afterState, currentState) {
			t.Fatalf("state changed after attribution rejection: %q, %v", afterState, readErr)
		}
	}
}

func TestReconcileConfigsRejectsUnattributedSidecarBesideSymlinkTarget(t *testing.T) {
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	if err := os.MkdirAll(realDir, 0o700); err != nil {
		t.Fatal(err)
	}
	realPath := filepath.Join(realDir, "configuration.toml")
	aliasPath := filepath.Join(dir, "alias.toml")
	original := "model_provider = \"native\"\nmodel = \"gpt-native\"\n"
	if err := os.WriteFile(realPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realPath, aliasPath); err != nil {
		t.Fatal(err)
	}
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	projection, err := projectCodex(original, block, runtime.Model, "", configuration.ModelProviderAIGW)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(realPath, []byte(projection), 0o600); err != nil {
		t.Fatal(err)
	}
	unattributed := codexState{
		OriginalProvider: `model_provider = "native"`,
		OriginalModel:    `model = "gpt-native"`,
		ManagedBlockHash: hashText(block),
	}
	unattributedData, err := json.Marshal(unattributed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexStatePath(aliasPath), append(unattributedData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = ReconcileConfigs(nil, []TargetRef{codexHomeTarget(aliasPath)}, runtime)
	if err == nil || !strings.Contains(err.Error(), "attribution is incomplete") {
		t.Fatalf("ReconcileConfigs() error = %v, want incomplete attribution", err)
	}
	data, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(data), "[model_providers.aigw]"); count != 1 {
		t.Fatalf("managed provider tables changed after rejection: %d:\n%s", count, data)
	}
	if _, err := os.Stat(codexStatePath(realPath)); !os.IsNotExist(err) {
		t.Fatalf("unexpected canonical sidecar created beside real target: %v", err)
	}
	stateData, err := os.ReadFile(codexStatePath(aliasPath))
	if err != nil {
		t.Fatal(err)
	}
	var state codexState
	if err := json.Unmarshal(stateData, &state); err != nil {
		t.Fatal(err)
	}
	if state.WriterID != "" || state.ProjectionMode != "" || state.TransactionID != "" {
		t.Fatalf("unattributed symlink sidecar changed: %#v", state)
	}
}

func TestValidateConfigRejectsForeignSidecarAttribution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := atomicTestRuntime()
	if err := SyncConfig(path, runtime); err != nil {
		t.Fatal(err)
	}
	stateData, err := os.ReadFile(codexStatePath(path))
	if err != nil {
		t.Fatal(err)
	}
	var state codexState
	if err := json.Unmarshal(stateData, &state); err != nil {
		t.Fatal(err)
	}
	state.WriterID = "foreign-projector"
	mutated, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexStatePath(path), append(mutated, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	err = ValidateConfig(path, runtime)
	if err == nil || !strings.Contains(err.Error(), "foreign writer") {
		t.Fatalf("ValidateConfig() error = %v, want foreign writer", err)
	}
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

func TestReadProjectionIdentityErrors(t *testing.T) {
	t.Run("sidecar is directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		if err := os.Mkdir(codexStatePath(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadProjectionIdentity(path); err == nil || !strings.Contains(err.Error(), "read Codex adapter state") {
			t.Fatalf("ReadProjectionIdentity() error = %v", err)
		}
	})

	t.Run("invalid sidecar", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeCodexFixture(t, codexStatePath(path), "{")
		if _, err := ReadProjectionIdentity(path); err == nil || !strings.Contains(err.Error(), "parse Codex adapter state") {
			t.Fatalf("ReadProjectionIdentity() error = %v", err)
		}
	})

	t.Run("foreign attribution", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeCodexStateFixture(t, path, codexState{
			ProjectionMode: ProjectionFullSelection,
			WriterID:       "foreign",
			TransactionID:  "foreign-transaction",
		})
		if _, err := ReadProjectionIdentity(path); err == nil || !strings.Contains(err.Error(), "foreign writer") {
			t.Fatalf("ReadProjectionIdentity() error = %v", err)
		}
	})
}
