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
		"aihubmix/kimi-k3.1-pro":      {"kimi-k3.1-pro", "kimi-k3.1-pro", domain.ProtocolChat},
		"dmxapi/kimi-k3.1-pro":        {"kimi-k3.1-pro", "kimi-k3.1-pro", domain.ProtocolResponses},
		"ucloud/kimi-k3.1-pro":        {"kimi-k3.1-pro", "kimi-k3.1-pro", domain.ProtocolResponses},
		"aihubmix/minimax-m3.1-flash": {"minimax-m3.1-flash", "minimax-m3.1-flash", domain.ProtocolResponses},
		"dmxapi/MiniMax-M3.1-Flash":   {"minimax-m3.1-flash", "MiniMax-M3.1-Flash", domain.ProtocolResponses},
		"ucloud/minimax-m3.1-flash":   {"minimax-m3.1-flash", "minimax-m3.1-flash", domain.ProtocolResponses},
		"aihubmix/solar-pro4":         {"solar-pro4", "solar-pro4", domain.ProtocolChat},
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
	// A superseded family stays only on a provider that holds no model of its
	// successor, by the owner's per-provider rule of 2026-11-18: a provider
	// that does not list the successor keeps the model that works on it.
	for retired, successor := range map[string]string{
		"deepseek-v4.1-flash": "deepseek-v4.1-flash-1017", "kimi-k3": "kimi-k3.1-pro", "minimax-m3": "minimax-m3.1-flash",
		"muse-spark-1.3": "muse-wonder", "qwen3.8-max": "qwen4-max", "qwen3.8-max-0902": "qwen4-max",
	} {
		families := map[string]map[string]bool{}
		for _, model := range manifest.Models {
			if families[model.Provider] == nil {
				families[model.Provider] = map[string]bool{}
			}
			families[model.Provider][model.Family] = true
		}
		for provider, held := range families {
			if held[retired] && held[successor] {
				t.Errorf("provider %s keeps superseded %q beside %q", provider, retired, successor)
			}
		}
	}
}

// The November 23 refresh admits each new provider model only on the provider and protocol on which it passed the
// three Hermes turns, replaces the Meta and StepFun previews, and drops the models that providers no longer list.
func TestTeamManifestKeepsTheNovember23Refresh(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	for id, expected := range map[string]struct {
		model, wire string
		protocol    domain.EndpointProtocol
	}{
		"aihubmix/gemini-4-neon":       {"gemini-4-neon", "gemini-4-neon", domain.ProtocolResponses},
		"dmxapi/gemini-4-carbon":       {"gemini-4-carbon", "gemini-4-carbon", domain.ProtocolResponses},
		"dmxapi/gemini-4-carbon-ssvip": {"gemini-4-carbon", "gemini-4-carbon-ssvip", domain.ProtocolResponses},
		"dmxapi/gpt-6.2-astra-ssvip":   {"gpt-6.2-astra", "gpt-6.2-astra-ssvip", domain.ProtocolResponses},
		"dmxapi/gpt-6.2-sol-ssvip":     {"gpt-6.2-sol", "gpt-6.2-sol-ssvip", domain.ProtocolResponses},
		"dmxapi/Ling-3.2-flash":        {"ling-3.2-flash", "Ling-3.2-flash", domain.ProtocolResponses},
		"ucloud/qwen4-max":             {"qwen4-max", "qwen4-max", domain.ProtocolResponses},
		"ucloud/muse-spark-1.3":        {"muse-spark-1.3", "muse-spark-1.3", domain.ProtocolChat},
		"aihubmix/muse-spark-1.3":      {"muse-spark-1.3", "muse-spark-1.3", domain.ProtocolChat},
	} {
		route, ok := teamModel(t, manifest, id)
		if !ok || route.Family != expected.model || route.ID != expected.wire ||
			!slices.Equal(route.AdmittedProtocols(), []domain.EndpointProtocol{expected.protocol}) {
			t.Errorf("qualified Route %q = %+v, want Model %q, wire %q and %s", id, route, expected.model, expected.wire, expected.protocol)
		}
	}
	for _, id := range []string{
		"dmxapi/muse-wonder", "ucloud/muse-wonder-1.0", "aihubmix/muse-wonder", "dmxapi/step-5-preview", "aihubmix/step-5-preview",
		"ucloud/gemini-4-carbon", "aihubmix/mercury-2.5", "ucloud/gemini-4-hydrogen", "suheai/gemini-4-argon", "suheai/gemini-4-carbon",
	} {
		if _, admitted := teamModel(t, manifest, id); admitted {
			t.Errorf("Route %q is in the shipped manifest", id)
		}
	}
}
