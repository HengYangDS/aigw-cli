package provider

import (
	"errors"
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
	if mixed.State != balanceFailed || mixed.Problem != "no balance credentials are stored" ||
		mixed.NextDetail != "aigw provider edit mixed --balance-token-stdin --balance-user-id <user-id>" ||
		document.Providers[1].State != balanceReported || document.NextDetail != mixed.NextDetail {
		t.Fatalf("balances = %+v", document)
	}
	if !strings.Contains(f.out.String(), `"ok": false`) {
		t.Fatalf("a failed probe's document = %s", f.out)
	}
	if err := f.run("", "balance"); err == nil || !strings.Contains(f.text(), "✗ Balance no balance credentials are stored Fix: aigw provider edit mixed") {
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
	if item := document.Providers[0]; item.Problem != "no Token is stored" || item.NextDetail != "aigw provider rotate mixed" {
		t.Fatalf("a provider without a Token = %+v", item)
	}
	f.http.status["balance.test"] = http.StatusInternalServerError
	_ = f.run("", "balance", "team", "--json")
	f.decode(t, &document)
	if item := document.Providers[0]; !strings.HasPrefix(item.Problem, "DMXAPI platform API returned HTTP 500") || item.NextAction != "aigw provider balance team" {
		t.Fatalf("a failed probe = %+v", item)
	}
	f.runtime.Secrets = emptyToken{Store: f.runtime.Secrets}
	_ = f.run("", "balance", "team", "--json")
	var empty balanceDocument
	f.decode(t, &empty)
	if item := empty.Providers[0]; item.Problem != "no Token is stored" || item.NextAction != "" || item.NextDetail != "aigw provider rotate team" {
		t.Fatalf("an empty Token = %+v", item)
	}
	f.runtime.Secrets = failingStore{}
	_ = f.run("", "balance", "team", "--json")
	f.decode(t, &document)
	if item := document.Providers[0]; item.Problem != "the credential backend could not read the Token" || item.NextAction != "aigw provider show team" {
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
	f.withEnvironment(map[string]string{"AIGW_BALANCE_TOKEN_TEAM": "system-token", "AIGW_BALANCE_USER_ID_TEAM": "42"})
	_ = f.run("", "balance", "team", "--json")
	var document balanceDocument
	f.decode(t, &document)
	if item := document.Providers[0]; item.NextDetail != "Set AIGW_TOKEN_TEAM in aigw's environment." {
		t.Fatalf("an environment without the Token = %+v", item)
	}
	f.withEnvironment(map[string]string{"AIGW_TOKEN_TEAM": teamToken})
	_ = f.run("", "balance", "team", "--json")
	f.decode(t, &document)
	if item := document.Providers[0]; item.NextDetail != "Set AIGW_BALANCE_TOKEN_TEAM and AIGW_BALANCE_USER_ID_TEAM in aigw's environment." {
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

// TestBalanceRerunsOnlyWhatALaterRequestMayNotRepeat shows the fix of each
// kind of probe failure: the same probe after no response or HTTP 408, 429 or
// 5xx; new balance credentials after HTTP 401 or 403 or a refused query; a
// new Token after the account lists none that matches; and the probe's
// settings after anything else. The environment backend's fixes set its
// variables before the probe runs again.
func TestBalanceRerunsOnlyWhatALaterRequestMayNotRepeat(t *testing.T) {
	edit := "aigw provider edit team --balance-token-stdin --balance-user-id <user-id>"
	settings := "Correct the balance probe of provider team in the configuration file; `aigw config path` prints its location."
	unlisted := `{"success":true,"data":{"page_size":100,"items":[{"name":"other","key":"xxxx**********0002","status":1}]}}`
	fullPage := `{"success":true,"data":{"page_size":100,"items":[` + strings.TrimSuffix(strings.Repeat(`{"key":"xxxx**********0002"},`, 100), ",") + `]}}`
	tooMany := "The provider's account lists more Tokens than the balance probe reads; delete the Tokens that you no longer use at the provider, then run `aigw provider balance team`."
	for _, test := range []struct {
		name, user, tokens string
		status             int
		failure            error
		fix, environment   string
	}{
		{name: "no response", failure: errors.New("offline"), fix: "aigw provider balance team", environment: "aigw provider balance team"},
		{name: "rate limit", status: http.StatusTooManyRequests, fix: "aigw provider balance team", environment: "aigw provider balance team"},
		{name: "server error", status: http.StatusBadGateway, fix: "aigw provider balance team", environment: "aigw provider balance team"},
		{name: "rejected", status: http.StatusUnauthorized, fix: edit,
			environment: "Set AIGW_BALANCE_TOKEN_TEAM and AIGW_BALANCE_USER_ID_TEAM to balance credentials that the provider accepts, then run `aigw provider balance team`."},
		{name: "forbidden", status: http.StatusForbidden, fix: edit},
		{name: "refused query", user: `{"success":false,"message":"no access"}`, fix: edit},
		{name: "unlisted Token", tokens: unlisted, fix: "aigw provider rotate team",
			environment: "Set AIGW_TOKEN_TEAM to the Token that the provider's account lists, then run `aigw provider balance team`."},
		{name: "wrong base URL", status: http.StatusNotFound, fix: settings, environment: settings},
		{name: "too many Tokens", tokens: fullPage, fix: tooMany, environment: tooMany},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, readOnly := range []bool{false, true} {
				want := test.fix
				f := newFixture(t)
				if readOnly {
					if test.environment == "" {
						continue
					}
					want = test.environment
					f.withEnvironment(map[string]string{"AIGW_TOKEN_TEAM": teamToken, "AIGW_BALANCE_TOKEN_TEAM": "system-token", "AIGW_BALANCE_USER_ID_TEAM": "42"})
				}
				f.http.user, f.http.tokens = test.user, test.tokens
				if test.failure != nil {
					f.http.failures["balance.test"] = test.failure
				} else if test.status != 0 {
					f.http.status["balance.test"] = test.status
				}
				if err := f.run("", "balance", "team", "--json"); err == nil {
					t.Fatal("a failed probe succeeded")
				}
				var document balanceDocument
				f.decode(t, &document)
				if item := document.Providers[0]; item.State != balanceFailed || item.NextAction+item.NextDetail != want {
					t.Fatalf("environment %t: %+v", readOnly, item)
				}
			}
		})
	}
}

func TestBalanceNamesAnIncompleteEnvironmentPairAndAFailedBackend(t *testing.T) {
	f := newFixture(t)
	f.withEnvironment(map[string]string{"AIGW_TOKEN_TEAM": teamToken, "AIGW_BALANCE_TOKEN_TEAM": "system-token"})
	_ = f.run("", "balance", "team", "--json")
	var document balanceDocument
	f.decode(t, &document)
	if item := document.Providers[0]; item.Problem != "the balance credentials are incomplete: AIGW_BALANCE_USER_ID_TEAM is not set, and the other balance variable alone supplies nothing" ||
		item.NextDetail != "Set AIGW_BALANCE_USER_ID_TEAM in aigw's environment." {
		t.Fatalf("half of the pair = %+v", item)
	}
	f = newFixture(t)
	f.runtime.Providers = &stubDiagnostics{getErr: errors.New("backend failed")}
	_ = f.run("", "balance", "team", "--json")
	f.decode(t, &document)
	if item := document.Providers[0]; item.Problem != "the credential backend could not read the balance credentials" || item.NextAction != "aigw provider show team" {
		t.Fatalf("a failed backend = %+v", item)
	}
}

func TestBalanceAndRemoveReportAnOutputThatFails(t *testing.T) {
	f := newFixture(t)
	f.runtime.Out = failingOutput{}
	for _, args := range [][]string{{"balance"}, {"balance", "--json"}, {"remove", "team", "--dry-run", "--json"}} {
		if err := f.run("", args...); err == nil || !strings.Contains(err.Error(), "output refused") {
			t.Fatalf("%q with a failing output = %v", args, err)
		}
	}
}

// failingOutput refuses every write.
type failingOutput struct{}

func (failingOutput) Write([]byte) (int, error) { return 0, errors.New("output refused") }

// emptyToken is a store whose Token slot is present and empty, as a file
// backend reports one.
type emptyToken struct{ secret.Store }

func (emptyToken) Get(string) (string, error) { return "", secret.ErrEmpty }
