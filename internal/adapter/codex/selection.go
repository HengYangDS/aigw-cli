package codex

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2/unstable"
)

// codexTOMLString serializes one admitted single-line selection. JSON string
// escaping is valid TOML basic-string syntax for this domain; native encoding
// replaces hand-written quoting while the admission rejects lossy UTF-8.
func codexTOMLString(value string) (string, error) {
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("Codex selection contains invalid UTF-8")
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return "", fmt.Errorf("Codex selection contains a control character")
		}
	}
	encoded, err := json.Marshal(value)
	return string(encoded), err
}

// codexSelectionBounds locates one root assignment without interpreting text
// inside strings, dotted keys, or client-owned tables as a managed selection.
// The native parser owns TOML grammar; the returned range includes indentation
// and the original trailing comment, but not the line ending.
func codexSelectionBounds(text, key string) (int, int, error) {
	var parser unstable.Parser
	parser.Reset([]byte(text))
	start, end := -1, -1
	root := true
	for parser.NextExpression() {
		node := parser.Expression()
		if node.Kind == unstable.Table || node.Kind == unstable.ArrayTable {
			root = false
		}
		if node.Kind != unstable.KeyValue || !root {
			continue
		}
		keys := node.Key()
		if !keys.Next() || !keys.IsLast() || string(keys.Node().Data) != key {
			continue
		}
		if start >= 0 {
			return -1, -1, fmt.Errorf("ambiguous Codex selection %q", key)
		}
		start = int(node.Raw.Offset)
		start = strings.LastIndexByte(text[:start], '\n') + 1
		end = int(node.Raw.Offset + node.Raw.Length)
		if remaining := strings.IndexByte(text[end:], '\n'); remaining >= 0 {
			end += remaining
		} else {
			end = len(text)
		}
		if end > start && text[end-1] == '\r' {
			end--
		}
	}
	if err := parser.Error(); err != nil {
		return -1, -1, fmt.Errorf("parse Codex selection: %w", err)
	}
	return start, end, nil
}

func codexSelectionLine(text, key string) (string, error) {
	start, end, err := codexSelectionBounds(text, key)
	if err != nil || start < 0 {
		return "", err
	}
	return text[start:end], nil
}

// setCodexSelection replaces literal source bytes, never regular-expression
// replacement syntax. An empty assignment withdraws the selected root key.
func setCodexSelection(text, key, assignment string) (string, error) {
	start, end, err := codexSelectionBounds(text, key)
	if err != nil {
		return "", err
	}
	if start < 0 {
		if assignment == "" {
			return text, nil
		}
		return assignment + "\n" + text, nil
	}
	if assignment != "" {
		return text[:start] + assignment + text[end:], nil
	}
	if end < len(text) && text[end] == '\r' {
		end++
	}
	if end < len(text) && text[end] == '\n' {
		end++
	}
	result := text[:start] + text[end:]
	if !strings.HasSuffix(text, "\n") {
		result = strings.TrimSuffix(result, "\n")
	}
	return result, nil
}
