package configuration

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func testDesired() Desired {
	return Desired{
		SelectedProvider: "aigw",
		SelectedModel:    "model-test",
		Providers: []Provider{{
			ID: "aigw", Models: []string{"model-test"}, Endpoint: "https://provider.test/v1",
			Protocol: "openai_responses", CredentialCommand: "aigw credential hermes projection-test",
		}},
	}
}

func readConfig(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestProjectionPreservesUnownedConfigurationAndWithdraws(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("# User preferences\nmodel:\n  provider: original\n  default: previous-model\n  base_url: https://previous.test\n  temperature: 0.3\nproviders:\n  personal:\n    base_url: https://personal.test\nterminal:\n  backend: local\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	desired := testDesired()
	plan, err := Prepare(path, &desired)
	if err != nil {
		t.Fatal(err)
	}
	untouched, _ := os.ReadFile(path)
	if !bytes.Equal(untouched, original) {
		t.Fatal("planning mutated the user's configuration")
	}
	receipt, err := plan.Apply()
	if err != nil {
		t.Fatal(err)
	}
	got := readConfig(t, path)
	model := configMap(t, got, "model")
	if model["provider"] != "aigw" || model["default"] != desired.SelectedModel || model["temperature"] != 0.3 {
		t.Fatalf("model projection = %#v", model)
	}
	providers := configMap(t, got, "providers")
	projected := configMap(t, providers, "aigw")
	if projected["key_cmd"] != desired.Providers[0].CredentialCommand || projected["base_url"] != desired.Providers[0].Endpoint || projected["transport"] != "codex_responses" {
		t.Fatalf("provider projection = %#v", projected)
	}
	if _, ok := providers["personal"]; !ok || got["terminal"] == nil {
		t.Fatal("unrelated configuration was removed")
	}
	data, _ := os.ReadFile(path)
	if !bytes.Contains(data, []byte("# User preferences")) {
		t.Fatal("user comment was removed")
	}
	if err := receipt.Rollback(); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(path)
	if !bytes.Equal(restored, original) {
		t.Fatal("compensation did not restore exact original bytes")
	}
	if _, err := os.Stat(path + ".aigw-state.json"); !os.IsNotExist(err) {
		t.Fatal("compensation left an ownership record")
	}
	plan, err = Prepare(path, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	plan, err = Prepare(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	restoredConfig := readConfig(t, path)
	if configMap(t, restoredConfig, "model")["provider"] != "original" {
		t.Fatal("withdrawal lost the previous provider")
	}
	if _, exists := configMap(t, restoredConfig, "providers")["aigw"]; exists {
		t.Fatal("withdrawal left its provider")
	}
}

func TestProjectionPublishesTheCuratedModelsForEachOwnedProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	desired := Desired{
		SelectedProvider: "aigw-ucloud-anthropic",
		SelectedModel:    "claude-fable-5-1",
		Providers: []Provider{
			{ID: "aigw-ucloud-anthropic", Endpoint: "https://messages.test", Protocol: "anthropic", CredentialCommand: "aigw credential hermes messages", Models: []string{"claude-fable-5-1", "claude-opus-5"}},
			{ID: "aigw-ucloud-responses", Endpoint: "https://responses.test/v1", Protocol: "openai_responses", CredentialCommand: "aigw credential hermes responses", Models: []string{"gpt-6-astra", "grok-4.6"}},
			{ID: "aigw-ucloud-chat", Endpoint: "https://chat.test/v1", Protocol: "openai_chat_completions", CredentialCommand: "aigw credential hermes chat", Models: []string{"gemini-3.8-flash"}},
		},
	}
	plan, err := Prepare(path, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	got := readConfig(t, path)
	model := configMap(t, got, "model")
	if model["provider"] != desired.SelectedProvider || model["default"] != desired.SelectedModel {
		t.Fatalf("selected model = %#v", model)
	}
	providers := configMap(t, got, "providers")
	for _, provider := range desired.Providers {
		projected := configMap(t, providers, provider.ID)
		models, ok := projected["models"].([]any)
		if projected["key_cmd"] != provider.CredentialCommand || projected["discover_models"] != false || !ok || len(models) != len(provider.Models) {
			t.Fatalf("provider %s projection = %#v", provider.ID, projected)
		}
	}
	if _, err := Prepare(path, nil); err != nil {
		t.Fatal(err)
	}
}

func TestProjectionNoopPreservesBytesAndUnownedEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	desired := testDesired()
	plan, err := Prepare(path, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	data = append(data, []byte("# Independent edit\nterminal:\n  backend: docker\n")...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err = Prepare(path, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Action != "unchanged" {
		t.Fatalf("action = %s", plan.Action)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(data, after) {
		t.Fatal("no-op rewrote user bytes")
	}
	plan, err = Prepare(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	if configMap(t, readConfig(t, path), "terminal")["backend"] != "docker" {
		t.Fatal("withdrawal erased an independent edit")
	}
}

func TestProjectionProtectsForeignProviderAndManagedEdits(t *testing.T) {
	for _, data := range []string{
		"providers:\n  aigw:\n    base_url: https://foreign.test\n",
		"model: [invalid]\n",
		"model:\n  provider: one\n  provider: two\n",
		"model: {}\n---\nmodel: {}\n",
	} {
		t.Run(data, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			desired := testDesired()
			if _, err := Prepare(path, &desired); err == nil {
				t.Fatal("accepted ambiguous or foreign-owned configuration")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	desired := testDesired()
	plan, err := Prepare(path, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	edited := bytes.ReplaceAll(data, []byte(desired.SelectedModel), []byte("user-model"))
	if err := os.WriteFile(path, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, desired := range []*Desired{&desired, nil} {
		if _, err := Prepare(path, desired); err == nil {
			t.Fatal("managed user edit was not protected")
		}
	}
}

func TestPreparedProjectionAndRollbackProtectConcurrentEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	desired := testDesired()
	plan, err := Prepare(path, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("terminal: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err == nil {
		t.Fatal("prepared write overwrote concurrent edit")
	}
	plan, err = Prepare(path, &desired)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := plan.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("terminal: {backend: ssh}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := receipt.Rollback(); err == nil {
		t.Fatal("rollback overwrote concurrent edit")
	}
}

func TestProjectionSupportsEveryHermesWireAndScalarModel(t *testing.T) {
	for protocol, native := range map[string]string{"openai_responses": "codex_responses", "anthropic": "anthropic_messages", "openai_chat_completions": "chat_completions"} {
		t.Run(protocol, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte("model: previous-model\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			desired := testDesired()
			desired.Providers[0].Protocol = protocol
			plan, err := Prepare(path, &desired)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := plan.Apply(); err != nil {
				t.Fatal(err)
			}
			model := configMap(t, readConfig(t, path), "model")
			if model["api_mode"] != native {
				t.Fatalf("native protocol = %#v", model)
			}
			plan, err = Prepare(path, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := plan.Apply(); err != nil {
				t.Fatal(err)
			}
			if readConfig(t, path)["model"] != "previous-model" {
				t.Fatal("scalar model was not restored")
			}
		})
	}
	desired := testDesired()
	desired.Providers[0].Protocol = "unrecognized"
	if _, err := Prepare(filepath.Join(t.TempDir(), "config.yaml"), &desired); err == nil || !strings.Contains(err.Error(), "protocol") {
		t.Fatalf("protocol validation = %v", err)
	}
}

func configMap(t *testing.T, document map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := document[key].(map[string]any)
	if !ok {
		t.Fatalf("%s is not a mapping: %#v", key, document[key])
	}
	return value
}
