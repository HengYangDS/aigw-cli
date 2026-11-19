package setup

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"
)

// acceptOnly makes the home's provider accept the one Token, on either
// protocol's header, and refuse every other with 401.
func acceptOnly(home *guidedHome, accepted string) {
	home.runtime.HTTP = setupHTTPClient(func(request *http.Request) (*http.Response, error) {
		status := http.StatusUnauthorized
		if request.Header.Get("Authorization") == "Bearer "+accepted || request.Header.Get("X-Api-Key") == accepted {
			status = http.StatusOK
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(home.models)), Request: request}, nil
	})
}

// teamDocumentOf parses the JSON document that a run wrote.
func teamDocumentOf(t *testing.T, home *guidedHome) (teamDocument, bool) {
	t.Helper()
	var document struct {
		teamDocument
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(home.out.Bytes(), &document); err != nil {
		t.Fatalf("%v\n%s", err, home.out)
	}
	return document.teamDocument, document.OK
}

// tokenOf is the report's row of one provider's Token.
func tokenOf(document teamDocument, provider string) tokenItem {
	for _, token := range document.Tokens {
		if token.Provider == provider {
			return token
		}
	}
	return tokenItem{}
}

// Choosing no provider at the question applies the team file without asking
// for a Token.
func TestTeamSetupAppliesTheFileWhenThePersonHoldsNoToken(t *testing.T) {
	home := newGuidedHome(t, claude.ID)
	prompt := &guidedPrompt{multi: [][]string{nil}}
	if err := home.run(t, prompt, "--from", writeTeamFile(t, teamFileText)); err != nil {
		t.Fatalf("setup: %v\n%s", err, home.out)
	}
	if !slices.Equal(prompt.asked, []string{providerQuestion}) {
		t.Fatalf("asked %q", prompt.asked)
	}
	if cfg := home.config(t); len(cfg.ProviderModelRefs("team")) == 0 || cfg.Recommendations[claude.ID].Primary.Model.IsZero() {
		t.Fatalf("the team file was not applied: %+v", cfg)
	}
	if !strings.Contains(home.out.String(), "Team file applied") || !strings.Contains(home.out.String(), "None; aigw provider rotate team") {
		t.Fatalf("report:\n%s", home.out)
	}
}

// A Token that the provider does not accept is entered again, or its provider
// skipped, while the team file is applied either way.
func TestTeamSetupOffersAnotherTokenOrASkip(t *testing.T) {
	path := writeTeamFile(t, teamFileText)
	again := "Enter the Team Gateway Again Token again"
	for _, test := range []struct {
		name, choice, state string
		secrets             []string
	}{
		{"entered again", "again", tokenStored, []string{"sk-wrong", "sk-team"}},
		{"skipped", "skip", tokenRejected, []string{"sk-wrong"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := newGuidedHome(t, claude.ID)
			acceptOnly(home, "sk-team")
			// --json asks nothing, even in a terminal.
			prompt := &guidedPrompt{}
			if err := home.run(t, prompt, "--from", path, "--json"); err != nil || len(prompt.asked) != 0 {
				t.Fatalf("--json: %v, asked %q", err, prompt.asked)
			}
			home.out.Reset()
			prompt = &guidedPrompt{multi: [][]string{{"team"}}, secrets: test.secrets, selections: []string{test.choice}}
			if err := home.run(t, prompt, "--from", path); err != nil {
				t.Fatalf("setup: %v\n%s", err, home.out)
			}
			question := "Team Gateway Again did not accept the Token (HTTP 401). What now?"
			if !slices.Contains(prompt.asked, question) || prompt.choices[len(prompt.choices)-1][0].Label != again {
				t.Fatalf("asked %q, offered %+v", prompt.asked, prompt.choices)
			}
			stored, err := home.secrets.Get("team")
			if (test.state == tokenStored) != (err == nil && stored == "sk-team") {
				t.Fatalf("the stored Token = %q, %v", stored, err)
			}
			if cfg := home.config(t); len(cfg.ProviderModelRefs("team")) == 0 {
				t.Fatal("the team file was not applied")
			}
			report := home.out.String()
			if test.state == tokenRejected && !strings.Contains(report, "Not accepted: Team Gateway Again did not accept the Token (HTTP 401); aigw provider rotate team") {
				t.Fatalf("report:\n%s", report)
			}
		})
	}
}

// A piped Token that its provider does not accept still lets the team file
// apply, and fails the command so that the script sees it.
func TestTeamSetupAppliesTheFileWhenAPipedTokenIsNotAccepted(t *testing.T) {
	home := newGuidedHome(t)
	home.runtime.Interactive = false
	acceptOnly(home, "sk-team")
	home.runtime.In = strings.NewReader("sk-wrong\n")
	err := home.run(t, &guidedPrompt{}, "--from", writeTeamFile(t, teamFileText), "--provider", "team", "--token-stdin", "--json")
	if problem := terminal.ProblemFor(err); err == nil || !strings.Contains(problem.Title, "Team Gateway Again did not accept the Token (HTTP 401)") ||
		!strings.Contains(problem.Impact, "The team file and every other Token were applied.") {
		t.Fatalf("problem %#v", terminal.ProblemFor(err))
	}
	document, ok := teamDocumentOf(t, home)
	if token := tokenOf(document, "team"); ok || token.State != tokenRejected || token.Reason != "Team Gateway Again did not accept the Token (HTTP 401)" ||
		token.NextAction+token.NextDetail != "aigw provider rotate team" {
		t.Fatalf("ok %t, Token %+v", ok, tokenOf(document, "team"))
	}
	if _, err := home.secrets.Get("team"); err == nil {
		t.Fatal("a rejected Token was stored")
	}
	if cfg := home.config(t); len(cfg.ProviderModelRefs("team")) == 0 {
		t.Fatal("the team file was not applied")
	}
}

// tokenPrompt is the prompt for provider label's Token.
func tokenPrompt(label string) string {
	return "Paste the " + label + " Token, or press Enter to skip " + label + ": "
}

// The reported case: the shipped team file, applied in a terminal by someone
// who holds one recommended provider's Token, configures every client from
// that provider and asks for no other Token; Enter alone at the question
// applies the file with no Token at all. Every provider that the file
// recommends is tried, so the test follows the file.
func TestTheShippedTeamFileServesAPersonWithOneProvidersToken(t *testing.T) {
	shipped := filepath.Join("..", "..", "..", "manifests", "team.toml")
	data, err := os.ReadFile(shipped)
	if err != nil {
		t.Fatal(err)
	}
	file, err := store.ParseTeamFile(data, clients.Admission())
	if err != nil {
		t.Fatal(err)
	}
	recommended := map[string]bool{}
	for _, recommendation := range file.Recommendations {
		for _, choice := range recommendation.Selections() {
			recommended[choice.Model.Provider()] = true
		}
	}
	installed := []string{claude.ID, claudedesktop.ID, codex.ID, hermes.ID}
	for _, provider := range slices.Sorted(maps.Keys(recommended)) {
		t.Run(provider, func(t *testing.T) {
			home := newGuidedHome(t, installed...)
			acceptOnly(home, "sk-"+provider)
			label := file.Providers[provider].Label
			prompt := &guidedPrompt{multi: [][]string{{provider}}, secrets: []string{"sk-" + provider}}
			if err := home.run(t, prompt, "--from", shipped); err != nil {
				t.Fatalf("setup: %v\n%s", err, home.out)
			}
			if !slices.Equal(prompt.asked, []string{providerQuestion, tokenPrompt(label)}) {
				t.Fatalf("asked %q", prompt.asked)
			}
			cfg := home.config(t)
			for client := range file.Recommendations {
				if selected := cfg.SelectedModel(client); selected.Provider() != provider {
					t.Errorf("%s selects %q, want a model of %s", client, selected, provider)
				}
			}
		})
	}
	home := newGuidedHome(t, installed...)
	prompt := &guidedPrompt{multi: [][]string{nil}}
	if err := home.run(t, prompt, "--from", shipped); err != nil {
		t.Fatalf("setup without a Token: %v\n%s", err, home.out)
	}
	if cfg := home.config(t); len(cfg.Providers) == 0 || !cfg.SelectedModel(claude.ID).IsZero() || !slices.Equal(prompt.asked, []string{providerQuestion}) {
		t.Fatalf("asked %q; providers %d, Claude selects %q", prompt.asked, len(cfg.Providers), cfg.SelectedModel(claude.ID))
	}
}

// Enter alone at a chosen provider's prompt skips it, as does an accessible
// question whose answers name no choice, which setup says; text that is not a
// Token offers to enter it again or skip, and a skip after it, chosen or by
// answers that name no choice, leaves the provider without one.
func TestTeamSetupSkipsWithoutAWrongToken(t *testing.T) {
	path := writeTeamFile(t, teamFileText)
	for _, test := range []struct {
		name   string
		prompt func() *guidedPrompt
		said   string
	}{
		{"Enter alone", func() *guidedPrompt { return &guidedPrompt{multi: [][]string{{"team"}}, secrets: []string{""}} }, ""},
		{"no provider named", func() *guidedPrompt { return &guidedPrompt{multiErr: terminal.ErrNoChoice} },
			"No answer named a provider, so setup asks for no Token."},
		{"not a Token, then skipped", func() *guidedPrompt {
			return &guidedPrompt{multi: [][]string{{"team"}}, secrets: []string{"two words"}, selections: []string{"skip"}}
		}, ""},
		{"not a Token, then no choice named", func() *guidedPrompt {
			return &guidedPrompt{multi: [][]string{{"team"}}, secrets: []string{"two words"}, selectErr: terminal.ErrNoChoice}
		}, "No answer named a choice, so Team Gateway Again was skipped."},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := newGuidedHome(t, claude.ID)
			if err := home.run(t, test.prompt(), "--from", path); err != nil {
				t.Fatalf("setup: %v\n%s", err, home.out)
			}
			if !strings.Contains(home.out.String(), "Team file applied") || !strings.Contains(home.out.String(), "None; aigw provider rotate team") ||
				!strings.Contains(strings.Join(strings.Fields(home.out.String()), " "), test.said) {
				t.Fatalf("report:\n%s", home.out)
			}
			if _, err := home.secrets.Get("team"); err == nil {
				t.Fatal("a Token was stored")
			}
		})
	}
	prompt := &guidedPrompt{multi: [][]string{{"team"}}, secrets: []string{"two words"}, selections: []string{"skip"}}
	if err := newGuidedHome(t, claude.ID).run(t, prompt, "--from", path); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(prompt.asked, "That is not a Token: Token input must contain one visible ASCII value. What now?") {
		t.Fatalf("asked %q", prompt.asked)
	}
}

