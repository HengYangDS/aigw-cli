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
		for client, want := range recommendations {
			runtime, resolveErr := selected.ResolveRuntime(client, "")
			if resolveErr != nil {
				t.Fatalf("resolve %s route for Account %q: %v", client, accountID, resolveErr)
			}
			modelOffered := false
			for _, route := range parsedManifest.Routes {
				modelOffered = modelOffered || route.Account == accountID && route.Model == want.model
			}
			if runtime.AccountID != accountID || modelOffered && runtime.Model != want.model || runtime.Protocol != want.runtimeProtocol {
				t.Fatalf("%s route for Account %q = Account %q model %q protocol %q, want model %q protocol %q", client, accountID, runtime.AccountID, runtime.Model, runtime.Protocol, want.model, want.runtimeProtocol)
			}
		}
	}
}

func TestTeamManifestUsesRequestedLogicalModels(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	want := []string{
		"claude-fable-5-1", "claude-opus-5-5", "claude-sonnet-5",
		"deepseek-v4-pro-0813", "gemini-3.1-pro-preview", "glm-5.3",
		"gpt-6-astra", "gpt-6-luna", "gpt-6-sol", "grok-4.6",
		"kimi-k3", "qwen3.8-max",
	}
	if got := slices.Sorted(maps.Keys(manifest.Models)); !slices.Equal(got, want) {
		t.Fatalf("team logical Models = %v, want %v", got, want)
	}
	if manifest.Models["claude-opus-5-5"].Label != "Claude Opus 5.5" {
		t.Errorf("Opus 5.5 identity = %#v", manifest.Models["claude-opus-5-5"])
	}
	for routeID, route := range manifest.Routes {
		if route.Model == "claude-opus-5-5" {
			t.Errorf("unverified Opus 5.5 Route %q was admitted", routeID)
		}
	}
}

func TestTeamManifestRetainsOnlyQualifiedAccountRoutes(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	wantWireIDs := map[string][]string{
		"aihubmix": {
			"claude-fable-5-1", "claude-sonnet-5", "deepseek-v4-pro-0813",
			"gemini-3.1-pro-preview", "glm-5.3", "gpt-6-astra", "grok-4.6",
			"kimi-k3", "qwen3.8-max",
		},
		"dmxapi": {
			"claude-fable-5-1", "claude-fable-5-1-cc", "claude-sonnet-5",
			"claude-sonnet-5-cc", "claude-sonnet-5-ssvip", "deepseek-v4-pro-0813",
			"gemini-3.1-pro-preview", "glm-5.3", "gpt-6-astra",
			"gpt-6-astra-cdx", "gpt-6-astra-ssvip", "grok-4.6", "kimi-k3",
			"qwen3.8-max",
		},
		"ucloud": {
			"claude-fable-5-1", "claude-sonnet-5", "deepseek-v4-pro-0813",
			"gemini-3.1-pro-preview", "glm-5.3", "gpt-6-astra", "gpt-6-luna",
			"gpt-6-sol", "grok-4.6", "kimi-k3", "qwen3.8-max",
		},
	}
	for accountID, wires := range wantWireIDs {
		want := make([]string, 0, len(wires))
		got := make([]string, 0, len(wires))
		for _, wire := range wires {
			want = append(want, accountID+"-"+wire)
		}
		for routeID, route := range manifest.Routes {
			if route.Account == accountID {
				got = append(got, routeID)
			}
		}
		slices.Sort(want)
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("%s Routes = %v, want %v", accountID, got, want)
		}
	}
	for routeID, route := range manifest.Routes {
		if route.UpstreamModelID() != strings.TrimPrefix(routeID, route.Account+"-") {
			t.Errorf("Route %q wire identity = %q", routeID, route.UpstreamModelID())
		}
		base := route.UpstreamModelID()
		for _, suffix := range []string{"-cc", "-ssvip", "-cdx"} {
			base = strings.TrimSuffix(base, suffix)
		}
		if route.Model != base || len(routeAdmittedProtocols(route)) == 0 {
			t.Errorf("Route %q uses Model %q and protocols %v, want Model %q", routeID, route.Model, routeAdmittedProtocols(route), base)
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
	for accountID, account := range parsedManifest.Accounts {
		for _, endpoint := range []string{account.Endpoints.OpenAIResponses, account.Endpoints.Anthropic} {
			if endpoint == "" {
				continue
			}
			parsed, parseErr := url.Parse(endpoint)
			if parseErr != nil || parsed.Hostname() == "" {
				t.Fatalf("team account %q contains invalid endpoint %q", accountID, endpoint)
			}
		}
	}
}

func TestTeamManifestPresentationSeparatesIdentityFromRecommendation(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	channel := regexp.MustCompile(`^[A-Z][A-Z0-9]*$`)
	for routeID, route := range manifest.Routes {
		if want := route.Account + "-" + route.UpstreamModelID(); routeID != want {
			t.Errorf("route ID %q must preserve Account and provider model identity: %q", routeID, want)
		}
		parts := strings.Split(route.Label, " · ")
		if len(parts) < 2 || len(parts) > 3 || parts[0] != manifest.Accounts[route.Account].Label {
			t.Errorf("route %q label must be Account · Model [· Channel]: %q", routeID, route.Label)
			continue
		}
		for _, part := range parts {
			if part == "" || strings.Join(strings.Fields(part), " ") != part {
				t.Errorf("route %q has noncanonical label spacing: %q", routeID, route.Label)
			}
		}
		if len(parts) == 3 && !channel.MatchString(parts[2]) {
			t.Errorf("route %q channel must be the provider's uppercase channel name: %q", routeID, parts[2])
		}
		if route.Purpose != "" {
			t.Errorf("model catalogue route %q must omit workflow purpose", routeID)
		}
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
