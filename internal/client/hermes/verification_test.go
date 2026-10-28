package hermes

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestVerificationProjectsAnotherModelWithoutChangingOwnedFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("model:\n  temperature: 0.3\nagent:\n  reasoning_overrides:\n    another-model: none\nsecurity:\n  allow_lazy_installs: true\n  policy: strict\nupdates:\n  check: true\ncli:\n  expose_on_path: true\nplugins: {enabled: [operator-plugin]}\nmemory: {provider: operator-memory, enabled: true}\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	desired := testDesired()
	plan, err := Prepare(path, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	state, err := os.ReadFile(path + stateSuffix)
	if err != nil {
		t.Fatal(err)
	}
	desired.SelectedModel = "another-model"
	desired.Providers[0].Models = append(desired.Providers[0].Models, desired.SelectedModel)
	plan, err = Prepare(path, &desired)
	if err != nil {
		t.Fatal(err)
	}
	data, err := plan.PrepareVerification()
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	model := configMap(t, result, "model")
	security := configMap(t, result, "security")
	if model["default"] != "another-model" || model["temperature"] != 0.3 || security["allow_lazy_installs"] != false || security["policy"] != "strict" || configMap(t, result, "updates")["check"] != false || configMap(t, result, "cli")["expose_on_path"] != false {
		t.Fatalf("isolated native configuration = %#v", result)
	}
	if enabled, ok := configMap(t, result, "plugins")["enabled"].([]any); !ok || len(enabled) != 0 {
		t.Fatal("verification could activate operator plugins")
	}
	if idle := configMap(t, result, "gateway")["unmanaged_idle_exit_seconds"]; idle != VerificationGatewayIdleExitSeconds {
		t.Fatalf("verification gateway idle exit = %#v, want %d seconds", idle, VerificationGatewayIdleExitSeconds)
	}
	if memory := configMap(t, result, "memory"); memory["provider"] != nil || memory["enabled"] != true {
		t.Fatal("verification activated operator memory or lost unrelated preferences")
	}
	for source, expected := range map[string][]byte{path: before, path + stateSuffix: state} {
		actual, err := os.ReadFile(source)
		if err != nil || !bytes.Equal(actual, expected) {
			t.Fatal("verification changed the source configuration or ownership record")
		}
	}
}

func TestVerificationRejectsAmbiguousNativePolicy(t *testing.T) {
	for _, content := range []string{"security: [invalid]", "updates: &policy {}", "security: *missing", "model: [invalid]", "gateway: [invalid]", "gateway: &gateway {}"} {
		t.Run(content, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			desired := testDesired()
			plan, err := Prepare(path, &desired)
			if err == nil {
				_, err = plan.PrepareVerification()
			}
			if err == nil {
				t.Fatal("ambiguous native policy was accepted")
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != content {
				t.Fatal("failed verification preparation changed the source")
			}
		})
	}
}

func TestVerificationRequiresExistingNativeConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")
	desired := testDesired()
	plan, err := Prepare(path, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.PrepareVerification(); err == nil || !strings.Contains(err.Error(), "native configuration is missing") {
		t.Fatalf("missing native configuration was synthesized: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("verification wrote a missing source: %v", err)
	}
	if _, err := Prepare(t.TempDir(), &desired); err == nil {
		t.Fatal("directory was accepted as native configuration")
	}
}
