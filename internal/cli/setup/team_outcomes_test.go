package setup

import (
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/terminal"
)

// notAccepted is the row of a provider whose Token the provider refused.
const notAccepted = "Not accepted: Team Gateway Again did not accept the Token (HTTP 401); aigw provider rotate team"

// answerInTurn makes the home's provider answer each request with the next
// status, and every later one with the last.
func answerInTurn(home *guidedHome, statuses ...int) {
	requests := 0
	home.runtime.HTTP = setupHTTPClient(func(request *http.Request) (*http.Response, error) {
		status := statuses[min(requests, len(statuses)-1)]
		requests++
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(home.models)), Request: request}, nil
	})
}

// unbroken is a report with each run of white space as one space, so that a
// sentence is found wherever the report wraps it.
func unbroken(report string) string {
	return strings.Join(strings.Fields(report), " ")
}

// A provider keeps the last verdict on its Token however the person then
// leaves its prompt: Enter alone after choosing to enter it again, a skip
// after text that is not a Token, or answers to "What now?" that name no
// choice, which skip the provider and which setup says. Each applies the team
// file.
func TestTeamSetupKeepsTheLastVerdictHoweverThePromptIsLeft(t *testing.T) {
	path := writeTeamFile(t, teamFileText)
	for _, test := range []struct {
		name   string
		prompt *guidedPrompt
		said   string
	}{
		{"entered again, then Enter alone", &guidedPrompt{multi: [][]string{{"team"}}, secrets: []string{"sk-wrong", ""}, selections: []string{"again"}}, ""},
		{"entered again, then text that is not a Token, then skipped", &guidedPrompt{multi: [][]string{{"team"}},
			secrets: []string{"sk-wrong", "two words"}, selections: []string{"again", "skip"}}, ""},
		{"no answer names a choice", &guidedPrompt{multi: [][]string{{"team"}}, secrets: []string{"sk-wrong"}, selectErr: terminal.ErrNoChoice},
			"No answer named a choice, so Team Gateway Again was skipped."},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := newGuidedHome(t, claude.ID)
			acceptOnly(home, "sk-team")
			if err := home.run(t, test.prompt, "--from", path); err != nil {
				t.Fatalf("setup: %v\n%s", err, home.out)
			}
			if report := home.out.String(); !strings.Contains(report, "Team file applied") || !strings.Contains(report, notAccepted) ||
				!strings.Contains(unbroken(report), test.said) || strings.Contains(report, "No answer named") != (test.said != "") {
				t.Fatalf("report:\n%s", report)
			}
			if cfg := home.config(t); len(cfg.ProviderModelRefs("team")) == 0 {
				t.Fatal("the team file was not applied")
			}
			if _, err := home.secrets.Get("team"); err == nil {
				t.Fatal("a refused Token was stored")
			}
		})
	}
}

// Of two Tokens that a provider did not accept, the report carries the verdict
// on the later one, whichever it is, and a validation without a verdict stays
// the verdict when the person then presses Enter alone.
func TestTeamSetupReportsTheVerdictOnTheLaterToken(t *testing.T) {
	path := writeTeamFile(t, teamFileText)
	unverified := "Not verified: the OpenAI Responses endpoint of Team Gateway Again returned HTTP 503; aigw provider rotate team"
	for _, test := range []struct {
		name       string
		statuses   []int
		secrets    []string
		selections []string
		want       string
	}{
		{"no verdict, entered again, then Enter alone", []int{http.StatusServiceUnavailable}, []string{"sk-first", ""}, []string{"again"}, unverified},
		{"refused, then no verdict", []int{http.StatusUnauthorized, http.StatusServiceUnavailable}, []string{"sk-first", "sk-second"}, []string{"again", "skip"}, unverified},
		{"no verdict, then refused", []int{http.StatusServiceUnavailable, http.StatusUnauthorized}, []string{"sk-first", "sk-second"}, []string{"again", "skip"}, notAccepted},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := newGuidedHome(t, claude.ID)
			answerInTurn(home, test.statuses...)
			prompt := &guidedPrompt{multi: [][]string{{"team"}}, secrets: test.secrets, selections: test.selections}
			if err := home.run(t, prompt, "--from", path); err != nil {
				t.Fatalf("setup: %v\n%s", err, home.out)
			}
			if report := home.out.String(); !strings.Contains(report, test.want) {
				t.Fatalf("report:\n%s", report)
			}
			if _, err := home.secrets.Get("team"); err == nil {
				t.Fatal("a Token that was not accepted was stored")
			}
		})
	}
}

