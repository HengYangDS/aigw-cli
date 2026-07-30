package adapters

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func TestCodexEndpointExtra(t *testing.T) {
	cases := []struct {
		name     string
		runtime  domain.Runtime
		expected string
		wantErr  bool
	}{
		{"missing", domain.Runtime{ProfileID: "p"}, "", true},
		{"valid", domain.Runtime{ProfileID: "p", Endpoint: "https://example.com/"}, "https://example.com/", false},
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

func TestValidateCodexConfigExtra(t *testing.T) {
	// Line 97: ValidateCodexConfig (71.0%)
	root := t.TempDir()
	path := filepath.Join(root, "config.toml")

	// Missing endpoint
	err := ValidateCodexConfig(path, domain.Runtime{ProfileID: "p"})
	if err == nil || !strings.Contains(err.Error(), "no Codex endpoint") {
		t.Errorf("expected endpoint error, got %v", err)
	}

	runtime := domain.Runtime{ProfileID: "p", Endpoint: "https://e.t"}
	// Missing file
	err = ValidateCodexConfig(path, runtime)
	if err == nil || !strings.Contains(err.Error(), "read Codex config") {
		t.Errorf("expected read error, got %v", err)
	}

	// Missing state
	writeExtraCodexFile(t, path, "")
	err = ValidateCodexConfig(path, runtime)
	if err == nil || !strings.Contains(err.Error(), "AIGW state is missing") {
		t.Errorf("expected missing state error, got %v", err)
	}

	// Hash mismatch
	state := codexState{
		ProjectionMode:   CodexProjectionFullSelection,
		WriterID:         CodexProjectionWriterID,
		TransactionID:    "t",
		ManagedBlockHash: "mismatch",
	}
	writeExtraCodexState(t, path, state)

	block := codexManagedBlock(runtime, runtime.Endpoint)
	content := "model_provider = \"aigw\" # managed by AIGW\n" + codexBegin + "\n" + block
	writeExtraCodexFile(t, path, content)

	err = ValidateCodexConfig(path, runtime)
	if err == nil || !strings.Contains(err.Error(), "state does not match") {
		t.Errorf("expected hash mismatch error, got %v", err)
	}
}

func TestPrepareCodexFallbackExtra(t *testing.T) {
	// Line 265: prepareCodexFallback (74.2%)
	root := t.TempDir()
	path := filepath.Join(root, "fallback.toml")

	// Air selects AIGW at top level
	configSnap := transaction.FileSnapshot{Exists: true, Data: []byte("model_provider = \"aigw\" # managed by AIGW\n")}
	stateSnap := transaction.FileSnapshot{Exists: false}
	_, err := prepareCodexFallback(CodexTargetRef{Path: path}, "block", configSnap, stateSnap, "tx")
	if err == nil || !strings.Contains(err.Error(), "selects AIGW at top level") {
		t.Errorf("expected top level error, got %v", err)
	}

	// Unowned fallback block
	configSnap = transaction.FileSnapshot{Exists: true, Data: []byte(codexFallbackBegin)}
	_, err = prepareCodexFallback(CodexTargetRef{Path: path}, "block", configSnap, stateSnap, "tx")
	if err == nil || !strings.Contains(err.Error(), "unowned AIGW fallback block") {
		t.Errorf("expected unowned block error, got %v", err)
	}
}

func TestIsExactTruncatedCodexProjectionExtra(t *testing.T) {
	// Line 85: isExactTruncatedCodexProjection (80.0%)
	if isExactTruncatedCodexProjection("too short", nil, domain.Runtime{}, "") {
		t.Error("expected false for short input")
	}
}

func TestInspectCodexConfigScenarios(t *testing.T) {
	root := t.TempDir()

	t.Run("invalid sidecar json", func(t *testing.T) {
		path := filepath.Join(root, "invalid-sidecar.toml")
		writeExtraCodexFile(t, path, "model_provider = \"native\"")
		writeExtraCodexFile(t, codexStatePath(path), "invalid json")

		ins, err := InspectCodexConfig(path)
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

		ins, err := InspectCodexConfig(path)
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

		ins, err := InspectCodexConfig(path)
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
			ProjectionMode: CodexProjectionFullSelection,
			WriterID:       CodexProjectionWriterID,
			TransactionID:  "some-tx",
		}
		writeExtraCodexState(t, path, state)

		ins, err := InspectCodexConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if ins.State != "stale-sidecar" {
			t.Errorf("got state %q, want stale-sidecar", ins.State)
		}
	})
}

func TestReadCodexProjectionIdentityExtra(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "identity.toml")

	// Not exist
	id, err := ReadCodexProjectionIdentity(path)
	if err != nil || id.Present {
		t.Errorf("expected not present, got %+v, err %v", id, err)
	}

	// Legacy
	writeExtraCodexFile(t, path, "")
	state := codexState{}
	writeExtraCodexState(t, path, state)
	id, err = ReadCodexProjectionIdentity(path)
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

func TestCodexLoginPlanExtra(t *testing.T) {
	// Line 45: CodexLoginPlan (77.8%)
	_, err := CodexLoginPlan("", "home", "tok")
	if err == nil || err.Error() != "Codex executable is not configured" {
		t.Errorf("expected executable error, got %v", err)
	}
	_, err = CodexLoginPlan("bin", "home", "")
	if err == nil || err.Error() != "Codex token is empty" {
		t.Errorf("expected token error, got %v", err)
	}
	plan, err := CodexLoginPlan("bin", "home", "tok")
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
	err := validateDesiredCodexTarget(CodexTargetRef{SurfaceID: "invalid"})
	if err == nil {
		t.Error("expected error for invalid surface")
	}
}

func TestTargetCodexStatePathExtra(t *testing.T) {
	// Line 554: targetCodexStatePath (66.7%)
	ref := CodexTargetRef{Path: "p"}
	if got := targetCodexStatePath(ref); got != "p.aigw-state.json" {
		t.Errorf("got %q", got)
	}
	ref.statePath = "s"
	if got := targetCodexStatePath(ref); got != "s" {
		t.Errorf("got %q", got)
	}
}

func TestStaleAirFullSelectionBlockExtra(t *testing.T) {
	// Line 309: staleAirFullSelectionBlock (81.0%)

	// Wrong projection mode
	_, err := staleAirFullSelectionBlock("", codexState{ProjectionMode: CodexProjectionFullSelection})
	if err == nil || !strings.Contains(err.Error(), "projection mode is") {
		t.Errorf("expected mode error, got %v", err)
	}

	// Retains original selection
	_, err = staleAirFullSelectionBlock("", codexState{ProjectionMode: CodexProjectionNamespacedFallback, OriginalProvider: "p"})
	if err == nil || !strings.Contains(err.Error(), "retains an original selection") {
		t.Errorf("expected original selection error, got %v", err)
	}
}

func TestProjectCodexFallbackExtra(t *testing.T) {
	// Line 593: projectCodexFallback (75.0%)
	got, prefix := projectCodexFallback("orig", "block")
	if got != "orig\nblock" || prefix != "\n" {
		t.Errorf("got %q, prefix %q", got, prefix)
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

	path := filepath.Join(t.TempDir(), "config.toml")
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
	_, _, err := prepareCodexReconciliation([]CodexTargetRef{{Path: ""}}, nil, domain.Runtime{})
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
		{codexState{}, CodexProjectionNamespacedFallback}, // legacy for fallback
		{codexState{ProjectionMode: "m"}, ""},             // incomplete
		{codexState{ProjectionMode: "invalid", WriterID: "w", TransactionID: "t"}, ""},
		{codexState{ProjectionMode: CodexProjectionFullSelection, WriterID: "other", TransactionID: "t"}, ""},
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
	target := CodexTargetRef{Path: "p", ProjectionMode: CodexProjectionFullSelection}
	configSnap := transaction.FileSnapshot{Exists: true, Data: []byte("")}
	stateSnap := transaction.FileSnapshot{Exists: false}

	// Already restored
	plan, err := prepareCodexRestore(target, configSnap, stateSnap)
	if err != nil || plan.plan.Action != "already-restored" {
		t.Errorf("expected already-restored, got %+v, err %v", plan, err)
	}

	// Unsupported mode in sidecar
	state := codexState{ProjectionMode: "invalid", WriterID: CodexProjectionWriterID, TransactionID: "t"}
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
	_, err := normalizeCodexTargets([]CodexTargetRef{{Path: "p"}})
	if err == nil {
		t.Error("expected error for missing fields")
	}

	// Duplicates
	target := CodexTargetRef{Path: "p", SurfaceID: "s", Authority: "a", ProjectionMode: "m"}
	_, err = normalizeCodexTargets([]CodexTargetRef{target, target})
	if err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Errorf("expected duplicate error, got %v", err)
	}
}

func TestRemoveCodexFallbackProjectionExtra(t *testing.T) {
	// Line 612: removeCodexFallbackProjection (61.9%)
	state := codexState{ManagedBlockHash: hashText(codexFallbackBegin + "\n" + codexFallbackEnd + "\n")}

	// Air top level selects AIGW
	_, err := removeCodexFallbackProjection("model_provider = \"aigw\" # managed by AIGW\n", state)
	if err == nil || !strings.Contains(err.Error(), "selects AIGW at top level") {
		t.Errorf("expected top level error, got %v", err)
	}

	// Missing fallback block
	_, err = removeCodexFallbackProjection("", state)
	if err == nil || !strings.Contains(err.Error(), "fallback block is missing") {
		t.Errorf("expected missing block error, got %v", err)
	}

	// Block changed
	current := codexFallbackBegin + "\n" + "changed\n" + codexFallbackEnd + "\n"
	_, err = removeCodexFallbackProjection(current, state)
	if err == nil || !strings.Contains(err.Error(), "fallback block changed") {
		t.Errorf("expected block changed error, got %v", err)
	}

	// Separator changed
	state.FallbackPrefix = "\n\n"
	current = "base\n" + codexFallbackBegin + "\n" + codexFallbackEnd + "\n"
	_, err = removeCodexFallbackProjection(current, state)
	if err == nil || !strings.Contains(err.Error(), "fallback separator changed") {
		t.Errorf("expected separator error, got %v", err)
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
		WriterID:         CodexProjectionWriterID,
		TransactionID:    "test-transaction",
	}
}

func TestInspectCodexConfigReachableStates(t *testing.T) {
	t.Run("missing config", func(t *testing.T) {
		inspection, err := InspectCodexConfig(filepath.Join(t.TempDir(), "missing.toml"))
		if err != nil {
			t.Fatal(err)
		}
		if inspection.State != "missing" || inspection.DiskSelection != "not-present" {
			t.Fatalf("inspection = %#v", inspection)
		}
	})

	t.Run("config is directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := InspectCodexConfig(path); err == nil || !strings.Contains(err.Error(), "read Codex config") {
			t.Fatalf("InspectCodexConfig() error = %v", err)
		}
	})

	t.Run("sidecar is directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		writeExtraCodexFile(t, path, "model_provider = \"native\"\n")
		if err := os.Mkdir(codexStatePath(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := InspectCodexConfig(path); err == nil || !strings.Contains(err.Error(), "read Codex sidecar") {
			t.Fatalf("InspectCodexConfig() error = %v", err)
		}
	})

	t.Run("legacy full selection", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		runtime := atomicTestRuntime()
		block := codexManagedBlock(runtime, runtime.Endpoint)
		writeExtraCodexFile(t, path, projectCodex("model_provider = \"native\"\n", block, ""))
		writeExtraCodexState(t, path, codexState{ManagedBlockHash: hashText(block)})

		inspection, err := InspectCodexConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if inspection.State != "legacy-full-selection" || inspection.ProjectionMode != CodexProjectionFullSelection || inspection.AttributionState != "legacy" || !inspection.AIGWManaged || !inspection.SidecarHashMatches {
			t.Fatalf("inspection = %#v", inspection)
		}
	})

	t.Run("full selection disk drift", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		runtime := atomicTestRuntime()
		block := codexManagedBlock(runtime, runtime.Endpoint)
		writeExtraCodexFile(t, path, "model_provider = \"native\"\n"+codexBegin+"\n"+block)
		writeExtraCodexState(t, path, attributedExtraCodexState(CodexProjectionFullSelection, block))

		inspection, err := InspectCodexConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if inspection.State != "aigw-drift" || !inspection.AIGWManaged || !inspection.SidecarHashMatches {
			t.Fatalf("inspection = %#v", inspection)
		}
	})

	t.Run("selected fallback conflict", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		runtime := atomicTestRuntime()
		block := codexFallbackBlock(runtime, runtime.Endpoint)
		writeExtraCodexFile(t, path, "model_provider = \"aigw_fallback\"\n"+block)
		writeExtraCodexState(t, path, attributedExtraCodexState(CodexProjectionNamespacedFallback, block))

		inspection, err := InspectCodexConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if inspection.State != "fallback-selected-conflict" || !inspection.AIGWManaged || !inspection.SidecarHashMatches {
			t.Fatalf("inspection = %#v", inspection)
		}
	})
}

