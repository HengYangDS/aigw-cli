package store

import (
	"aigw-cli/internal/domain"
	"bytes"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func loadTeamManifest(t *testing.T) ([]byte, domain.Config) {
	t.Helper()
	manifestDirectory := filepath.Join("..", "..", "manifests")
	files, err := filepath.Glob(filepath.Join(manifestDirectory, "*.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || filepath.Base(files[0]) != "team.toml" {
		t.Fatalf("product manifests = %v, want only the reviewed team manifest", files)
	}

	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	parsedManifest, err := ParseTeamFile(data, testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	return data, parsedManifest
}

// teamModel returns the team file's model that name, `provider/model`, names.
func teamModel(t *testing.T, file domain.Config, name string) (domain.Model, bool) {
	t.Helper()
	ref, err := domain.ParseModelRef(name)
	if err != nil {
		t.Fatal(err)
	}
	model, ok := file.Models[ref]
	return model, ok
}

func TestTeamFileIsReviewedVersionEight(t *testing.T) {
	_, parsedManifest := loadTeamManifest(t)
	if parsedManifest.Version != 8 || len(parsedManifest.Providers) != 3 || len(parsedManifest.Families) == 0 || len(parsedManifest.Models) != 80 {
		t.Fatalf("team file = version %d, %d providers, %d families, %d models", parsedManifest.Version, len(parsedManifest.Providers), len(parsedManifest.Families), len(parsedManifest.Models))
	}
	if got := parsedManifest.ProviderIDs(); !slices.Equal(got, []string{"dmxapi", "ucloud", "aihubmix"}) {
		t.Fatalf("team providers in file order = %v", got)
	}
	recommendations := map[string]struct {
		model          string
		storedProtocol domain.EndpointProtocol
	}{
		ClientClaude:        {model: "claude-opus-5-5"},
		ClientClaudeDesktop: {model: "claude-opus-5-5"},
		ClientCodex:         {model: "gpt-6.1-sol"},
		ClientHermes:        {model: "gpt-6.1-sol", storedProtocol: domain.ProtocolResponses},
	}
	if len(parsedManifest.Recommendations) != len(recommendations) {
		t.Fatalf("team manifest recommended routes = %#v", parsedManifest.Recommendations)
	}
	for client, want := range recommendations {
		recommendation := parsedManifest.Recommendations[client].Primary
		route := parsedManifest.Models[recommendation.Model]
		if route.Family != want.model || recommendation.Protocol != want.storedProtocol {
			t.Fatalf("recommended %s Route = %#v with stored protocol %q, want Model %q with stored protocol %q", client, route, recommendation.Protocol, want.model, want.storedProtocol)
		}
	}
}

func TestTeamManifestSelectsOnlyDeclaredRecommendationsForConnectedProvider(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	protocols := map[string]domain.EndpointProtocol{
		ClientClaude: domain.ProtocolAnthropic, ClientClaudeDesktop: domain.ProtocolAnthropic,
		ClientCodex: domain.ProtocolResponses, ClientHermes: domain.ProtocolResponses,
	}
	for accountID := range manifest.Providers {
		cfg, _, err := domain.MergeTeamFile(domain.NewConfig(testAdmission()), manifest, false)
		if err != nil {
			t.Fatal(err)
		}
		selected, err := cfg.SelectModelsForConnectedProviders([]string{accountID})
		if err != nil {
			t.Fatal(err)
		}
		for client, recommendation := range manifest.Recommendations {
			var wantRoute domain.ModelRef
			for _, option := range recommendation.Selections() {
				if manifest.Models[option.Model].Provider == accountID {
					wantRoute = option.Model
					break
				}
			}
			routeID := selected.SelectedModel(client)
			if routeID != wantRoute {
				t.Errorf("%s with Account %q selected %q, want declared recommendation %q", client, accountID, routeID, wantRoute)
				continue
			}
			if wantRoute.IsZero() {
				continue
			}
			runtime, err := selected.ResolveRuntime(client, domain.ModelRef{})
			if err != nil {
				t.Fatalf("resolve %s Route %q: %v", client, routeID, err)
			}
			if runtime.ProviderID != accountID ||
				selected.Models[routeID].Family != manifest.Models[wantRoute].Family ||
				runtime.Protocol != protocols[client] {
				t.Errorf("%s Route %q resolves to Account %q Model %q protocol %q",
					client, routeID, runtime.ProviderID, selected.Models[routeID].Family, runtime.Protocol)
			}
		}
	}
}

func TestTeamRecommendationsPermitSparseProvidersAndPreserveExplicitBindings(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["aihubmix"] = domain.Provider{Label: "AIHubMix", Endpoints: domain.Endpoints{Anthropic: "https://hub.test"}}
	cfg.Providers["ucloud"] = domain.Provider{Label: "UCloud", Endpoints: domain.Endpoints{Responses: "https://cloud.test/v1"}}
	cfg.Families["fable"] = domain.Family{Label: "Fable"}
	cfg.Families["sol"] = domain.Family{Label: "Sol"}
	cfg.Models[domain.NewModelRef("aihubmix", "fable")] = domain.Model{Provider: "aihubmix", Family: "fable", ID: "fable", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {domain.CapabilityText}}}
	cfg.Models[domain.NewModelRef("ucloud", "sol")] = domain.Model{Provider: "ucloud", Family: "sol", ID: "sol", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {domain.CapabilityText}}}
	cfg.Recommendations[ClientClaude] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: domain.NewModelRef("aihubmix", "fable")}}
	cfg.Recommendations[ClientCodex] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: domain.NewModelRef("ucloud", "sol")}}

	cloud, err := cfg.SelectModelsForConnectedProviders([]string{"ucloud"})
	if err != nil || !cloud.SelectedModel(ClientClaude).IsZero() || cloud.SelectedModel(ClientCodex) != domain.NewModelRef("ucloud", "sol") {
		t.Fatalf("sparse UCloud activation = %+v, %v", cloud.Clients, err)
	}
	hub, err := cfg.SelectModelsForConnectedProviders([]string{"aihubmix"})
	if err != nil || hub.SelectedModel(ClientClaude) != domain.NewModelRef("aihubmix", "fable") || !hub.SelectedModel(ClientCodex).IsZero() {
		t.Fatalf("sparse AIHubMix activation = %+v, %v", hub.Clients, err)
	}
	cfg.SetSelectedModel(ClientCodex, domain.NewModelRef("ucloud", "sol"), "")
	hub, err = cfg.SelectModelsForConnectedProviders([]string{"aihubmix"})
	if err != nil || hub.SelectedModel(ClientCodex) != domain.NewModelRef("ucloud", "sol") || hub.SelectedModel(ClientClaude) != domain.NewModelRef("aihubmix", "fable") {
		t.Fatalf("explicit Codex binding was replaced: %+v, %v", hub.Clients, err)
	}
}

