package setup

import (
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/terminal"
)

// notAccepted is the row of a provider whose Token the provider refused.
const notAccepted = "Not accepted: Team Gateway Again did not accept the Token (HTTP 401); aigw provider rotate team"

// A provider keeps the last verdict on its Token however the person then
// leaves its prompt: Enter alone after choosing to enter it again, a skip
// after text that is not a Token, or answers to "What now?" that name no
// choice, which skip the provider. Each applies the team file.
func TestTeamSetupKeepsTheLastVerdictHoweverThePromptIsLeft(t *testing.T) {
	path := writeTeamFile(t, teamFileText)
	for _, test := range []struct {
		name   string
		prompt *guidedPrompt
	}{
		{"entered again, then Enter alone", &guidedPrompt{multi: [][]string{{"team"}}, secrets: []string{"sk-wrong", ""}, selections: []string{"again"}}},
		{"entered again, then text that is not a Token, then skipped", &guidedPrompt{multi: [][]string{{"team"}},
			secrets: []string{"sk-wrong", "two words"}, selections: []string{"again", "skip"}}},
		{"no answer names a choice", &guidedPrompt{multi: [][]string{{"team"}}, secrets: []string{"sk-wrong"}, selectErr: terminal.ErrNoChoice}},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := newGuidedHome(t, claude.ID)
			acceptOnly(home, "sk-team")
			if err := home.run(t, test.prompt, "--from", path); err != nil {
				t.Fatalf("setup: %v\n%s", err, home.out)
			}
			if report := home.out.String(); !strings.Contains(report, "Team file applied") || !strings.Contains(report, notAccepted) {
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
// that this run accepted, its problem says that the Token was not stored;
// before one, it says only that nothing changed.
func TestTeamSetupSaysThatACancelDropsAnAcceptedToken(t *testing.T) {
	path := writeTeamFile(t, teamFileText)
	for _, test := range []struct {
		name, cancelled, title, impact string
		secrets                        []string
	}{
		{"after an accepted Token", "Paste the Team Gateway Again", "The Token prompt for provider team was cancelled",
			"Nothing changed. No Token that this run accepted was stored.", []string{"sk-spare"}},
		{"before any Token", "Paste the Spare", "The Token prompt for provider spare was cancelled", "Nothing changed.", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := newGuidedHome(t, claude.ID)
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

// A provider that the command names more than once is asked once, and one
// that the person skips with Enter alone is reported with the step that
// stores its Token; a preview, for which that Token is the run's own input,
// names none.
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
// Tokens the person holds, or a named provider's Token, which the preview
// takes as accepted; where the run asks nothing, it says nothing.
func TestTeamPreviewSaysWhatTheRunAsks(t *testing.T) {
	path := writeTeamFile(t, teamFileText)
	which := "The run asks which Tokens you hold of: Spare, Team Gateway Again. This preview assumes none."
	named := "The run asks for the Token of: Team Gateway Again. This preview takes it as accepted."
	for _, test := range []struct {
		name     string
		terminal bool
		stored   bool
		args     []string
		want     string
	}{
		{"in a terminal", true, false, nil, which},
		{"a named provider", true, false, []string{"--provider", "team"}, named},
		{"a named provider with a stored Token", true, true, []string{"--provider", "team"}, ""},
		{"without a terminal", false, false, nil, ""},
		{"as JSON", true, false, []string{"--json"}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := newGuidedHome(t, claude.ID)
			home.runtime.Interactive = test.terminal
			if test.stored {
				if err := home.secrets.Set("team", "sk-stored"); err != nil {
					t.Fatal(err)
				}
			}
			prompt := &guidedPrompt{}
			if err := home.run(t, prompt, append([]string{"--from", path, "--dry-run"}, test.args...)...); err != nil {
				t.Fatalf("preview: %v\n%s", err, home.out)
			}
			report := strings.Join(strings.Fields(home.out.String()), " ")
			if len(prompt.asked) != 0 || strings.Contains(report, "The run asks") != (test.want != "") || !strings.Contains(report, test.want) {
				t.Fatalf("asked %q; want %q in:\n%s", prompt.asked, test.want, home.out)
			}
		})
	}
}