func TestCodexFallbackBlockInBoundaries(t *testing.T) {
	for _, test := range []struct {
		name    string
		current string
		want    string
	}{
		{name: "missing", current: "external = true\n"},
		{name: "incomplete", current: codexFallbackBegin + "\nprovider = true\n"},
		{
			name:    "CRLF terminator",
			current: codexFallbackBegin + "\r\nprovider = true\r\n" + codexFallbackEnd + "\r\nexternal = true\r\n",
			want:    codexFallbackBegin + "\r\nprovider = true\r\n" + codexFallbackEnd + "\r\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := codexFallbackBlockIn(test.current)
			if test.want == "" {
				if err == nil {
					t.Fatalf("codexFallbackBlockIn() = %q, want error", got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("codexFallbackBlockIn() = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestValidateCodexConfigReachableErrors(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	validState := attributedExtraCodexState(CodexProjectionFullSelection, block)

	t.Run("sidecar is directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		writeExtraCodexFile(t, path, projectCodex("", block, runtime.Model))
		if err := os.Mkdir(codexStatePath(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := ValidateCodexConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "read Codex adapter state") {
			t.Fatalf("ValidateCodexConfig() error = %v", err)
		}
	})

	t.Run("invalid sidecar", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		writeExtraCodexFile(t, path, projectCodex("", block, runtime.Model))
		writeExtraCodexFile(t, codexStatePath(path), "{")
		if err := ValidateCodexConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "parse Codex adapter state") {
			t.Fatalf("ValidateCodexConfig() error = %v", err)
		}
	})

	t.Run("managed block missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		writeExtraCodexFile(t, path, fmt.Sprintf("model = %q # managed by AIGW\n%s\n", runtime.Model, codexSelection))
		writeExtraCodexState(t, path, validState)
		if err := ValidateCodexConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "provider block is missing") {
			t.Fatalf("ValidateCodexConfig() error = %v", err)
		}
	})

	t.Run("provider profile mismatch", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		otherBlock := codexManagedBlock(runtime, "https://other.example/v1")
		writeExtraCodexFile(t, path, projectCodex("", otherBlock, runtime.Model))
		writeExtraCodexState(t, path, attributedExtraCodexState(CodexProjectionFullSelection, otherBlock))
		if err := ValidateCodexConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "provider block does not match") {
			t.Fatalf("ValidateCodexConfig() error = %v", err)
		}
	})

	t.Run("provider selection mismatch", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		writeExtraCodexFile(t, path, "model_provider = \"native\"\n"+codexBegin+"\n"+block)
		writeExtraCodexState(t, path, validState)
		if err := ValidateCodexConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "provider selection does not match") {
			t.Fatalf("ValidateCodexConfig() error = %v", err)
		}
	})
}