func TestTeamManifestUsesRequestedFamilies(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	want := []string{
		"claude-fable-5-2", "claude-haiku-5-5", "claude-opus-5-5", "claude-sonnet-5-5",
		"cohere-command-a",
		"deepseek-v4.1-flash-1017", "doubao-seed-2-1-pro-260915", "ernie-5.1", "gemini-4-argon", "gemini-4-carbon",
		"gemini-4-hydrogen", "glm-5.3", "gpt-6-astra", "gpt-6-luna", "gpt-6.1-sol", "gpt-6.1-terra", "grok-4.7",
		"hy4", "kimi-k3.1-pro", "laguna-s-2.1", "ling-3.0-flash", "longcat-2.5", "mercury-2.5", "mimo-v2.6-pro",
		"minimax-m3.1-flash", "mistral-large-3", "muse-wonder", "nemotron-3-ultra-550b-a55b",
		"qwen3.8-max-0902", "solar-pro4", "step-5-preview",
	}
	if got := slices.Sorted(maps.Keys(manifest.Families)); !slices.Equal(got, want) {
		t.Fatalf("team logical Models = %v, want %v", got, want)
	}
	order := manifest.FamilyIDs()
	if !slices.Equal(order[:8], []string{"claude-fable-5-2", "claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5",
		"gpt-6-astra", "gpt-6.1-sol", "gpt-6.1-terra", "gpt-6-luna"}) {
		t.Errorf("team families in file order = %v", order)
	}
	if manifest.Families["claude-opus-5-5"].Label != "Claude Opus 5.5" {
		t.Errorf("Opus 5.5 identity = %#v", manifest.Families["claude-opus-5-5"])
	}
	for _, account := range []string{"aihubmix", "dmxapi", "ucloud"} {
		for _, model := range []string{"claude-opus-5-5", "claude-sonnet-5-5"} {
			id := account + "/" + model
			route, ok := teamModel(t, manifest, id)
			if !ok || route.Provider != account || route.Family != model || route.ID != model {
				t.Errorf("verified Route %q = %+v, want exact Account, Model, and wire identity", id, route)
			}
		}
		// Doubao keeps one family, Volcengine's recommended 260915 build, which only DMXAPI serves.
		models := []string{"deepseek-v4.1-flash-1017"}
		if account == "dmxapi" {
			models = append(models, "doubao-seed-2-1-pro-260915")
		}
		if _, retained := teamModel(t, manifest, account+"/doubao-seed-2-1-pro-260628"); retained {
			t.Errorf("the second Doubao family remains on %s", account)
		}
		for _, model := range models {
			id := account + "/" + model
			route, ok := teamModel(t, manifest, id)
			if !ok || route.Provider != account || route.Family != model || route.ID != model {
				t.Errorf("qualified general Route %q = %+v", id, route)
			}
		}
	}
}

