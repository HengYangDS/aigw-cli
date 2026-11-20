package codex

import (
	"aigw-cli/internal/domain"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLegacyCodexProjection(t *testing.T, path string, runtimeModel string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\nmodel = \"gpt-native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := atomicTestRuntime()
	runtime.UpstreamModel = runtimeModel
	if _, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(path)}, runtime, runtime, false); err != nil {
		t.Fatal(err)
	}
	statePath := codexStatePath(path)
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal(stateData, &state); err != nil {
		t.Fatal(err)
	}
	delete(state, "projected_model")
	legacyState, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, append(legacyState, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertCodexSyncRejectsDrift(t *testing.T, path, drifted, want string) {
	t.Helper()
	runtime := ownershipTestRuntime()
	if err := os.WriteFile(path, []byte(drifted), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(path)}, runtime, runtime, false)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("ReconcileConfigs() error = %v, want %q", err, want)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != drifted {
		t.Fatal("ReconcileConfigs changed rejected semantic drift")
	}
}

func ownershipTestRuntime() domain.Runtime {
	runtime := atomicTestRuntime()
	runtime.Model = domain.NewModelRef("gateway", "gpt")
	runtime.ModelLabel = "GPT"
	runtime.Endpoint = "https://example.test/v1"
	runtime.UpstreamModel = "gpt-test"
	return runtime
}

func TestCodexResyncRejectsChangedUnmarkedRootModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	runtime := ownershipTestRuntime()
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\nmodel = \"gpt-original\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(path)}, runtime, runtime, false); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(projected), `model = "gpt-test" # managed by AIGW`, `model = "gpt-other"`, 1)
	assertCodexSyncRejectsDrift(t, path, drifted, "model selection changed")
}

func TestReconcileConfigsTransitionAttributesLegacyRootSelections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	previous := atomicTestRuntime()
	writeLegacyCodexProjection(t, path, previous.UpstreamModel)
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	unmarked := strings.Replace(string(projected), `model_provider = "aigw" # managed by AIGW`, `model_provider = "aigw"`, 1)
	unmarked = strings.Replace(unmarked, `model = "gpt-5.6-terra" # managed by AIGW`, `model = "gpt-5.6-terra"`, 1)
	if err := os.WriteFile(path, []byte(unmarked), 0o600); err != nil {
		t.Fatal(err)
	}

	next := previous
	next.Model = domain.NewModelRef("gateway", "next")
	next.ModelLabel = "Next"
	next.Endpoint = "https://next.test/v1"
	next.UpstreamModel = "gpt-next"
	target := HomeTarget(path)
	if _, err := ReconcileConfigs([]TargetRef{target}, []TargetRef{target}, previous, next, false); err != nil {
		t.Fatalf("legacy sidecar transition rejected unchanged root values: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), `model = "gpt-next" # managed by AIGW`) || !strings.Contains(string(after), `base_url = "https://next.test/v1"`) {
		t.Fatalf("transition did not project the next Route:\n%s", after)
	}
}

func TestReconcileConfigsTransitionRejectsChangedLegacyRootModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	previous := atomicTestRuntime()
	writeLegacyCodexProjection(t, path, previous.UpstreamModel)
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(projected), `model = "gpt-5.6-terra" # managed by AIGW`, `model = "gpt-foreign"`, 1)
	if err := os.WriteFile(path, []byte(drifted), 0o600); err != nil {
		t.Fatal(err)
	}

	next := previous
	next.UpstreamModel = "gpt-next"
	target := HomeTarget(path)
	_, err = ReconcileConfigs([]TargetRef{target}, []TargetRef{target}, previous, next, false)
	if err == nil || !strings.Contains(err.Error(), "model selection changed") {
		t.Fatalf("transition error = %v, want model selection conflict", err)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != drifted {
		t.Fatal("failed transition changed the Codex projection")
	}
}