func TestCodexUserConfigAtReadErrors(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)

	t.Run("invalid sidecar", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "config.toml")
		statePath := filepath.Join(root, "state.json")
		writeExtraCodexFile(t, path, "external = true\n")
		writeExtraCodexFile(t, statePath, "{")
		if _, _, err := codexUserConfigAt(path, statePath, runtime, block); err == nil || !strings.Contains(err.Error(), "parse Codex adapter state") {
			t.Fatalf("codexUserConfigAt() error = %v", err)
		}
	})

	t.Run("config is directory with sidecar", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "config.toml")
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
		path := filepath.Join(root, "config.toml")
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

func TestStaleAirFullSelectionBlockAdditionalConflicts(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	projected := projectCodex("model_provider = \"native\"\n", block, "")

	for _, test := range []struct {
		name    string
		current string
		state   codexState
		want    string
	}{
		{
			name:    "fallback marker present",
			current: projected + codexFallbackBegin + "\n",
			state:   codexState{ProjectionMode: CodexProjectionNamespacedFallback},
			want:    "fallback block",
		},
		{
			name: "provider table before ownership marker",
			current: codexSelection + "\n" +
				"[model_providers.aigw]\n" + codexBegin + "\n" + codexEnd + "\n",
			state: codexState{ProjectionMode: CodexProjectionNamespacedFallback},
			want:  "provider table is missing",
		},
		{
			name:    "sidecar hash already matches",
			current: projected,
			state:   codexState{ProjectionMode: CodexProjectionNamespacedFallback, ManagedBlockHash: hashText(block)},
			want:    "sidecar matches",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := staleAirFullSelectionBlock(test.current, test.state); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("staleAirFullSelectionBlock() error = %v", err)
			}
		})
	}
}

