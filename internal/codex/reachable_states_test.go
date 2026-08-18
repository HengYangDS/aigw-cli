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

	t.Run("unattributed full selection", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		runtime := atomicTestRuntime()
		block := codexManagedBlock(runtime, runtime.Endpoint)
		projection, err := projectCodex("model_provider = \"native\"\n", block, "")
		if err != nil {
			t.Fatal(err)
		}
		writeExtraCodexFile(t, path, projection)
		writeExtraCodexState(t, path, codexState{ManagedBlockHash: hashText(block)})

		inspection, err := InspectConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if inspection.State != "ownership-conflict" || inspection.AttributionState != "foreign-or-incomplete" || inspection.AIGWManaged || inspection.SidecarHashMatches {
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

	t.Run("recognized projection is managed", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		runtime := atomicTestRuntime()
		block := codexManagedBlock(runtime, runtime.Endpoint)
		projection, err := projectCodex("external = true\n", block, runtime.Model)
		if err != nil {
			t.Fatal(err)
		}
		writeExtraCodexFile(t, path, projection)
		writeExtraCodexState(t, path, attributedExtraCodexState(ProjectionFullSelection, block))

		inspection, err := InspectConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if inspection.State != "aigw-managed" || !inspection.AIGWManaged || !inspection.SidecarHashMatches {
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
		projection, err := projectCodex("", block, runtime.Model)
		if err != nil {
			t.Fatal(err)
		}
		writeExtraCodexFile(t, path, projection)
		if err := os.Mkdir(codexStatePath(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := ValidateConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "read Codex adapter state") {
			t.Fatalf("ValidateConfig() error = %v", err)
		}
	})

	t.Run("invalid sidecar", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		projection, err := projectCodex("", block, runtime.Model)
		if err != nil {
			t.Fatal(err)
		}
		writeExtraCodexFile(t, path, projection)
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
		projection, err := projectCodex("", otherBlock, runtime.Model)
		if err != nil {
			t.Fatal(err)
		}
		writeExtraCodexFile(t, path, projection)
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

	t.Run("scheduler key missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		projection, err := projectCodex("", block, runtime.Model)
		if err != nil {
			t.Fatal(err)
		}
		projection = strings.Replace(projection, "max_depth = 1 # managed by AIGW\n", "", 1)
		writeExtraCodexFile(t, path, projection)
		writeExtraCodexState(t, path, validState)
		if err := ValidateConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "scheduler key") {
			t.Fatalf("ValidateConfig() error = %v", err)
		}
	})

	t.Run("scheduler state hash mismatch", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		projection, err := projectCodex("", block, runtime.Model)
		if err != nil {
			t.Fatal(err)
		}
		state := validState
		state.ProjectedSchedulerHash = "changed"
		writeExtraCodexFile(t, path, projection)
		writeExtraCodexState(t, path, state)
		if err := ValidateConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "scheduler keys changed") {
			t.Fatalf("ValidateConfig() error = %v", err)
		}
	})
}

func TestCodexUserConfigRejectsInvalidState(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	config := transaction.FileSnapshot{Exists: true, Data: []byte("external = true\n")}
	state := transaction.FileSnapshot{Exists: true, Data: []byte("{")}
	if _, _, err := codexUserConfig(config, state, runtime, block); err == nil || !strings.Contains(err.Error(), "parse Codex adapter state") {
		t.Fatalf("codexUserConfig() error = %v", err)
	}
}

func TestCodexUserConfigRejectsInvalidCapturedSchedulerState(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	projection, err := projectCodex("external = true\n", block, runtime.Model)
	if err != nil {
		t.Fatal(err)
	}
	state := attributedExtraCodexState(ProjectionFullSelection, block)
	state.OriginalScheduler = map[string]*int{"invalid": nil}
	stateData, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	config := transaction.FileSnapshot{Exists: true, Data: []byte(projection)}
	sidecar := transaction.FileSnapshot{Exists: true, Data: stateData}
	if _, _, err := codexUserConfig(config, sidecar, runtime, block); err == nil || !strings.Contains(err.Error(), "invalid Codex scheduler state key") {
		t.Fatalf("codexUserConfig() error = %v", err)
	}
}

