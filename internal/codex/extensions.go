package codex

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2/unstable"
)

// codexExtensionKeys are the configuration roots under which Codex starts the
// operator's own processes or connects to services under its authorization.
var codexExtensionKeys = []string{"hooks", "marketplaces", "mcp_servers", "memories", "notify", "plugins"}

// withoutCodexExtensions keeps every setting of a Codex configuration and drops
// the operator's extensions, so a verification home neither starts them nor
// lets their failures decide the result.
func withoutCodexExtensions(text string) (string, error) {
	var parser unstable.Parser
	parser.Reset([]byte(text))
	var ranges [][2]int
	inTable, extension := false, false
	for parser.NextExpression() {
		node := parser.Expression()
		if node.Kind == unstable.Table || node.Kind == unstable.ArrayTable {
			keys := node.Key()
			inTable = true
			extension = keys.Next() && slices.Contains(codexExtensionKeys, string(keys.Node().Data))
			if extension {
				start := strings.LastIndexByte(text[:node.Child().Raw.Offset], '\n') + 1
				ranges = append(ranges, [2]int{start, codexSourceLineEnd(text, start)})
			}
			continue
		}
		if node.Kind != unstable.KeyValue {
			continue
		}
		end := codexSourceLineEnd(text, int(node.Raw.Offset+node.Raw.Length))
		if extension {
			ranges[len(ranges)-1][1] = end
			continue
		}
		keys := node.Key()
		if !inTable && keys.Next() && slices.Contains(codexExtensionKeys, string(keys.Node().Data)) {
			start := strings.LastIndexByte(text[:keys.Node().Raw.Offset], '\n') + 1
			ranges = append(ranges, [2]int{start, end})
		}
	}
	if err := parser.Error(); err != nil {
		return "", fmt.Errorf("parse Codex configuration: %w", err)
	}
	for _, span := range slices.Backward(ranges) {
		text = text[:span[0]] + text[span[1]:]
	}
	return text, validateCodexTOML(text)
}
