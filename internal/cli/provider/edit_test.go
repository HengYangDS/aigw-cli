package provider

import (
	"errors"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

func TestEditChangesTheLabelAndKeepsTheToken(t *testing.T) {
	f := newFixture(t)
	if err := f.run("", "edit", "team", "--label", "Team Two", "--json"); err != nil {
		t.Fatal(err)
	}
	var document changeDocument
	f.decode(t, &document)
	if document.Label != "Team Two" || document.TokenValidated != nil || len(document.ClearedCapabilities) != 0 || document.NextAction != "aigw provider show team" {
		t.Fatalf("edit = %+v", document)
	}
	if f.load(t).Providers["team"].Label != "Team Two" || f.token("team") != teamToken || len(f.http.sent()) != 0 {
		t.Fatal("a label edit did not save, changed the Token or sent a request")
	}
}

func TestEditReplacingAnEndpointClearsWhatWasQualifiedOrSetUpForIt(t *testing.T) {
	f := newFixture(t)
	if err := f.run("", "edit", "team", "--endpoint", "anthropic=https://new.test/", "--json"); err != nil {
		t.Fatal(err)
	}
	var document changeDocument
	f.decode(t, &document)
	if len(document.ClearedCapabilities) != 1 || document.ClearedCapabilities[0].Protocol != "anthropic" ||
		document.ClearedCapabilities[0].Model != "team/team-model" || strings.Join(document.ClearedCapabilities[0].Capabilities, ",") != "tool_search" ||
		len(document.ClearedForwarding) != 0 || document.NextAction != "aigw check" {
		t.Fatalf("anthropic edit = %+v", document)
	}
	cfg := f.load(t)
	if cfg.Providers["team"].Endpoints.Anthropic != "https://new.test" || len(cfg.Models["team"].Protocols[domain.ProtocolAnthropic]) != 0 ||
		cfg.Clients["codex"].ForwardingEndpoint == "" {
		t.Fatalf("saved anthropic edit = %+v", cfg)
	}
	// The Responses endpoint, which Codex reaches through forwarding, keeps
	// Codex's retry budgets, which belong to the provider.
	if err := f.run("", "edit", "team", "--endpoint", "responses=https://new.test/v1"); err != nil {
		t.Fatal(err)
	}
	if text := f.text(); !strings.Contains(text, "! Forwarding cleared Codex") || strings.Contains(text, "Capabilities cleared") {
		t.Fatalf("responses edit = %s", text)
	}
	cfg = f.load(t)
	if binding := cfg.Clients["codex"]; binding.ForwardingEndpoint != "" || binding.RetryPolicy == nil || *binding.RetryPolicy.RequestMaxRetries != 3 {
		t.Fatalf("Codex after the responses edit = %+v", binding)
	}
	// A new protocol and the same URL again clear nothing.
	if err := f.run("", "edit", "team", "--endpoint", "chat=https://new.test/v1", "--endpoint", "anthropic=https://new.test", "--json"); err != nil {
		t.Fatal(err)
	}
	document = changeDocument{}
	f.decode(t, &document)
	if len(document.ClearedCapabilities) != 0 || len(document.ClearedForwarding) != 0 || f.load(t).Providers["team"].Endpoints.OpenAIChatCompletions != "https://new.test/v1" {
		t.Fatalf("an added endpoint = %+v", document)
	}
}

func TestEditPreviewWritesNothing(t *testing.T) {
	f := newFixture(t)
	before := f.configuration(t)
	if err := f.run("", "edit", "team", "--endpoint", "anthropic=https://new.test", "--balance-token-stdin", "--balance-user-id", "7", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	text := f.text()
	for _, want := range []string{"Provider edited (preview)", "! Capabilities cleared team/team-model on Anthropic Messages: tool_search",
		"✓ Balance token Read and stored when the edit is applied", "Nothing was written",
		"Next aigw provider edit team --balance-token-stdin --balance-user-id 7 --endpoint anthropic=https://new.test"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview lacks %q: %s", want, text)
		}
	}
	credential, _ := f.runtime.Providers.Get("team")
	if f.configuration(t) != before || credential.SystemToken != "system-token" {
		t.Fatal("a preview wrote the configuration or the balance credentials")
	}
	if err := f.run("", "edit", "team", "--no-balance-token", "--dry-run"); err != nil || !strings.Contains(f.text(), "Removed when the edit is applied") {
		t.Fatalf("removal preview = %v %s", err, f.text())
	}
}

func TestEditStoresAndRemovesBalanceCredentials(t *testing.T) {
	f := newFixture(t)
	before := f.configuration(t)
	if err := f.run("new-system\n", "edit", "team", "--balance-token-stdin", "--balance-user-id", "7", "--json"); err != nil {
		t.Fatal(err)
	}
	var document changeDocument
	f.decode(t, &document)
	credential, err := f.runtime.Providers.Get("team")
	if err != nil || credential != (secret.DiagnosticCredential{SystemToken: "new-system", UserID: "7"}) || document.BalanceCredential != "stored" ||
		document.NextAction != "aigw provider balance team" || f.token("team") != teamToken || f.configuration(t) != before {
		t.Fatalf("stored balance credentials = %+v, %v; document %+v", credential, err, document)
	}
	f.runtime.Interactive = true
	ask := &prompter{values: []string{"prompted-system"}}
	f.runtime.Prompt = ask
	if err := f.run("", "edit", "team", "--balance-token-stdin", "--balance-user-id", "8"); err != nil {
		t.Fatal(err)
	}
	if credential, _ := f.runtime.Providers.Get("team"); credential.SystemToken != "prompted-system" || ask.asked[0] != "Paste the Team balance token: " ||
		!strings.Contains(f.text(), "✓ Balance token Stored") {
		t.Fatalf("prompted balance token = %+v, %q", credential, ask.asked)
	}
	if err := f.run("", "edit", "team", "--no-balance-token"); err != nil || !strings.Contains(f.text(), "✓ Balance token Removed") {
		t.Fatalf("removal = %v %s", err, f.text())
	}
	if present, _ := f.runtime.Providers.Exists("team"); present || f.token("team") != teamToken {
		t.Fatal("removing the balance credentials kept them or changed the Token")
	}
	// Removing absent balance credentials changes nothing and succeeds.
	if err := f.run("", "edit", "mixed", "--no-balance-token"); err != nil {
		t.Fatal(err)
	}
}

func TestEditRefusals(t *testing.T) {
	f := newFixture(t)
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"edit", "team"}, "provider edit needs --label, --endpoint, --balance-token-stdin with --balance-user-id, or --no-balance-token"},
		{[]string{"edit", "team", "--label", ""}, "--label requires a non-empty value"},
		{[]string{"edit", "team", "--balance-token-stdin"}, "--balance-token-stdin and --balance-user-id go together"},
		{[]string{"edit", "team", "--balance-user-id", "7"}, "--balance-token-stdin and --balance-user-id go together"},
		{[]string{"edit", "team", "--balance-token-stdin", "--balance-user-id", " "}, "--balance-user-id requires a non-empty value"},
		{[]string{"edit", "team", "--balance-token-stdin", "--balance-user-id", "7", "--no-balance-token"}, "--no-balance-token removes the balance credentials"},
		{[]string{"edit", "team", "--endpoint", "responses="}, "--endpoint responses: URL must use http or https"},
		{[]string{"edit"}, "aigw provider edit needs a <provider> argument"},
	} {
		if err := f.run("", test.args...); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("%q = %v, want %q", test.args, err, test.want)
		}
	}
	p := problem(f.run("system\n", "edit", "mixed", "--balance-token-stdin", "--balance-user-id", "7"))
	if p.Title != "provider mixed has no balance probe, so a balance token has no use" || p.Fix != "aigw config export" {
		t.Fatalf("a provider without a balance probe = %+v", p)
	}
	if p := problem(f.run("", "edit", "tem", "--label", "x")); p.Title != `unknown provider "tem"; did you mean team?` {
		t.Fatalf("unknown provider = %+v", p)
	}
	f.withEnvironment(map[string]string{})
	p = problem(f.run("system\n", "edit", "team", "--balance-token-stdin", "--balance-user-id", "7"))
	if p.Title != "the environment backend reads the team balance credentials from AIGW_DIAGNOSTIC_SYSTEM_TOKEN_TEAM and AIGW_DIAGNOSTIC_USER_ID_TEAM and cannot store them" ||
		p.Fix != "Set AIGW_DIAGNOSTIC_SYSTEM_TOKEN_TEAM and AIGW_DIAGNOSTIC_USER_ID_TEAM in aigw's environment." {
		t.Fatalf("environment balance credentials = %+v", p)
	}
	if p := problem(f.run("", "edit", "team", "--no-balance-token")); p.Fix != "Unset AIGW_DIAGNOSTIC_SYSTEM_TOKEN_TEAM and AIGW_DIAGNOSTIC_USER_ID_TEAM in aigw's environment." {
		t.Fatalf("environment removal = %+v", p)
	}
}

