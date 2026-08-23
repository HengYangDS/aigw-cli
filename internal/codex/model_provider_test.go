package codex_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
)

func TestCodexSyncProjectsExplicitProviderAndRestoresOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "model_provider = \"native\"\nmodel = \"gpt-native\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := codexRuntime("aws", "AWS", "https://gateway.test/openai/v1", "openai.gpt-5.6-sol")
	runtime.ModelProvider = "amazon-bedrock"
	runtime.CredentialCommand = "/opt/aigw"

	if err := codex.SyncConfig(path, runtime); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(first)
	for _, want := range []string{
		`model_provider = "amazon-bedrock" # managed by AIGW`,
		`model = "openai.gpt-5.6-sol" # managed by AIGW`,
		`[model_providers.amazon-bedrock]`,
		`base_url = "https://gateway.test/openai/v1"`,
		`wire_api = "responses"`,
		`[model_providers.amazon-bedrock.auth]`,
		`command = "/opt/aigw"`,
		`args = ["credential", "codex"]`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("projection lacks %q:\n%s", want, text)
		}
	}
	for _, forbidden := range []string{`[model_providers.aigw]`, `requires_openai_auth`, `model_catalog_json`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("projection contains %q:\n%s", forbidden, text)
		}
	}
	if err := codex.SyncConfig(path, runtime); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != text {
		t.Fatalf("second sync changed projection:\nfirst:\n%s\nsecond:\n%s", text, second)
	}
	if err := codex.ValidateConfig(path, runtime); err != nil {
		t.Fatal(err)
	}
	state, err := os.ReadFile(path + ".aigw-state.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(state), `"projected_provider": "amazon-bedrock"`) {
		t.Fatalf("sidecar lacks projected provider:\n%s", state)
	}
	if err := codex.DisableConfig(path); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != original {
		t.Fatalf("restored config differs:\n%s", restored)
	}
}

func TestCodexSyncPreservesDefaultProjectionAndTransitionsFromNative(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := codexRuntime("native", "Native", "https://gateway.test/openai/v1", "wire-model")
	runtime.ModelProvider = "amazon-bedrock"
	runtime.CredentialCommand = "/opt/aigw"
	if err := codex.SyncConfig(path, runtime); err != nil {
		t.Fatal(err)
	}

	runtime.ModelProvider = configuration.ModelProviderAIGW
	runtime.CredentialCommand = ""
	if err := codex.SyncConfig(path, runtime); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`model_provider = "aigw" # managed by AIGW`, `[model_providers.aigw]`, `requires_openai_auth = true`} {
		if !strings.Contains(text, want) {
			t.Fatalf("default projection lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, `[model_providers.amazon-bedrock]`) {
		t.Fatalf("native provider survived transition:\n%s", text)
	}
	state, err := os.ReadFile(path + ".aigw-state.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(state), "projected_provider") {
		t.Fatalf("default sidecar changed existing shape:\n%s", state)
	}
}

func TestCodexSyncRejectsInvalidNativeCredentialCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := codexRuntime("native", "Native", "https://gateway.test/openai/v1", "wire-model")
	runtime.ModelProvider = "amazon-bedrock"
	for _, command := range []string{"", "aigw"} {
		runtime.CredentialCommand = command
		if err := codex.SyncConfig(path, runtime); err == nil {
			t.Fatalf("credential command %q was accepted", command)
		}
	}
}

func TestCodexValidateResolvesCurrentAIGWExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	runtime := codexRuntime("native", "Native", "https://gateway.test/openai/v1", "wire-model")
	runtime.ModelProvider = "amazon-bedrock"
	runtime.CredentialCommand = executable
	if err := codex.SyncConfig(path, runtime); err != nil {
		t.Fatal(err)
	}

	runtime.CredentialCommand = ""
	if err := codex.ValidateConfig(path, runtime); err != nil {
		t.Fatal(err)
	}
	inspection, err := codex.InspectConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.State != "aigw-managed" || !inspection.AIGWManaged || !inspection.SidecarHashMatches {
		t.Fatalf("inspection = %#v", inspection)
	}
}