// Answers that name no choice after one provider's refusal skip that provider
// alone: the Token that another provider accepted earlier in the run is stored.
func TestTeamSetupStoresTheAcceptedTokenWhenAnotherQuestionGetsNoChoice(t *testing.T) {
	home := newGuidedHome(t, claude.ID)
	acceptOnly(home, "sk-spare")
	prompt := &guidedPrompt{multi: [][]string{{"spare", "team"}}, secrets: []string{"sk-spare", "sk-wrong"}, selectErr: terminal.ErrNoChoice}
	if err := home.run(t, prompt, "--from", writeTeamFile(t, teamFileText)); err != nil {
		t.Fatalf("setup: %v\n%s", err, home.out)
	}
	if token, err := home.secrets.Get("spare"); err != nil || token != "sk-spare" {
		t.Fatalf("the accepted Token = %q, %v", token, err)
	}
	if report := home.out.String(); !strings.Contains(report, notAccepted) {
		t.Fatalf("report:\n%s", report)
	}
}

// A prompt that the person cancels ends setup before any write. After a Token
// that this run accepted, its problem says that the Token was not stored,
// whichever prompt of a later provider is cancelled; before one, it says only
// that nothing changed.
func TestTeamSetupSaysThatACancelDropsAnAcceptedToken(t *testing.T) {
	path := writeTeamFile(t, teamFileText)
	for _, test := range []struct {
		name, cancelled, title, impact string
		secrets                        []string
	}{
		{"after an accepted Token", "Paste the Team Gateway Again", "The Token prompt for provider team was cancelled",
			"Nothing changed. No Token that this run accepted was stored.", []string{"sk-spare"}},
		{"at the question after a later provider's refusal", "Team Gateway Again did not accept", "The choice for provider team was cancelled",
			"Nothing changed. No Token that this run accepted was stored.", []string{"sk-spare", "sk-wrong"}},
		{"before any Token", "Paste the Spare", "The Token prompt for provider spare was cancelled", "Nothing changed.", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := newGuidedHome(t, claude.ID)
			acceptOnly(home, "sk-spare")
			prompt := &guidedPrompt{multi: [][]string{{"spare", "team"}}, secrets: test.secrets, fail: test.cancelled}
			err := home.run(t, prompt, "--from", path)
			if problem := terminal.ProblemFor(err); err == nil || problem.Title != test.title || problem.Impact != test.impact {
				t.Fatalf("problem %#v, error %v", problem, err)
			}
			if _, err := home.secrets.Get("spare"); err == nil {
				t.Fatal("a cancelled setup stored a Token")
			}
			if cfg := home.config(t); len(cfg.Providers) != 0 {
				t.Fatalf("a cancelled setup wrote the configuration: %v", cfg.ProviderIDs())
			}
		})
	}
}