// A validation that got no verdict offers to try the same Token again, and a
// skip after it reports the provider as not verified.
func TestTeamSetupTriesATokenAgainAfterNoVerdict(t *testing.T) {
	path := writeTeamFile(t, teamFileText)
	for _, test := range []struct {
		name, choice, state string
	}{
		{"tried again", "retry", tokenStored},
		{"skipped", "skip", tokenUnverified},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := newGuidedHome(t, claude.ID)
			requests := 0
			home.runtime.HTTP = setupHTTPClient(func(request *http.Request) (*http.Response, error) {
				requests++
				status := http.StatusServiceUnavailable
				if requests > 1 {
					status = http.StatusOK
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(home.models)), Request: request}, nil
			})
			prompt := &guidedPrompt{multi: [][]string{{"team"}}, secrets: []string{"sk-team"}, selections: []string{test.choice}}
			if err := home.run(t, prompt, "--from", path); err != nil {
				t.Fatalf("setup: %v\n%s", err, home.out)
			}
			if offered := prompt.choices[len(prompt.choices)-1]; len(offered) != 3 || offered[0].Value != "retry" {
				t.Fatalf("offered %+v", offered)
			}
			_, err := home.secrets.Get("team")
			if (test.state == tokenStored) != (err == nil) {
				t.Fatalf("stored: %v", err)
			}
			if test.state == tokenUnverified && !strings.Contains(home.out.String(), "Not verified: the OpenAI Responses endpoint of Team Gateway Again returned HTTP 503; aigw provider rotate team") {
				t.Fatalf("report:\n%s", home.out)
			}
		})
	}
}

