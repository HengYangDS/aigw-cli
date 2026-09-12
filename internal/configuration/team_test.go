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

func TestTeamConfigurationManifestIsReviewedVersionFour(t *testing.T) {
	_, parsedManifest := loadTeamManifest(t)
	if parsedManifest.Version != 4 || len(parsedManifest.Accounts) != 3 || len(parsedManifest.Profiles) == 0 {
		t.Fatalf("team manifest = version %d, %d Accounts, %d profiles", parsedManifest.Version, len(parsedManifest.Accounts), len(parsedManifest.Profiles))
	}
	for _, accountID := range []string{"aihubmix", "dmxapi", "ucloud"} {
		if _, ok := parsedManifest.Accounts[accountID]; !ok {
			t.Fatalf("team manifest missing account %q", accountID)
		}
	}
	recommendedModels := map[string]string{
		ClientClaude: "claude-fable-5-1",
		ClientCodex:  "gpt-6-astra",
	}
	if len(parsedManifest.RecommendedRoutes) != len(recommendedModels) {
		t.Fatalf("team manifest recommended routes = %#v", parsedManifest.RecommendedRoutes)
	}
	for client, want := range recommendedModels {
		profile := parsedManifest.Profiles[parsedManifest.RecommendedRoutes[client]]
		if profile.Client != client || profile.Model != want {
			t.Fatalf("recommended %s profile = %#v, want model %q", client, profile, want)
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
		for client, wantModel := range recommendedModels {
			runtime, resolveErr := selected.ResolveRuntime(client, "")
			if resolveErr != nil {
				t.Fatalf("resolve %s route for Account %q: %v", client, accountID, resolveErr)
			}
			modelOffered := false
			for _, profile := range parsedManifest.Profiles {
				modelOffered = modelOffered || profile.Account == accountID && profile.Client == client && profile.Model == wantModel
			}
			if runtime.AccountID != accountID || modelOffered && runtime.Model != wantModel {
				t.Fatalf("%s route for Account %q = Account %q model %q, want model %q", client, accountID, runtime.AccountID, runtime.Model, wantModel)
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
	for profileID, profile := range manifest.Profiles {
		if want := profile.Account + "-" + profile.Model; profileID != want {
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
	for client, model := range map[string]string{ClientClaude: "claude-fable-5-1", ClientCodex: "gpt-6-astra"} {
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
	cfg.Profiles = manifest.Profiles
	cfg.Routes = manifest.RecommendedRoutes
	canonical, err := Export(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, canonical) {
		t.Error("team manifest must use the native canonical export order and layout")
	}
}
