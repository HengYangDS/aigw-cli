package codex

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
)

// codexProviderProjection is the provider value AIGW writes and subsequently owns.
// Its source tables may move independently of surrounding comments or user tables.
type codexProviderProjection struct {
	Name    *string `toml:"name"`
	BaseURL string  `toml:"base_url"`
	WireAPI string  `toml:"wire_api"`
	Auth    *struct {
		Command string   `toml:"command"`
		Args    []string `toml:"args"`
	} `toml:"auth"`
}

func (projection codexProviderProjection) render(provider string) string {
	block := codexProviderTable(provider) + "\n"
	if projection.Name != nil {
		block += "name = " + strconv.Quote(*projection.Name) + "\n"
	}
	block += "base_url = " + strconv.Quote(projection.BaseURL) + "\nwire_api = " + strconv.Quote(projection.WireAPI) + "\n"
	if projection.Auth != nil {
		args := make([]string, len(projection.Auth.Args))
		for index, arg := range projection.Auth.Args {
			args[index] = strconv.Quote(arg)
		}
		block += "\n[model_providers." + provider + ".auth]\ncommand = " + strconv.Quote(projection.Auth.Command) + "\nargs = [" + strings.Join(args, ", ") + "]\n"
	}
	return block + codexEnd + "\n"
}

func codexProviderRanges(text, provider string) ([][2]int, error) {
	var parser unstable.Parser
	parser.Reset([]byte(text))
	var ranges [][2]int
	owned := false
	for parser.NextExpression() {
		node := parser.Expression()
		if node.Kind == unstable.Table || node.Kind == unstable.ArrayTable {
			keys := node.Key()
			owned = keys.Next() && string(keys.Node().Data) == "model_providers" &&
				keys.Next() && string(keys.Node().Data) == provider
			if owned {
				start := strings.LastIndexByte(text[:node.Child().Raw.Offset], '\n') + 1
				ranges = append(ranges, [2]int{start, codexSourceLineEnd(text, start)})
			}
			continue
		}
		if node.Kind == unstable.KeyValue && owned {
			ranges[len(ranges)-1][1] = codexSourceLineEnd(text, int(node.Raw.Offset+node.Raw.Length))
		}
	}
	if err := parser.Error(); err != nil {
		return nil, fmt.Errorf("parse Codex provider tables: %w", err)
	}
	if len(ranges) == 0 {
		return nil, fmt.Errorf("Codex config conflict: AIGW-managed provider block is missing")
	}
	return ranges, nil
}

func codexSourceLineEnd(text string, start int) int {
	if remaining := strings.IndexByte(text[start:], '\n'); remaining >= 0 {
		return start + remaining + 1
	}
	return len(text)
}

func codexManagedBlockForProviderIn(current, provider string) (string, error) {
	ranges, err := codexProviderRanges(current, provider)
	if err != nil {
		return "", err
	}
	var source strings.Builder
	for _, span := range ranges {
		source.WriteString(current[span[0]:span[1]])
		source.WriteByte('\n')
	}
	var document struct {
		Providers map[string]codexProviderProjection `toml:"model_providers"`
	}
	decoder := toml.NewDecoder(strings.NewReader(source.String())).DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return "", fmt.Errorf("Codex config conflict: AIGW-managed provider block changed: %w", err)
	}
	projection := document.Providers[provider]
	if projection.BaseURL == "" || projection.WireAPI != "responses" ||
		projection.Auth != nil && (projection.Auth.Command == "" || len(projection.Auth.Args) == 0) {
		return "", fmt.Errorf("Codex config conflict: AIGW-managed provider block is incomplete")
	}
	return projection.render(provider), nil
}

func removeCodexProviderMarkers(text string) string {
	parser := unstable.Parser{KeepComments: true}
	parser.Reset([]byte(text))
	var markers [][2]int
	for parser.NextExpression() {
		node := parser.Expression()
		if node.Kind != unstable.Comment || string(node.Data) != codexBegin && string(node.Data) != codexEnd {
			continue
		}
		start := strings.LastIndexByte(text[:node.Raw.Offset], '\n') + 1
		markers = append(markers, [2]int{start, codexSourceLineEnd(text, int(node.Raw.Offset+node.Raw.Length))})
	}
	for _, span := range slices.Backward(markers) {
		text = text[:span[0]] + text[span[1]:]
	}
	return text
}
