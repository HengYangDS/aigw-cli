package hermes

import (
	"os"
	"slices"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/platform"

	"go.yaml.in/yaml/v4"
)

// listedIDs returns the IDs of models, in their order.
func listedIDs(models []Model) []string {
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	return ids
}

// projectedFile parses the Hermes configuration at path.
func projectedFile(t *testing.T, path string) *yaml.Node {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// writtenModels decodes the models of a provider entry as AIGW writes them: a
// mapping from each ID to a mapping that holds exactly the integer
// `context_length`. It returns them in the file's order.
func writtenModels(t *testing.T, entry *yaml.Node) []Model {
	t.Helper()
	models := field(entry, "models")
	if models == nil || models.Kind != yaml.MappingNode {
		t.Fatalf("the entry's models are not a mapping: %+v", models)
	}
	written := make([]Model, 0, len(models.Content)/2)
	for index := 0; index+1 < len(models.Content); index += 2 {
		id := models.Content[index].Value
		var settings map[string]int
		if err := models.Content[index+1].Decode(&settings); err != nil {
			t.Fatalf("the settings of model %s: %v", id, err)
		}
		window, found := settings["context_length"]
		if !found || len(settings) != 1 {
			t.Fatalf("the settings of model %s = %v, want exactly context_length", id, settings)
		}
		written = append(written, Model{ID: id, ContextWindow: window})
	}
	return written
}

// modelShape is one shape in which a file that AIGW wrote lists the models of
// a provider entry. rewrite turns the file that this code projected at path,
// with its record, into that shape. kept says whether a projection of the same
// models leaves a file of that shape byte for byte.
type modelShape struct {
	name    string
	rewrite func(t *testing.T, path string)
	kept    bool
}

var modelShapes = []modelShape{
	{name: "a mapping from ID", rewrite: func(*testing.T, string) {}, kept: true},
	{name: "a list of IDs", rewrite: earlierRelease},
}

// earlierRelease rewrites the Hermes configuration that this code projected at
// path as a release before the mapping wrote it: each provider entry that AIGW
// owns lists its models as a sequence of IDs, and AIGW's record holds the
// digest of that file.
func earlierRelease(t *testing.T, path string) {
	t.Helper()
	root := projectedFile(t, path)
	recorded, err := os.ReadFile(path + adapter.StateSuffix)
	if err != nil {
		t.Fatal(err)
	}
	state, err := decodeOwnership(recorded)
	if err != nil {
		t.Fatal(err)
	}
	for _, providerID := range state.ProviderIDs {
		entry := field(field(root, "providers"), providerID)
		ids := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, model := range writtenModels(t, entry) {
			id := &yaml.Node{}
			id.SetString(model.ID)
			ids.Content = append(ids.Content, id)
		}
		setField(entry, "models", ids)
	}
	owned, err := managedBytes(root, state.ProviderIDs)
	if err != nil {
		t.Fatal(err)
	}
	state.ManagedDigest = platform.SHA256(owned)
	config, err := encode(root)
	if err != nil {
		t.Fatal(err)
	}
	record, err := adapter.EncodeJSON(state)
	if err != nil {
		t.Fatal(err)
	}
	for target, data := range map[string][]byte{path: config, path + adapter.StateSuffix: record} {
		if err := os.WriteFile(target, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// entryModels returns the `models` node of a provider entry written as text,
// or nil when the entry has none.
func entryModels(t *testing.T, entry string) *yaml.Node {
	t.Helper()
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(entry), &document); err != nil {
		t.Fatal(err)
	}
	return field(document.Content[0], "models")
}

// TestModelIDsReadsBothShapesOfAProvidersModels reads the models that a provider
// entry lists as the mapping from ID that AIGW writes and as the list of IDs
// that an earlier release wrote, in the node's order, and no ID from any
// other shape.
func TestModelIDsReadsBothShapesOfAProvidersModels(t *testing.T) {
	for _, shape := range []struct {
		name  string
		entry string
		want  []string
	}{
		{name: "a mapping from ID", entry: lines(
			"models:",
			"  wire-z:",
			"    context_length: 256000",
			"  wire-a:",
			"    context_length: 1000000",
		), want: []string{"wire-z", "wire-a"}},
		{name: "a list of IDs", entry: lines(
			"models:",
			"  - wire-z",
			"  - wire-a",
		), want: []string{"wire-z", "wire-a"}},
		{name: "an empty mapping", entry: lines("models: {}")},
		{name: "an empty list", entry: lines("models: []")},
		{name: "an empty value", entry: lines("models:")},
		{name: "a missing node", entry: lines("name: Entry")},
		{name: "a scalar", entry: lines("models: wire-z")},
		{name: "a list of mappings", entry: lines(
			"models:",
			"  - id: wire-z",
			"  - id: wire-a",
		)},
		{name: "a list that holds a mapping", entry: lines(
			"models:",
			"  - wire-z",
			"  - id: wire-a",
		)},
		{name: "a mapping to a value that is no mapping", entry: lines(
			"models:",
			"  wire-z:",
			"    context_length: 256000",
			"  wire-a: 1000000",
		)},
	} {
		t.Run(shape.name, func(t *testing.T) {
			if got := modelIDs(entryModels(t, shape.entry)); !slices.Equal(got, shape.want) {
				t.Fatalf("the listed model IDs = %q, want %q", got, shape.want)
			}
		})
	}
}
