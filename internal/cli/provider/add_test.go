package provider

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestAddValidatesTheTokenWithOneRequestThenStoresAndSaves(t *testing.T) {
	f := newFixture(t)
	err := f.run("sk-fresh-0001\n", "add", "fresh", "--label", "Fresh", "--endpoint", "responses=https://fresh.test/v1/",
		"--endpoint", "chat=https://fresh.test/v1", "--token-stdin", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var document changeDocument
	f.decode(t, &document)
	if document.ID != "fresh" || document.Label != "Fresh" || len(document.Endpoints) != 2 || document.Endpoints[0].URL != "https://fresh.test/v1" ||
		len(document.Models) != 0 || !*document.TokenValidated || !*document.TokenStored || document.NextDetail != "aigw model add fresh/<model>" {
		t.Fatalf("added provider = %+v", document)
	}
	if requests := f.http.sent(); len(requests) != 1 || requests[0] != "GET https://fresh.test/v1/models" {
		t.Fatalf("validation requests = %q", requests)
	}
	if f.token("fresh") != "sk-fresh-0001" || strings.Contains(f.out.String(), "sk-fresh-0001") {
		t.Fatalf("stored Token %q, in output %t", f.token("fresh"), strings.Contains(f.out.String(), "sk-fresh-0001"))
	}
	if provider := f.load(t).Providers["fresh"]; provider.Label != "Fresh" || provider.Endpoints.OpenAIChatCompletions != "https://fresh.test/v1" {
		t.Fatalf("saved provider = %+v", provider)
	}
	// A prompt at a terminal reads the Token without echoing it, and the
	// label defaults to the ID.
	f.runtime.Interactive = true
	ask := &prompter{values: []string{"sk-other-0001"}}
	f.runtime.Prompt = ask
	if err := f.run("", "add", "other", "--endpoint", "anthropic=https://other.test"); err != nil {
		t.Fatal(err)
	}
	if text := f.text(); len(ask.asked) != 1 || ask.asked[0] != "Paste the other Token: " || !strings.Contains(text, "AIGW Provider added") ||
		!strings.Contains(text, "✓ Token Accepted by the provider and stored") || !strings.Contains(text, "Next aigw model add other/<model>") {
		t.Fatalf("prompted %q, output %s", ask.asked, text)
	}
}

func TestAddPreviewReadsNoTokenAndSendsNothing(t *testing.T) {
	f := newFixture(t)
	before := f.configuration(t)
	if err := f.run("", "add", "fresh", "--endpoint", "anthropic=https://fresh.test", "--dry-run", "--json"); err != nil {
		t.Fatal(err)
	}
	var document changeDocument
	f.decode(t, &document)
	// Applying it prompts for the Token, so the step is for a person.
	if !document.DryRun || *document.TokenValidated || *document.TokenStored || document.NextDetail != "aigw provider add fresh --endpoint anthropic=https://fresh.test" {
		t.Fatalf("preview = %+v", document)
	}
	if err := f.run("", "add", "fresh", "--endpoint", "anthropic=https://fresh.test", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if text := f.text(); !strings.Contains(text, "Provider added (preview)") || !strings.Contains(text, "· Token Not read in a preview, and no request was sent") ||
		!strings.Contains(text, "Nothing was written") {
		t.Fatalf("human preview = %s", text)
	}
	if len(f.http.sent()) != 0 || f.configuration(t) != before || f.token("fresh") != "" {
		t.Fatal("a preview sent a request or wrote something")
	}
}

func TestAddStoresNothingThatTheProviderDoesNotAccept(t *testing.T) {
	retry := "aigw provider add fresh --endpoint anthropic=https://fresh.test --token-stdin"
	for _, test := range []struct {
		name, title, fix string
		status           int
		failure          error
	}{
		{name: "rejected", status: http.StatusUnauthorized, title: "Token validation failed: Claude authentication was rejected (HTTP 401)", fix: retry},
		{name: "server error", status: http.StatusServiceUnavailable, title: "Token validation failed: Claude endpoint returned HTTP 503", fix: retry},
		{name: "unreachable", failure: errors.New("offline"), title: "Token validation failed: Claude endpoint is unreachable: offline", fix: retry},
		{name: "wrong endpoint", status: http.StatusNotFound, title: "Token validation failed: Claude endpoint returned HTTP 404",
			fix: "aigw provider add fresh --token-stdin --endpoint anthropic=<url>"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			if test.failure != nil {
				f.http.failures["fresh.test"] = test.failure
			} else {
				f.http.status["fresh.test"] = test.status
			}
			p := problem(f.run("sk-fresh-0001\n", "add", "fresh", "--endpoint", "anthropic=https://fresh.test", "--token-stdin"))
			if p.Title != test.title || p.Fix != test.fix || p.Impact != "No Token was stored." {
				t.Fatalf("refusal = %+v", p)
			}
			if _, added := f.load(t).Providers["fresh"]; added || f.token("fresh") != "" {
				t.Fatal("a refused Token added the provider or stored the Token")
			}
		})
	}
}

func TestAddFixesAWrongEndpointInProseWhenAWordNeedsQuoting(t *testing.T) {
	f := newFixture(t)
	f.http.status["fresh.test"] = http.StatusNotFound
	prose := "Run the same command with a corrected --endpoint anthropic=<url>."
	for _, args := range [][]string{
		{"add", "fresh", "--endpoint", "anthropic=https://fresh.test", "--endpoint", "chat=https://chat.test/v1?model=a&b=c", "--token-stdin"},
		{"add", "fresh", "--label", "Two words", "--endpoint", "anthropic=https://fresh.test", "--token-stdin"},
	} {
		if p := problem(f.run("sk-fresh-0001\n", args...)); p.Fix != prose {
			t.Fatalf("%q fix = %q", args, p.Fix)
		}
	}
}

func TestAddRefusesBeforeReadingConfigurationOrInput(t *testing.T) {
	f := newFixture(t)
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"add"}, "aigw provider add needs a <provider> argument"},
		{[]string{"add", "Bad", "--endpoint", "anthropic=https://x.test"}, `"Bad" is not a provider ID`},
		{[]string{"add", "fresh"}, "aigw provider add needs an --endpoint <protocol>=<url>"},
		{[]string{"add", "fresh", "--endpoint", "anthropic"}, "--endpoint takes <protocol>=<url>"},
		{[]string{"add", "fresh", "--endpoint", "openai_responses=https://x.test"}, "--endpoint needs a protocol before =: anthropic, responses, or chat"},
		{[]string{"add", "fresh", "--endpoint", "chat=https://x.test", "--endpoint", "chat=https://y.test"}, "--endpoint names chat twice"},
		{[]string{"add", "fresh", "--endpoint", "chat=http://user:secret@x.test"}, "--endpoint chat: URL userinfo is forbidden"},
		{[]string{"add", "fresh", "--label", " ", "--endpoint", "chat=https://x.test"}, "--label requires a non-empty value"},
		{[]string{"add", "fresh", "--endpoint", "chat=https://x.test"},
			"a Token is read from --token-stdin without a terminal or with --json; pipe the Token to `aigw provider add fresh --endpoint chat=https://x.test --token-stdin`"},
		{[]string{"add", "fresh", "--label", "Two words", "--endpoint", "chat=https://x.test"},
			"a Token is read from --token-stdin without a terminal or with --json; run `aigw provider add --help`"},
	} {
		err := f.run("", test.args...)
		if err == nil || !strings.Contains(err.Error(), test.want) || strings.Contains(err.Error(), "secret") {
			t.Fatalf("%q = %v, want %q", test.args, err, test.want)
		}
	}
	// JSON never prompts, even at a terminal.
	f.runtime.Interactive = true
	if err := f.run("", "add", "fresh", "--endpoint", "chat=https://x.test", "--json"); err == nil || !strings.Contains(err.Error(), "--token-stdin") {
		t.Fatalf("JSON at a terminal = %v", err)
	}
	if len(f.http.sent()) != 0 {
		t.Fatal("a refusal sent a request")
	}
}

