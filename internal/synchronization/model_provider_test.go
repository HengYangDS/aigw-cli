package synchronization

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
)

func TestNativeModelProviderChangesProjectionAndUsesCredentialHelper(t *testing.T) {
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := testConfig(target)
	after := before.Clone()
	profile := after.Profiles["gpt"]
	profile.ModelProvider = "amazon-bedrock"
	profile.Models[configuration.ClientCodex] = "openai.gpt-5.6-sol"
	after.Profiles["gpt"] = profile

	if !ProjectionChanged(before, after) {
		t.Fatal("model provider change must change the projection")
	}
	syncer := Synchronizer{Discovery: targetDiscovery(target), AIGWExecutable: "/opt/aigw"}
	if err := syncer.Reconcile(context.Background(), before, after); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `model_provider = "amazon-bedrock"`) || !strings.Contains(text, `command = "/opt/aigw"`) {
		t.Fatalf("native projection missing provider or helper:\n%s", text)
	}
}

func TestNativeModelProviderSkipsGenericLoginAndDefaultTransitionRebindsIt(t *testing.T) {
	native := testConfig("/target")
	profile := native.Profiles["gpt"]
	profile.ModelProvider = "amazon-bedrock"
	native.Profiles["gpt"] = profile

	if AuthenticationChanged(configuration.NewConfig(), native) {
		t.Fatal("native provider must not request generic Codex login")
	}
	if err := (Synchronizer{}).BindAuthenticationTargets(context.Background(), native, nil); err != nil {
		t.Fatalf("native provider authentication = %v", err)
	}

	defaultProvider := native.Clone()
	profile = defaultProvider.Profiles["gpt"]
	profile.ModelProvider = ""
	defaultProvider.Profiles["gpt"] = profile
	if !AuthenticationChanged(native, defaultProvider) {
		t.Fatal("transition to default provider must bind generic Codex authentication")
	}
	if AuthenticationChanged(defaultProvider, native) {
		t.Fatal("transition to command-authenticated provider must not bind generic Codex authentication")
	}
}
