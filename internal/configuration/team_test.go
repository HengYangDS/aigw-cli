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
		model           string
		storedProtocol  EndpointProtocol
		runtimeProtocol EndpointProtocol
	}{
		ClientClaude:        {model: "claude-fable-5-1", runtimeProtocol: ProtocolAnthropic},
		ClientClaudeDesktop: {model: "claude-fable-5-1", runtimeProtocol: ProtocolAnthropic},
		ClientCodex:         {model: "gpt-6-sol", runtimeProtocol: ProtocolOpenAIResponses},
		ClientHermes:        {model: "gpt-6-sol", storedProtocol: ProtocolOpenAIResponses, runtimeProtocol: ProtocolOpenAIResponses},
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
		for client, want := range recommendations {
			routeID := selected.SelectedRoute(client)
			if routeID == "" {
				continue
			}
			runtime, resolveErr := selected.ResolveRuntime(client, "")
			if resolveErr != nil {
				t.Fatalf("resolve %s route for Account %q: %v", client, accountID, resolveErr)
			}
			modelOffered := false
			for _, route := range parsedManifest.Routes {
				modelOffered = modelOffered || route.Account == accountID && route.Model == want.model
			}
			if runtime.AccountID != accountID || modelOffered && selected.Routes[routeID].Model != want.model || runtime.Protocol != want.runtimeProtocol {
				t.Fatalf("%s route for Account %q = Account %q canonical Model %q protocol %q, want Model %q protocol %q", client, accountID, runtime.AccountID, selected.Routes[routeID].Model, runtime.Protocol, want.model, want.runtimeProtocol)
			}
			activated++
		}
		if activated == 0 {
			t.Fatalf("Account %q has no usable recommended Client Binding", accountID)
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
		"deepseek-v4-pro-0813", "gemini-3.1-pro-preview", "glm-5.3",
		"gpt-6-astra", "gpt-6-luna", "gpt-6-sol", "grok-4.6",
		"kimi-k3", "minimax-m3", "qwen3.8-max",
	}
	if got := slices.Sorted(maps.Keys(manifest.Models)); !slices.Equal(got, want) {
		t.Fatalf("team logical Models = %v, want %v", got, want)
	}
	if manifest.Models["claude-opus-5-5"].Label != "Claude Opus 5.5" {
		t.Errorf("Opus 5.5 identity = %#v", manifest.Models["claude-opus-5-5"])
	}
	for routeID, route := range manifest.Routes {
		if route.Model == "claude-opus-5-5" || route.Model == "minimax-m3" {
			t.Errorf("unverified provider Route %q was admitted", routeID)
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

func TestTeamManifestPrefersUCloudGPTSixRoutes(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	wantCodexRoutes := []string{"ucloud-gpt-6-sol", "ucloud-gpt-6-astra", "ucloud-gpt-6-luna"}
	codexRoutes := manifest.Recommendations[ClientCodex].Selections()
	if len(codexRoutes) < len(wantCodexRoutes) {
		t.Fatalf("Codex recommendations = %#v, want at least %#v", codexRoutes, wantCodexRoutes)
	}
	for index, selection := range codexRoutes[:len(wantCodexRoutes)] {
		if selection.Route != wantCodexRoutes[index] {
			t.Errorf("Codex recommendation %d = %q, want %q", index, selection.Route, wantCodexRoutes[index])
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
	for client, model := range map[string]string{ClientClaude: "claude-fable-5-1", ClientCodex: "gpt-6-astra", ClientHermes: "gpt-6-astra"} {
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