// The balance credentials change before the configuration is saved, so a
// balance write that fails changes nothing.
func TestEditChangesNothingWhenTheBalanceWriteFails(t *testing.T) {
	f := newFixture(t)
	f.runtime.Providers = failingDiagnostics{}
	p := problem(f.run("system\n", "edit", "team", "--label", "Renamed", "--balance-token-stdin", "--balance-user-id", "7"))
	if p.Title != "the balance credentials of provider team could not be stored" || p.Impact != "Nothing changed." || p.Fix != "aigw provider show team" {
		t.Fatalf("a failed balance write = %+v", p)
	}
	if f.load(t).Providers["team"].Label != "Team" {
		t.Fatal("the label was saved")
	}
	if err := f.run("", "edit", "team", "--no-balance-token"); err == nil {
		t.Fatal("a failed balance removal succeeded")
	}
	// A balance token that is not one visible value stores nothing.
	if err := f.run("two words\n", "edit", "team", "--label", "Again", "--balance-token-stdin", "--balance-user-id", "7"); err == nil || f.load(t).Providers["team"].Label != "Team" {
		t.Fatalf("malformed balance input = %v", err)
	}
}

func TestEditStoresNoBalanceCredentialsWhenTheSaveFails(t *testing.T) {
	f := newFixture(t)
	f.runtime.Interactive = true
	// Another command changes the configuration while the balance token is
	// typed.
	f.runtime.Prompt = &prompter{values: []string{"new-system"}, before: func() {
		cfg := f.load(t)
		provider := cfg.Providers["mixed"]
		provider.Label = "Changed"
		cfg.Providers["mixed"] = provider
		if err := f.runtime.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
	}}
	err := f.run("", "edit", "team", "--label", "Renamed", "--balance-token-stdin", "--balance-user-id", "7")
	if err == nil || !strings.Contains(err.Error(), "configuration preimage changed") {
		t.Fatalf("a changed configuration = %v", err)
	}
	if credential, _ := f.runtime.Providers.Get("team"); credential.SystemToken != "system-token" || f.load(t).Providers["team"].Label != "Team" {
		t.Fatal("a failed save stored the balance credentials or the label")
	}
}