func TestReadCodexProjectionIdentityErrors(t *testing.T) {
	t.Run("sidecar is directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.Mkdir(codexStatePath(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadCodexProjectionIdentity(path); err == nil || !strings.Contains(err.Error(), "read Codex adapter state") {
			t.Fatalf("ReadCodexProjectionIdentity() error = %v", err)
		}
	})

	t.Run("invalid sidecar", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		writeExtraCodexFile(t, codexStatePath(path), "{")
		if _, err := ReadCodexProjectionIdentity(path); err == nil || !strings.Contains(err.Error(), "parse Codex adapter state") {
			t.Fatalf("ReadCodexProjectionIdentity() error = %v", err)
		}
	})

	t.Run("foreign attribution", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		writeExtraCodexState(t, path, codexState{
			ProjectionMode: CodexProjectionFullSelection,
			WriterID:       "foreign",
			TransactionID:  "foreign-transaction",
		})
		if _, err := ReadCodexProjectionIdentity(path); err == nil || !strings.Contains(err.Error(), "foreign writer") {
			t.Fatalf("ReadCodexProjectionIdentity() error = %v", err)
		}
	})
}

func TestCodexReconciliationPreflightErrors(t *testing.T) {
	t.Run("invalid after target", func(t *testing.T) {
		if _, err := PlanCodexReconciliation(nil, []CodexTargetRef{{}}, atomicTestRuntime()); err == nil || !strings.Contains(err.Error(), "requires surface_id") {
			t.Fatalf("PlanCodexReconciliation() error = %v", err)
		}
	})

	t.Run("endpoint missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		writeExtraCodexFile(t, path, "external = true\n")
		if _, err := PlanCodexReconciliation(nil, []CodexTargetRef{standaloneCodexTarget(path)}, domain.Runtime{ProfileID: "missing-endpoint"}); err == nil || !strings.Contains(err.Error(), "no Codex endpoint") {
			t.Fatalf("PlanCodexReconciliation() error = %v", err)
		}
	})

	t.Run("config missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.toml")
		if _, err := PlanCodexReconciliation(nil, []CodexTargetRef{standaloneCodexTarget(path)}, atomicTestRuntime()); err == nil || !strings.Contains(err.Error(), "config does not exist") || !strings.Contains(err.Error(), "prepare Codex target") {
			t.Fatalf("PlanCodexReconciliation() error = %v", err)
		}
	})

	t.Run("sidecar is directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		writeExtraCodexFile(t, path, "external = true\n")
		if err := os.Mkdir(codexStatePath(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := PlanCodexReconciliation(nil, []CodexTargetRef{standaloneCodexTarget(path)}, atomicTestRuntime()); err == nil || !strings.Contains(err.Error(), "prepare Codex target") {
			t.Fatalf("PlanCodexReconciliation() error = %v", err)
		}
	})

	t.Run("unsupported desired mode", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		writeExtraCodexFile(t, path, "external = true\n")
		target := codexReconciliationTarget{ref: CodexTargetRef{Path: path, ProjectionMode: "unsupported"}, desired: true}
		if _, err := prepareCodexReconciliationTarget(target, atomicTestRuntime(), atomicTestRuntime().Endpoint, "transaction"); err == nil || !strings.Contains(err.Error(), "unsupported Codex projection mode") {
			t.Fatalf("prepareCodexReconciliationTarget() error = %v", err)
		}
	})

	t.Run("invalid desired authority", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		writeExtraCodexFile(t, path, "external = true\n")
		target := standaloneCodexTarget(path)
		target.Authority = "foreign"
		if _, err := PlanCodexReconciliation(nil, []CodexTargetRef{target}, atomicTestRuntime()); err == nil || !strings.Contains(err.Error(), "cannot use authority") {
			t.Fatalf("PlanCodexReconciliation() error = %v", err)
		}
	})

	t.Run("symlink loop", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "loop.toml")
		if err := os.Symlink(path, path); err != nil {
			t.Fatal(err)
		}
		if _, err := normalizeCodexTargets([]CodexTargetRef{standaloneCodexTarget(path)}); err == nil || !strings.Contains(err.Error(), "resolve Codex target symlinks") {
			t.Fatalf("normalizeCodexTargets() error = %v", err)
		}
	})
}

