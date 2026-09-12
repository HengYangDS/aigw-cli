package markdown

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestCheckPolicyUsesBundledSchemas(t *testing.T) {
	repository, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name    string
		content string
		valid   bool
	}{
		{"valid", "MD060: { style: aligned }\n", true},
		{"unknown option", "unknown_option: true\n", false},
		{"unknown rule", "MD999: true\n", false},
		{"invalid rule option", "MD060: { style: invalid }\n", false},
		{"wrong option type", "globs: true\n", false},
		{"malformed YAML", "MD060: [\n", false},
		{"null policy", "null\n", false},
		{"scalar policy", "true\n", false},
		{"non-string key", "1: true\n", false},
		{"duplicate key", "MD060: true\nMD060: true\n", false},
		{"multiple documents", "MD060: true\n---\nunknown_option: true\n", false},
		{"scope is not a rule", "ignores: ['**/*']\n", false},
		{"warning severity", "MD060: warning\n", true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root := markdownPolicyCheckout(t, repository)
			path := filepath.Join(root, ".config", "checks", "markdown", "policy.yaml")
			if err := os.WriteFile(path, []byte(scenario.content), 0o600); err != nil {
				t.Fatal(err)
			}
			err := CheckPolicy(root)
			if (err == nil) != scenario.valid {
				t.Fatalf("Markdown policy validity = %t, want %t: %v", err == nil, scenario.valid, err)
			}
			actual, readErr := os.ReadFile(path)
			if readErr != nil || string(actual) != scenario.content {
				t.Fatalf("validation changed its input: %v", readErr)
			}
		})
	}
}

func markdownPolicyCheckout(t *testing.T, repository string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "checkout with spaces")
	for _, relative := range []string{
		".config/checks/markdown/policy.yaml",
		"node_modules/markdownlint/schema/markdownlint-config-schema-strict.json",
	} {
		content, err := os.ReadFile(filepath.Join(repository, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestCheckPolicyRequiresEveryLocalInput(t *testing.T) {
	repository, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{
		".config/checks/markdown/policy.yaml",
		"node_modules/markdownlint/schema/markdownlint-config-schema-strict.json",
	} {
		t.Run(relative, func(t *testing.T) {
			root := markdownPolicyCheckout(t, repository)
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(relative))); err != nil {
				t.Fatal(err)
			}
			if err := CheckPolicy(root); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("missing policy input was not reported: %v", err)
			}
		})
	}
}

func TestBundledSchemaRequiresValidSelfIdentifiedInput(t *testing.T) {
	for _, scenario := range []struct{ name, source, diagnostic string }{
		{"invalid JSON", "{", "decode bundled schema"},
		{"missing identity", `{ "type": "object" }`, "has no identity"},
		{"empty identity", `{ "$id": "", "type": "object" }`, "has no identity"},
		{"invalid schema", `{ "$id": "https://example.invalid/schema", "type": "unknown" }`, "compile bundled schema"},
		{"external reference", `{ "$id": "https://example.invalid/schema", "$ref": "https://example.invalid/remote" }`, "no URLLoader set"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "schema.json")
			if err := os.WriteFile(path, []byte(scenario.source), 0o600); err != nil {
				t.Fatal(err)
			}
			compiler := jsonschema.NewCompiler()
			compiler.UseLoader(nil)
			if _, err := bundledSchema(compiler, path); err == nil || !strings.Contains(err.Error(), scenario.diagnostic) {
				t.Fatalf("schema failure did not identify %q: %v", scenario.diagnostic, err)
			}
		})
	}
}

func TestBundledSchemaRejectsCompetingIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(path, []byte(`{"$id":"https://example.invalid/schema","type":"object"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(nil)
	if _, err := bundledSchema(compiler, path); err != nil {
		t.Fatal(err)
	}
	if _, err := bundledSchema(compiler, path); err == nil {
		t.Fatal("schema identity was silently replaced")
	}
}
