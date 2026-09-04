package synchronization

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
)

func TestClientNativeModelProviderChangesProjectionWithoutAIGWCredentialHelper(t *testing.T) {
	target := filepath.Join(t.TempDir(), "config.toml")
	credentialCommand := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := testConfig(target)
	after := before.Clone()
	profile := after.Profiles["gpt"]
	profile.ModelProvider = "amazon-bedrock"
	profile.Authentication = configuration.AuthenticationClientNative
	profile.Model = "openai.gpt-5.6-sol"
	after.Profiles["gpt"] = profile

	if !(Synchronizer{}).ProjectionChanged(before, after) {
		t.Fatal("model provider change must change the projection")
	}
	syncer := Synchronizer{Discovery: targetDiscovery(target), AIGWExecutable: credentialCommand}
	if err := syncer.Reconcile(context.Background(), before, after); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `model_provider = "amazon-bedrock"`) {
		t.Fatalf("native projection missing provider:\n%s", text)
	}
	for _, forbidden := range []string{"command = " + credentialCommand, `[model_providers.amazon-bedrock.auth]`, `args = ["credential", "codex"]`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("native projection contains AIGW command authentication %q:\n%s", forbidden, text)
		}
	}
}

func TestNativeModelProviderSkipsGenericLoginAndDefaultTransitionRebindsIt(t *testing.T) {
	native := testConfig("/target")
	profile := native.Profiles["gpt"]
	profile.ModelProvider = "amazon-bedrock"
	profile.Authentication = configuration.AuthenticationClientNative
	native.Profiles["gpt"] = profile

	syncer := Synchronizer{}
	if syncer.CredentialBindingChanged(configuration.NewConfig(), native) {
		t.Fatal("native provider must not request generic Codex login")
	}
	if err := syncer.BindCredential(context.Background(), native, configuration.ClientCodex, nil); err != nil {
		t.Fatalf("native provider authentication = %v", err)
	}

	defaultProvider := native.Clone()
	profile = defaultProvider.Profiles["gpt"]
	profile.ModelProvider = ""
	profile.Authentication = ""
	defaultProvider.Profiles["gpt"] = profile
	if !syncer.CredentialBindingChanged(native, defaultProvider) {
		t.Fatal("transition to default provider must bind generic Codex authentication")
	}
	if syncer.CredentialBindingChanged(defaultProvider, native) {
		t.Fatal("transition to command-authenticated provider must not bind generic Codex authentication")
	}
}
