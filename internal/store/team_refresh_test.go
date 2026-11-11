package store

import (
	"aigw-cli/internal/domain"
	"slices"
	"testing"
)

// The October 27 refresh admits each successor only on the Account and protocol
// that passed bounded inference and the native Hermes file-tool and recall
// journey; failed or unlisted Routes stay out.
func TestTeamManifestKeepsOnlyQualifiedRefreshModels(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	for id, expected := range map[string]struct {
		model, wire string
		protocol    domain.EndpointProtocol
	}{
		"aihubmix/kimi-k3.1-pro":          {"kimi-k3.1-pro", "kimi-k3.1-pro", domain.ProtocolChat},
		"dmxapi/kimi-k3.1-pro":            {"kimi-k3.1-pro", "kimi-k3.1-pro", domain.ProtocolResponses},
		"ucloud/kimi-k3.1-pro":            {"kimi-k3.1-pro", "kimi-k3.1-pro", domain.ProtocolResponses},
		"aihubmix/minimax-m3.1-flash":     {"minimax-m3.1-flash", "minimax-m3.1-flash", domain.ProtocolResponses},
		"dmxapi/MiniMax-M3.1-Flash":       {"minimax-m3.1-flash", "MiniMax-M3.1-Flash", domain.ProtocolResponses},
		"ucloud/minimax-m3.1-flash":       {"minimax-m3.1-flash", "minimax-m3.1-flash", domain.ProtocolResponses},
		"aihubmix/muse-wonder":            {"muse-wonder", "muse-wonder", domain.ProtocolResponses},
		"dmxapi/muse-wonder":              {"muse-wonder", "muse-wonder", domain.ProtocolResponses},
		"ucloud/muse-wonder-1.0":          {"muse-wonder", "muse-wonder-1.0", domain.ProtocolResponses},
		"aihubmix/qwen3.8-max-2026-09-02": {"qwen3.8-max-0902", "qwen3.8-max-2026-09-02", domain.ProtocolResponses},
		"dmxapi/qwen3.8-max-0902":         {"qwen3.8-max-0902", "qwen3.8-max-0902", domain.ProtocolResponses},
		"aihubmix/solar-pro4":             {"solar-pro4", "solar-pro4", domain.ProtocolChat},
	} {
		route, ok := teamModel(t, manifest, id)
		if !ok || route.Family != expected.model || route.ID != expected.wire ||
			!slices.Equal(route.AdmittedProtocols(), []domain.EndpointProtocol{expected.protocol}) {
			t.Errorf("qualified Route %q = %+v, want Model %q, wire %q and %s", id, route, expected.model, expected.wire, expected.protocol)
		}
	}
	// UCloud lists only the undated Qwen ID, whose build its Account does not
	// document; AIHubMix serves Kimi K3.1 Pro only on Chat Completions, as
	// asserted above.
	for _, id := range []string{"ucloud/qwen3.8-max-0902", "ucloud/qwen3.8-max"} {
		if _, admitted := teamModel(t, manifest, id); admitted {
			t.Errorf("unqualified Route %q was admitted", id)
		}
	}
	for _, retired := range []string{"deepseek-v4.1-flash", "kimi-k3", "minimax-m3", "muse-spark-1.3", "qwen3.8-max"} {
		if _, present := manifest.Families[retired]; present {
			t.Errorf("superseded Model %q remains a logical Model", retired)
		}
	}
}