func TestTeamManifestGrokUpgradeKeepsAllThreeProviders(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	for _, account := range []string{"aihubmix", "dmxapi", "ucloud"} {
		id := account + "/grok-4.7"
		route, ok := teamModel(t, manifest, id)
		if !ok || route.Provider != account || route.Family != "grok-4.7" || route.ID != "grok-4.7" ||
			!slices.Equal(route.AdmittedProtocols(), []domain.EndpointProtocol{domain.ProtocolResponses}) {
			t.Errorf("qualified Grok Route %q = %+v", id, route)
		}
	}
	if _, present := manifest.Families["grok-4.6"]; present {
		t.Error("retired Grok 4.6 remains a logical Model")
	}
}

func TestTeamManifestKeepsOnlyQualifiedMiMoModels(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	route, ok := teamModel(t, manifest, "ucloud/mimo-v2.6-pro")
	if !ok || route.Provider != "ucloud" || route.Family != "mimo-v2.6-pro" || route.ID != "mimo-v2.6-pro" ||
		!slices.Contains(route.AdmittedProtocols(), domain.ProtocolResponses) {
		t.Errorf("qualified UCloud MiMo Route = %+v", route)
	}
	for _, account := range []string{"aihubmix", "dmxapi"} {
		if _, admitted := teamModel(t, manifest, account+"/mimo-v2.6-pro"); admitted {
			t.Errorf("unavailable or unqualified %s MiMo Route was admitted", account)
		}
	}
}

func TestTeamManifestUsesQualifiedChatProtocolForUCloudGLM(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	route, ok := teamModel(t, manifest, "ucloud/glm-5.3")
	if !ok || !slices.Equal(route.AdmittedProtocols(), []domain.EndpointProtocol{domain.ProtocolChat}) {
		t.Fatalf("UCloud GLM 5.3 protocols = %v, want Chat Completions", route.AdmittedProtocols())
	}
}

