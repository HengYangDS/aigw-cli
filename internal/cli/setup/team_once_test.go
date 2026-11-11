package setup

import (
	"strings"
	"testing"
)

// A model that the team file and the local configuration both name is
// reported once: in one list of the changes, and in one row per protocol of
// what stays local, even when the file's capabilities were dropped and the
// protocol has no endpoint.
func TestTeamSetupReportsAModelOnceThatTheFileAndTheConfigurationBothName(t *testing.T) {
	home := newGuidedHome(t)
	home.runtime.Interactive = false
	configuredTeam(t, home)
	path := writeTeamFile(t, `version = 8

[[providers]]
id = "team"
label = "Team"
endpoints = {anthropic = "http://127.0.0.1:8792/team", chat = "https://elsewhere.test/v1"}

[[providers.models]]
id = "team-model"
family = "team-model"
protocols = {anthropic = ["text"], chat = ["text"]}

[[families]]
id = "team-model"
label = "Team Model"
`)
	if err := home.run(t, &guidedPrompt{}, "--from", path, "--dry-run"); err != nil {
		t.Fatalf("preview: %v\n%s", err, home.out)
	}
	text := home.out.String()
	var rows []string
	for line := range strings.SplitSeq(text, "\n") {
		if strings.Contains(line, "team/team-model chat") {
			rows = append(rows, line)
		}
	}
	if len(rows) != 1 || !strings.Contains(rows[0], "Unusable until the provider has the endpoint: aigw provider edit team --endpoint chat=<url>") || !strings.Contains(rows[0], "dropped text") {
		t.Fatalf("team/team-model chat is reported in %d rows, want one that says both:\n%s", len(rows), text)
	}
	if strings.Count(text, "Models               team/team-model") != 1 || strings.Contains(text, "Added\n  Models") {
		t.Fatalf("team/team-model is not named once among the changes:\n%s", text)
	}
}
