package hermes

import (
	"slices"
	"strconv"

	"go.yaml.in/yaml/v4"
)

// Model is one model that a Hermes provider entry lists: its ID, and the
// context window, in tokens, at which Hermes runs it.
type Model struct {
	ID            string
	ContextWindow int
}

// listsModel reports whether models holds the model with the given ID.
func listsModel(models []Model, id string) bool {
	return slices.ContainsFunc(models, func(model Model) bool { return model.ID == id })
}

// modelsNode is the `models` value that AIGW writes for a provider entry: a
// mapping, in the order of models, from each ID to the one setting that AIGW
// owns for a model, its context window as Hermes' `context_length`.
func modelsNode(models []Model) *yaml.Node {
	node := mapping()
	for _, model := range models {
		settings := mapping()
		setField(settings, "context_length", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(model.ContextWindow)})
		setField(node, model.ID, settings)
	}
	return node
}

// modelsInOrder reports whether root's entry of each desired provider lists
// that provider's models in their order. The digest of the owned settings does
// not see the order of a mapping's keys, so a projection compares it here.
func modelsInOrder(root *yaml.Node, desired Desired) bool {
	providers := field(root, "providers")
	for _, provider := range desired.Providers {
		listed := modelIDs(field(field(providers, provider.ID), "models"))
		if !slices.EqualFunc(listed, provider.Models, func(id string, model Model) bool { return id == model.ID }) {
			return false
		}
	}
	return true
}

// modelIDs returns the IDs of the models that a provider entry lists, in the
// order of its `models` node. It reads the two shapes that AIGW has written:
// the mapping from each ID to that model's settings, and the sequence of IDs
// that the releases before the mapping wrote. A node of any other shape lists
// no model.
func modelIDs(models *yaml.Node) []string {
	if models == nil {
		return nil
	}
	var ids []string
	if models.Kind == yaml.SequenceNode {
		for _, id := range models.Content {
			if id.Kind != yaml.ScalarNode {
				return nil
			}
			ids = append(ids, id.Value)
		}
	}
	if models.Kind == yaml.MappingNode {
		for index := 0; index+1 < len(models.Content); index += 2 {
			id, settings := models.Content[index], models.Content[index+1]
			if id.Kind != yaml.ScalarNode || settings.Kind != yaml.MappingNode {
				return nil
			}
			ids = append(ids, id.Value)
		}
	}
	return ids
}
