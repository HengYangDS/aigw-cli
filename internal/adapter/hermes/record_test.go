package hermes

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"aigw-cli/internal/adapter"

	"go.yaml.in/yaml/v4"
)

// TestRecordsOfTheReleasedFormRead decodes each pinned ownership record, which
// the Hermes code of AIGW 0.8.1 wrote, and encodes it again: the record reads,
// and its form keeps every byte.
func TestRecordsOfTheReleasedFormRead(t *testing.T) {
	records, err := filepath.Glob(filepath.Join("testdata", "projection-bytes", "*", "record.json.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) == 0 {
		t.Fatal("no pinned ownership record")
	}
	for _, path := range records {
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			state, err := decodeOwnership(want)
			if err != nil {
				t.Fatal(err)
			}
			got, err := adapter.EncodeJSON(state)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("the record changed when read and written again\n got: %q\nwant: %q", got, want)
			}
		})
	}
}

// TestRecordedNodeKeepsTheLibraryNode converts a node with every field set to
// the record's form and back.
func TestRecordedNodeKeepsTheLibraryNode(t *testing.T) {
	alias := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "target", Anchor: "shared"}
	node := &yaml.Node{
		Kind: yaml.MappingNode, Style: yaml.FlowStyle, Tag: "!!map", Anchor: "outer",
		HeadComment: "# head", LineComment: "# line", FootComment: "# foot", Line: 3, Column: 5,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Style: yaml.DoubleQuotedStyle, Tag: "!!str", Value: "key"},
			{Kind: yaml.AliasNode, Value: "shared", Alias: alias},
		},
	}
	got := recordNode(node).node()
	if !reflect.DeepEqual(got, node) {
		t.Fatalf("round trip changed the node\n got: %#v\nwant: %#v", got, node)
	}
	if recordNode(nil) != nil || (*recordedNode)(nil).node() != nil {
		t.Fatal("a missing node gained a value")
	}
	empty := recordNode(&yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{}}).node()
	if empty.Content == nil {
		t.Fatal("an empty content list became a missing one")
	}
}
