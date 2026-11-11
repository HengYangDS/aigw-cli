package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

func TestCodexConnectionRetryProjectionIsExplicitAndGuarded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "model = \"native-model\"\nmodel_reasoning_effort = \"xhigh\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := domain.Runtime{
		Client: ID, Model: domain.NewModelRef("test", "gpt-6.1-sol"), ModelLabel: "Test",
		ProviderID: "test", Endpoint: "https://endpoint.test/v1", UpstreamModel: "gpt-6.1-sol",
		ModelProvider: "test-provider", Authentication: domain.AuthenticationClientNative,
	}
	if _, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(path)}, runtime, runtime, false); err != nil {
		t.Fatal(err)
	}
	defaults, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(defaults), "max_retries") {
		t.Fatal("undeclared retry defaults were projected")
	}
	if err := json.Unmarshal([]byte(`{"retry_policy":{"request_max_retries":0,"stream_max_retries":0}}`), &runtime); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(path)}, runtime, runtime, false); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"request_max_retries = 0", "stream_max_retries = 0", `model_reasoning_effort = "xhigh"`} {
		if !strings.Contains(string(projected), want) {
			t.Fatalf("native projection lacks %q", want)
		}
	}
	if _, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(path)}, runtime, runtime, false); err != nil {
		t.Fatalf("repeat sync refused its owned policy: %v", err)
	}
	again, err := os.ReadFile(path)
	if err != nil || string(again) != string(projected) {
		t.Fatalf("repeat sync changed provider bytes: %v", err)
	}
	edited := strings.Replace(string(projected), "request_max_retries = 0", "request_max_retries = 1", 1)
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(path)}, runtime, runtime, false); err == nil || !strings.Contains(err.Error(), "Codex config conflict") {
		t.Fatalf("external policy edit was not a guarded conflict: %v", err)
	}
	retained, err := os.ReadFile(path)
	if err != nil || string(retained) != edited {
		t.Fatalf("refused sync overwrote an external edit: %v", err)
	}
	if err := os.WriteFile(path, projected, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileConfigs([]TargetRef{HomeTarget(path)}, nil, domain.Runtime{}, domain.Runtime{}, false); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil || string(restored) != original {
		t.Fatalf("disable did not restore original native defaults: %v", err)
	}
}
