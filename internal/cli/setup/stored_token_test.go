package setup

import (
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
)

// TestGuidedSetupUsesAStoredTokenOfTheProviderThatItAdds adds a provider
// whose Token an earlier setup or release stored: setup asks for no Token,
// sends no validation request, keeps the stored one, says so and never
// prints it.
func TestGuidedSetupUsesAStoredTokenOfTheProviderThatItAdds(t *testing.T) {
	home := newGuidedHome(t, claude.ID)
	if err := home.secrets.Set("team", "sk-stored-earlier"); err != nil {
		t.Fatal(err)
	}
	answers := newTeamAnswers()
	answers.secrets, answers.fail = nil, "Paste the"
	// Claude Code is the one installed client, so setup asks only for the
	// model.
	answers.selections = append(answers.selections, "team-model")
	if err := home.run(t, answers); err != nil {
		t.Fatalf("guided setup: %v\n%s", err, home.out)
	}
	if slices.ContainsFunc(answers.asked, func(label string) bool { return strings.HasPrefix(label, "Paste the") }) {
		t.Fatalf("setup asked for a stored Token: %q", answers.asked)
	}
	cfg := home.config(t)
	if _, added := cfg.Providers["team"]; !added {
		t.Fatalf("the provider was not added:\n%s", home.out)
	}
	if token, err := home.secrets.Get("team"); err != nil || token != "sk-stored-earlier" {
		t.Fatalf("the stored Token = %q, %v", token, err)
	}
	if text := home.out.String(); !regexp.MustCompile(`(?m)^ +Provider +Team Gateway \(team\), added$`).MatchString(text) ||
		!regexp.MustCompile(`(?m)^ +✓ Token +Already stored; used without a prompt$`).MatchString(text) || strings.Contains(text, "sk-stored-earlier") {
		t.Fatalf("the report:\n%s", text)
	}
}

// TestTeamSetupUsesAStoredTokenOfANamedProvider names a provider that the
// team file adds and whose Token is stored: with or without a terminal, and
// with --json, setup asks for nothing, defers nothing, keeps the Token and
// says so; a piped Token still replaces it after the provider accepts it.
func TestTeamSetupUsesAStoredTokenOfANamedProvider(t *testing.T) {
	path := writeTeamFile(t, teamFileText)
	for _, test := range []struct {
		name        string
		interactive bool
		args        []string
		want        string
	}{
		{"without a terminal", false, nil, `(?m)^ +✓ team +Already stored; used without a prompt$`},
		{"in a terminal", true, nil, `(?m)^ +✓ team +Already stored; used without a prompt$`},
		{"with --json", true, []string{"--json"}, `"provider": "team",\n\s+"state": "present"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := newGuidedHome(t, claude.ID)
			home.runtime.Interactive = test.interactive
			if err := home.secrets.Set("team", "sk-stored-earlier"); err != nil {
				t.Fatal(err)
			}
			prompt := &guidedPrompt{fail: "Paste the"}
			if err := home.run(t, prompt, append([]string{"--from", path, "--provider", "team"}, test.args...)...); err != nil {
				t.Fatalf("setup: %v\n%s", err, home.out)
			}
			if len(prompt.asked) != 0 {
				t.Fatalf("setup asked %q", prompt.asked)
			}
			if token, err := home.secrets.Get("team"); err != nil || token != "sk-stored-earlier" {
				t.Fatalf("the stored Token = %q, %v", token, err)
			}
			if text := home.out.String(); !regexp.MustCompile(test.want).MatchString(text) || strings.Contains(text, "sk-stored-earlier") {
				t.Fatalf("the report lacks %q:\n%s", test.want, text)
			}
			if cfg := home.config(t); cfg.SelectedModel(claude.ID).Provider() != "team" {
				t.Fatalf("the recommended model was deferred: %+v", cfg.Clients[claude.ID])
			}
		})
	}
	// A piped Token replaces the stored one once the provider accepts it,
	// and one that it refuses changes nothing.
	home := newGuidedHome(t, claude.ID)
	if err := home.secrets.Set("team", "sk-stored-earlier"); err != nil {
		t.Fatal(err)
	}
	home.runtime.Interactive = false
	home.status = http.StatusUnauthorized
	home.runtime.In = strings.NewReader("sk-refused\n")
	if err := home.run(t, &guidedPrompt{}, "--from", path, "--provider", "team", "--token-stdin"); err == nil {
		t.Fatal("a refused piped Token was accepted")
	}
	if token, err := home.secrets.Get("team"); err != nil || token != "sk-stored-earlier" {
		t.Fatalf("a refused piped Token changed the stored one to %q, %v", token, err)
	}
	home.status = http.StatusOK
	home.runtime.In = strings.NewReader("sk-piped\n")
	if err := home.run(t, &guidedPrompt{}, "--from", path, "--provider", "team", "--token-stdin"); err != nil {
		t.Fatalf("a piped Token: %v\n%s", err, home.out)
	}
	if token, err := home.secrets.Get("team"); err != nil || token != "sk-piped" {
		t.Fatalf("the piped Token = %q, %v", token, err)
	}
}
