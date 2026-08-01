package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/transaction"
)

func TestCodexEndpointExtra(t *testing.T) {
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

func TestRestoreModelSelectionExtra(t *testing.T) {
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

func TestValidateConfigExtra(t *testing.T) {
	// Line 97: ValidateConfig (71.0%)
	root := t.TempDir()
	path := filepath.Join(root, "configuration.toml")

	// Missing endpoint
	err := ValidateConfig(path, configuration.Runtime{ProfileID: "p"})
	if err == nil || !strings.Contains(err.Error(), "no Codex endpoint") {
		t.Errorf("expected endpoint error, got %v", err)
	}

	runtime := configuration.Runtime{ProfileID: "p", Endpoint: "https://e.t"}
	// Missing file
	err = ValidateConfig(path, runtime)
	if err == nil || !strings.Contains(err.Error(), "read Codex config") {
		t.Errorf("expected read error, got %v", err)
	}

	// Missing state
	writeExtraCodexFile(t, path, "")
	err = ValidateConfig(path, runtime)
	if err == nil || !strings.Contains(err.Error(), "AIGW state is missing") {
		t.Errorf("expected missing state error, got %v", err)
	}

	// Hash mismatch
	state := codexState{
		ProjectionMode:   ProjectionFullSelection,
		WriterID:         ProjectionWriterID,
		TransactionID:    "t",
		ManagedBlockHash: "mismatch",
	}
	writeExtraCodexState(t, path, state)

	block := codexManagedBlock(runtime, runtime.Endpoint)
	content := "model_provider = \"aigw\" # managed by AIGW\n" + codexBegin + "\n" + block
	writeExtraCodexFile(t, path, content)

	err = ValidateConfig(path, runtime)
	if err == nil || !strings.Contains(err.Error(), "state does not match") {
		t.Errorf("expected hash mismatch error, got %v", err)
	}
}

func TestIsExactTruncatedCodexProjectionExtra(t *testing.T) {
	// Line 85: isExactTruncatedCodexProjection (80.0%)
	if isExactTruncatedCodexProjection("too short", nil, configuration.Runtime{}, "") {
		t.Error("expected false for short input")
	}
}

func TestInspectConfigScenarios(t *testing.T) {
	root := t.TempDir()

	t.Run("invalid sidecar json", func(t *testing.T) {
		path := filepath.Join(root, "invalid-sidecar.toml")
		writeExtraCodexFile(t, path, "model_provider = \"native\"")
		writeExtraCodexFile(t, codexStatePath(path), "invalid json")

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
		writeExtraCodexFile(t, path, codexBegin)

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
		writeExtraCodexFile(t, path, "")
		state := codexState{WriterID: "foreign"}
		writeExtraCodexState(t, path, state)

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
		writeExtraCodexFile(t, path, "")
		state := codexState{
			ProjectionMode: ProjectionFullSelection,
			WriterID:       ProjectionWriterID,
			TransactionID:  "some-tx",
		}
		writeExtraCodexState(t, path, state)

		ins, err := InspectConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if ins.State != "stale-sidecar" {
			t.Errorf("got state %q, want stale-sidecar", ins.State)
		}
	})
}

func TestReadProjectionIdentityExtra(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "identity.toml")

	// Not exist
	id, err := ReadProjectionIdentity(path)
	if err != nil || id.Present {
		t.Errorf("expected not present, got %+v, err %v", id, err)
	}

	// Legacy
	writeExtraCodexFile(t, path, "")
	state := codexState{}
	writeExtraCodexState(t, path, state)
	id, err = ReadProjectionIdentity(path)
	if err != nil || id.AttributionState != "legacy" {
		t.Errorf("expected legacy, got %+v, err %v", id, err)
	}
}

func TestCanonicalCodexTargetPathExtra(t *testing.T) {
	// Line 516: canonicalCodexTargetPath (55.6%)
	root := t.TempDir()
	path := filepath.Join(root, "target.toml")

	got, err := canonicalCodexTargetPath(path)
	if err != nil || !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q, err %v", got, err)
	}

	// Symlink
	link := filepath.Join(root, "link.toml")
	writeExtraCodexFile(t, path, "")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}

	path, _ = canonicalCodexTargetPath(path)
	got, err = canonicalCodexTargetPath(link)
	if err != nil || got != path {
		t.Errorf("expected resolved symlink %q, got %q, err %v", path, got, err)
	}
}

