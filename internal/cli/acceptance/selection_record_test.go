package cli_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/domain"
)

// A command about something else records the choice together with AIGW's
// record of Codex's file. With a record left behind, the next choice of the
// earlier model in Codex would read as no choice, and synchronization would
// write the recorded one over it.
func TestARecordedChoiceKeepsAIGWsRecordOfTheFile(t *testing.T) {
	app, output, target := codexChoiceApp(t)
	chooseInCodex(t, target, "gpt-sol", "gpt-terra")
	edit := []string{"provider", "edit", "spare", "--label", "Other"}
	if got := followedIn(t, app, output, append(edit, "--dry-run", "--json")...); !slices.Equal(got, codexChoice) {
		t.Fatalf("the preview follows %+v", got)
	}
	output.Reset()
	if err := cli.Execute(app, append(edit, "--json")); err != nil {
		t.Fatalf("%v: %v\n%s", edit, err, output)
	}
	var result struct {
		Followed []followed `json:"followed_selections"`
		Files    []struct {
			Client string `json:"client"`
		} `json:"client_files"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil || !slices.Equal(result.Followed, codexChoice) || len(result.Files) != 1 || result.Files[0].Client != codex.ID {
		t.Fatalf("the run follows %+v and names files %+v: %v\n%s", result.Followed, result.Files, err, output)
	}
	if written := string(readFile(t, target)); !strings.Contains(written, `model = "gpt-terra" # managed by AIGW`) {
		t.Fatalf("AIGW's record of Codex's file stayed behind:\n%s", written)
	}
	if binding := selection(t, app, codex.ID); binding.Model != domain.NewModelRef("gateway", "gpt-terra") {
		t.Fatalf("the configuration does not name the chosen model: %#v", binding)
	}
	syncAndCheckPass(t, app, output, codex.ID)

	// The earlier model, chosen in Codex again, is a choice like any other.
	chooseInCodex(t, target, "gpt-terra", "gpt-sol")
	back := []followed{{Client: "codex", Model: "gateway/gpt-sol", Earlier: "gateway/gpt-terra", ReturnsWith: "aigw use codex gateway/gpt-terra"}}
	if got := followedIn(t, app, output, "status", "--json"); !slices.Equal(got, back) {
		t.Fatalf("status follows %+v, want the choice back", got)
	}
	if got := followedIn(t, app, output, "sync", "--json"); !slices.Equal(got, back) {
		t.Fatalf("sync follows %+v, want the choice back", got)
	}
	if written := string(readFile(t, target)); !strings.Contains(written, `model = "gpt-sol" # managed by AIGW`) {
		t.Fatalf("synchronization wrote over the model chosen in Codex:\n%s", written)
	}
}

// Hermes: a command that names nothing of Hermes, here an edit of another
// client's retry budget, records the choice with AIGW's record of Hermes'
// file, so that the earlier model, chosen in Hermes again, is followed and
// kept.
func TestARecordedHermesChoiceKeepsAIGWsRecordOfTheFile(t *testing.T) {
	app, output, _, target := shippedHermesApp(t)
	hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
	if got := followedIn(t, app, output, "client", "edit", codex.ID, "--request-retries", "2", "--json"); !slices.Equal(got, hermesChoice) {
		t.Fatalf("the run follows %+v\n%s", got, output)
	}
	hermesNativeSelection(t, target, "aigw-dmxapi-responses", "gpt-6.2-sol", nil)
	back := []followed{{
		Client: "hermes", Model: "dmxapi/gpt-6.2-sol", Protocol: "responses", Earlier: "dmxapi/claude-opus-5-5", EarlierProtocol: "anthropic",
		ReturnsWith: "aigw use hermes dmxapi/claude-opus-5-5 --protocol anthropic",
	}}
	for _, command := range [][]string{{"status", "--json"}, {"sync", "--json"}} {
		if got := followedIn(t, app, output, command...); !slices.Equal(got, back) {
			t.Fatalf("%v follows %+v, want the choice back\n%s", command, got, output)
		}
	}
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-dmxapi-responses" || model != "gpt-6.2-sol" {
		t.Fatalf("synchronization wrote over the model chosen in Hermes: %s %s", provider, model)
	}
	syncAndCheckPass(t, app, output, hermes.ID)
}