func TestTeamManifestKeepsQualifiedAdditionalVendorModels(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	if manifest.Providers["aihubmix"].Endpoints.Chat != "https://api.inferera.com/v1" {
		t.Errorf("AIHubMix Chat Completions endpoint = %q", manifest.Providers["aihubmix"].Endpoints.Chat)
	}
	want := map[string]struct {
		model    string
		wire     string
		protocol domain.EndpointProtocol
	}{
		"aihubmix/cohere-command-a":                {"cohere-command-a", "cohere-command-a", domain.ProtocolChat},
		"aihubmix/ernie-5.1":                       {"ernie-5.1", "ernie-5.1", domain.ProtocolResponses},
		"aihubmix/laguna-s-2.1":                    {"laguna-s-2.1", "laguna-s-2.1", domain.ProtocolChat},
		"aihubmix/ling-3.0-flash":                  {"ling-3.0-flash", "ling-3.0-flash", domain.ProtocolChat},
		"aihubmix/mercury-2.5":                     {"mercury-2.5", "mercury-2.5", domain.ProtocolChat},
		"aihubmix/mistral-large-3":                 {"mistral-large-3", "mistral-large-3", domain.ProtocolChat},
		"aihubmix/nemotron-3-ultra-550b-a55b-free": {"nemotron-3-ultra-550b-a55b", "nemotron-3-ultra-550b-a55b-free", domain.ProtocolChat},
		"aihubmix/step-5-preview":                  {"step-5-preview", "step-5-preview", domain.ProtocolChat},
	}
	for id, expected := range want {
		route, ok := teamModel(t, manifest, id)
		if !ok || route.Provider != "aihubmix" || route.Family != expected.model || route.ID != expected.wire ||
			!slices.Equal(route.AdmittedProtocols(), []domain.EndpointProtocol{expected.protocol}) {
			t.Errorf("qualified vendor Route %q = %+v, want %+v", id, route, expected)
		}
	}
	for _, superseded := range []string{"aihubmix/command-a-03-2025", "aihubmix/nemotron-3-super-120b-a12b-free"} {
		if _, retained := teamModel(t, manifest, superseded); retained {
			t.Errorf("superseded Route %q remains in the shipped manifest", superseded)
		}
	}
}

func TestTeamManifestRefreshPreservesExplicitSupersededModels(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	for _, selected := range []struct{ model, label, route, wire string }{
		{"command-a-03-2025", "Cohere Command A", "aihubmix/command-a-03-2025", "command-a-03-2025"},
		{"nemotron-3-super-120b-a12b", "NVIDIA Nemotron 3 Super", "aihubmix/nemotron-3-super-120b-a12b-free", "nemotron-3-super-120b-a12b-free"},
	} {
		t.Run(selected.model, func(t *testing.T) {
			cfg := domain.NewConfig(testAdmission())
			cfg.Providers["aihubmix"] = manifest.Providers["aihubmix"]
			cfg.Families[selected.model] = domain.Family{Label: selected.label}
			ref := domain.NewModelRef("aihubmix", selected.wire)
			cfg.Models[ref] = domain.Model{Provider: "aihubmix", Family: selected.model, ID: selected.wire,
				Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolChat: {}}}
			cfg.SetSelectedModel(ClientHermes, ref, "")
			merged, _, err := domain.MergeTeamFile(cfg, manifest, false)
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := merged.ResolveRuntime(ClientHermes, domain.ModelRef{})
			if err != nil || merged.SelectedModel(ClientHermes) != ref || runtime.UpstreamModel != selected.wire {
				t.Fatalf("team refresh replaced explicit Route %q: %+v, %v", selected.route, runtime, err)
			}
		})
	}
}