func TestAddRefusesAnExistingProviderAndAnEmptyConfiguration(t *testing.T) {
	f := newFixture(t)
	p := problem(f.run("sk-x-0001\n", "add", "team", "--endpoint", "anthropic=https://x.test", "--token-stdin"))
	if p.Title != "provider team already exists; `aigw provider edit` changes it and `aigw provider rotate` replaces its Token" || p.Fix != "aigw provider edit team --help" {
		t.Fatalf("an existing provider = %+v", p)
	}
	f.runtime.Config = emptyStore(t)
	if p := problem(f.run("sk-x-0001\n", "add", "fresh", "--endpoint", "anthropic=https://x.test", "--token-stdin")); p.Fix != "aigw setup" {
		t.Fatalf("an empty configuration = %+v", p)
	}
	if len(f.http.sent()) != 0 {
		t.Fatal("a refusal sent a request")
	}
}

func TestAddValidatesTheEnvironmentsTokenAndStoresNothing(t *testing.T) {
	f := newFixture(t)
	f.withEnvironment(map[string]string{"AIGW_TOKEN_FRESH": "sk-fresh-0001"})
	if err := f.run("", "add", "fresh", "--endpoint", "anthropic=https://fresh.test", "--json"); err != nil {
		t.Fatal(err)
	}
	var document changeDocument
	f.decode(t, &document)
	if !*document.TokenValidated || *document.TokenStored || len(f.http.sent()) != 1 {
		t.Fatalf("environment addition = %+v after %q", document, f.http.sent())
	}
	if err := f.run("", "add", "other", "--endpoint", "anthropic=https://fresh.test"); err == nil {
		t.Fatal("an environment without the Token added a provider")
	} else if p := problem(err); p.Title != "the environment backend has no other Token: AIGW_TOKEN_OTHER is not set" ||
		p.Fix != "Set AIGW_TOKEN_OTHER in aigw's environment, then run `aigw provider add other --endpoint anthropic=https://fresh.test`." {
		t.Fatalf("missing variable = %+v", p)
	}
	if p := problem(f.run("", "add", "other", "--label", "Two words", "--endpoint", "anthropic=https://fresh.test")); p.Fix != "Set AIGW_TOKEN_OTHER in aigw's environment, then run the same command again." {
		t.Fatalf("missing variable in prose = %+v", p)
	}
	p := problem(f.run("sk-other-0001\n", "add", "other", "--endpoint", "anthropic=https://fresh.test", "--token-stdin"))
	if p.Title != "the environment backend reads the other Token from AIGW_TOKEN_OTHER and cannot store it" ||
		p.Fix != "Set AIGW_TOKEN_OTHER in aigw's environment, then run `aigw provider add other --endpoint anthropic=https://fresh.test`." {
		t.Fatalf("--token-stdin with the environment = %+v", p)
	}
	if err := f.run("", "add", "other", "--endpoint", "anthropic=https://fresh.test", "--dry-run"); err != nil || !strings.Contains(f.text(), "Not read in a preview") {
		t.Fatalf("an environment preview = %v %s", err, f.text())
	}
	f.withEnvironment(map[string]string{"AIGW_TOKEN_THIRD": "sk-third-0001"})
	if err := f.run("", "add", "third", "--endpoint", "anthropic=https://fresh.test"); err != nil || !strings.Contains(f.text(), "Accepted by the provider; AIGW_TOKEN_THIRD supplies it") {
		t.Fatalf("an environment Token = %v %s", err, f.text())
	}
}

func TestAddRestoresTheTokenWhenTheSaveFails(t *testing.T) {
	f := newFixture(t)
	f.runtime.Interactive = true
	// Another command changes the configuration while the Token is typed.
	f.runtime.Prompt = &prompter{values: []string{"sk-fresh-0001"}, before: func() {
		cfg := f.load(t)
		provider := cfg.Providers["mixed"]
		provider.Label = "Changed"
		cfg.Providers["mixed"] = provider
		if err := f.runtime.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
	}}
	if err := f.run("", "add", "fresh", "--endpoint", "anthropic=https://fresh.test"); err == nil || !strings.Contains(err.Error(), "configuration preimage changed") {
		t.Fatalf("a changed configuration = %v", err)
	}
	if f.token("fresh") != "" {
		t.Fatal("a failed save kept the Token")
	}
	if _, added := f.load(t).Providers["fresh"]; added {
		t.Fatal("a failed save added the provider")
	}
}
