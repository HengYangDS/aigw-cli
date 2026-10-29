package configuration

import (
	"aigw-cli/internal/domain"
	"slices"
	"testing"
)

// The October 27 refresh admits each successor only on the Account and protocol
// that passed bounded inference and the native Hermes file-tool and recall
// journey; failed or unlisted Routes stay out.
func TestTeamManifestKeepsOnlyQualifiedRefreshRoutes(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	for id, expected := range map[string]struct {
		model, wire string
		protocol    domain.EndpointProtocol
	}{
		"aihubmix-kimi-k3.1-pro":      {"kimi-k3.1-pro", "kimi-k3.1-pro", domain.ProtocolOpenAIChatCompletions},
		"dmxapi-kimi-k3.1-pro":        {"kimi-k3.1-pro", "kimi-k3.1-pro", domain.ProtocolOpenAIResponses},
		"ucloud-kimi-k3.1-pro":        {"kimi-k3.1-pro", "kimi-k3.1-pro", domain.ProtocolOpenAIResponses},
		"aihubmix-minimax-m3.1-flash": {"minimax-m3.1-flash", "minimax-m3.1-flash", domain.ProtocolOpenAIResponses},
		"dmxapi-minimax-m3.1-flash":   {"minimax-m3.1-flash", "MiniMax-M3.1-Flash", domain.ProtocolOpenAIResponses},
		"ucloud-minimax-m3.1-flash":   {"minimax-m3.1-flash", "minimax-m3.1-flash", domain.ProtocolOpenAIResponses},
		"aihubmix-muse-wonder":        {"muse-wonder", "muse-wonder", domain.ProtocolOpenAIResponses},
		"dmxapi-muse-wonder":          {"muse-wonder", "muse-wonder", domain.ProtocolOpenAIResponses},
		"ucloud-muse-wonder":          {"muse-wonder", "muse-wonder-1.0", domain.ProtocolOpenAIResponses},
		"aihubmix-qwen3.8-max-0902":   {"qwen3.8-max-0902", "qwen3.8-max-2026-09-02", domain.ProtocolOpenAIResponses},
		"dmxapi-qwen3.8-max-0902":     {"qwen3.8-max-0902", "qwen3.8-max-0902", domain.ProtocolOpenAIResponses},
		"aihubmix-solar-pro4":         {"solar-pro4", "solar-pro4", domain.ProtocolOpenAIChatCompletions},
	} {
		route, ok := manifest.Routes[id]
		if !ok || route.Model != expected.model || route.UpstreamModelID() != expected.wire ||
			!slices.Equal(route.AdmittedProtocols(), []domain.EndpointProtocol{expected.protocol}) {
			t.Errorf("qualified Route %q = %+v, want Model %q, wire %q and %s", id, route, expected.model, expected.wire, expected.protocol)
		}
	}
	// UCloud lists only the undated Qwen ID, whose build its Account does not
	// document; AIHubMix serves Kimi K3.1 Pro only on Chat Completions, as
	// asserted above.
	for _, id := range []string{"ucloud-qwen3.8-max-0902", "ucloud-qwen3.8-max"} {
		if _, admitted := manifest.Routes[id]; admitted {
			t.Errorf("unqualified Route %q was admitted", id)
		}
	}
	for _, retired := range []string{"deepseek-v4.1-flash", "kimi-k3", "minimax-m3", "muse-spark-1.3", "qwen3.8-max"} {
		if _, present := manifest.Models[retired]; present {
			t.Errorf("superseded Model %q remains a logical Model", retired)
		}
	}
}