// failingDiagnostics is a balance credential store that fails every write.
type failingDiagnostics struct{}

func (failingDiagnostics) Get(string) (secret.DiagnosticCredential, error) {
	return secret.DiagnosticCredential{}, errors.New("backend failed")
}
func (failingDiagnostics) Set(string, secret.DiagnosticCredential) error {
	return errors.New("backend failed")
}
func (failingDiagnostics) Delete(string) error         { return errors.New("backend failed") }
func (failingDiagnostics) Exists(string) (bool, error) { return false, errors.New("backend failed") }

// An endpoint change keeps the retry budgets of the clients that reach it,
// because they belong to the provider, and says so.
func TestEditReportsTheRetryBudgetsThatAnEndpointChangeKeeps(t *testing.T) {
	f := newFixture(t)
	if err := f.run("", "edit", "team", "--endpoint", "responses=https://new.test/v1", "--json"); err != nil {
		t.Fatal(err)
	}
	var document changeDocument
	f.decode(t, &document)
	if strings.Join(document.KeptRetryBudgets, ",") != "codex" || strings.Join(document.ClearedForwarding, ",") != "codex" {
		t.Fatalf("edit = %+v", document)
	}
	if err := f.run("", "edit", "team", "--endpoint", "responses=https://newer.test/v1"); err != nil || !strings.Contains(f.text(), "· Retry budgets kept Codex") {
		t.Fatalf("human edit = %v %s", err, f.text())
	}
	// Another protocol's endpoint keeps no client's budgets.
	if err := f.run("", "edit", "team", "--endpoint", "anthropic=https://new.test", "--json"); err != nil {
		t.Fatal(err)
	}
	document = changeDocument{}
	f.decode(t, &document)
	if len(document.KeptRetryBudgets) != 0 {
		t.Fatalf("an Anthropic edit kept budgets for %q", document.KeptRetryBudgets)
	}
}

// At a terminal, --balance-token-stdin prompts, which --json never does, so
// the pair is refused, with a fix that pipes the balance token.
func TestEditRefusesABalancePromptWithJSON(t *testing.T) {
	f := newFixture(t)
	f.runtime.Interactive = true
	p := problem(f.run("", "edit", "team", "--balance-token-stdin", "--balance-user-id", "7", "--json"))
	if p.Title != "--balance-token-stdin prompts for the balance token at a terminal, and --json never prompts" ||
		p.Fix != "Pipe the balance token to `aigw provider edit team --balance-token-stdin --balance-user-id 7 --json`." {
		t.Fatalf("a balance prompt with JSON = %+v", p)
	}
	if p := problem(f.run("", "edit", "team", "--balance-token-stdin", "--balance-user-id", "two words", "--json")); p.Fix != "Pipe the balance token to the same command." {
		t.Fatalf("a fix that would need quoting = %+v", p)
	}
	// Piped in, with or without JSON, the balance token is read from standard
	// input.
	f.runtime.Interactive = false
	if err := f.run("piped-system\n", "edit", "team", "--balance-token-stdin", "--balance-user-id", "7", "--json"); err != nil {
		t.Fatal(err)
	}
	if credential, _ := f.runtime.Providers.Get("team"); credential.SystemToken != "piped-system" {
		t.Fatalf("piped balance token = %+v", credential)
	}
}

func TestProviderFlagsThatTakeOneValueRefuseASecond(t *testing.T) {
	f := newFixture(t)
	for _, args := range [][]string{
		{"add", "fresh", "--label", "One", "--label", "Two", "--endpoint", "chat=https://x.test", "--dry-run"},
		{"edit", "team", "--label", "One", "--label", "Two"},
		{"edit", "team", "--balance-token-stdin", "--balance-user-id", "7", "--balance-user-id", "8"},
	} {
		if err := f.run("system\n", args...); err == nil || !strings.Contains(err.Error(), "this flag takes one value") {
			t.Fatalf("%q = %v", args, err)
		}
	}
	if f.load(t).Providers["team"].Label != "Team" {
		t.Fatal("a refused flag changed the provider")
	}
}
