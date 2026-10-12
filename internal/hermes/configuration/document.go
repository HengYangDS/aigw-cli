package configuration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/rogpeppe/go-internal/robustio"
	"go.yaml.in/yaml/v3"
)

var modelKeys = []string{"provider", "default", "base_url", "api_mode"}

const legacyProviderID = "aigw"

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

// PrepareVerification preserves native settings in an isolated projection and
// disables dependency installation and passive software-update checks.
func (plan *Plan) PrepareVerification() ([]byte, error) {
	if !plan.configBefore.Exists {
		return nil, errors.New("Hermes native configuration is missing; run aigw sync")
	}
	data := plan.configBefore.Data
	if plan.Action == "write" {
		data = plan.configAfter
	}
	root, err := parse(data)
	if err != nil {
		return nil, err
	}
	for _, policy := range []struct{ section, setting string }{
		{"security", "allow_lazy_installs"},
		{"updates", "check"},
		{"cli", "expose_on_path"},
	} {
		section := field(root, policy.section)
		if section == nil {
			section = mapping()
			setField(root, policy.section, section)
		}
		if section.Kind != yaml.MappingNode || section.Anchor != "" {
			return nil, fmt.Errorf("Hermes %s must be an independent mapping for verification", policy.section)
		}
		setField(section, policy.setting, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"})
	}
	return encode(root)
}

// CreateVerificationHome borrows committed dependencies, not their mutable boot
// state. Native dependency leases remain Hermes-owned and expire on process exit.
func CreateVerificationHome(target string) (string, func() error, error) {
	if runtime.GOOS == "windows" {
		return "", nil, errors.New("Hermes verification cannot isolate native launcher publication on Windows; no client process was started")
	}
	root := filepath.Dir(target)
	if filepath.Base(filepath.Dir(root)) == "profiles" {
		root = filepath.Dir(filepath.Dir(root))
	}
	home, err := os.MkdirTemp("", "aigw-hermes-verify-")
	if err != nil {
		return "", nil, errors.New("cannot create an isolated Hermes verification home")
	}
	cleanup := func() error { return robustio.RemoveAll(home) }
	fail := func(err error) (string, func() error, error) {
		return "", nil, errors.Join(err, cleanup())
	}
	physicalHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		return fail(errors.New("cannot resolve the isolated Hermes verification home"))
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return fail(errors.New("cannot resolve the native Hermes home for verification"))
	}
	for _, nativeRoot := range []string{root, filepath.Join(userHome, ".hermes")} {
		physicalRoot, err := filepath.EvalSymlinks(nativeRoot)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fail(errors.New("cannot resolve the native Hermes data root"))
		}
		if relative, err := filepath.Rel(physicalRoot, physicalHome); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return fail(errors.New("Hermes verification requires temporary storage outside its native data root"))
		}
	}
	for _, name := range []string{"tools", "installs"} {
		source := filepath.Join(root, name)
		if _, err := os.Stat(source); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return fail(errors.New("cannot read the installed Hermes dependency state"))
		}
		if name == "tools" {
			if err := os.Symlink(source, filepath.Join(home, name)); err != nil {
				return fail(errors.New("cannot reuse the installed Hermes tools without provisioning"))
			}
			continue
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return fail(errors.New("cannot read the installed Hermes dependency selections"))
		}
		for _, entry := range entries {
			installed := filepath.Join(source, entry.Name())
			info, err := os.Stat(installed)
			if err != nil {
				return fail(errors.New("cannot resolve the installed Hermes dependency selection"))
			}
			if !info.IsDir() {
				continue
			}
			if err := borrowVerificationInstall(installed, filepath.Join(home, "installs", entry.Name())); err != nil {
				return fail(err)
			}
		}
	}
	return home, cleanup, nil
}

func borrowVerificationInstall(installed, private string) error {
	if _, err := os.Stat(filepath.Join(installed, "facts.json")); err == nil {
		if lock, err := os.Stat(filepath.Join(installed, ".install.lock")); err != nil || !lock.Mode().IsRegular() {
			return errors.New("Hermes dependency selection lock is unavailable; restore the native installation before verification")
		}
	}
	if err := os.MkdirAll(private, 0o700); err != nil {
		return errors.New("cannot isolate Hermes boot maintenance")
	}
	for _, dependency := range []string{"facts.json", "environments", "pm-runtime", ".install.lock"} {
		borrowed := filepath.Join(installed, dependency)
		if _, err := os.Stat(borrowed); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return errors.New("cannot read a committed Hermes dependency")
		}
		if err := os.Symlink(borrowed, filepath.Join(private, dependency)); err != nil {
			return errors.New("cannot reuse a committed Hermes dependency without provisioning")
		}
	}
	return nil
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

func managedBytes(root *yaml.Node, providerIDs []string) ([]byte, error) {
	selected := mapping()
	setField(selected, "model", selectedModel(root))
	providers := mapping()
	for _, providerID := range providerIDs {
		setField(providers, providerID, field(field(root, "providers"), providerID))
	}
	if !slices.Contains(providerIDs, legacyProviderID) {
		setField(providers, legacyProviderID, field(field(root, "providers"), legacyProviderID))
	}
	setField(selected, "providers", providers)
	var value map[string]any
	if err := selected.Decode(&value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func nativeProtocol(wire string) (string, error) {
	protocol, found := map[string]string{
		"openai_responses":        "codex_responses",
		"anthropic":               "anthropic_messages",
		"openai_chat_completions": "chat_completions",
	}[wire]
	if !found {
		return "", fmt.Errorf("unsupported Hermes protocol %q", wire)
	}
	return protocol, nil
}

func project(root *yaml.Node, desired Desired) {
	model := field(root, "model")
	if model == nil || model.Kind != yaml.MappingNode {
		model = mapping()
		setField(root, "model", model)
	}
	setString(model, "provider", desired.SelectedProvider)
	setString(model, "default", desired.SelectedModel)
	selected := desired.Providers[slices.IndexFunc(desired.Providers, func(provider Provider) bool { return provider.ID == desired.SelectedProvider })]
	setString(model, "base_url", selected.Endpoint)
	protocol, _ := nativeProtocol(selected.Protocol)
	setString(model, "api_mode", protocol)
	providers := field(root, "providers")
	if providers == nil {
		providers = mapping()
		setField(root, "providers", providers)
	}
	for _, desiredProvider := range desired.Providers {
		provider := mapping()
		if desiredProvider.Name != "" {
			setString(provider, "name", desiredProvider.Name)
		}
		setString(provider, "base_url", desiredProvider.Endpoint)
		protocol, _ := nativeProtocol(desiredProvider.Protocol)
		setString(provider, "transport", protocol)
		setString(provider, "key_cmd", desiredProvider.CredentialCommand)
		models := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, modelID := range desiredProvider.Models {
			model := &yaml.Node{}
			model.SetString(modelID)
			models.Content = append(models.Content, model)
		}
		setField(provider, "models", models)
		discovery := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}
		setField(provider, "discover_models", discovery)
		setField(providers, desiredProvider.ID, provider)
	}
}

func ownedProviderOrder(root *yaml.Node, ids []string) []string {
	providers := field(root, "providers")
	var order []string
	if providers != nil {
		for index := 0; index < len(providers.Content); index += 2 {
			id := providers.Content[index].Value
			if slices.Contains(ids, id) {
				order = append(order, id)
			}
		}
	}
	return order
}