func TestTeamManifestModelsUseCanonicalIDsAndExactProviderWireIDs(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	cfg, _, err := domain.MergeTeamFile(domain.NewConfig(testAdmission()), manifest, false)
	if err != nil {
		t.Fatal(err)
	}
	for routeID, route := range manifest.Models {
		if route.Family != strings.ToLower(route.Family) {
			t.Errorf("model %s's family %q is not lower-case", routeID, route.Family)
		}
		if _, ok := manifest.Providers[route.Provider]; !ok {
			t.Errorf("Route %q references unknown Account %q", routeID, route.Provider)
		}
		if _, ok := manifest.Families[route.Family]; !ok {
			t.Errorf("Route %q references unknown Model %q", routeID, route.Family)
		}
		compatible, compatibilityErr := cfg.ProtocolClientIDs(routeID)
		if route.ID == "" || len(route.AdmittedProtocols()) == 0 || compatibilityErr != nil || len(compatible) == 0 {
			t.Errorf("Route %q has no exact wire ID or compatible client/protocol: %v", routeID, compatibilityErr)
		}
	}
	wireIdentities := map[string]struct{ model, wire string }{
		"aihubmix/claude-haiku-5-5":      {"claude-haiku-5-5", "claude-haiku-5-5"},
		"dmxapi/claude-haiku-5-5":        {"claude-haiku-5-5", "claude-haiku-5-5"},
		"dmxapi/claude-haiku-5-5-cc":     {"claude-haiku-5-5", "claude-haiku-5-5-cc"},
		"ucloud/claude-haiku-5-5":        {"claude-haiku-5-5", "claude-haiku-5-5"},
		"dmxapi/claude-fable-5-2-cc":     {"claude-fable-5-2", "claude-fable-5-2-cc"},
		"dmxapi/claude-opus-5-5-cc":      {"claude-opus-5-5", "claude-opus-5-5-cc"},
		"dmxapi/claude-opus-5-5-ssvip":   {"claude-opus-5-5", "claude-opus-5-5-ssvip"},
		"dmxapi/claude-sonnet-5-5":       {"claude-sonnet-5-5", "claude-sonnet-5-5"},
		"dmxapi/claude-sonnet-5-5-cc":    {"claude-sonnet-5-5", "claude-sonnet-5-5-cc"},
		"dmxapi/claude-sonnet-5-5-ssvip": {"claude-sonnet-5-5", "claude-sonnet-5-5-ssvip"},
		"dmxapi/gpt-6.1-sol-cdx":         {"gpt-6.1-sol", "gpt-6.1-sol-cdx"},
		"dmxapi/gpt-6.1-sol-ssvip":       {"gpt-6.1-sol", "gpt-6.1-sol-ssvip"},
		"dmxapi/gpt-6.1-terra":           {"gpt-6.1-terra", "gpt-6.1-terra"},
		"dmxapi/gpt-6.1-terra-cdx":       {"gpt-6.1-terra", "gpt-6.1-terra-cdx"},
		"dmxapi/gpt-6.1-terra-ssvip":     {"gpt-6.1-terra", "gpt-6.1-terra-ssvip"},
		"dmxapi/gemini-4-argon-ssvip":    {"gemini-4-argon", "gemini-4-argon-ssvip"},
	}
	for routeID, want := range wireIdentities {
		route, ok := teamModel(t, manifest, routeID)
		if !ok || route.Family != want.model || route.ID != want.wire {
			t.Errorf("channel Route %q = %+v, want canonical Model %q and wire %q", routeID, route, want.model, want.wire)
		}
	}
	if _, admitted := teamModel(t, manifest, "dmxapi/claude-fable-5-1-cc"); admitted {
		t.Error("the unqualified DMXAPI Fable 5.1 CC channel remains in the shipped manifest")
	}
	for _, obsolete := range []string{
		"aihubmix/claude-sonnet-5", "dmxapi/claude-sonnet-5", "dmxapi/claude-sonnet-5-cc",
		"dmxapi/claude-sonnet-5-ssvip", "ucloud/claude-sonnet-5",
	} {
		if _, retained := teamModel(t, manifest, obsolete); retained {
			t.Errorf("obsolete Sonnet 5 Route %q remains in the shipped team manifest", obsolete)
		}
	}
}

