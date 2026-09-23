package configuration

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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
		ClientCodex:         {model: "gpt-6-astra", runtimeProtocol: ProtocolOpenAIResponses},
		ClientHermes:        {model: "claude-fable-5-1", storedProtocol: ProtocolAnthropic, runtimeProtocol: ProtocolAnthropic},
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
			for _, profile := range parsedManifest.Routes {
				modelOffered = modelOffered || profile.Account == accountID && profile.Model == want.model
			}
			if runtime.AccountID != accountID || modelOffered && runtime.Model != want.model || runtime.Protocol != want.runtimeProtocol {
				t.Fatalf("%s route for Account %q = Account %q model %q protocol %q, want model %q protocol %q", client, accountID, runtime.AccountID, runtime.Model, runtime.Protocol, want.model, want.runtimeProtocol)
			}
		}
	}
}

func TestTeamManifestSeparatesGeneralModelsFromAccountRoutes(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	want := map[string][]string{
		"grok":     {"grok-4.6", "grok-4.3"},
		"gemini":   {"gemini-3.1-pro-preview", "gemini-3.8-flash"},
		"deepseek": {"deepseek-v4-pro-0813", "deepseek-v4-flash-0731"},
		"qwen":     {"qwen3.8-max", "qwen3.7-plus"},
		"glm":      {"glm-5.3", "glm-5.3-flash"},
		"kimi":     {"kimi-k3", "kimi-k2.7-code-highspeed"},
	}
	for accountID := range manifest.Accounts {
		for family, models := range want {
			for _, modelID := range models {
				if _, ok := manifest.Models[modelID]; !ok {
					t.Errorf("team manifest missing %s Model %q", family, modelID)
				}
				routeID := accountID + "-" + modelID
				route, ok := manifest.Routes[routeID]
				if !ok {
					t.Errorf("team manifest missing %s %s Route %q", accountID, family, routeID)
					continue
				}
				if route.Account != accountID || route.Model != modelID || len(routeAdmittedProtocols(route)) == 0 {
					t.Errorf("team Route %q = %#v", routeID, route)
				}
			}
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
	for profileID, profile := range manifest.Routes {
		if want := profile.Account + "-" + upstreamModel(profile); profileID != want {
			t.Errorf("profile ID %q must preserve Account and provider model identity: %q", profileID, want)
		}
		parts := strings.Split(profile.Label, " · ")
		if len(parts) < 2 || len(parts) > 3 || parts[0] != manifest.Accounts[profile.Account].Label {
			t.Errorf("profile %q label must be Account · Model [· Channel]: %q", profileID, profile.Label)
			continue
		}
		for _, part := range parts {
			if part == "" || strings.Join(strings.Fields(part), " ") != part {
				t.Errorf("profile %q has noncanonical label spacing: %q", profileID, profile.Label)
			}
		}
		if len(parts) == 3 && !channel.MatchString(parts[2]) {
			t.Errorf("profile %q channel must be the provider's uppercase channel name: %q", profileID, parts[2])
		}
		if profile.Purpose != "" {
			t.Errorf("model catalogue profile %q must omit workflow purpose", profileID)
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
	for client, model := range map[string]string{ClientClaude: "claude-fable-5-1", ClientCodex: "gpt-6-astra", ClientHermes: "claude-fable-5-1"} {
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
