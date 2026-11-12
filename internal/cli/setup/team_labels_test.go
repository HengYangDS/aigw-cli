package setup

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

// A team file replaces the label that a local `model edit --label` set, so
// the preview names each such model with both labels, beside the provider
// label that the file replaces too, in human output and in JSON, the run
// reports the same, and a second apply of the file changes nothing.
func TestTeamPreviewNamesTheLabelsThatTheFileReplaces(t *testing.T) {
	home := newGuidedHome(t)
	home.runtime.Interactive = false
	configuredTeam(t, home)
	cfg, err := home.runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ref := domain.NewModelRef("team", "team-model")
	model := cfg.Models[ref]
	model.Label = "Edited Team Model"
	cfg.Models[ref] = model
	if err := home.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	path := writeTeamFile(t, teamFileText)

	if err := home.run(t, &guidedPrompt{}, "--from", path, "--dry-run"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Model labels", "team/team-model", "Edited Team Model becomes none"} {
		if !strings.Contains(home.out.String(), want) {
			t.Errorf("the preview lacks %q:\n%s", want, home.out)
		}
	}
	var documents [2]teamDocument
	for index, args := range [][]string{{"--from", path, "--dry-run", "--json"}, {"--from", path, "--json"}} {
		home.out.Reset()
		if err := home.run(t, &guidedPrompt{}, args...); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(home.out.Bytes(), &documents[index]); err != nil {
			t.Fatalf("%v\n%s", err, home.out)
		}
	}
	want := []changedLabel{{Provider: "team", Local: "Team", File: "Team Gateway Again"}, {Model: "team/team-model", Local: "Edited Team Model"}}
	if !reflect.DeepEqual(documents[0].ChangedLabels, want) || !reflect.DeepEqual(documents[1].ChangedLabels, want) {
		t.Fatalf("changed labels: preview %+v, run %+v, want %+v", documents[0].ChangedLabels, documents[1].ChangedLabels, want)
	}
	home.out.Reset()
	if err := home.run(t, &guidedPrompt{}, "--from", path, "--dry-run", "--json"); err != nil {
		t.Fatal(err)
	}
	var again teamDocument
	if err := json.Unmarshal(home.out.Bytes(), &again); err != nil || len(again.ChangedLabels) != 0 || len(again.Changed.Models) != 0 {
		t.Fatalf("a second preview still reports labels or models: %v %+v", err, again)
	}
	// A label that the file sets is reported as set.
	home.out.Reset()
	labelled := strings.Replace(teamFileText, `id = "team-model"
family = "team-model"`, `id = "team-model"
family = "team-model"
label = "Team Model Label"`, 1)
	if err := home.run(t, &guidedPrompt{}, "--from", writeTeamFile(t, labelled), "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(home.out.String(), "none becomes Team Model Label") {
		t.Errorf("the set label is not named:\n%s", home.out)
	}
}
