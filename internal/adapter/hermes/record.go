package hermes

import "go.yaml.in/yaml/v3"

// recordedNode is the form in which the ownership record keeps a YAML node.
// Its fields and their names are those of go.yaml.in/yaml/v3's Node, in that
// order, so a record keeps its bytes when the YAML library adds a field to
// its own node; released AIGW refuses a record with a field it does not know.
type recordedNode struct {
	Kind        uint32          `json:"Kind"`
	Style       uint32          `json:"Style"`
	Tag         string          `json:"Tag"`
	Value       string          `json:"Value"`
	Anchor      string          `json:"Anchor"`
	Alias       *recordedNode   `json:"Alias"`
	Content     []*recordedNode `json:"Content"`
	HeadComment string          `json:"HeadComment"`
	LineComment string          `json:"LineComment"`
	FootComment string          `json:"FootComment"`
	Line        int             `json:"Line"`
	Column      int             `json:"Column"`
}

// recordNode copies node into the record's form; nil stays nil.
func recordNode(node *yaml.Node) *recordedNode {
	if node == nil {
		return nil
	}
	recorded := &recordedNode{
		Kind: uint32(node.Kind), Style: uint32(node.Style), Tag: node.Tag, Value: node.Value, Anchor: node.Anchor,
		Alias: recordNode(node.Alias), HeadComment: node.HeadComment, LineComment: node.LineComment,
		FootComment: node.FootComment, Line: node.Line, Column: node.Column,
	}
	if node.Content != nil {
		recorded.Content = make([]*recordedNode, len(node.Content))
		for index, child := range node.Content {
			recorded.Content[index] = recordNode(child)
		}
	}
	return recorded
}

// node returns the library's node for a recorded one; nil stays nil.
func (recorded *recordedNode) node() *yaml.Node {
	if recorded == nil {
		return nil
	}
	node := &yaml.Node{
		Kind: yaml.Kind(recorded.Kind), Style: yaml.Style(recorded.Style), Tag: recorded.Tag, Value: recorded.Value,
		Anchor: recorded.Anchor, Alias: recorded.Alias.node(), HeadComment: recorded.HeadComment,
		LineComment: recorded.LineComment, FootComment: recorded.FootComment, Line: recorded.Line, Column: recorded.Column,
	}
	if recorded.Content != nil {
		node.Content = make([]*yaml.Node, len(recorded.Content))
		for index, child := range recorded.Content {
			node.Content[index] = child.node()
		}
	}
	return node
}