// A piped Token that its provider refuses, for a provider whose Token the store
// holds, keeps that Token serving its clients; the person sees the problem in
// place of the success line, and the command fails.
func TestTeamSetupKeepsAStoredTokenWhenAPipedOneIsRefused(t *testing.T) {
	home := newGuidedHome(t, claude.ID)
	home.runtime.Interactive = false
	configuredTeam(t, home)
	acceptOnly(home, "sk-stored")
	home.runtime.In = strings.NewReader("sk-wrong\n")
	if err := home.run(t, &guidedPrompt{}, "--from", writeTeamFile(t, teamFileText), "--provider", "team", "--token-stdin"); err == nil {
		t.Fatalf("a refused piped Token passed:\n%s", home.out)
	}
	report := home.out.String()
	for _, want := range []string{
		"Team file applied", "Kept the stored Token; the new one was not stored: Team Gateway Again did not accept the Token (HTTP 401)",
		"Problem", "Token validation failed: Team Gateway Again did not accept the Token (HTTP 401)",
		"No Token was stored; the previous Token is unchanged. The team file and every other Token were applied.",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "Saved;") || strings.Contains(report, "aigw provider rotate team") && strings.Contains(report, "team/team-model; aigw provider rotate team") {
		t.Errorf("report claims success or a waiting client:\n%s", report)
	}
	if token, err := home.secrets.Get("team"); err != nil || token != "sk-stored" {
		t.Fatalf("the stored Token = %q, %v", token, err)
	}
}