func TestPrepareCodexFallbackAdditionalStates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	runtime := atomicTestRuntime()
	block := codexFallbackBlock(runtime, runtime.Endpoint)
	config := transaction.FileSnapshot{Exists: true, Data: []byte("external = true\n"), Mode: 0o600}

	t.Run("invalid sidecar", func(t *testing.T) {
		state := transaction.FileSnapshot{Exists: true, Data: []byte("{")}
		if _, err := prepareCodexFallback(CodexTargetRef{Path: path}, block, config, state, "transaction"); err == nil || !strings.Contains(err.Error(), "parse Codex adapter state") {
			t.Fatalf("prepareCodexFallback() error = %v", err)
		}
	})

	t.Run("owned block missing", func(t *testing.T) {
		stateData, err := json.Marshal(attributedExtraCodexState(CodexProjectionNamespacedFallback, block))
		if err != nil {
			t.Fatal(err)
		}
		state := transaction.FileSnapshot{Exists: true, Data: stateData}
		if _, err := prepareCodexFallback(CodexTargetRef{Path: path}, block, config, state, "transaction"); err == nil || !strings.Contains(err.Error(), "fallback block is missing") {
			t.Fatalf("prepareCodexFallback() error = %v", err)
		}
	})

	t.Run("already converged", func(t *testing.T) {
		convergedPath := filepath.Join(t.TempDir(), "config.toml")
		writeExtraCodexFile(t, convergedPath, "model_provider = \"jetbrains\"\n")
		target := airFallbackCodexTarget(convergedPath)
		if _, err := ReconcileCodexConfigs(nil, []CodexTargetRef{target}, atomicTestRuntime()); err != nil {
			t.Fatal(err)
		}
		plans, err := PlanCodexReconciliation(nil, []CodexTargetRef{target}, atomicTestRuntime())
		if err != nil {
			t.Fatal(err)
		}
		if len(plans) != 1 || plans[0].Action != "already-converged" {
			t.Fatalf("plans = %#v", plans)
		}
	})
}