// A provider that the command names more than once is one provider: it is
// asked once, and its Token is read from standard input as one named provider's
// is. One that the person skips with Enter alone is reported with the step
// that stores its Token; a preview, for which that Token is the run's own
// input, names none.
func TestTeamSetupAsksANamedProviderOnceAndNamesItsStepWhenSkipped(t *testing.T) {
	path := writeTeamFile(t, teamFileText)
	home := newGuidedHome(t, claude.ID)
	acceptOnly(home, "sk-team")
	prompt := &guidedPrompt{secrets: []string{"sk-team"}}
	if err := home.run(t, prompt, "--from", path, "--provider", "team", "--provider", "team"); err != nil {
		t.Fatalf("setup: %v\n%s", err, home.out)
	}
	if !slices.Equal(prompt.asked, []string{tokenPrompt("Team Gateway Again")}) {
		t.Fatalf("asked %q", prompt.asked)
	}
	if token, err := home.secrets.Get("team"); err != nil || token != "sk-team" {
		t.Fatalf("the stored Token = %q, %v", token, err)
	}
	home = newGuidedHome(t, claude.ID)
	home.runtime.Interactive = false
	acceptOnly(home, "sk-team")
	home.runtime.In = strings.NewReader("sk-team\n")
	if err := home.run(t, &guidedPrompt{}, "--from", path, "--provider", "team", "--provider", "team", "--token-stdin"); err != nil {
		t.Fatalf("piped setup: %v\n%s", err, home.out)
	}
	if token, err := home.secrets.Get("team"); err != nil || token != "sk-team" {
		t.Fatalf("the piped Token = %q, %v", token, err)
	}

	step := "None; aigw provider rotate team"
	home = newGuidedHome(t, claude.ID)
	if err := home.run(t, &guidedPrompt{secrets: []string{""}}, "--from", path, "--provider", "team"); err != nil {
		t.Fatalf("setup: %v\n%s", err, home.out)
	}
	if report := home.out.String(); !strings.Contains(report, "Team file applied") || !strings.Contains(report, step) {
		t.Fatalf("report:\n%s", report)
	}
	home = newGuidedHome(t, claude.ID)
	if err := home.run(t, &guidedPrompt{}, "--from", path, "--provider", "team", "--dry-run"); err != nil {
		t.Fatalf("preview: %v\n%s", err, home.out)
	}
	if report := home.out.String(); !strings.Contains(report, "Team file preview") || strings.Contains(report, step) {
		t.Fatalf("preview:\n%s", report)
	}
}

// A preview asks nothing. In a terminal it says what the run asks for: which
// Tokens the person holds, or the named providers' Tokens, which the preview
// takes as accepted; where the run asks nothing, it says nothing.
func TestTeamPreviewSaysWhatTheRunAsks(t *testing.T) {
	path := writeTeamFile(t, teamFileText)
	which := "The run asks which providers' Tokens you hold (Spare, Team Gateway Again); this preview assumes none."
	named := "The run asks for the Team Gateway Again Token; this preview takes it as accepted."
	both := "The run asks for the Tokens of Spare, Team Gateway Again; this preview takes them as accepted."
	for _, test := range []struct {
		name     string
		terminal bool
		stored   []string
		args     []string
		want     string
	}{
		{"in a terminal", true, nil, nil, which},
		{"a named provider", true, nil, []string{"--provider", "team"}, named},
		{"two named providers", true, nil, []string{"--provider", "spare", "--provider", "team"}, both},
		{"a named provider with a stored Token", true, []string{"team"}, []string{"--provider", "team"}, ""},
		{"every provider with a stored Token", true, []string{"spare", "team"}, nil, ""},
		{"without a terminal", false, nil, nil, ""},
		{"as JSON", true, nil, []string{"--json"}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := newGuidedHome(t, claude.ID)
			home.runtime.Interactive = test.terminal
			for _, provider := range test.stored {
				if err := home.secrets.Set(provider, "sk-stored"); err != nil {
					t.Fatal(err)
				}
			}
			prompt := &guidedPrompt{}
			if err := home.run(t, prompt, append([]string{"--from", path, "--dry-run"}, test.args...)...); err != nil {
				t.Fatalf("preview: %v\n%s", err, home.out)
			}
			report := unbroken(home.out.String())
			if len(prompt.asked) != 0 || strings.Contains(report, "The run asks") != (test.want != "") || !strings.Contains(report, test.want) {
				t.Fatalf("asked %q; want %q in:\n%s", prompt.asked, test.want, home.out)
			}
		})
	}
}