func TestCodexUserConfigRejectsSchedulerRestoreError(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	projection, err := projectCodex("external = true\n", block, runtime.Model)
	if err != nil {
		t.Fatal(err)
	}
	state := attributedExtraCodexState(ProjectionFullSelection, block)
	state.OriginalScheduler = map[string]*int{"invalid": nil}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := codexUserConfig(
		transaction.FileSnapshot{Exists: true, Data: []byte(projection)},
		transaction.FileSnapshot{Exists: true, Data: data},
		runtime,
		block,
	); err == nil || !strings.Contains(err.Error(), "invalid Codex scheduler state key") {
		t.Fatalf("codexUserConfig() error = %v", err)
	}
}

func TestCompleteExactTruncatedCodexProjectionRejectsAmbiguities(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	state := codexState{
		ManagedBlockHash: hashText(block),
		OriginalScheduler: map[string]*int{
			"agents.max_concurrent_threads_per_session":                  nil,
			"agents.max_depth":                                           nil,
			"features.multi_agent_v2.max_concurrent_threads_per_session": nil,
		},
	}
	truncated := strings.TrimSuffix(block, codexEnd+"\n")

	for _, test := range []struct {
		name    string
		current string
		state   codexState
	}{
		{
			name:    "selection is not managed",
			current: `model_provider = "native"` + "\n" + codexBegin + "\n" + truncated,
			state:   state,
		},
		{
			name:    "managed model differs",
			current: codexSelection + "\n" + `model = "other" # managed by AIGW` + "\n" + codexBegin + "\n" + truncated,
			state:   state,
		},
		{
			name:    "provider table missing",
			current: codexSelection + "\n" + codexBegin + "\n",
			state:   state,
		},
		{
			name:    "completion marker already present",
			current: codexSelection + "\n" + fmt.Sprintf("model = %q # managed by AIGW\n", runtime.Model) + codexBegin + "\n" + block,
			state:   state,
		},
		{
			name:    "truncated bytes mismatch",
			current: codexSelection + "\n" + codexBegin + "\n[model_providers.aigw]\nchanged = true\n",
			state:   state,
		},
		{
			name:    "state block hash missing",
			current: codexSelection + "\n" + fmt.Sprintf("model = %q # managed by AIGW\n", runtime.Model) + codexBegin + "\n" + truncated,
			state:   codexState{},
		},
		{
			name:    "foreign content before next table",
			current: codexSelection + "\n" + codexBegin + "\n" + truncated + "foreign = true\n[other]\n",
			state:   state,
		},
		{
			name:    "foreign trailing content",
			current: codexSelection + "\n" + fmt.Sprintf("model = %q # managed by AIGW\n", runtime.Model) + codexBegin + "\n" + truncated + "foreign = true\n",
			state:   state,
		},
		{
			name:    "blank lines before next table are recoverable",
			current: codexSelection + "\n" + fmt.Sprintf("model = %q # managed by AIGW\n", runtime.Model) + codexBegin + "\n" + truncated + "\n[next]\nvalue = 1\n",
			state:   state,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			completed, ok := completeExactTruncatedCodexProjection(test.current, test.state, runtime, block)
			if test.name == "blank lines before next table are recoverable" {
				if !ok || !strings.Contains(completed, codexEnd+"\n[next]") {
					t.Fatalf("completeExactTruncatedCodexProjection() = %t:\n%s", ok, completed)
				}
				return
			}
			if ok {
				t.Fatalf("completeExactTruncatedCodexProjection() admitted:\n%s", completed)
			}
		})
	}
}

