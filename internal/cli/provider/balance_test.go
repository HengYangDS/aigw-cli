package provider

import (
	"net/http"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// probeMixed gives the mixed provider a DMXAPI balance probe.
func (f *fixture) probeMixed(t *testing.T, kind string) {
	t.Helper()
	cfg := f.load(t)
	provider := cfg.Providers["mixed"]
	provider.BalanceProbe = &domain.BalanceProbe{Kind: kind, BaseURL: "https://balance.test"}
	cfg.Providers["mixed"] = provider
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestBalanceReportsEveryProviderAndAProviderWithoutAProbeDoesNotFail(t *testing.T) {
	f := newFixture(t)
	if err := f.run("", "balance", "--json"); err != nil {
		t.Fatal(err)
	}
	var document balanceDocument
	f.decode(t, &document)
	if len(document.Providers) != 2 || document.Providers[0].ID != "mixed" || document.Providers[0].State != balanceNoProbe ||
		document.Providers[1].State != balanceReported || *document.Providers[1].Balance != 10 || document.Providers[1].Token.Name != "team" ||
		document.Providers[1].Token.Remaining != 5 || document.Providers[1].Token.RemainingRequests != 42 || document.NextAction != "aigw check" {
		t.Fatalf("balances = %+v", document)
	}
	if err := f.run("", "balance"); err != nil {
		t.Fatal(err)
	}
	text := f.text()
	for _, want := range []string{"AIGW Balances", "Mixed · Balance No balance probe", "Team Balance $10.0000", "✓ Token status Enabled",
		"Remaining quota $5.0000", "Remaining requests 42 requests", "Next aigw check"} {
		if !strings.Contains(text, want) {
			t.Fatalf("balances lack %q: %s", want, text)
		}
	}
	if strings.Contains(f.out.String(), teamToken) || strings.Contains(f.out.String(), "system-token") {
		t.Fatal("balance wrote a credential")
	}
	if err := f.run("", "balance", "team", "mixed", "team", "--json"); err != nil {
		t.Fatal(err)
	}
	f.decode(t, &document)
	if len(document.Providers) != 2 || document.Providers[0].ID != "team" || document.Providers[1].ID != "mixed" {
		t.Fatalf("named balances = %+v", document.Providers)
	}
}

func TestBalanceFailureHidesNoOtherProviderAndExitsWithItsFix(t *testing.T) {
	f := newFixture(t)
	f.probeMixed(t, "dmxapi")
	if err := f.run("", "balance", "--json"); err == nil {
		t.Fatal("a failed probe succeeded")
	}
	var document balanceDocument
	f.decode(t, &document)
	mixed := document.Providers[0]
	if mixed.State != balanceFailed || mixed.Problem != "the balance credentials cannot be read: none is stored" ||
		mixed.NextDetail != "aigw provider edit mixed --balance-token-stdin --balance-user-id <user-id>" ||
		document.Providers[1].State != balanceReported || document.NextDetail != mixed.NextDetail {
		t.Fatalf("balances = %+v", document)
	}
	if !strings.Contains(f.out.String(), `"ok": false`) {
		t.Fatalf("a failed probe's document = %s", f.out)
	}
	if err := f.run("", "balance"); err == nil || !strings.Contains(f.text(), "✗ Balance the balance credentials cannot be read: none is stored Fix: aigw provider edit mixed") {
		t.Fatalf("human failure = %v %s", err, f.text())
	}
}

func TestBalanceNamesTheFixOfEachFailure(t *testing.T) {
	f := newFixture(t)
	f.probeMixed(t, "dmxapi")
	if err := f.runtime.Providers.Set("mixed", secret.DiagnosticCredential{SystemToken: "system-token", UserID: "42"}); err != nil {
		t.Fatal(err)
	}
	_ = f.run("", "balance", "mixed", "--json")
	var document balanceDocument
	f.decode(t, &document)
	if item := document.Providers[0]; item.Problem != "the Token cannot be read: none is stored" || item.NextDetail != "aigw provider rotate mixed" {
		t.Fatalf("a provider without a Token = %+v", item)
	}
	f.http.status["balance.test"] = http.StatusInternalServerError
	_ = f.run("", "balance", "team", "--json")
	f.decode(t, &document)
	if item := document.Providers[0]; !strings.HasPrefix(item.Problem, "DMXAPI platform API returned HTTP 500") || item.NextAction != "aigw provider balance team" {
		t.Fatalf("a failed probe = %+v", item)
	}
	f.runtime.Secrets = failingStore{}
	_ = f.run("", "balance", "team", "--json")
	f.decode(t, &document)
	if item := document.Providers[0]; item.Problem != "the Token cannot be read: the credential backend failed, which does not mean that none is stored" || item.NextAction != "aigw check" {
		t.Fatalf("a failed backend = %+v", item)
	}
	f.probeMixed(t, "future")
	if err := f.run("", "balance", "mixed", "--json"); err != nil {
		t.Fatal(err)
	}
	f.decode(t, &document)
	if item := document.Providers[0]; item.State != balanceNoProbe || item.Detail != "This AIGW build has no future balance probe" {
		t.Fatalf("an unbundled probe = %+v", item)
	}
}

func TestBalanceWithTheEnvironmentNamesItsVariables(t *testing.T) {
	f := newFixture(t)
	f.withEnvironment(map[string]string{"AIGW_DIAGNOSTIC_SYSTEM_TOKEN_TEAM": "system-token", "AIGW_DIAGNOSTIC_USER_ID_TEAM": "42"})
	_ = f.run("", "balance", "team", "--json")
	var document balanceDocument
	f.decode(t, &document)
	if item := document.Providers[0]; item.NextDetail != "Set AIGW_TOKEN_TEAM in aigw's environment." {
		t.Fatalf("an environment without the Token = %+v", item)
	}
	f.withEnvironment(map[string]string{"AIGW_TOKEN_TEAM": teamToken})
	_ = f.run("", "balance", "team", "--json")
	f.decode(t, &document)
	if item := document.Providers[0]; item.NextDetail != "Set AIGW_DIAGNOSTIC_SYSTEM_TOKEN_TEAM and AIGW_DIAGNOSTIC_USER_ID_TEAM in aigw's environment." {
		t.Fatalf("an environment without balance credentials = %+v", item)
	}
}

func TestBalanceRefusesUnknownProvidersAndAnEmptyConfiguration(t *testing.T) {
	f := newFixture(t)
	if p := problem(f.run("", "balance", "tem")); p.Title != `unknown provider "tem"; did you mean team?` || p.Fix != "aigw provider list" {
		t.Fatalf("unknown provider = %+v", p)
	}
	f.runtime.Config = emptyStore(t)
	if p := problem(f.run("", "balance")); p.Title != "no provider is configured" || p.Fix != "aigw setup" {
		t.Fatalf("no provider = %+v", p)
	}
	if len(f.http.sent()) != 0 {
		t.Fatal("a refusal sent a request")
	}
}
