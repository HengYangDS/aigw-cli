package provider

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRotateReplacesTheTokenOnlyAfterTheProviderAcceptsIt(t *testing.T) {
	f := newFixture(t)
	before := f.configuration(t)
	if err := f.run("sk-team-token-0002\n", "rotate", "team", "--token-stdin", "--json"); err != nil {
		t.Fatal(err)
	}
	var document rotateDocument
	f.decode(t, &document)
	if document.ID != "team" || !document.TokenValidated || !document.TokenStored || document.NextAction != "aigw check" {
		t.Fatalf("rotation = %+v", document)
	}
	// The Responses endpoint, which the provider has, validates the Token.
	if requests := f.http.sent(); len(requests) != 1 || requests[0] != "GET https://team.test/v1/models" {
		t.Fatalf("validation requests = %q", requests)
	}
	if f.token("team") != "sk-team-token-0002" || f.configuration(t) != before || strings.Contains(f.out.String(), "sk-team-token-0002") {
		t.Fatal("rotation did not store only the Token, or wrote it")
	}
	f.runtime.Interactive = true
	ask := &prompter{values: []string{"sk-team-token-0003"}}
	f.runtime.Prompt = ask
	if err := f.run("", "rotate", "team"); err != nil {
		t.Fatal(err)
	}
	if text := f.text(); f.token("team") != "sk-team-token-0003" || ask.asked[0] != "Paste the Team Token: " ||
		!strings.Contains(text, "AIGW Token replaced") || !strings.Contains(text, "✓ Token Accepted by the provider and stored") || !strings.Contains(text, "Next aigw check") {
		t.Fatalf("prompted rotation = %s", text)
	}
	ask.values = []string{"two words"}
	if err := f.run("", "rotate", "team"); err == nil || f.token("team") != "sk-team-token-0003" {
		t.Fatalf("a malformed prompted Token = %v", err)
	}
}

func TestRotateKeepsThePreviousTokenWhenTheProviderDoesNotAcceptTheNewOne(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		fix    string
	}{
		{"rejected", http.StatusForbidden, "Pipe a Token that provider team accepts into `aigw provider rotate team --token-stdin`."},
		{"rate limit", http.StatusTooManyRequests, "aigw provider rotate team --token-stdin"},
		{"wrong endpoint", http.StatusNotFound, "aigw provider edit team --endpoint responses=<url>"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			f.http.status["team.test"] = test.status
			p := problem(f.run("sk-team-token-0002\n", "rotate", "team", "--token-stdin"))
			if p.Fix != test.fix || p.Impact != "No Token was stored; the previous Token is unchanged." || f.token("team") != teamToken {
				t.Fatalf("refusal = %+v, Token %q", p, f.token("team"))
			}
		})
	}
	// A provider without a Token has none to keep.
	f := newFixture(t)
	f.http.status["mixed.test"] = http.StatusUnauthorized
	if p := problem(f.run("sk-mixed-0001\n", "rotate", "mixed", "--token-stdin")); p.Impact != "No Token was stored." || f.token("mixed") != "" {
		t.Fatalf("a provider without a Token = %+v", p)
	}
}

func TestRotateRefusesWhatItCannotStore(t *testing.T) {
	f := newFixture(t)
	f.withEnvironment(map[string]string{"AIGW_TOKEN_TEAM": teamToken})
	p := problem(f.run("sk-team-token-0002\n", "rotate", "team", "--token-stdin"))
	if p.Title != "the environment backend reads the team Token from AIGW_TOKEN_TEAM and cannot replace it" || p.Fix != "Set AIGW_TOKEN_TEAM in aigw's environment." {
		t.Fatalf("environment rotation = %+v", p)
	}
	f = newFixture(t)
	for _, interactive := range []bool{false, true} {
		f.runtime.Interactive = interactive
		err := f.run("", "rotate", "team", "--json")
		if p := problem(err); err == nil || p.Title != "a Token is read from --token-stdin without a terminal or with --json; pipe the Token to `aigw provider rotate team --token-stdin`" {
			t.Fatalf("interactive %t: %+v", interactive, p)
		}
	}
	if p := problem(f.run("", "rotate", "tem", "--token-stdin")); p.Title != `unknown provider "tem"; did you mean team?` {
		t.Fatalf("unknown provider = %+v", p)
	}
	if len(f.http.sent()) != 0 {
		t.Fatal("a refusal sent a request")
	}
}

func TestRotateStoresNothingOnceCancelled(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f.runtime.In = strings.NewReader("sk-team-token-0002\n")
	root := &cobra.Command{Use: "aigw", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(NewCommand(f.runtime))
	root.SetOut(f.out)
	root.SetArgs([]string{"provider", "rotate", "team", "--token-stdin"})
	if err := root.ExecuteContext(ctx); err == nil {
		t.Fatal("a cancelled rotation succeeded")
	}
	if f.token("team") != teamToken {
		t.Fatal("a cancelled rotation stored the Token")
	}
}
