// Package markdown validates repository Markdown policy against locked upstream schemas.
package markdown

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
)

// CheckPolicy validates one checkout's policy without fetching schemas or changing files.
func CheckPolicy(root string) error {
	path := filepath.Join(root, ".config", "checks", "markdown", "policy.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var document any
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("decode Markdown policy: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("Markdown policy must contain exactly one YAML document")
	}
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(nil)
	rulesSchema, err := bundledSchema(compiler, filepath.Join(root, "node_modules", "markdownlint", "schema", "markdownlint-config-schema-strict.json"))
	if err != nil {
		return err
	}
	if err := rulesSchema.Validate(document); err != nil {
		return fmt.Errorf("validate Markdown rules: %w", err)
	}
	return nil
}

func bundledSchema(compiler *jsonschema.Compiler, path string) (*jsonschema.Schema, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document map[string]any
	if err := json.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("decode bundled schema %s: %w", path, err)
	}
	identity, ok := document["$id"].(string)
	if !ok || identity == "" {
		return nil, fmt.Errorf("bundled schema %s has no identity", path)
	}
	if err := compiler.AddResource(identity, document); err != nil {
		return nil, err
	}
	compiled, err := compiler.Compile(identity)
	if err != nil {
		return nil, fmt.Errorf("compile bundled schema %s: %w", path, err)
	}
	return compiled, nil
}