func TestReconcileConfigsAuthorizedTransitionReplacesChangedRootSelections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	previous := atomicTestRuntime()
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\nmodel = \"gpt-native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := HomeTarget(path)
	if _, err := ReconcileConfigs(nil, []TargetRef{target}, previous, previous, false); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(projected), `model_provider = "aigw" # managed by AIGW`, `model_provider = "native"`, 1)
	drifted = strings.Replace(drifted, `model = "gpt-5.6-terra" # managed by AIGW`, `model = "gpt-user"`, 1)
	if err := os.WriteFile(path, []byte(drifted), 0o600); err != nil {
		t.Fatal(err)
	}

	next := previous
	next.Model = domain.NewModelRef("gateway", "next")
	next.ModelLabel = "Next"
	next.Endpoint = "https://next.test/v1"
	next.UpstreamModel = "gpt-next"
	if _, err := ReconcileConfigs([]TargetRef{target}, []TargetRef{target}, previous, next, true); err != nil {
		t.Fatalf("authorized transition rejected root selection replacement: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`model_provider = "aigw" # managed by AIGW`,
		`model = "gpt-next" # managed by AIGW`,
		`base_url = "https://next.test/v1"`,
	} {
		if !strings.Contains(string(after), want) {
			t.Fatalf("authorized transition lacks %q:\n%s", want, after)
		}
	}
}

// A transition admits a root model that is the selection it starts from: a
// configuration that names it has followed Codex's own choice. It replaces
// that model with another selection, withdraws it, and still refuses a model
// that is neither recorded, starting nor about to be written.
func TestReconcileConfigsTransitionAdmitsTheModelItStartsFrom(t *testing.T) {
	project := func(t *testing.T) (string, domain.Runtime) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "configuration.toml")
		recorded := ownershipTestRuntime()
		if err := os.WriteFile(path, []byte("model_provider = \"native\"\nmodel = \"gpt-original\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(path)}, recorded, recorded, false); err != nil {
			t.Fatal(err)
		}
		projected, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		chosen := strings.Replace(string(projected), `model = "gpt-test" # managed by AIGW`, `model = "gpt-chosen"`, 1)
		if err := os.WriteFile(path, []byte(chosen), 0o600); err != nil {
			t.Fatal(err)
		}
		return path, recorded
	}
	follows := func(recorded domain.Runtime) domain.Runtime {
		followed := recorded
		followed.Model, followed.UpstreamModel = domain.NewModelRef("gateway", "chosen"), "gpt-chosen"
		return followed
	}
	t.Run("to another selection", func(t *testing.T) {
		path, recorded := project(t)
		next := recorded
		next.Model, next.UpstreamModel = domain.NewModelRef("gateway", "next"), "gpt-next"
		target := []TargetRef{HomeTarget(path)}
		if _, err := ReconcileConfigs(target, target, recorded, next, false); err == nil || !strings.Contains(err.Error(), "model selection changed") {
			t.Fatalf("a transition from the recorded model replaced Codex's choice: %v", err)
		}
		if _, err := ReconcileConfigs(target, target, follows(recorded), next, false); err != nil {
			t.Fatalf("a transition from the followed model: %v", err)
		}
		if after, err := os.ReadFile(path); err != nil || !strings.Contains(string(after), `model = "gpt-next" # managed by AIGW`) {
			t.Fatalf("the next selection was not written: %v\n%s", err, after)
		}
	})
	t.Run("withdrawal", func(t *testing.T) {
		path, recorded := project(t)
		target := []TargetRef{HomeTarget(path)}
		if _, err := ReconcileConfigs(target, nil, recorded, domain.Runtime{}, false); err == nil || !strings.Contains(err.Error(), "model selection changed") {
			t.Fatalf("a withdrawal from the recorded model removed Codex's choice: %v", err)
		}
		if _, err := ReconcileConfigs(target, nil, follows(recorded), domain.Runtime{}, false); err != nil {
			t.Fatalf("a withdrawal from the followed model: %v", err)
		}
		if after, err := os.ReadFile(path); err != nil || !strings.Contains(string(after), `model = "gpt-original"`) || strings.Contains(string(after), "gpt-chosen") {
			t.Fatalf("the withdrawal did not restore the original selection: %v\n%s", err, after)
		}
	})
}