func TestTeamManifestRecommendationsRespectQualifiedModels(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	sol := "gpt-6.1-sol"
	for client, want := range map[string][]string{
		ClientClaude:        {"dmxapi/claude-opus-5-5", "ucloud/claude-opus-5-5"},
		ClientClaudeDesktop: {"dmxapi/claude-opus-5-5", "ucloud/claude-opus-5-5"},
		ClientCodex:         {"dmxapi/" + sol + "-cdx", "ucloud/" + sol},
		ClientHermes:        {"dmxapi/" + sol, "ucloud/" + sol},
	} {
		choices := manifest.Recommendations[client].Selections()
		if len(choices) != len(want) {
			t.Errorf("%s recommendations = %#v, want %q", client, choices, want)
			continue
		}
		for index, selection := range choices {
			if selection.Model.String() != want[index] {
				t.Errorf("%s recommendation %d = %q, want %q", client, index, selection.Model, want[index])
			}
		}
	}
	cfg, _, err := domain.MergeTeamFile(domain.NewConfig(testAdmission()), manifest, false)
	if err != nil {
		t.Fatal(err)
	}
	cfg.SetSelectedModel(ClientCodex, domain.NewModelRef("ucloud", sol), "")
	selected, err := cfg.SelectModelsForConnectedProviders([]string{"dmxapi"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.SelectedModel(ClientCodex) != domain.NewModelRef("ucloud", sol) {
		t.Fatalf("qualified recommendation replaced explicit Codex selection: %+v", selected.Clients)
	}
	if _, exists := manifest.Families["gpt-6-sol"]; exists {
		t.Error("retired GPT-6 Sol remains in the shipped model catalog")
	}
	for routeID, route := range manifest.Models {
		if route.Family == "gpt-6-sol" {
			t.Errorf("retired GPT-6 Sol Route %q remains", routeID)
		}
	}
}

func TestTeamProviderEndpointsHaveHosts(t *testing.T) {
	_, parsedManifest := loadTeamManifest(t)
	hosts := map[string]string{
		"aihubmix": "api.inferera.com", "dmxapi": "www.dmxapi.cn", "ucloud": "api.modelverse.cn",
	}
	for accountID, account := range parsedManifest.Providers {
		for _, endpoint := range []string{account.Endpoints.Responses, account.Endpoints.Chat, account.Endpoints.Anthropic} {
			if endpoint == "" {
				continue
			}
			parsed, parseErr := url.Parse(endpoint)
			if parseErr != nil || parsed.Hostname() != hosts[accountID] {
				t.Fatalf("team Account %q contains an unreviewed endpoint %q", accountID, endpoint)
			}
		}
	}
}

func TestTeamManifestDerivesPlainModelLabelsAndKeepsChannelOverrides(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	cfg, _, err := domain.MergeTeamFile(domain.NewConfig(testAdmission()), manifest, false)
	if err != nil {
		t.Fatal(err)
	}
	channel := regexp.MustCompile(`^[A-Z][A-Z0-9]*$`)
	for routeID, route := range manifest.Models {
		base := manifest.Providers[route.Provider].Label + " · " + manifest.Families[route.Family].Label
		if route.Label == "" {
			if got := cfg.ModelLabel(routeID); got != base {
				t.Errorf("plain Route %q label = %q, want %q", routeID, got, base)
			}
			continue
		}
		parts := strings.Split(route.Label, " · ")
		if len(parts) != 3 || parts[0]+" · "+parts[1] != base || !channel.MatchString(parts[2]) {
			t.Errorf("Route %q must omit a plain label or distinguish an uppercase channel: %q", routeID, route.Label)
			continue
		}
		if got := cfg.ModelLabel(routeID); got != route.Label {
			t.Errorf("channel Route %q label = %q, want %q", routeID, got, route.Label)
		}
	}
}

func TestModelLabelDerivesDisplayWithoutChangingExactWireModel(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["ucloud"] = domain.Provider{
		Label: "UCloud", Endpoints: domain.Endpoints{Responses: "https://ucloud.test/v1"},
	}
	cfg.Families["minimax-m3"] = domain.Family{Label: "MiniMax M3"}
	cfg.Models[domain.NewModelRef("ucloud", "MiniMax-M3")] = domain.Model{
		Provider: "ucloud", Family: "minimax-m3", ID: "MiniMax-M3",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {domain.CapabilityText}},
	}
	if got := cfg.ModelLabel(domain.NewModelRef("ucloud", "MiniMax-M3")); got != "UCloud · MiniMax M3" {
		t.Fatalf("derived Route label = %q", got)
	}
	runtime, err := cfg.ResolveRuntime(ClientCodex, domain.NewModelRef("ucloud", "MiniMax-M3"))
	if err != nil || runtime.ModelLabel != "UCloud · MiniMax M3" || runtime.UpstreamModel != "MiniMax-M3" {
		t.Fatalf("resolved label and exact wire model = %+v, %v", runtime, err)
	}
	route := cfg.Models[domain.NewModelRef("ucloud", "MiniMax-M3")]
	route.Label = "UCloud · MiniMax M3 · Channel"
	cfg.Models[domain.NewModelRef("ucloud", "MiniMax-M3")] = route
	if got := cfg.ModelLabel(domain.NewModelRef("ucloud", "MiniMax-M3")); got != route.Label {
		t.Fatalf("explicit Route label = %q, want %q", got, route.Label)
	}
}

func TestTeamManifestUsesNativeExportLayout(t *testing.T) {
	data, manifest := loadTeamManifest(t)
	cfg, _, err := domain.MergeTeamFile(domain.NewConfig(testAdmission()), manifest, false)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := ExportTeamFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, canonical) {
		t.Errorf("team manifest must use the native canonical export order and layout:\nwant:\n%s\ngot:\n%s", canonical, data)
	}
}
