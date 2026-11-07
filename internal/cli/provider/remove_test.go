package provider

import (
	"strings"
	"testing"

	"aigw-cli/internal/secret"

	"github.com/spf13/cobra"
)

// credentialMixed gives the mixed provider a Token and balance credentials.
func (f *fixture) credentialMixed(t *testing.T) {
	t.Helper()
	if err := f.runtime.Secrets.Set("mixed", "sk-mixed-token-0001"); err != nil {
		t.Fatal(err)
	}
	if err := f.runtime.Providers.Set("mixed", secret.DiagnosticCredential{SystemToken: "mixed-system", UserID: "9"}); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveDeletesTheProviderItsModelsAndCredentialsAndDropsRecommendations(t *testing.T) {
	f := newFixture(t)
	f.credentialMixed(t)
	if err := f.run("", "remove", "mixed", "--json"); err != nil {
		t.Fatal(err)
	}
	var document removeDocument
	f.decode(t, &document)
	if document.Status != removalDone || document.Token != credentialPresent || document.BalanceCredential != credentialPresent ||
		strings.Join(document.Models, ",") != "mixed/chat-only" || strings.Join(document.Recommendations, ",") != "hermes" ||
		document.NextAction != "aigw provider list" || document.DryRun {
		t.Fatalf("removal = %+v", document)
	}
	cfg := f.load(t)
	if _, kept := cfg.Providers["mixed"]; kept || len(cfg.Models) != 2 || len(cfg.Recommendations) != 0 {
		t.Fatalf("configuration after the removal = %+v", cfg)
	}
	if present, _ := f.runtime.Providers.Exists("mixed"); present || f.token("mixed") != "" || f.token("team") != teamToken {
		t.Fatal("the removal kept a credential or deleted another provider's")
	}
}

func TestRemovePreviewReportsEverythingAndWritesNothing(t *testing.T) {
	f := newFixture(t)
	f.credentialMixed(t)
	before := f.configuration(t)
	if err := f.run("", "remove", "mixed", "--dry-run", "--json"); err != nil {
		t.Fatal(err)
	}
	var document removeDocument
	f.decode(t, &document)
	if !document.DryRun || document.Status != removalPlanned || document.NextAction != "aigw provider remove mixed" {
		t.Fatalf("preview = %+v", document)
	}
	if err := f.run("", "remove", "mixed", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	text := f.text()
	for _, want := range []string{"Provider removed (preview)", "Models mixed/chat-only", "✓ Token Deleted with the provider",
		"✓ Balance token Deleted with the provider", "Recommendations Dropped for Hermes", "Nothing was written", "Next aigw provider remove mixed"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview lacks %q: %s", want, text)
		}
	}
	if f.configuration(t) != before || f.token("mixed") == "" {
		t.Fatal("a preview wrote something")
	}
}

func TestRemoveRefusesAProviderWhoseModelAClientSelects(t *testing.T) {
	f := newFixture(t)
	before := f.configuration(t)
	p := problem(f.run("", "remove", "team"))
	if p.Title != "Claude, Codex use team/team-model, team/codex-model, so provider team cannot be removed" || p.Fix != "aigw use claude <provider/model>" {
		t.Fatalf("refusal = %+v", p)
	}
	// A blocked JSON preview writes its own document and fails.
	if err := f.run("", "remove", "team", "--dry-run", "--json"); err == nil {
		t.Fatal("a blocked preview succeeded")
	}
	var document removeDocument
	f.decode(t, &document)
	if !document.DryRun || document.Status != removalBlocked || document.Problem != p.Title || document.NextDetail != "aigw use claude <provider/model>" ||
		!strings.Contains(f.out.String(), `"ok": false`) {
		t.Fatalf("blocked preview = %+v\n%s", document, f.out)
	}
	if p := problem(f.run("", "remove", "team", "--dry-run")); p.Fix != "aigw use claude <provider/model>" {
		t.Fatalf("blocked human preview = %+v", p)
	}
	if f.configuration(t) != before || f.token("team") != teamToken {
		t.Fatal("a refusal wrote something")
	}
	if p := problem(f.run("", "remove", "tem")); p.Title != `unknown provider "tem"; did you mean team?` {
		t.Fatalf("unknown provider = %+v", p)
	}
}

func TestRemoveKeepsAModelForTheConfiguration(t *testing.T) {
	f := newFixture(t)
	cfg := f.load(t)
	cfg.Clients = nil
	cfg.Recommendations = nil
	delete(cfg.Models, "mixed-chat")
	delete(cfg.Providers, "mixed")
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	p := problem(f.run("", "remove", "team"))
	if p.Title != "every model belongs to provider team, and aigw needs at least one model" || p.Fix != "aigw provider add <provider> --endpoint <protocol>=<url>" {
		t.Fatalf("the only provider = %+v", p)
	}
	cfg = f.load(t)
	cfg.Providers["spare"] = cfg.Providers["team"]
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if p := problem(f.run("", "remove", "team")); p.Fix != "aigw model add <provider/model>" {
		t.Fatalf("a provider that holds every model = %+v", p)
	}
}

func TestRemoveWithTheEnvironmentNamesTheVariablesToUnset(t *testing.T) {
	f := newFixture(t)
	f.withEnvironment(map[string]string{"AIGW_TOKEN_MIXED": "sk-mixed-token-0001"})
	if err := f.run("", "remove", "mixed"); err != nil {
		t.Fatal(err)
	}
	if text := f.text(); !strings.Contains(text, "! Token Supplied by the environment; unset AIGW_TOKEN_MIXED") || !strings.Contains(text, "Balance token None stored") {
		t.Fatalf("environment removal = %s", text)
	}
	if _, kept := f.load(t).Providers["mixed"]; kept {
		t.Fatal("the environment backend kept the provider")
	}
}

func TestRemoveDeletesNothingThatItCannotRestore(t *testing.T) {
	f := newFixture(t)
	before := f.configuration(t)
	f.runtime.Secrets = failingStore{}
	p := problem(f.run("", "remove", "mixed"))
	if f.configuration(t) != before {
		t.Fatal("a refused removal changed the configuration")
	}
	if p.Title != "the mixed Token could not be read and deleted, so the provider was not removed" || p.Impact != "Nothing was removed." || p.Fix != "aigw check" {
		t.Fatalf("an unreadable Token = %+v", p)
	}
	if err := f.run("", "remove", "mixed", "--dry-run"); err != nil || !strings.Contains(f.text(), "Unknown: the credential backend could not tell") {
		t.Fatalf("a preview with an unreadable Token = %v %s", err, f.text())
	}
	f = newFixture(t)
	f.credentialMixed(t)
	f.runtime.Providers = failingDiagnostics{}
	p = problem(f.run("", "remove", "mixed"))
	if p.Title != "the mixed balance credentials could not be read and deleted, so the provider was not removed" ||
		p.Impact != "Nothing was removed." || p.Fix != "aigw provider edit mixed --no-balance-token" {
		t.Fatalf("unreadable balance credentials = %+v", p)
	}
	if _, kept := f.load(t).Providers["mixed"]; !kept || f.token("mixed") != "sk-mixed-token-0001" {
		t.Fatal("a refused removal removed the provider or did not restore the Token")
	}
}

func TestRemoveRestoresTheCredentialsWhenTheSaveFails(t *testing.T) {
	f := newFixture(t)
	f.credentialMixed(t)
	cfg := f.load(t)
	after := cfg.Clone()
	removeProvider(&after, "mixed")
	// The stored configuration no longer matches what the removal read.
	stale := cfg.Clone()
	provider := stale.Providers["mixed"]
	provider.Label = "Stale"
	stale.Providers["mixed"] = provider
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	document := removeDocument{Token: credentialPresent, BalanceCredential: credentialPresent}
	if err := commitRemoval(cmd, f.runtime, stale, after, "mixed", document); err == nil || !strings.Contains(err.Error(), "configuration preimage changed") {
		t.Fatalf("a stale removal = %v", err)
	}
	credential, err := f.runtime.Providers.Get("mixed")
	if f.token("mixed") != "sk-mixed-token-0001" || err != nil || credential.SystemToken != "mixed-system" {
		t.Fatal("a failed save did not restore the credentials")
	}
}
