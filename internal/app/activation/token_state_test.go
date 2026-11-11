package activation

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"aigw-cli/internal/adapter/claude"
	domainreadiness "aigw-cli/internal/app/readiness"
	builtinclients "aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// tokenFiles is a file backend, whose metadata tells a stored Token from an
// empty one, in a private folder.
func tokenFiles(t *testing.T) (secret.Store, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "secrets")
	store, err := secret.Select(secret.Selection{Backend: "file", GOOS: runtime.GOOS, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	return store, root
}

func teamRecommendation() domain.Config {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Providers["team"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Models[domain.NewModelRef("team", "fable")] = domain.Model{Provider: "team", Family: "fable", ID: "fable", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Recommendations[claude.ID] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: domain.NewModelRef("team", "fable")}}
	return cfg
}

// A recommendation whose provider got its deferred Token is applied by the
// team file again, which `aigw provider rotate` already did its part of.
func TestAStoredDeferredTokenNamesTheTeamFileAgain(t *testing.T) {
	cfg := teamRecommendation()
	store, _ := tokenFiles(t)
	deferred := AssessActivation(cfg, store)
	if got := deferred.NextActionFor(nil); got != "aigw provider rotate team" {
		t.Fatalf("a deferred Token names %q", got)
	}
	if err := store.Set("team", "sk-team-0001"); err != nil {
		t.Fatal(err)
	}
	stored := AssessActivation(cfg, store)
	if got := stored.NextActionFor(nil); got != "aigw setup --from <file>" || stored.State != domainreadiness.Deferred {
		t.Fatalf("a stored Token names %q, state %q", got, stored.State)
	}
}

// An activation fix with one compatible provider names that provider's
// command; several are a choice.
func TestOneCompatibleProviderNamesItsCommand(t *testing.T) {
	cfg := teamRecommendation()
	store, _ := tokenFiles(t)
	one := AssessActivation(cfg, store)
	if got := one.NextActionFor(nil); got != "aigw provider rotate team" {
		t.Fatalf("one provider names %q", got)
	}
	cfg.Providers["spare"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://spare.test"}}
	cfg.Models[domain.NewModelRef("spare", "fable")] = domain.Model{Provider: "spare", Family: "fable", ID: "fable", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	recommendation := cfg.Recommendations[claude.ID]
	recommendation.Alternatives = []domain.ClientSelection{{Model: domain.NewModelRef("spare", "fable")}}
	cfg.Recommendations[claude.ID] = recommendation
	want := "Choose one compatible provider: aigw provider rotate spare or aigw provider rotate team"
	two := AssessActivation(cfg, store)
	if got := two.NextActionFor(nil); got != want {
		t.Fatalf("two providers name %q, want %q", got, want)
	}
}

// An empty stored Token is none: a selected, enabled client is deferred with
// the fix that stores one, never configured.
func TestAnEmptyTokenIsNotAvailable(t *testing.T) {
	cfg := teamRecommendation()
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("team", "fable"), "")
	cfg.SetClientActivation(claude.ID, true, "/opt/claude", nil)
	store, root := tokenFiles(t)
	if err := store.Set("team", "sk-team-0001"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tokens", "team"), nil, 0o600); err != nil {
		if err = os.WriteFile(filepath.Join(root, "team"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got := AssessActivation(cfg, store)
	if got.VerificationPrerequisite != "" || got.ClientCredentialPrerequisites[claude.ID] != "run `aigw provider rotate team`" {
		t.Fatalf("an empty Token = %+v", got)
	}
}
