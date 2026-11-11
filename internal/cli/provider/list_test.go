package provider

import (
	"errors"
	"os"
	"strings"
	"testing"

	"aigw-cli/internal/terminal"
)

func TestListShowsEveryProviderInConfigurationOrderWithoutReadingATokenOrSending(t *testing.T) {
	f := newFixture(t)
	if err := f.run("", "list", "--json"); err != nil {
		t.Fatal(err)
	}
	var document listDocument
	f.decode(t, &document)
	if len(document.Providers) != 2 || document.Providers[0].ID != "mixed" || document.Providers[1].ID != "team" {
		t.Fatalf("providers = %+v", document.Providers)
	}
	team := document.Providers[1]
	if team.BalanceProbe != "dmxapi" || len(team.Endpoints) != 2 || team.Endpoints[0].Protocol != "anthropic" || team.Endpoints[1].Protocol != "responses" ||
		strings.Join(team.Models, ",") != "team/codex-model,team/team-model" {
		t.Fatalf("team = %+v", team)
	}
	if err := f.run("", "list"); err != nil {
		t.Fatal(err)
	}
	if text := f.text(); !strings.Contains(text, "team Team · Anthropic Messages, OpenAI Responses · 2 models") ||
		!strings.Contains(text, "mixed Mixed · Anthropic Messages, OpenAI Chat Completions · 1 model") {
		t.Fatalf("list = %s", text)
	}
	if requests := f.http.sent(); len(requests) != 0 {
		t.Fatalf("list sent %q", requests)
	}
	if err := f.run("", "list", "stray"); err == nil {
		t.Fatal("list accepted an argument")
	}
}

func TestListWithoutProvidersNamesSetup(t *testing.T) {
	f := newFixture(t)
	f.runtime.Config = emptyStore(t)
	if err := f.run("", "list"); err != nil {
		t.Fatal(err)
	}
	if text := f.text(); !strings.Contains(text, "No provider is configured.") || !strings.Contains(text, "Next aigw setup") {
		t.Fatalf("empty list = %s", text)
	}
}

func TestShowReportsCredentialsWithoutReadingThem(t *testing.T) {
	f := newFixture(t)
	if err := f.run("", "show", "team", "--json"); err != nil {
		t.Fatal(err)
	}
	var document showDocument
	f.decode(t, &document)
	if document.Token != credentialPresent || document.BalanceCredential != credentialPresent ||
		strings.Join(document.SelectedBy, ",") != "claude,codex" || document.BalanceProbe != "dmxapi" {
		t.Fatalf("team = %+v", document)
	}
	if err := f.run("", "show", "mixed"); err != nil {
		t.Fatal(err)
	}
	text := f.text()
	for _, want := range []string{"Provider mixed", "Label Mixed", "Anthropic Messages https://mixed.test", "OpenAI Chat Completions https://mixed.test/v1",
		"Models mixed/chat-only", "! Token Missing", "Next aigw provider rotate mixed"} {
		if !strings.Contains(text, want) {
			t.Fatalf("show lacks %q: %s", want, text)
		}
	}
	if strings.Contains(text, "Balance") {
		t.Fatalf("a provider without a balance probe shows balance credentials: %s", text)
	}
	if err := f.run("", "show", "team"); err != nil {
		t.Fatal(err)
	}
	if text := f.text(); !strings.Contains(text, "✓ Token Stored") || !strings.Contains(text, "Selected by Claude, Codex") ||
		!strings.Contains(text, "Balance probe dmxapi") || !strings.Contains(text, "✓ Balance token Stored") || !strings.Contains(text, "Next aigw model list --provider team") {
		t.Fatalf("show team = %s", text)
	}
	if strings.Contains(f.out.String(), teamToken) {
		t.Fatal("show wrote the Token")
	}
}

func TestShowNamesTheEnvironmentAndAFailedBackend(t *testing.T) {
	f := newFixture(t)
	f.withEnvironment(map[string]string{"AIGW_TOKEN_TEAM": teamToken})
	if err := f.run("", "show", "team"); err != nil {
		t.Fatal(err)
	}
	if text := f.text(); !strings.Contains(text, "✓ Token Set in AIGW_TOKEN_TEAM") ||
		!strings.Contains(text, "! Balance token Not set: AIGW_BALANCE_TOKEN_TEAM and AIGW_BALANCE_USER_ID_TEAM") {
		t.Fatalf("environment show = %s", text)
	}
	if err := f.run("", "show", "mixed"); err != nil || !strings.Contains(f.text(), "Next aigw model list --provider mixed") {
		t.Fatalf("an environment without the Token names rotate: %v %s", err, f.text())
	}
	f.runtime.Secrets = failingStore{}
	if err := f.run("", "show", "mixed", "--json"); err != nil {
		t.Fatal(err)
	}
	var document showDocument
	f.decode(t, &document)
	// A backend that cannot tell whether it holds the Token is not ticked as
	// available beside a Token that reads unknown.
	if document.Token != credentialUnobserved || document.CredentialBackend.Availability != "unavailable" {
		t.Fatalf("a failed backend = %q, %+v", document.Token, document.CredentialBackend)
	}
	if err := f.run("", "show", "mixed"); err != nil || !strings.Contains(f.text(), "! Token Unknown: the credential backend could not tell") ||
		strings.Contains(f.text(), "✓ Credential backend") || !strings.Contains(f.text(), "! Credential backend") {
		t.Fatalf("a failed backend = %v %s", err, f.text())
	}
}

