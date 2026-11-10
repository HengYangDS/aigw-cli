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
	cfg, _, err := domain.MergeTeamFile(domain.NewConfig(testAdmission()), manifest, false)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Providers["unranked"] = domain.Provider{Label: "Unranked", Endpoints: domain.Endpoints{Anthropic: "https://unranked.test"}}
	cfg.Families["unranked"] = domain.Family{Label: "Unranked"}
	cfg.Models["zulu-unranked"] = testModel("Unranked", "zulu", "unranked", domain.ProtocolAnthropic)
	cfg.Models["unranked"] = testModel("Unranked", "unranked", "first", domain.ProtocolAnthropic)
	want := []string{"zulu-z", "zulu-a", "zulu-unranked", "alpha", "unranked"}
	if got := cfg.CatalogModelIDs(); !slices.Equal(got, want) {
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
	if reparsed.Providers["zulu"] != cfg.Providers["zulu"] || reparsed.Families["first"] != cfg.Families["first"] {
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
	if got := reloaded.CatalogModelIDs(); !slices.Equal(got, want) {
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
			if err := os.WriteFile(store.componentPath(catalogComponent), catalog, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Load(); err == nil || !strings.Contains(err.Error(), "order must not be negative") {
				t.Fatalf("invalid catalog rank was admitted: %v", err)
			}
			after, err := os.ReadFile(store.Path())
			if err != nil {
				t.Fatal(err)
			}
			afterCatalog, err := os.ReadFile(store.componentPath(catalogComponent))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, before) || !bytes.Equal(afterCatalog, catalog) {
				t.Fatal("refused catalog read changed stored bytes")
			}
		})
	}
}

func TestCatalogModelIDsGroupsServicesBeforeFamilies(t *testing.T) {
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
			cfg.Providers["alpha"] = domain.Provider{Label: "Alpha", Endpoints: endpoints, Order: 2 * rank}
			cfg.Providers["zulu"] = domain.Provider{Label: "Zulu", Endpoints: endpoints, Order: rank}
			cfg.Families["strong"] = domain.Family{Label: "Strong", Order: rank}
			cfg.Families["weak"] = domain.Family{Label: "Weak", Order: 2 * rank}
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
				cfg.Models[row.id] = testModel(row.id, row.account, row.model, row.protocol)
			}
			cfg.Models["z-multi"] = domain.Model{Provider: "zulu", Family: "strong", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}, domain.ProtocolAnthropic: {}}}
			cfg.SetSelectedModel(ClientCodex, "z-responses-strong", "")
			cfg.Normalize()
			canonical := cfg.ModelIDs()
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
			if got := cfg.CatalogModelIDs(); !slices.Equal(got, want) {
				t.Fatalf("service/Model order = %v, want %v", got, want)
			}
			after, err := Export(cfg)
			if err != nil || !bytes.Equal(before, after) || !slices.Equal(canonical, cfg.ModelIDs()) {
				t.Fatalf("presentation changed canonical enumeration, bindings, or export: %v", err)
			}
		})
	}
}

func TestCatalogOrderSurvivesVerifiedBackupAndCompensation(t *testing.T) {
	store := NewStore(t.TempDir()+"/config.toml", testAdmission())
	cfg := convergenceConfig("current")
	account := cfg.Providers["current"]
	account.Order = 10
	cfg.Providers["current"] = account
	cfg.Normalize()
	model := cfg.Families["current-model"]
	model.Order = 20
	cfg.Families["current-model"] = model
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, sessions(ClientCodex)); err != nil {
		t.Fatal(err)
	}
	presentation := []byte("[accounts]\ncurrent = 30\n[models]\ncurrent-model = 40\n")
	if err := os.WriteFile(store.componentPath(catalogComponent), presentation, 0o600); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := store.LoadVerifiedCheckpoint()
	if err != nil || checkpoint.Config.Providers["current"].Order != 30 || checkpoint.Config.Families["current-model"].Order != 40 {
		t.Fatalf("verified routing did not retain current presentation: %v", err)
	}
	// A relabel backs up the generation that holds the presentation.
	current, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(relabeled(current, "Relabeled")); err != nil {
		t.Fatal(err)
	}
	backup, err := store.LoadBackup()
	if err != nil || backup.Providers["current"].Order != 30 || backup.Families["current-model"].Order != 40 {
		t.Fatalf("the backup lost presentation: %v", err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	account.Order = 50
	cfg.Providers["current"] = account
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