func TestLoginPlanExtra(t *testing.T) {
	// Line 45: LoginPlan (77.8%)
	_, err := LoginPlan("", "home", "tok")
	if err == nil || err.Error() != "Codex executable is not configured" {
		t.Errorf("expected executable error, got %v", err)
	}
	_, err = LoginPlan("bin", "home", "")
	if err == nil || err.Error() != "Codex token is empty" {
		t.Errorf("expected token error, got %v", err)
	}
	plan, err := LoginPlan("bin", "home", "tok")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range plan.Env {
		if e == "CODEX_HOME=home" {
			found = true
			break
		}
	}
	if !found {
		t.Error("CODEX_HOME not found in env")
	}
}

func TestValidateDesiredCodexTargetExtra(t *testing.T) {
	// Line 561: validateDesiredCodexTarget (75.0%)
	err := validateDesiredCodexTarget(TargetRef{SurfaceID: "invalid"})
	if err == nil {
		t.Error("expected error for invalid surface")
	}
}

func TestTargetCodexStatePathExtra(t *testing.T) {
	// Line 554: targetCodexStatePath (66.7%)
	ref := TargetRef{Path: "p"}
	if got := targetCodexStatePath(ref); got != "p.aigw-state.json" {
		t.Errorf("got %q", got)
	}
	ref.statePath = "s"
	if got := targetCodexStatePath(ref); got != "s" {
		t.Errorf("got %q", got)
	}
}