func TestShowRefusesAnUnknownProviderWithTheClosestMatch(t *testing.T) {
	f := newFixture(t)
	p := problem(f.run("", "show", "tem"))
	if p.Title != `unknown provider "tem"; did you mean team?` || p.Fix != "aigw provider list" {
		t.Fatalf("unknown provider = %+v", p)
	}
	if p := problem(f.run("", "show")); p.Title != "aigw provider show needs a <provider> argument; run `aigw provider show --help`" || p.Fix != "aigw provider show --help" {
		t.Fatalf("missing argument = %+v", p)
	}
	if p := problem(f.run("", "show", "team", "mixed")); !strings.Contains(p.Title, "takes one <provider> argument, not 2") {
		t.Fatalf("two arguments = %+v", p)
	}
}

func TestCompletion(t *testing.T) {
	f := newFixture(t)
	command := NewCommand(f.runtime)
	show, _, _ := command.Find([]string{"show"})
	if completions, _ := show.ValidArgsFunction(show, nil, ""); len(completions) != 2 {
		t.Fatalf("show completes %q", completions)
	}
	if completions, _ := show.ValidArgsFunction(show, []string{"team"}, ""); len(completions) != 0 {
		t.Fatalf("show completes a second argument: %q", completions)
	}
	if completions, _ := completeEndpoint(nil, nil, ""); strings.Join(completions, ",") != "anthropic=,responses=,chat=" {
		t.Fatalf("--endpoint completes %q", completions)
	}
}

// failingStore is a credential backend that fails every operation.
type failingStore struct{}

func (failingStore) Get(string) (string, error)  { return "", errors.New("backend failed") }
func (failingStore) Set(string, string) error    { return errors.New("backend failed") }
func (failingStore) Delete(string) error         { return errors.New("backend failed") }
func (failingStore) Exists(string) (bool, error) { return false, errors.New("backend failed") }

// show names the credential backend, which tells a backend that failed from
// a credential that is missing.
func TestShowNamesTheCredentialBackend(t *testing.T) {
	f := newFixture(t)
	if err := f.run("", "show", "team", "--json"); err != nil {
		t.Fatal(err)
	}
	var document showDocument
	f.decode(t, &document)
	if document.CredentialBackend != (credentialBackend{Kind: "memory", Availability: "available", Mutability: "read_write"}) {
		t.Fatalf("backend = %+v", document.CredentialBackend)
	}
	if err := f.run("", "show", "team"); err != nil || !strings.Contains(f.text(), "✓ Credential backend memory backend, available") {
		t.Fatalf("show = %v %s", err, f.text())
	}
	f.withEnvironment(map[string]string{})
	if err := f.run("", "show", "team"); err != nil || !strings.Contains(f.text(), "✓ Credential backend env backend, read-only, available") {
		t.Fatalf("environment show = %v %s", err, f.text())
	}
	if field := backendField(credentialBackend{Kind: "keyring", Availability: "unavailable", Mutability: "read_write"}); field.State != terminal.Warn || field.Value != "keyring backend, unavailable" {
		t.Fatalf("an unavailable backend = %+v", field)
	}
}

// A configuration that a newer AIGW wrote is refused, and the fix names the
// installed AIGW and the configuration's location.
func TestProviderCommandsRefuseAConfigurationFromANewerSchema(t *testing.T) {
	f := newFixture(t)
	data := strings.Replace(f.configuration(t), "version = 8", "version = 99", 1)
	if err := os.WriteFile(f.path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"list"}, {"show", "team"}, {"balance"}} {
		p := problem(f.run("", args...))
		if !strings.Contains(p.Title, "unsupported configuration version: found 99") || !strings.Contains(p.Fix, "`aigw config path`") {
			t.Fatalf("%q = %+v", args, p)
		}
	}
}
