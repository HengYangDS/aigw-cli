package configuration

import (
	"slices"

	"go.yaml.in/yaml/v3"
)

func selectionOnlyDrift(root *yaml.Node, state ownership, previous *Desired) bool {
	if previous == nil {
		return false
	}
	model := field(root, "model")
	selected := field(model, "provider")
	if selected == nil || !slices.Contains(state.ProviderIDs, selected.Value) {
		return false
	}
	provider := field(field(root, "providers"), selected.Value)
	for _, keys := range [][2]string{{"base_url", "base_url"}, {"api_mode", "transport"}} {
		actual, owned := field(model, keys[0]), field(provider, keys[1])
		if actual == nil || owned == nil || actual.Value != owned.Value {
			return false
		}
	}
	defaultModel, models := field(model, "default"), field(provider, "models")
	if defaultModel == nil || models == nil || !slices.ContainsFunc(models.Content, func(node *yaml.Node) bool { return node.Value == defaultModel.Value }) {
		return false
	}
	prior := previous.Providers[slices.IndexFunc(previous.Providers, func(provider Provider) bool { return provider.ID == previous.SelectedProvider })]
	replacement := mapping()
	setString(replacement, "provider", prior.ID)
	setString(replacement, "default", previous.SelectedModel)
	setString(replacement, "base_url", prior.Endpoint)
	protocol, _ := nativeProtocol(prior.Protocol)
	setString(replacement, "api_mode", protocol)
	setField(root, "model", replacement)
	defer setField(root, "model", model)
	proven, err := managedBytes(root, state.ProviderIDs)
	return err == nil && digest(proven) == state.ManagedDigest
}
