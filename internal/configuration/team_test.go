package configuration

import (
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

func loadTeamManifest(t *testing.T) ([]byte, Manifest) {
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
	parsedManifest, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return data, parsedManifest
}

func TestTeamConfigurationManifestIsReviewedVersionSeven(t *testing.T) {
	_, parsedManifest := loadTeamManifest(t)
	if parsedManifest.Version != 7 || len(parsedManifest.Accounts) != 3 || len(parsedManifest.Models) == 0 || len(parsedManifest.Routes) == 0 {
		t.Fatalf("team manifest = version %d, %d Accounts, %d Models, %d Routes", parsedManifest.Version, len(parsedManifest.Accounts), len(parsedManifest.Models), len(parsedManifest.Routes))
	}
	for _, accountID := range []string{"aihubmix", "dmxapi", "ucloud"} {
		if _, ok := parsedManifest.Accounts[accountID]; !ok {
			t.Fatalf("team manifest missing account %q", accountID)
		}
	}
	recommendations := map[string]struct {
		model          string
		storedProtocol EndpointProtocol
	}{
		ClientClaude:        {model: "claude-opus-5-5"},
		ClientClaudeDesktop: {model: "claude-opus-5-5"},
		ClientCodex:         {model: "gpt-6-sol"},
		ClientHermes:        {model: "gpt-6-sol", storedProtocol: ProtocolOpenAIResponses},
	}
	if len(parsedManifest.Recommendations) != len(recommendations) {
		t.Fatalf("team manifest recommended routes = %#v", parsedManifest.Recommendations)
	}
	for client, want := range recommendations {
		recommendation := parsedManifest.Recommendations[client].Primary
		route := parsedManifest.Routes[recommendation.Route]
		if route.Model != want.model || recommendation.Protocol != want.storedProtocol {
			t.Fatalf("recommended %s Route = %#v with stored protocol %q, want Model %q with stored protocol %q", client, route, recommendation.Protocol, want.model, want.storedProtocol)
		}
	}
}

func TestTeamManifestActivatesAnyOneConnectedAccount(t *testing.T) {
	_, parsedManifest := loadTeamManifest(t)
	protocols := map[string]EndpointProtocol{
		ClientClaude: ProtocolAnthropic, ClientClaudeDesktop: ProtocolAnthropic,
		ClientCodex: ProtocolOpenAIResponses, ClientHermes: ProtocolOpenAIResponses,
	}
	for accountID := range parsedManifest.Accounts {
		cfg, mergeErr := Merge(NewConfig(), parsedManifest)
		if mergeErr != nil {
			t.Fatal(mergeErr)
		}
		selected, selectErr := cfg.SelectRoutesForConnectedAccounts([]string{accountID})
		if selectErr != nil {
			t.Fatal(selectErr)
		}
		activated := 0
		for client, recommendation := range parsedManifest.Recommendations {
			routeID := selected.SelectedRoute(client)
			if routeID == "" {
				continue
			}
			runtime, resolveErr := selected.ResolveRuntime(client, "")
			if resolveErr != nil {
				t.Fatalf("resolve %s route for Account %q: %v", client, accountID, resolveErr)
			}
			expectedModel := ""
			for _, option := range recommendation.Selections() {
				candidate := parsedManifest.Routes[option.Route]
				if candidate.Account == accountID {
					expectedModel = candidate.Model
					break
				}
			}
			if expectedModel == "" || runtime.AccountID != accountID || selected.Routes[routeID].Model != expectedModel || runtime.Protocol != protocols[client] {
				t.Fatalf("%s route for Account %q = Account %q canonical Model %q protocol %q, want Model %q protocol %q", client, accountID, runtime.AccountID, selected.Routes[routeID].Model, runtime.Protocol, expectedModel, protocols[client])
			}
			activated++
		}
		if activated != len(parsedManifest.Recommendations) {
			t.Fatalf("Account %q activates %d of %d recommended Clients", accountID, activated, len(parsedManifest.Recommendations))
		}
	}
}

func TestTeamRecommendationsPermitSparseAccountsAndPreserveExplicitBindings(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["aihubmix"] = Account{Label: "AIHubMix", Endpoints: Endpoints{Anthropic: "https://hub.test"}}
	cfg.Accounts["ucloud"] = Account{Label: "UCloud", Endpoints: Endpoints{OpenAIResponses: "https://cloud.test/v1"}}
	cfg.Models["fable"] = Model{Label: "Fable"}
	cfg.Models["sol"] = Model{Label: "Sol"}
	cfg.Routes["aihubmix-fable"] = Route{Account: "aihubmix", Model: "fable", Interfaces: map[EndpointProtocol][]Capability{ProtocolAnthropic: {CapabilityText}}}
	cfg.Routes["ucloud-sol"] = Route{Account: "ucloud", Model: "sol", Interfaces: map[EndpointProtocol][]Capability{ProtocolOpenAIResponses: {CapabilityText}}}
	cfg.SetRecommendedRoute(ClientClaude, "aihubmix-fable")
	cfg.SetRecommendedRoute(ClientCodex, "ucloud-sol")

	cloud, err := cfg.SelectRoutesForConnectedAccounts([]string{"ucloud"})
	if err != nil || cloud.SelectedRoute(ClientClaude) != "" || cloud.SelectedRoute(ClientCodex) != "ucloud-sol" {
		t.Fatalf("sparse UCloud activation = %+v, %v", cloud.Clients, err)
	}
	hub, err := cfg.SelectRoutesForConnectedAccounts([]string{"aihubmix"})
	if err != nil || hub.SelectedRoute(ClientClaude) != "aihubmix-fable" || hub.SelectedRoute(ClientCodex) != "" {
		t.Fatalf("sparse AIHubMix activation = %+v, %v", hub.Clients, err)
	}
	cfg.SetSelectedRoute(ClientCodex, "ucloud-sol")
	hub, err = cfg.SelectRoutesForConnectedAccounts([]string{"aihubmix"})
	if err != nil || hub.SelectedRoute(ClientCodex) != "ucloud-sol" || hub.SelectedRoute(ClientClaude) != "aihubmix-fable" {
		t.Fatalf("explicit Codex binding was replaced: %+v, %v", hub.Clients, err)
	}
}

func TestTeamManifestUsesRequestedLogicalModels(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	want := []string{
		"claude-fable-5-1", "claude-opus-5-5", "claude-sonnet-5",
		"command-a-03-2025",
		"deepseek-v4.1-flash", "doubao-seed-2-1-pro-260628", "ernie-5.1", "gemini-3.1-pro-preview", "glm-5.3",
		"gpt-6-astra", "gpt-6-luna", "gpt-6-sol", "grok-4.7",
		"hy3", "kimi-k3", "laguna-s-2.1", "ling-3.0-flash", "longcat-2.0", "mercury-2.5", "mimo-v2.6-pro",
		"minimax-m3", "mistral-large-3", "muse-spark-1.3", "nemotron-3-ultra-550b-a55b",
		"qwen3.8-max", "solar-pro4", "step-3.7-flash",
	}
	if got := slices.Sorted(maps.Keys(manifest.Models)); !slices.Equal(got, want) {
		t.Fatalf("team logical Models = %v, want %v", got, want)
	}
	if manifest.Models["claude-opus-5-5"].Label != "Claude Opus 5.5" {
		t.Errorf("Opus 5.5 identity = %#v", manifest.Models["claude-opus-5-5"])
	}
	for _, account := range []string{"aihubmix", "dmxapi", "ucloud"} {
		for _, model := range []string{"claude-opus-5-5"} {
			id := account + "-" + model
			route, ok := manifest.Routes[id]
			if !ok || route.Account != account || route.Model != model || route.UpstreamModelID() != model {
				t.Errorf("verified Route %q = %+v, want exact Account, Model, and wire identity", id, route)
			}
		}
		for _, model := range []string{"deepseek-v4.1-flash", "doubao-seed-2-1-pro-260628"} {
			id := account + "-" + model
			route, ok := manifest.Routes[id]
			if !ok || route.Account != account || route.Model != model || route.UpstreamModelID() != model {
				t.Errorf("qualified general Route %q = %+v", id, route)
			}
		}
	}
}

func TestTeamManifestGPTAccountRoutesFollowInferenceEvidence(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	want := map[string][]string{
		"gpt-6-astra": {"aihubmix", "dmxapi", "ucloud"},
		"gpt-6-luna":  {"aihubmix", "dmxapi", "ucloud"},
		"gpt-6-sol":   {"aihubmix", "dmxapi", "ucloud"},
	}
	for model, accounts := range want {
		for _, account := range accounts {
			id := account + "-" + model
			route, ok := manifest.Routes[id]
			if !ok || route.Account != account || route.Model != model || route.UpstreamModelID() != model ||
				!slices.Equal(route.AdmittedProtocols(), []EndpointProtocol{ProtocolOpenAIResponses}) {
				t.Errorf("verified GPT Route %q = %+v", id, route)
			}
		}
	}
}

func TestTeamManifestGrokUpgradeKeepsAllThreeAccounts(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	for _, account := range []string{"aihubmix", "dmxapi", "ucloud"} {
		id := account + "-grok-4.7"
		route, ok := manifest.Routes[id]
		if !ok || route.Account != account || route.Model != "grok-4.7" || route.UpstreamModelID() != "grok-4.7" ||
			!slices.Equal(route.AdmittedProtocols(), []EndpointProtocol{ProtocolOpenAIResponses}) {
			t.Errorf("qualified Grok Route %q = %+v", id, route)
		}
	}
	if _, present := manifest.Models["grok-4.6"]; present {
		t.Error("retired Grok 4.6 remains a logical Model")
	}
}

func TestTeamManifestKeepsOnlyQualifiedMiniMaxAndMuseRoutes(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	if _, admitted := manifest.Routes["dmxapi-minimax-m3"]; admitted {
		t.Error("timed-out DMXAPI MiniMax M3 Route was admitted")
	}
	for _, account := range []string{"aihubmix", "ucloud"} {
		id := account + "-minimax-m3"
		route, ok := manifest.Routes[id]
		wire := "minimax-m3"
		if account == "ucloud" {
			wire = "MiniMax-M3"
		}
		if !ok || route.Account != account || route.Model != "minimax-m3" || route.UpstreamModelID() != wire {
			t.Errorf("MiniMax Route %q = %+v, want wire %q", id, route, wire)
		}
	}
	muse, ok := manifest.Routes["aihubmix-muse-spark-1.3"]
	if !ok || muse.Account != "aihubmix" || muse.Model != "muse-spark-1.3" || muse.UpstreamModelID() != "muse-spark-1.3" {
		t.Errorf("verified Meta Route = %+v", muse)
	}
	for _, account := range []string{"dmxapi", "ucloud"} {
		if _, admitted := manifest.Routes[account+"-muse-spark-1.3"]; admitted {
			t.Errorf("unverified Meta Route on %s was admitted", account)
		}
	}
}

func TestTeamManifestKeepsOnlyQualifiedMiMoRoutes(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	for _, account := range []string{"aihubmix", "ucloud"} {
		id := account + "-mimo-v2.6-pro"
		route, ok := manifest.Routes[id]
		if !ok || route.Account != account || route.Model != "mimo-v2.6-pro" || route.UpstreamModelID() != "mimo-v2.6-pro" ||
			!slices.Contains(route.AdmittedProtocols(), ProtocolOpenAIResponses) {
			t.Errorf("qualified MiMo Route %q = %+v", id, route)
		}
	}
	if _, admitted := manifest.Routes["dmxapi-mimo-v2.6-pro"]; admitted {
		t.Error("unverified DMXAPI MiMo Route was admitted")
	}
}

func TestTeamManifestUsesQualifiedChatProtocolForUCloudGLM(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	route, ok := manifest.Routes["ucloud-glm-5.3"]
	if !ok || !slices.Equal(route.AdmittedProtocols(), []EndpointProtocol{ProtocolOpenAIChatCompletions}) {
		t.Fatalf("UCloud GLM 5.3 protocols = %v, want Chat Completions", route.AdmittedProtocols())
	}
}

func TestTeamManifestKeepsQualifiedAdditionalVendorRoutes(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	if manifest.Accounts["aihubmix"].Endpoints.OpenAIChatCompletions != "https://api.inferera.com/v1" {
		t.Errorf("AIHubMix Chat Completions endpoint = %q", manifest.Accounts["aihubmix"].Endpoints.OpenAIChatCompletions)
	}
	want := map[string]struct {
		model    string
		wire     string
		protocol EndpointProtocol
	}{
		"aihubmix-command-a-03-2025":               {"command-a-03-2025", "command-a-03-2025", ProtocolOpenAIChatCompletions},
		"aihubmix-ernie-5.1":                       {"ernie-5.1", "ernie-5.1", ProtocolOpenAIResponses},
		"aihubmix-hy3":                             {"hy3", "hy3", ProtocolOpenAIChatCompletions},
		"aihubmix-laguna-s-2.1":                    {"laguna-s-2.1", "laguna-s-2.1", ProtocolOpenAIChatCompletions},
		"aihubmix-ling-3.0-flash":                  {"ling-3.0-flash", "ling-3.0-flash", ProtocolOpenAIChatCompletions},
		"aihubmix-longcat-2.0":                     {"longcat-2.0", "longcat-2.0", ProtocolOpenAIChatCompletions},
		"aihubmix-mercury-2.5":                     {"mercury-2.5", "mercury-2.5", ProtocolOpenAIChatCompletions},
		"aihubmix-mistral-large-3":                 {"mistral-large-3", "mistral-large-3", ProtocolOpenAIChatCompletions},
		"aihubmix-nemotron-3-ultra-550b-a55b-free": {"nemotron-3-ultra-550b-a55b", "nemotron-3-ultra-550b-a55b-free", ProtocolOpenAIChatCompletions},
		"aihubmix-solar-pro4":                      {"solar-pro4", "solar-pro4", ProtocolOpenAIChatCompletions},
		"aihubmix-step-3.7-flash":                  {"step-3.7-flash", "step-3.7-flash", ProtocolOpenAIChatCompletions},
	}
	for id, expected := range want {
		route, ok := manifest.Routes[id]
		if !ok || route.Account != "aihubmix" || route.Model != expected.model || route.UpstreamModelID() != expected.wire ||
			!slices.Equal(route.AdmittedProtocols(), []EndpointProtocol{expected.protocol}) {
			t.Errorf("qualified vendor Route %q = %+v, want %+v", id, route, expected)
		}
	}
}

func TestTeamManifestRoutesUseCanonicalIDsAndExactProviderWireIDs(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	cfg, err := Merge(NewConfig(), manifest)
	if err != nil {
		t.Fatal(err)
	}
	for routeID, route := range manifest.Routes {
		if routeID != strings.ToLower(routeID) || route.Model != strings.ToLower(route.Model) {
			t.Errorf("Route %q or canonical Model %q is not lower-case", routeID, route.Model)
		}
		if _, ok := manifest.Accounts[route.Account]; !ok {
			t.Errorf("Route %q references unknown Account %q", routeID, route.Account)
		}
		if _, ok := manifest.Models[route.Model]; !ok {
			t.Errorf("Route %q references unknown Model %q", routeID, route.Model)
		}
		compatible, compatibilityErr := cfg.CompatibleClientIDs(routeID)
		if route.UpstreamModelID() == "" || len(routeAdmittedProtocols(route)) == 0 || compatibilityErr != nil || len(compatible) == 0 {
			t.Errorf("Route %q has no exact wire ID or compatible client/protocol: %v", routeID, compatibilityErr)
		}
	}
	variants := map[string]struct{ model, wire string }{
		"dmxapi-claude-fable-5-1-cc":   {"claude-fable-5-1", "claude-fable-5-1-cc"},
		"dmxapi-claude-sonnet-5-cc":    {"claude-sonnet-5", "claude-sonnet-5-cc"},
		"dmxapi-claude-sonnet-5-ssvip": {"claude-sonnet-5", "claude-sonnet-5-ssvip"},
		"dmxapi-gpt-6-astra-cdx":       {"gpt-6-astra", "gpt-6-astra-cdx"},
		"dmxapi-gpt-6-astra-ssvip":     {"gpt-6-astra", "gpt-6-astra-ssvip"},
	}
	for routeID, want := range variants {
		route, ok := manifest.Routes[routeID]
		if !ok || route.Model != want.model || route.UpstreamModelID() != want.wire {
			t.Errorf("channel Route %q = %+v, want canonical Model %q and wire %q", routeID, route, want.model, want.wire)
		}
	}
}

func TestTeamManifestRecommendsCurrentVerifiedModelsWithoutChangingSelections(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	for client, model := range map[string]string{
		ClientClaude: "claude-opus-5-5", ClientClaudeDesktop: "claude-opus-5-5",
		ClientCodex: "gpt-6-sol", ClientHermes: "gpt-6-sol",
	} {
		choices := manifest.Recommendations[client].Selections()
		want := []string{"dmxapi-" + model, "aihubmix-" + model, "ucloud-" + model}
		if client == ClientCodex || client == ClientHermes {
			want = []string{"ucloud-" + model, "aihubmix-" + model, "dmxapi-" + model, "dmxapi-gpt-6-luna"}
		}
		if len(choices) != len(want) {
			t.Errorf("%s recommendations = %#v, want %q", client, choices, want)
			continue
		}
		for index, selection := range choices {
			if selection.Route != want[index] {
				t.Errorf("%s recommendation %d = %q, want %q", client, index, selection.Route, want[index])
			}
		}
	}
}

func TestTeamAccountEndpointsHaveHosts(t *testing.T) {
	_, parsedManifest := loadTeamManifest(t)
	hosts := map[string]string{
		"aihubmix": "api.inferera.com", "dmxapi": "www.dmxapi.cn", "ucloud": "api.modelverse.cn",
	}
	for accountID, account := range parsedManifest.Accounts {
		for _, endpoint := range []string{account.Endpoints.OpenAIResponses, account.Endpoints.OpenAIChatCompletions, account.Endpoints.Anthropic} {
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

func TestTeamManifestDerivesPlainRouteLabelsAndKeepsChannelOverrides(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	cfg, err := Merge(NewConfig(), manifest)
	if err != nil {
		t.Fatal(err)
	}
	channel := regexp.MustCompile(`^[A-Z][A-Z0-9]*$`)
	for routeID, route := range manifest.Routes {
		if route.Purpose != "" {
			t.Errorf("model catalogue Route %q must omit workflow purpose", routeID)
		}
		base := manifest.Accounts[route.Account].Label + " · " + manifest.Models[route.Model].Label
		if route.Label == "" {
			if got := cfg.RouteLabel(routeID); got != base {
				t.Errorf("plain Route %q label = %q, want %q", routeID, got, base)
			}
			continue
		}
		parts := strings.Split(route.Label, " · ")
		if len(parts) != 3 || parts[0]+" · "+parts[1] != base || !channel.MatchString(parts[2]) {
			t.Errorf("Route %q must omit a plain label or distinguish an uppercase channel: %q", routeID, route.Label)
			continue
		}
		if got := cfg.RouteLabel(routeID); got != route.Label {
			t.Errorf("channel Route %q label = %q, want %q", routeID, got, route.Label)
		}
	}
}

func TestRouteLabelDerivesDisplayWithoutChangingExactWireModel(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["ucloud"] = Account{
		Label: "UCloud", Endpoints: Endpoints{OpenAIResponses: "https://ucloud.test/v1"},
	}
	cfg.Models["minimax-m3"] = Model{Label: "MiniMax M3"}
	cfg.Routes["ucloud-minimax-m3"] = Route{
		Account: "ucloud", Model: "minimax-m3", UpstreamModel: "MiniMax-M3",
		Interfaces: map[EndpointProtocol][]Capability{ProtocolOpenAIResponses: {CapabilityText}},
	}
	if got := cfg.RouteLabel("ucloud-minimax-m3"); got != "UCloud · MiniMax M3" {
		t.Fatalf("derived Route label = %q", got)
	}
	runtime, err := cfg.ResolveRuntime(ClientCodex, "ucloud-minimax-m3")
	if err != nil || runtime.RouteLabel != "UCloud · MiniMax M3" || runtime.Model != "MiniMax-M3" {
		t.Fatalf("resolved label and exact wire model = %+v, %v", runtime, err)
	}
	route := cfg.Routes["ucloud-minimax-m3"]
	route.Label = "UCloud · MiniMax M3 · Channel"
	cfg.Routes["ucloud-minimax-m3"] = route
	if got := cfg.RouteLabel("ucloud-minimax-m3"); got != route.Label {
		t.Fatalf("explicit Route label = %q, want %q", got, route.Label)
	}
}

func TestTeamManifestSelectsRecommendedModelsForAIHubMix(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	cfg, err := Merge(NewConfig(), manifest)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := cfg.SelectRoutesForConnectedAccounts([]string{"aihubmix"})
	if err != nil {
		t.Fatal(err)
	}
	for client, model := range map[string]string{ClientClaude: "claude-opus-5-5", ClientCodex: "gpt-6-sol", ClientHermes: "gpt-6-sol"} {
		runtime, resolveErr := selected.ResolveRuntime(client, "")
		if resolveErr != nil {
			t.Fatal(resolveErr)
		}
		if runtime.AccountID != "aihubmix" || runtime.Model != model {
			t.Errorf("AIHubMix %s setup selected %s on %s, want %s", client, runtime.Model, runtime.AccountID, model)
		}
	}
}

func TestTeamManifestUsesNativeExportLayout(t *testing.T) {
	data, manifest := loadTeamManifest(t)
	cfg := NewConfig()
	cfg.Accounts = manifest.Accounts
	cfg.Models = manifest.Models
	cfg.Routes = manifest.Routes
	cfg.Recommendations = manifest.Recommendations
	canonical, err := Export(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, canonical) {
		t.Errorf("team manifest must use the native canonical export order and layout:\nwant:\n%s\ngot:\n%s", canonical, data)
	}
}
