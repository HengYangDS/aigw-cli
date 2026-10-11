package configuration

import (
	"bytes"
	"os"
	"slices"
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
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Merge(NewConfig(), manifest)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"zulu-z", "zulu-a", "alpha"}
	if got := cfg.CatalogRouteIDs(); !slices.Equal(got, want) {
		t.Fatalf("declared Account/Model order = %v, want %v", got, want)
	}
	exported, err := Export(cfg)
	if err != nil {
		t.Fatal(err)
	}
	reparsed, err := Parse(exported)
	if err != nil {
		t.Fatal(err)
	}
	if reparsed.Accounts["zulu"] != cfg.Accounts["zulu"] || reparsed.Models["first"] != cfg.Models["first"] {
		t.Fatal("export lost declared presentation metadata")
	}
	store := NewStore(t.TempDir() + "/config.toml")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	canonical, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(canonical, []byte("order =")) {
		t.Fatal("catalogue metadata reached the published credential reader's strict configuration")
	}
	reloaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.CatalogRouteIDs(); !slices.Equal(got, want) {
		t.Fatalf("stored catalogue order = %v, want %v", got, want)
	}
}

func TestCatalogOrderSurvivesVerifiedBackupAndCompensation(t *testing.T) {
	store := NewStore(t.TempDir() + "/config.toml")
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
		t.Fatalf("catalogue compensation changed the preimage: %v", err)
	}
}
