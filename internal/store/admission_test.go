package store

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

// TestStoreAndManifestFollowTheInjectedAdmission shows that loading, saving,
// checkpoints and manifests read the admission that the store and the
// configuration carry: a client that only it admits round-trips, and the same
// bytes refuse that client under an admission without it.
func TestStoreAndManifestFollowTheInjectedAdmission(t *testing.T) {
	future := domain.ClientSpec{ID: "future", Label: "Future", EndpointProtocols: []domain.EndpointProtocol{domain.ProtocolResponses}}
	admission := domain.NewAdmission(future)
	cfg := domain.NewConfig(admission)
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Responses: "https://gateway.test/v1"}}
	cfg.Models[domain.NewModelRef("gateway", "model")] = testModel("Route", "gateway", "model", domain.ProtocolResponses)
	cfg.Recommendations["future"] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: domain.NewModelRef("gateway", "model")}}
	cfg.SetSelectedModel("future", domain.NewModelRef("gateway", "model"), "")
	cfg.SetClientActivation("future", true, "/opt/future", nil)
	path := filepath.Join(t.TempDir(), "aigw.toml")
	store := NewStore(path, admission)
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil || !reflect.DeepEqual(loaded.EnabledClientIDs(), []string{"future"}) {
		t.Fatalf("Load() enabled %v, %v", loaded.EnabledClientIDs(), err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), loaded, sessions("future")); err != nil {
		t.Fatal(err)
	}
	if checkpoint, err := store.LoadVerifiedCheckpoint(); err != nil || checkpoint.Config.SelectedModel("future") != domain.NewModelRef("gateway", "model") {
		t.Fatalf("LoadVerifiedCheckpoint() = %#v, %v", checkpoint, err)
	}
	manifest, err := ExportTeamFile(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseTeamFile(manifest, admission); err != nil {
		t.Fatalf("ParseTeamFile() refused its own export: %v", err)
	}

	if _, err := NewStore(path, testAdmission()).Load(); err == nil || !strings.Contains(err.Error(), `"future"`) {
		t.Fatalf("an admission without the client loaded its binding: %v", err)
	}
	if _, err := ParseTeamFile(manifest, testAdmission()); err == nil || !strings.Contains(err.Error(), `"future"`) {
		t.Fatalf("an admission without the client parsed its recommendation: %v", err)
	}
	cfg.Clients[ClientCodex] = domain.ClientBinding{Model: domain.NewModelRef("gateway", "model")}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), `unknown client binding "codex"`) {
		t.Fatalf("a client outside the admission was accepted: %v", err)
	}
}
