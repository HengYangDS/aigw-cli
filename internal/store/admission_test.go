package store

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

// TestPublishedProfilesMigrateByTheAdmittedProfileProtocol admits fixture
// clients so that the version 3 profile migration is shown to read each
// client's published profile protocol from its specification.
func TestPublishedProfilesMigrateByTheAdmittedProfileProtocol(t *testing.T) {
	responses := []domain.EndpointProtocol{domain.ProtocolOpenAIResponses}
	native := domain.ClientSpec{ID: "native", Label: "Native", EndpointProtocols: responses, PublishedProfileProtocol: domain.ProtocolOpenAIResponses}
	provider := domain.ClientSpec{ID: "provider", Label: "Provider", EndpointProtocols: responses}
	admission := domain.NewAdmission(native, provider)
	published := publishedConfig{
		Accounts: map[string]domain.Account{"gateway": {Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}},
		Profiles: map[string]publishedProfile{"route": {Label: "Route", Account: "gateway", Client: "native", Model: "model"}},
	}
	migrated, err := migratePublishedConfig(published, admission)
	if err != nil {
		t.Fatal(err)
	}
	if protocols := migrated.Routes["route"].AdmittedProtocols(); !reflect.DeepEqual(protocols, responses) {
		t.Fatalf("migrated Route protocols = %v", protocols)
	}
	published.Profiles["route"] = publishedProfile{Label: "Route", Account: "gateway", Client: "provider", Model: "model"}
	if _, err := migratePublishedConfig(published, admission); err == nil || !strings.Contains(err.Error(), `no admitted client protocol for "provider"`) {
		t.Fatalf("a profile of a client without a published protocol migrated: %v", err)
	}
}

// TestStoreAndManifestFollowTheInjectedAdmission shows that loading, saving,
// checkpoints and manifests read the admission that the store and the
// configuration carry: a client that only it admits round-trips, and the same
// bytes refuse that client under an admission without it.
func TestStoreAndManifestFollowTheInjectedAdmission(t *testing.T) {
	future := domain.ClientSpec{ID: "future", Label: "Future", EndpointProtocols: []domain.EndpointProtocol{domain.ProtocolOpenAIResponses}}
	admission := domain.NewAdmission(future)
	cfg := domain.NewConfig(admission)
	cfg.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Routes["route"] = testRoute("Route", "gateway", "model", domain.ProtocolOpenAIResponses)
	cfg.Recommendations["future"] = domain.ClientRecommendation{Primary: domain.ClientSelection{Route: "route"}}
	cfg.SetSelectedRoute("future", "route", "")
	cfg.SetClientActivation("future", true, "/opt/future", nil)
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path, admission)
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil || !reflect.DeepEqual(loaded.EnabledClientIDs(), []string{"future"}) {
		t.Fatalf("Load() enabled %v, %v", loaded.EnabledClientIDs(), err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), loaded, []string{"future"}); err != nil {
		t.Fatal(err)
	}
	if checkpoint, err := store.LoadVerifiedCheckpoint(); err != nil || checkpoint.Config.SelectedRoute("future") != "route" {
		t.Fatalf("LoadVerifiedCheckpoint() = %#v, %v", checkpoint, err)
	}
	manifest, err := Export(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(manifest, admission); err != nil {
		t.Fatalf("Parse() refused its own export: %v", err)
	}

	if _, err := NewStore(path, testAdmission()).Load(); err == nil || !strings.Contains(err.Error(), `"future"`) {
		t.Fatalf("an admission without the client loaded its binding: %v", err)
	}
	if _, err := Parse(manifest, testAdmission()); err == nil || !strings.Contains(err.Error(), `"future"`) {
		t.Fatalf("an admission without the client parsed its recommendation: %v", err)
	}
	cfg.Clients[ClientCodex] = domain.ClientBinding{Route: "route"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), `unknown client binding "codex"`) {
		t.Fatalf("a client outside the admission was accepted: %v", err)
	}
}