func TestPrepareStaleAirRecoveryRejectsUnattributedStates(t *testing.T) {
	t.Run("orphaned residue", func(t *testing.T) {
		config := transaction.FileSnapshot{Exists: true, Data: []byte(codexBegin + "\n")}
		if _, err := prepareStaleAirFullSelectionRecovery(CodexTargetRef{Path: "air.toml"}, config, transaction.FileSnapshot{}); err == nil || !strings.Contains(err.Error(), "residue without an attributable sidecar") {
			t.Fatalf("prepareStaleAirFullSelectionRecovery() error = %v", err)
		}
	})

	t.Run("legacy sidecar", func(t *testing.T) {
		config := transaction.FileSnapshot{Exists: true, Data: []byte("external = true\n")}
		state := transaction.FileSnapshot{Exists: true, Data: []byte("{}")}
		if _, err := prepareStaleAirFullSelectionRecovery(CodexTargetRef{Path: "air.toml"}, config, state); err == nil || !strings.Contains(err.Error(), "legacy Codex sidecar cannot authorize an Air fallback") {
			t.Fatalf("prepareStaleAirFullSelectionRecovery() error = %v", err)
		}
	})
}

func TestRemoveCodexFallbackProjectionBoundaries(t *testing.T) {
	t.Run("incomplete block", func(t *testing.T) {
		if _, err := removeCodexFallbackProjection(codexFallbackBegin+"\n", codexState{}); err == nil || !strings.Contains(err.Error(), "fallback block is incomplete") {
			t.Fatalf("removeCodexFallbackProjection() error = %v", err)
		}
	})

	t.Run("CRLF block", func(t *testing.T) {
		block := codexFallbackBegin + "\r\n" + codexFallbackEnd + "\r\n"
		removed, err := removeCodexFallbackProjection(block, codexState{ManagedBlockHash: hashText(block)})
		if err != nil || removed != "" {
			t.Fatalf("removeCodexFallbackProjection() = %q, %v", removed, err)
		}
	})

	t.Run("recorded separator", func(t *testing.T) {
		block := codexFallbackBegin + "\n" + codexFallbackEnd + "\n"
		removed, err := removeCodexFallbackProjection("external = true\n"+block, codexState{ManagedBlockHash: hashText(block), FallbackPrefix: "\n"})
		if err != nil || removed != "external = true" {
			t.Fatalf("removeCodexFallbackProjection() = %q, %v", removed, err)
		}
	})
}
