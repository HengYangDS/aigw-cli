package claude

import (
	"strings"

	"aigw-cli/internal/domain"
)

// anthropicModelPrefix begins every Anthropic model ID that Claude Code knows.
const anthropicModelPrefix = "claude-"

// modelOverrides is the map that Claude Code's settings carry for the
// selection: each Anthropic model ID that a listed model of the selected
// provider serves under another ID, to that ID. Claude Code then sends the
// provider's ID for each picker row and alias and runs it at that model's
// window. A family that is no Anthropic model ID stays out, since Claude Code
// ignores it. The result is nil when no listed model serves one.
func modelOverrides(runtime domain.Runtime) map[string]string {
	var overrides map[string]string
	for family, id := range runtime.FamilyVariants {
		if !strings.HasPrefix(family, anthropicModelPrefix) {
			continue
		}
		if overrides == nil {
			overrides = map[string]string{}
		}
		overrides[family] = id
	}
	return overrides
}