func TestClassifyCodexDiskSelectionExtra(t *testing.T) {
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

func TestRollbackCodexArtifactsExtra(t *testing.T) {
	if err := rollbackCodexArtifacts(nil); err != nil {
		t.Fatalf("rollbackCodexArtifacts(nil) error = %v", err)
	}

	path := filepath.Join(t.TempDir(), "configuration.toml")
	writeExtraCodexFile(t, path, "original\n")
	before, err := transaction.CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	post, err := transaction.WriteFileAtomicIfUnchanged(path, before, []byte("transaction postimage\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	writeExtraCodexFile(t, path, "newer external edit\n")

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

func TestPrepareCodexReconciliationExtra(t *testing.T) {
	// Target union error
	_, _, err := prepareCodexReconciliation([]TargetRef{{Path: ""}}, nil, configuration.Runtime{})
	if err == nil {
		t.Error("expected error for invalid target")
	}
}

func TestValidateCodexStateAttributionExtra(t *testing.T) {
	// Line 440: validateCodexStateAttribution (84.6%)
	cases := []struct {
		state codexState
		mode  string
	}{
		{codexState{ProjectionMode: "m"}, ""}, // incomplete
		{codexState{ProjectionMode: "invalid", WriterID: "w", TransactionID: "t"}, ""},
		{codexState{ProjectionMode: ProjectionFullSelection, WriterID: "other", TransactionID: "t"}, ""},
	}
	for _, c := range cases {
		_, err := validateCodexStateAttribution(c.state, c.mode)
		if err == nil {
			t.Errorf("expected error for state %+v, mode %q", c.state, c.mode)
		}
	}
}

func TestPrepareCodexRestoreExtra(t *testing.T) {
	// Line 311: prepareCodexRestore (76.9%)
	target := TargetRef{Path: "p", ProjectionMode: ProjectionFullSelection}
	configSnap := transaction.FileSnapshot{Exists: true, Data: []byte("")}
	stateSnap := transaction.FileSnapshot{Exists: false}

	// Already restored
	plan, err := prepareCodexRestore(target, configSnap, stateSnap)
	if err != nil || plan.plan.Action != "already-restored" {
		t.Errorf("expected already-restored, got %+v, err %v", plan, err)
	}

	// Unsupported mode in sidecar
	state := codexState{ProjectionMode: "invalid", WriterID: ProjectionWriterID, TransactionID: "t"}
	data, _ := json.Marshal(state)
	stateSnap = transaction.FileSnapshot{Exists: true, Data: data}
	_, err = prepareCodexRestore(target, configSnap, stateSnap)
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected unsupported error, got %v", err)
	}
}

func TestNormalizeCodexTargetsExtra(t *testing.T) {
	// Line 489: normalizeCodexTargets (85.0%)
	// Missing field
	_, err := normalizeCodexTargets([]TargetRef{{Path: "p"}})
	if err == nil {
		t.Error("expected error for missing fields")
	}

	// Duplicates
	target := TargetRef{Path: "p", SurfaceID: "s", Authority: "a", ProjectionMode: "m"}
	_, err = normalizeCodexTargets([]TargetRef{target, target})
	if err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Errorf("expected duplicate error, got %v", err)
	}
}

func writeExtraCodexFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeExtraCodexState(t *testing.T, path string, state codexState) {
	t.Helper()
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	writeExtraCodexFile(t, codexStatePath(path), string(data)+"\n")
}

func attributedExtraCodexState(mode, block string) codexState {
	return codexState{
		ManagedBlockHash: hashText(block),
		ProjectionMode:   mode,
		WriterID:         ProjectionWriterID,
		TransactionID:    "test-transaction",
	}
}

func TestInspectConfigReachableStates(t *testing.T) {
	t.Run("missing config", func(t *testing.T) {
		inspection, err := InspectConfig(filepath.Join(t.TempDir(), "missing.toml"))
		if err != nil {
			t.Fatal(err)
		}
		if inspection.State != "missing" || inspection.DiskSelection != "not-present" {
			t.Fatalf("inspection = %#v", inspection)
		}
	})

	t.Run("config is directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := InspectConfig(path); err == nil || !strings.Contains(err.Error(), "read Codex config") {
			t.Fatalf("InspectConfig() error = %v", err)
		}
	})

	t.Run("sidecar is directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeExtraCodexFile(t, path, "model_provider = \"native\"\n")
		if err := os.Mkdir(codexStatePath(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := InspectConfig(path); err == nil || !strings.Contains(err.Error(), "read Codex sidecar") {
			t.Fatalf("InspectConfig() error = %v", err)
		}
	})

	t.Run("legacy full selection", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		runtime := atomicTestRuntime()
		block := codexManagedBlock(runtime, runtime.Endpoint)
		writeExtraCodexFile(t, path, projectCodex("model_provider = \"native\"\n", block, ""))
		writeExtraCodexState(t, path, codexState{ManagedBlockHash: hashText(block)})

		inspection, err := InspectConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if inspection.State != "legacy-full-selection" || inspection.ProjectionMode != ProjectionFullSelection || inspection.AttributionState != "legacy" || !inspection.AIGWManaged || !inspection.SidecarHashMatches {
			t.Fatalf("inspection = %#v", inspection)
		}
	})

	t.Run("full selection disk drift", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		runtime := atomicTestRuntime()
		block := codexManagedBlock(runtime, runtime.Endpoint)
		writeExtraCodexFile(t, path, "model_provider = \"native\"\n"+codexBegin+"\n"+block)
		writeExtraCodexState(t, path, attributedExtraCodexState(ProjectionFullSelection, block))

		inspection, err := InspectConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if inspection.State != "aigw-drift" || !inspection.AIGWManaged || !inspection.SidecarHashMatches {
			t.Fatalf("inspection = %#v", inspection)
		}
	})

}

func TestValidateConfigReachableErrors(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	validState := attributedExtraCodexState(ProjectionFullSelection, block)

	t.Run("sidecar is directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeExtraCodexFile(t, path, projectCodex("", block, runtime.Model))
		if err := os.Mkdir(codexStatePath(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := ValidateConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "read Codex adapter state") {
			t.Fatalf("ValidateConfig() error = %v", err)
		}
	})

	t.Run("invalid sidecar", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeExtraCodexFile(t, path, projectCodex("", block, runtime.Model))
		writeExtraCodexFile(t, codexStatePath(path), "{")
		if err := ValidateConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "parse Codex adapter state") {
			t.Fatalf("ValidateConfig() error = %v", err)
		}
	})

	t.Run("managed block missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeExtraCodexFile(t, path, fmt.Sprintf("model = %q # managed by AIGW\n%s\n", runtime.Model, codexSelection))
		writeExtraCodexState(t, path, validState)
		if err := ValidateConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "provider block is missing") {
			t.Fatalf("ValidateConfig() error = %v", err)
		}
	})

	t.Run("provider profile mismatch", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		otherBlock := codexManagedBlock(runtime, "https://other.example/v1")
		writeExtraCodexFile(t, path, projectCodex("", otherBlock, runtime.Model))
		writeExtraCodexState(t, path, attributedExtraCodexState(ProjectionFullSelection, otherBlock))
		if err := ValidateConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "provider block does not match") {
			t.Fatalf("ValidateConfig() error = %v", err)
		}
	})

	t.Run("provider selection mismatch", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeExtraCodexFile(t, path, "model_provider = \"native\"\n"+codexBegin+"\n"+block)
		writeExtraCodexState(t, path, validState)
		if err := ValidateConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "provider selection does not match") {
			t.Fatalf("ValidateConfig() error = %v", err)
		}
	})
}

