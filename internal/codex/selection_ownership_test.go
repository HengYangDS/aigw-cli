package codex

import (
	configuration "aigw-cli/internal/configuration"
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
	runtime.Model = runtimeModel
	if _, err := ReconcileConfigs(nil, []TargetRef{codexHomeTarget(path)}, runtime); err != nil {
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
	err := SyncConfig(path, runtime)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("SyncConfig() error = %v, want %q", err, want)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != drifted {
		t.Fatal("SyncConfig changed rejected semantic drift")
	}
}

func ownershipTestRuntime() configuration.Runtime {
	runtime := atomicTestRuntime()
	runtime.RouteID = "gpt"
	runtime.RouteLabel = "GPT"
	runtime.Endpoint = "https://example.test/v1"
	runtime.Model = "gpt-test"
	return runtime
}

func TestCodexResyncRejectsChangedUnmarkedRootModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	runtime := ownershipTestRuntime()
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\nmodel = \"gpt-original\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SyncConfig(path, runtime); err != nil {
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
	writeLegacyCodexProjection(t, path, previous.Model)
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
	next.RouteID = "next"
	next.RouteLabel = "Next"
	next.Endpoint = "https://next.test/v1"
	next.Model = "gpt-next"
	target := codexHomeTarget(path)
	if _, err := ReconcileConfigsTransition([]TargetRef{target}, []TargetRef{target}, previous, next); err != nil {
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
	writeLegacyCodexProjection(t, path, previous.Model)
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(projected), `model = "gpt-5.6-terra" # managed by AIGW`, `model = "gpt-foreign"`, 1)
	if err := os.WriteFile(path, []byte(drifted), 0o600); err != nil {
		t.Fatal(err)
	}

	next := previous
	next.Model = "gpt-next"
	target := codexHomeTarget(path)
	_, err = ReconcileConfigsTransition([]TargetRef{target}, []TargetRef{target}, previous, next)
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
	target := codexHomeTarget(path)
	if _, err := ReconcileConfigs(nil, []TargetRef{target}, previous); err != nil {
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
	next.RouteID = "next"
	next.RouteLabel = "Next"
	next.Endpoint = "https://next.test/v1"
	next.Model = "gpt-next"
	if _, err := ReconcileConfigsAuthorizedTransition([]TargetRef{target}, []TargetRef{target}, previous, next); err != nil {
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
