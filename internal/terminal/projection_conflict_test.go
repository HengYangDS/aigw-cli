package terminal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/domain"
)

func TestManagedProviderSchemaConflictIsNotTOMLSyntax(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := domain.Runtime{
		Client: codex.ID, RouteID: "test", RouteLabel: "Test",
		AccountID: "test", Endpoint: "https://endpoint.test/v1", Model: "gpt-6.1-sol",
		ModelProvider: "test-provider", Authentication: domain.AuthenticationClientNative,
	}
	if err := codex.SyncConfig(path, runtime); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(data), "wire_api = \"responses\"", "wire_api = \"responses\"\nunknown_budget = 0", 1)
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	conflict := codex.ValidateConfig(path, runtime)
	if conflict == nil {
		t.Fatal("fixture did not produce a managed schema conflict")
	}
	for _, jsonMode := range []bool{false, true} {
		var out bytes.Buffer
		renderer := NewRenderer(&out, false)
		RenderError(renderer, fmt.Errorf("sync preflight: %w", conflict), jsonMode)
		if renderer.Err() != nil || !strings.Contains(out.String(), "Codex config conflict") || strings.Contains(out.String(), "Cannot decode TOML") {
			t.Fatalf("semantic conflict was misclassified: %q (%v)", out.String(), renderer.Err())
		}
	}
}
