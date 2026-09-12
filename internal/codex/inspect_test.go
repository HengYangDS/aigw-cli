package codex

import (
	"aigw-cli/internal/configuration"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
		{"model_provider", "invalid"},
	}
	for _, c := range cases {
		if got := classifyCodexDiskSelection(c.input); got != c.expected {
			t.Errorf("classifyCodexDiskSelection(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestInspectConfigFilesystemStates(t *testing.T) {
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
		writeCodexFixture(t, path, "model_provider = \"native\"\n")
		if err := os.Mkdir(codexStatePath(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := InspectConfig(path); err == nil || !strings.Contains(err.Error(), "read Codex sidecar") {
			t.Fatalf("InspectConfig() error = %v", err)
		}
	})
}

func TestInspectConfigOwnershipStates(t *testing.T) {
	t.Run("unattributed full selection", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		runtime := atomicTestRuntime()
		block := codexManagedBlock(runtime, runtime.Endpoint)
		projection, err := projectCodex("model_provider = \"native\"\n", block, "", "", configuration.ModelProviderAIGW)
		if err != nil {
			t.Fatal(err)
		}
		writeCodexFixture(t, path, projection)
		writeCodexStateFixture(t, path, codexState{ManagedBlockHash: hashText(block)})

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
		projection, err := projectCodex("", block, "", "", configuration.ModelProviderAIGW)
		if err != nil {
			t.Fatal(err)
		}
		projection, err = setCodexSelection(projection, "model_provider", `model_provider = "native"`)
		if err != nil {
			t.Fatal(err)
		}
		writeCodexFixture(t, path, projection)
		writeCodexStateFixture(t, path, attributedCodexStateFixture(block))

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
		projection, err := projectCodex("external = true\n", block, runtime.Model, "", configuration.ModelProviderAIGW)
		if err != nil {
			t.Fatal(err)
		}
		writeCodexFixture(t, path, projection)
		writeCodexStateFixture(t, path, attributedCodexStateFixture(block))

		inspection, err := InspectConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if inspection.State != "aigw-managed" || !inspection.AIGWManaged || !inspection.SidecarHashMatches {
			t.Fatalf("inspection = %#v", inspection)
		}
	})

	t.Run("scheduler ownership drift", func(t *testing.T) {
		runtime := atomicTestRuntime()
		block := codexManagedBlock(runtime, runtime.Endpoint)
		projection, err := projectCodex("external = true\n", block, runtime.Model, "", configuration.ModelProviderAIGW)
		if err != nil {
			t.Fatal(err)
		}
		for _, mutate := range []func(*codexState){
			func(state *codexState) { state.ProjectedSchedulerHash = "" },
			func(state *codexState) { state.OriginalScheduler = nil },
			func(state *codexState) { delete(state.OriginalScheduler, "agents.max_threads") },
		} {
			path := filepath.Join(t.TempDir(), "configuration.toml")
			state := attributedCodexStateFixture(block)
			mutate(&state)
			writeCodexFixture(t, path, projection)
			writeCodexStateFixture(t, path, state)

			inspection, err := InspectConfig(path)
			if err != nil {
				t.Fatal(err)
			}
			if inspection.State != "aigw-drift" || !inspection.AIGWManaged || !inspection.SidecarHashMatches {
				t.Fatalf("inspection = %#v", inspection)
			}
		}
	})
}
