package store

import (
	"aigw-cli/internal/domain"
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestDeclaredCatalogOrderSurvivesManifestAndStore(t *testing.T) {
	manifest, err := Parse([]byte(`version = 7
[accounts.alpha]
label = 'Alpha'
order = 20
[accounts.alpha.endpoints]
anthropic = 'https://alpha.test'
[accounts.zulu]
label = 'Zulu'
order = 10
[accounts.zulu.endpoints]
anthropic = 'https://zulu.test'
[models.first]
label = 'First'
order = 10
[models.second]
label = 'Second'
order = 20
[routes.alpha]
account = 'alpha'
model = 'first'
[routes.alpha.interfaces]
anthropic = []
[routes.zulu-a]
account = 'zulu'
model = 'second'
[routes.zulu-a.interfaces]
anthropic = []
[routes.zulu-z]
account = 'zulu'
model = 'first'
[routes.zulu-z.interfaces]
anthropic = []
`), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := domain.Merge(domain.NewConfig(testAdmission()), manifest)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Accounts["unranked"] = domain.Account{Label: "Unranked", Endpoints: domain.Endpoints{Anthropic: "https://unranked.test"}}
	cfg.Models["unranked"] = domain.Model{Label: "Unranked"}
	cfg.Routes["zulu-unranked"] = testRoute("Unranked", "zulu", "unranked", domain.ProtocolAnthropic)
	cfg.Routes["unranked"] = testRoute("Unranked", "unranked", "first", domain.ProtocolAnthropic)
	want := []string{"zulu-z", "zulu-a", "zulu-unranked", "alpha", "unranked"}
	if got := cfg.CatalogRouteIDs(); !slices.Equal(got, want) {
		t.Fatalf("declared Account/Model order = %v, want %v", got, want)
	}
	exported, err := Export(cfg)
	if err != nil {
		t.Fatal(err)
	}
	reparsed, err := Parse(exported, testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	if reparsed.Accounts["zulu"] != cfg.Accounts["zulu"] || reparsed.Models["first"] != cfg.Models["first"] {
		t.Fatal("export lost declared presentation metadata")
	}
	store := NewStore(t.TempDir()+"/config.toml", testAdmission())
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	canonical, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(canonical, []byte("order =")) {
		t.Fatal("catalog metadata reached the published credential reader's strict configuration")
	}
	reloaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.CatalogRouteIDs(); !slices.Equal(got, want) {
		t.Fatalf("stored catalog order = %v, want %v", got, want)
	}
}

func TestCatalogOrderRejectsNegativeRanksWithoutChangingStoredBytes(t *testing.T) {
	for _, subject := range []string{"accounts", "models"} {
		t.Run(subject, func(t *testing.T) {
			store := NewStore(t.TempDir()+"/config.toml", testAdmission())
			cfg := convergenceConfig("current")
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(store.Path())
			if err != nil {
				t.Fatal(err)
			}
			identifier := "current"
			if subject == "models" {
				identifier = "current-model"
			}
			catalog := []byte("[" + subject + "]\n" + identifier + " = -1\n")
			if err := os.WriteFile(store.catalogPath(), catalog, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Load(); err == nil || !strings.Contains(err.Error(), "order must not be negative") {
				t.Fatalf("invalid catalog rank was admitted: %v", err)
			}
			after, err := os.ReadFile(store.Path())
			if err != nil {
				t.Fatal(err)
			}
			afterCatalog, err := os.ReadFile(store.catalogPath())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, before) || !bytes.Equal(afterCatalog, catalog) {
				t.Fatal("refused catalog read changed stored bytes")
			}
		})
	}
}

func TestCatalogRouteIDsGroupsServicesBeforeModels(t *testing.T) {
	for _, ranked := range []bool{true, false} {
		name := "unranked"
		if ranked {
			name = "ranked"
		}
		t.Run(name, func(t *testing.T) {
			cfg := domain.NewConfig(testAdmission())
			endpoints := domain.Endpoints{Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1", OpenAIChatCompletions: "https://team.test/v1"}
			rank := 0
			if ranked {
				rank = 10
			}
			cfg.Accounts["alpha"] = domain.Account{Label: "Alpha", Endpoints: endpoints, Order: 2 * rank}
			cfg.Accounts["zulu"] = domain.Account{Label: "Zulu", Endpoints: endpoints, Order: rank}
			cfg.Models["strong"] = domain.Model{Label: "Strong", Order: rank}
			cfg.Models["weak"] = domain.Model{Label: "Weak", Order: 2 * rank}
			for _, row := range []struct {
				id, account, model string
				protocol           domain.EndpointProtocol
			}{
				{"z-chat", "zulu", "strong", domain.ProtocolOpenAIChatCompletions},
				{"z-responses-strong", "zulu", "strong", domain.ProtocolOpenAIResponses},
				{"z-responses-weak", "zulu", "weak", domain.ProtocolOpenAIResponses},
				{"z-native-strong", "zulu", "strong", domain.ProtocolAnthropic},
				{"z-native-weak", "zulu", "weak", domain.ProtocolAnthropic},
				{"a-native", "alpha", "strong", domain.ProtocolAnthropic},
			} {
				cfg.Routes[row.id] = testRoute(row.id, row.account, row.model, row.protocol)
			}
			cfg.Routes["z-multi"] = domain.Route{Account: "zulu", Model: "strong", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}, domain.ProtocolAnthropic: {}}}
			cfg.SetSelectedRoute(ClientCodex, "z-responses-strong", "")
			cfg.Normalize()
			canonical := cfg.RouteIDs()
			before, err := Export(cfg)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"z-native-strong", "z-native-weak", "z-multi", "z-responses-strong", "z-responses-weak", "z-chat"}
			if ranked {
				want = append(want, "a-native")
			} else {
				want = append([]string{"a-native"}, want...)
			}
			if got := cfg.CatalogRouteIDs(); !slices.Equal(got, want) {
				t.Fatalf("service/Model order = %v, want %v", got, want)
			}
			after, err := Export(cfg)
			if err != nil || !bytes.Equal(before, after) || !slices.Equal(canonical, cfg.RouteIDs()) {
				t.Fatalf("presentation changed canonical enumeration, bindings, or export: %v", err)
			}
		})
	}
}

func TestCatalogOrderSurvivesVerifiedBackupAndCompensation(t *testing.T) {
	store := NewStore(t.TempDir()+"/config.toml", testAdmission())
	cfg := convergenceConfig("current")
	account := cfg.Accounts["current"]
	account.Order = 10
	cfg.Accounts["current"] = account
	cfg.Normalize()
	model := cfg.Models["current-model"]
	model.Order = 20
	cfg.Models["current-model"] = model
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{ClientCodex}); err != nil {
		t.Fatal(err)
	}
	presentation := []byte("[accounts]\ncurrent = 30\n[models]\ncurrent-model = 40\n")
	if err := os.WriteFile(store.catalogPath(), presentation, 0o600); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := store.LoadVerifiedCheckpoint()
	if err != nil || checkpoint.Config.Accounts["current"].Order != 30 || checkpoint.Config.Models["current-model"].Order != 40 {
		t.Fatalf("verified routing did not retain current presentation: %v", err)
	}
	state, err := store.CaptureVerifiedBackupState()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ConvergeVerifiedBackup(state.Snapshot); err != nil {
		t.Fatal(err)
	}
	backup, err := store.LoadBackup()
	if err != nil || backup.Accounts["current"].Order != 30 || backup.Models["current-model"].Order != 40 {
		t.Fatalf("verified backup lost presentation: %v", err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	account.Order = 50
	cfg.Accounts["current"] = account
	after, err := store.Commit(before, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreSnapshot(before, after); err != nil {
		t.Fatal(err)
	}
	got, err := store.CaptureSnapshot()
	if err != nil || !before.equal(got) {
		t.Fatalf("catalog compensation changed the preimage: %v", err)
	}
}