func TestCompleteExactTruncatedProjectionRejectsForeignContentAfterValidPrefix(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	state := codexState{ManagedBlockHash: hashText(block)}
	truncated := strings.TrimSuffix(block, codexEnd+"\n")
	current := codexSelection + "\n" + fmt.Sprintf("model = %q # managed by AIGW\n", runtime.Model) + codexBegin + "\n" + truncated + "foreign = true\n[other]\n"
	if completed, ok := completeExactTruncatedCodexProjection(current, state, runtime, block); ok {
		t.Fatalf("foreign content was admitted:\n%s", completed)
	}
}

func TestRemoveCodexProjectionRestoresAbsentProvider(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	current, err := projectCodex("external = true\n", block, runtime.Model)
	if err != nil {
		t.Fatal(err)
	}
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

func TestRemoveCodexProjectionRestoresAbsentOriginalSelection(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	current, err := projectCodex("external = true\n", block, runtime.Model)
	if err != nil {
		t.Fatal(err)
	}
	state := codexState{
		ManagedBlockHash: hashText(block),
		// The older projected key set, i.e. a sidecar written before the [agents]
		// alias was retired. Removal must still clear AIGW's own max_threads even
		// though this state records no original for it.
		OriginalScheduler: map[string]*int{
			"agents.max_concurrent_threads_per_session":                  nil,
			"agents.max_depth":                                           nil,
			"features.multi_agent_v2.max_concurrent_threads_per_session": nil,
		},
	}

	restored, err := removeCodexProjection(current, state)
	if err != nil {
		t.Fatal(err)
	}
	if restored != "external = true\n" {
		t.Fatalf("restored config = %q", restored)
	}
}

func TestRemoveCodexProjectionRejectsChangedManagedBlock(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	current, err := projectCodex("external = true\n", block, runtime.Model)
	if err != nil {
		t.Fatal(err)
	}
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
		if _, err := PlanReconciliation(nil, []TargetRef{codexHomeTarget(path)}, configuration.Runtime{ProfileID: "missing-endpoint"}); err == nil || !strings.Contains(err.Error(), "no Codex endpoint") {
			t.Fatalf("PlanReconciliation() error = %v", err)
		}
	})

	t.Run("config missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.toml")
		if _, err := PlanReconciliation(nil, []TargetRef{codexHomeTarget(path)}, atomicTestRuntime()); err == nil || !strings.Contains(err.Error(), "config does not exist") || !strings.Contains(err.Error(), "prepare Codex target") {
			t.Fatalf("PlanReconciliation() error = %v", err)
		}
	})

	t.Run("config is directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := PlanReconciliation(nil, []TargetRef{codexHomeTarget(path)}, atomicTestRuntime()); err == nil || !strings.Contains(err.Error(), "read") {
			t.Fatalf("PlanReconciliation() error = %v", err)
		}
	})

	t.Run("sidecar is directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeExtraCodexFile(t, path, "external = true\n")
		if err := os.Mkdir(codexStatePath(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := PlanReconciliation(nil, []TargetRef{codexHomeTarget(path)}, atomicTestRuntime()); err == nil || !strings.Contains(err.Error(), "prepare Codex target") {
			t.Fatalf("PlanReconciliation() error = %v", err)
		}
	})

	t.Run("desired mode must be validated before preparation", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeExtraCodexFile(t, path, "external = true\n")
		target := codexHomeTarget(path)
		target.ProjectionMode = "unsupported"
		if _, err := PlanReconciliation(nil, []TargetRef{target}, atomicTestRuntime()); err == nil || !strings.Contains(err.Error(), "cannot use authority") {
			t.Fatalf("PlanReconciliation() error = %v", err)
		}
	})

	t.Run("invalid desired authority", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeExtraCodexFile(t, path, "external = true\n")
		target := codexHomeTarget(path)
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
		if _, err := normalizeCodexTargets([]TargetRef{codexHomeTarget(path)}); err == nil || !strings.Contains(err.Error(), "resolve Codex target symlinks") {
			t.Fatalf("normalizeCodexTargets() error = %v", err)
		}
	})
}
