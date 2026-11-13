package store

import (
	"aigw-cli/internal/domain"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestTeamManifestKeepsOneFamilyPerCuratedLine(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	// The owner's rule of 2026-11-13: each Anthropic, OpenAI and Google product
	// line keeps only its newest model, so a successor such as GPT-6.2 Astra
	// replaces GPT-6 Astra instead of joining it.
	// A family of those vendors that names no line fails too: skipping it would
	// let gpt-7, or a dated gpt-6.2-astra-1120, stand beside the line it replaces.
	line := regexp.MustCompile(`^(?:(claude)-([a-z]+)-\d|(gpt)-\d[\d.]*-([a-z]+)(?:-|$)|(gemini)-\d[\d.]*-([a-z]+)(?:-|$))`)
	// The vendor word counts wherever it stands and however it is spelled, so
	// GPT-6.2-astra, gpt_6.2-astra or openai-gpt-6.2-astra cannot slip past.
	curated := regexp.MustCompile(`(?i)claude|gpt|gemini`)
	lines := map[string]string{}
	for family := range manifest.Families {
		match := line.FindStringSubmatch(family)
		if match == nil {
			if curated.MatchString(family) {
				t.Errorf("family %s of a curated vendor names no product line", family)
			}
			continue
		}
		key := match[1] + match[3] + match[5] + "/" + match[2] + match[4] + match[6]
		if previous, held := lines[key]; held {
			t.Errorf("line %s holds both %s and %s", key, previous, family)
		}
		lines[key] = family
	}
	want := []string{
		"claude/fable", "claude/haiku", "claude/opus", "claude/sonnet",
		"gemini/argon", "gemini/carbon", "gemini/hydrogen",
		"gpt/astra", "gpt/luna", "gpt/sol", "gpt/terra",
	}
	if got := slices.Sorted(maps.Keys(lines)); !slices.Equal(got, want) {
		t.Errorf("curated lines = %v, want %v", got, want)
	}
	// A successor filed as a model of the family it replaces would join the
	// line unseen: a curated family's wire identity is its own id, with at
	// most a channel suffix.
	for routeID, route := range manifest.Models {
		if line.MatchString(route.Family) && !strings.HasPrefix(route.ID, route.Family) {
			t.Errorf("model %s of curated family %s carries another identity", routeID, route.Family)
		}
	}
}

func TestTeamManifestGPTProviderModelsFollowInferenceEvidence(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	want := map[string][]string{
		"gpt-6.1-sol": {"aihubmix", "dmxapi", "ucloud"},
		// DMXAPI's plain Terra passed the resumed tool turn on 2026-11-12; its CDX
		// and SSVIP channels are separate wire identities.
		"gpt-6.1-terra": {"aihubmix", "dmxapi", "ucloud"},
		// GPT-6 Astra and GPT-6 Luna passed all three Hermes turns on every
		// provider on 2026-11-13.
		"gpt-6-astra": {"aihubmix", "dmxapi", "ucloud"},
		"gpt-6-luna":  {"aihubmix", "dmxapi", "ucloud"},
	}
	for model, accounts := range want {
		for _, account := range accounts {
			id := account + "/" + model
			route, ok := teamModel(t, manifest, id)
			if !ok || route.Provider != account || route.Family != model || route.ID != model ||
				!slices.Equal(route.AdmittedProtocols(), []domain.EndpointProtocol{domain.ProtocolResponses}) {
				t.Errorf("verified GPT Route %q = %+v", id, route)
			}
		}
	}
}

func TestTeamManifestFrontierRefreshUsesQualifiedProviderProtocols(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	want := map[string]struct {
		model    string
		wire     string
		protocol domain.EndpointProtocol
	}{
		"aihubmix/gemini-4-argon": {"gemini-4-argon", "gemini-4-argon", domain.ProtocolResponses},
		"dmxapi/gemini-4-argon":   {"gemini-4-argon", "gemini-4-argon", domain.ProtocolResponses},
		"ucloud/gemini-4-argon":   {"gemini-4-argon", "gemini-4-argon", domain.ProtocolChat},
		// AIHubMix's Responses relay returned empty output on Hydrogen's resumed
		// tool turn on 2026-11-13, while its Chat Completions passed all three turns.
		"dmxapi/gemini-4-hydrogen":   {"gemini-4-hydrogen", "gemini-4-hydrogen", domain.ProtocolResponses},
		"aihubmix/gemini-4-hydrogen": {"gemini-4-hydrogen", "gemini-4-hydrogen", domain.ProtocolChat},
		"dmxapi/hy4":                 {"hy4", "hy4", domain.ProtocolResponses},
		"aihubmix/hy4":               {"hy4", "hy4", domain.ProtocolChat},
		"dmxapi/LongCat-2.5":         {"longcat-2.5", "LongCat-2.5", domain.ProtocolResponses},
		"dmxapi/step-5-preview":      {"step-5-preview", "step-5-preview", domain.ProtocolResponses},
	}
	for id, expected := range want {
		route, ok := teamModel(t, manifest, id)
		if !ok || route.Family != expected.model || route.ID != expected.wire ||
			!slices.Equal(route.AdmittedProtocols(), []domain.EndpointProtocol{expected.protocol}) {
			t.Errorf("qualified frontier Route %q = %+v, want %+v", id, route, expected)
		}
	}
	for _, superseded := range []string{"aihubmix/gemini-3.8-flash", "dmxapi/gemini-3.8-flash", "ucloud/gemini-3.8-flash",
		"dmxapi/hy4-preview", "dmxapi/longcat-2.5-preview", "dmxapi/doubao-seed-2-1-pro-260628"} {
		if _, retained := teamModel(t, manifest, superseded); retained {
			t.Errorf("superseded Route %q remains in the shipped manifest", superseded)
		}
	}
}