func TestCodexUserConfigAtReadErrors(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)

	t.Run("invalid sidecar", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "configuration.toml")
		statePath := filepath.Join(root, "state.json")
		writeExtraCodexFile(t, path, "external = true\n")
		writeExtraCodexFile(t, statePath, "{")
		if _, _, err := codexUserConfigAt(path, statePath, runtime, block); err == nil || !strings.Contains(err.Error(), "parse Codex adapter state") {
			t.Fatalf("codexUserConfigAt() error = %v", err)
		}
	})

	t.Run("config is directory with sidecar", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "configuration.toml")
		statePath := filepath.Join(root, "state.json")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		writeExtraCodexFile(t, statePath, "{}")
		if _, _, err := codexUserConfigAt(path, statePath, runtime, block); err == nil || !strings.Contains(err.Error(), "read Codex config") {
			t.Fatalf("codexUserConfigAt() error = %v", err)
		}
	})

	t.Run("sidecar is directory", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "configuration.toml")
		statePath := filepath.Join(root, "state.json")
		writeExtraCodexFile(t, path, "external = true\n")
		if err := os.Mkdir(statePath, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, _, err := codexUserConfigAt(path, statePath, runtime, block); err == nil || !strings.Contains(err.Error(), "read Codex adapter state") {
			t.Fatalf("codexUserConfigAt() error = %v", err)
		}
	})

	t.Run("missing config without sidecar", func(t *testing.T) {
		root := t.TempDir()
		if _, _, err := codexUserConfigAt(filepath.Join(root, "missing.toml"), filepath.Join(root, "missing-state.json"), runtime, block); err == nil || !strings.Contains(err.Error(), "read Codex config") {
			t.Fatalf("codexUserConfigAt() error = %v", err)
		}
	})
}

func TestCompleteExactTruncatedCodexProjectionRejectsAmbiguities(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	state := codexState{ManagedBlockHash: hashText(block)}
	truncated := strings.TrimSuffix(block, codexEnd+"\n")

	for _, test := range []struct {
		name    string
		current string
		state   codexState
	}{
		{
			name:    "provider table missing",
			current: codexSelection + "\n" + codexBegin + "\n",
			state:   state,
		},
		{
			name:    "truncated bytes mismatch",
			current: codexSelection + "\n" + codexBegin + "\n[model_providers.aigw]\nchanged = true\n",
			state:   state,
		},
		{
			name:    "foreign content before next table",
			current: codexSelection + "\n" + codexBegin + "\n" + truncated + "foreign = true\n[other]\n",
			state:   state,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if completed, ok := completeExactTruncatedCodexProjection(test.current, test.state, runtime, block); ok {
				t.Fatalf("completeExactTruncatedCodexProjection() admitted:\n%s", completed)
			}
		})
	}
}

func TestRemoveCodexProjectionRestoresAbsentProvider(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	current := projectCodex("external = true\n", block, runtime.Model)
	state := codexState{
		OriginalModel:    `model = "native-model"`,
		ManagedBlockHash: hashText(block),
	}

	restored, err := removeCodexProjection(current, state)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(restored, "model_provider") || !strings.Contains(restored, `model = "native-model"`) || !strings.Contains(restored, "external = true") {
		t.Fatalf("restored config = %q", restored)
	}
	if got := removeCodexBeginMarker("external = true\n"); got != "external = true\n" {
		t.Fatalf("removeCodexBeginMarker() = %q", got)
	}
}

