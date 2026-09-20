package configuration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.yaml.in/yaml/v3"
)

var modelKeys = []string{"provider", "default", "base_url", "api_mode"}

func mapping() *yaml.Node { return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"} }

func field(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			return node.Content[index+1]
		}
	}
	return nil
}

func setField(node *yaml.Node, key string, value *yaml.Node) {
	for index := 0; index < len(node.Content); index += 2 {
		if node.Content[index].Value != key {
			continue
		}
		if value == nil {
			node.Content = append(node.Content[:index], node.Content[index+2:]...)
		} else {
			node.Content[index+1] = value
		}
		return
	}
	if value != nil {
		name := &yaml.Node{}
		name.SetString(key)
		node.Content = append(node.Content, name, value)
	}
}

func setString(node *yaml.Node, key, value string) {
	child := &yaml.Node{}
	child.SetString(value)
	setField(node, key, child)
}

func parse(data []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	err := decoder.Decode(&document)
	if errors.Is(err, io.EOF) {
		return mapping(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read Hermes YAML: %w", err)
	}
	if err := decoder.Decode(&yaml.Node{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("Hermes configuration must contain exactly one YAML document")
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("Hermes configuration must be a YAML mapping")
	}
	var checked map[string]any
	if err := root.Decode(&checked); err != nil {
		return nil, fmt.Errorf("ambiguous Hermes configuration: %w", err)
	}
	for _, key := range []string{"model", "providers"} {
		node := field(root, key)
		if node == nil {
			continue
		}
		if node.Anchor != "" || node.Kind == yaml.AliasNode {
			return nil, fmt.Errorf("Hermes %s uses a shared YAML anchor; use an independent value before enabling projection", key)
		}
		if key == "model" && node.Kind == yaml.ScalarNode && node.Tag == "!!str" {
			continue
		}
		if node.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("Hermes %s must be a mapping", key)
		}
	}
	return root, nil
}

func encode(root *yaml.Node) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(root); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func selectedModel(root *yaml.Node) *yaml.Node {
	model := field(root, "model")
	if model == nil || model.Kind != yaml.MappingNode {
		return model
	}
	selected := mapping()
	for _, key := range modelKeys {
		setField(selected, key, field(model, key))
	}
	return selected
}

func managedBytes(root *yaml.Node) ([]byte, error) {
	selected := mapping()
	setField(selected, "model", selectedModel(root))
	setField(selected, "provider", field(field(root, "providers"), "aigw"))
	var value map[string]any
	if err := selected.Decode(&value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func (route Route) nativeProtocol() (string, error) {
	protocol, found := map[string]string{
		"openai_responses":        "codex_responses",
		"anthropic":               "anthropic_messages",
		"openai_chat_completions": "chat_completions",
	}[route.Protocol]
	if !found {
		return "", fmt.Errorf("unsupported Hermes protocol %q", route.Protocol)
	}
	for name, value := range map[string]string{"model": route.Model, "endpoint": route.Endpoint, "credential command": route.CredentialCommand} {
		if strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("Hermes %s is required", name)
		}
	}
	return protocol, nil
}

func project(root *yaml.Node, route Route, protocol string) {
	model := field(root, "model")
	if model == nil || model.Kind != yaml.MappingNode {
		model = mapping()
		setField(root, "model", model)
	}
	setString(model, "provider", "aigw")
	setString(model, "default", route.Model)
	setString(model, "base_url", route.Endpoint)
	setString(model, "api_mode", protocol)
	providers := field(root, "providers")
	if providers == nil {
		providers = mapping()
		setField(root, "providers", providers)
	}
	provider := mapping()
	setString(provider, "base_url", route.Endpoint)
	setString(provider, "transport", protocol)
	setString(provider, "key_cmd", route.CredentialCommand)
	setField(providers, "aigw", provider)
}
