package main

import (
	"slices"
	"strings"
	"testing"
)

// TestFixLoopRefusesWhatItCannotFollow pins fixloop's refusals: a document
// that reports no failure or names no fix, a fix that is not an aigw command
// or is the failing command itself, a step for the script that -did does not
// state, and -did when every fix runs as written.
func TestFixLoopRefusesWhatItCannotFollow(t *testing.T) {
	command := []string{"aigw", "use", "claude", "team/team-model", "--json"}
	for _, tc := range []struct {
		name, document string
		did            bool
		want           string
	}{
		{"human output", "Problem\n  refused\n", false, "not a JSON object"},
		{"a success", `{"ok": true, "next_action": "aigw sync"}`, false, "reports no failure"},
		{"no outcome", `{"next_action": "aigw sync"}`, false, "reports no failure"},
		{"no fix", `{"ok": false, "error": "refused"}`, false, "names no fix"},
		{"the command itself", `{"ok": false, "error": "refused", "next_action": "aigw use claude team/team-model"}`, false, "the failing command itself"},
		{"a shell command", `{"ok": false, "error": "refused", "next_action": "rm -r .aigw"}`, false, "not an aigw command"},
		{"an unstated step", `{"ok": false, "error": "refused", "next_detail": "Move the file ~/work aside"}`, false, "state it with -did"},
		{"a step that is not asked for", `{"ok": false, "error": "refused", "next_action": "aigw sync"}`, true, "every fix runs as written"},
		{"a failed row's command", `{"ok": false, "clients": [{"state": "degraded", "failure": {"problem": "HTTP 503"}, "next_action": "aigw use claude team/team-model --json"}]}`, false, "the failing command itself"},
	} {
		plan, err := planFix(tc.document)
		if err == nil {
			err = plan.admits(command, tc.did)
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", tc.name, err, tc.want)
		}
	}
	plan, err := planFix(`{"ok": false, "error": "refused", "next_action": "aigw sync"}`)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.admits([]string{"sh", "-c", "aigw use claude"}, false); err == nil || !strings.Contains(err.Error(), "runs only aigw") {
		t.Errorf("a shell rerun: err = %v", err)
	}
	if err := plan.admits(command, false); err != nil {
		t.Errorf("a fix that differs from the command: %v", err)
	}
}

// TestFixLoopFollowsEachFailedRow shows that the fix of each row that reports
// a problem counts, as does an unused Token's, and that a passed row's does
// not.
func TestFixLoopFollowsEachFailedRow(t *testing.T) {
	plan, err := planFix(`{
  "ok": false,
  "clients": [
    {"id": "claude", "state": "configured", "next_action": "aigw check claude"},
    {"id": "codex", "state": "degraded", "failure": {"problem": "HTTP 503: the provider failed"}, "next_action": "aigw check codex"},
    {"id": "hermes", "state": "failed", "next_detail": "aigw use hermes <provider/model>"}
  ],
  "providers": [{"id": "broken", "problem": "OpenAI Responses: HTTP 503", "next_action": "aigw model search --provider broken"}],
  "unused_tokens": {"state": "listed", "providers": [{"provider": "orphan", "next_action": "aigw provider remove orphan"}]},
  "problem": "2 clients are not ready",
  "next_action": "aigw check codex"
}`)
	if err != nil {
		t.Fatal(err)
	}
	wantActions := []string{"aigw check codex", "aigw model search --provider broken", "aigw provider remove orphan"}
	wantProblems := []string{"2 clients are not ready", "HTTP 503: the provider failed", "OpenAI Responses: HTTP 503"}
	if !slices.Equal(plan.actions, wantActions) || !slices.Equal(plan.problems, wantProblems) {
		t.Fatalf("actions %q, problems %q", plan.actions, plan.problems)
	}
	if !slices.Equal(plan.details, []string{"aigw use hermes <provider/model>"}) {
		t.Fatalf("details %q", plan.details)
	}
}

// TestFixLoopJudgesTheNextRun pins when a problem recurs and what fixloop
// decides from it, with and without -input.
func TestFixLoopJudgesTheNextRun(t *testing.T) {
	problems := []string{`unknown client "claud"`}
	same := func(text string) string { return text }
	for _, tc := range []struct {
		name, stdout string
		again        bool
	}{
		{"the same refusal", `{"ok": false, "error": "unknown client \"claud\"; did you mean claude?"}`, true},
		{"another refusal", `{"ok": false, "error": "provider team has no Token"}`, false},
		{"a success", `{"ok": true}`, false},
		{"human output", "Problem\n  unknown client \"claud\"; did you mean claude?\n", true},
	} {
		if got := recurring(problems, tc.stdout, "", same); (len(got) > 0) != tc.again {
			t.Errorf("%s: recurring = %q", tc.name, got)
		}
	}
	// Both sides are compared after normalize, which spells a path one way.
	unify := func(text string) string { return strings.ReplaceAll(text, "/private/work", "/work") }
	if got := recurring([]string{"/private/work/home is missing"}, "Problem\n  /work/home is missing\n", "", unify); len(got) != 1 {
		t.Errorf("a path spelled two ways: recurring = %q", got)
	}

	command := []string{"aigw", "use", "claud", "team/team-model", "--json"}
	for _, tc := range []struct {
		name          string
		input, failed bool
		again         []string
		want          string
	}{
		{"the command runs", false, false, nil, ""},
		{"another problem", false, true, nil, ""},
		{"the same problem", false, true, problems, "fails again"},
		{"an input still refused", true, true, problems, ""},
		{"an input refused in other words", true, true, nil, ""},
		{"an input that now runs", true, false, nil, "drop -input"},
	} {
		err := verdict(command, tc.input, tc.failed, tc.again)
		if (tc.want == "" && err != nil) || (tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want))) {
			t.Errorf("%s: verdict = %v, want %q", tc.name, err, tc.want)
		}
	}
}