func TestRemoveCodexProjectionRejectsChangedManagedBlock(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	current := projectCodex("external = true\n", block, runtime.Model)
	state := codexState{ManagedBlockHash: hashText(block)}
	current = strings.Replace(current, runtime.Endpoint, "https://changed.example/v1", 1)

	if _, err := removeCodexProjection(current, state); err == nil || !strings.Contains(err.Error(), "provider block changed") {
		t.Fatalf("removeCodexProjection() error = %v", err)
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
		writeExtraCodexFile(t, codexStatePath(path), "{")
		if _, err := ReadProjectionIdentity(path); err == nil || !strings.Contains(err.Error(), "parse Codex adapter state") {
			t.Fatalf("ReadProjectionIdentity() error = %v", err)
		}
	})

	t.Run("foreign attribution", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeExtraCodexState(t, path, codexState{
			ProjectionMode: ProjectionFullSelection,
			WriterID:       "foreign",
			TransactionID:  "foreign-transaction",
		})
		if _, err := ReadProjectionIdentity(path); err == nil || !strings.Contains(err.Error(), "foreign writer") {
			t.Fatalf("ReadProjectionIdentity() error = %v", err)
		}
	})
}

func TestCodexReconciliationPreflightErrors(t *testing.T) {
	t.Run("invalid after target", func(t *testing.T) {
		if _, err := PlanReconciliation(nil, []TargetRef{{}}, atomicTestRuntime()); err == nil || !strings.Contains(err.Error(), "requires surface_id") {
			t.Fatalf("PlanReconciliation() error = %v", err)
		}
	})

	t.Run("endpoint missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeExtraCodexFile(t, path, "external = true\n")
		if _, err := PlanReconciliation(nil, []TargetRef{standaloneCodexTarget(path)}, configuration.Runtime{ProfileID: "missing-endpoint"}); err == nil || !strings.Contains(err.Error(), "no Codex endpoint") {
			t.Fatalf("PlanReconciliation() error = %v", err)
		}
	})

	t.Run("config missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.toml")
		if _, err := PlanReconciliation(nil, []TargetRef{standaloneCodexTarget(path)}, atomicTestRuntime()); err == nil || !strings.Contains(err.Error(), "config does not exist") || !strings.Contains(err.Error(), "prepare Codex target") {
			t.Fatalf("PlanReconciliation() error = %v", err)
		}
	})

	t.Run("sidecar is directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeExtraCodexFile(t, path, "external = true\n")
		if err := os.Mkdir(codexStatePath(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := PlanReconciliation(nil, []TargetRef{standaloneCodexTarget(path)}, atomicTestRuntime()); err == nil || !strings.Contains(err.Error(), "prepare Codex target") {
			t.Fatalf("PlanReconciliation() error = %v", err)
		}
	})

	t.Run("unsupported desired mode", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeExtraCodexFile(t, path, "external = true\n")
		target := codexReconciliationTarget{ref: TargetRef{Path: path, ProjectionMode: "unsupported"}, desired: true}
		if _, err := prepareCodexReconciliationTarget(target, atomicTestRuntime(), atomicTestRuntime().Endpoint, "transaction"); err == nil || !strings.Contains(err.Error(), "unsupported Codex projection mode") {
			t.Fatalf("prepareCodexReconciliationTarget() error = %v", err)
		}
	})

	t.Run("invalid desired authority", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeExtraCodexFile(t, path, "external = true\n")
		target := standaloneCodexTarget(path)
		target.Authority = "foreign"
		if _, err := PlanReconciliation(nil, []TargetRef{target}, atomicTestRuntime()); err == nil || !strings.Contains(err.Error(), "cannot use authority") {
			t.Fatalf("PlanReconciliation() error = %v", err)
		}
	})

	t.Run("symlink loop", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "loop.toml")
		if err := os.Symlink(path, path); err != nil {
			t.Fatal(err)
		}
		if _, err := normalizeCodexTargets([]TargetRef{standaloneCodexTarget(path)}); err == nil || !strings.Contains(err.Error(), "resolve Codex target symlinks") {
			t.Fatalf("normalizeCodexTargets() error = %v", err)
		}
	})
}
