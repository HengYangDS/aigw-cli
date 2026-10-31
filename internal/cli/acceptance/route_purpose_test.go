package cli_test

import (
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"encoding/json"
	"strings"
	"testing"
)

func TestRoutePurposeIsOptionalHumanGuidance(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "current", "team", "Team Gateway", domain.Endpoints{Anthropic: "https://team.test"}, claude.ID, "claude-current")
	cfg.SetSelectedRoute(claude.ID, "current", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"route", "add", "claude-fable-5", "--account", "team", "--model", "claude-fable-5", "--protocol", "anthropic", "--label", "Claude Fable 5", "--purpose", "Default agent"}); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes["claude-fable-5"].Purpose != "Default agent" {
		t.Fatalf("purpose = %q", got.Routes["claude-fable-5"].Purpose)
	}
	if err := secretStore.Set("team", "team-token"); err != nil {
		t.Fatal(err)
	}
	selector := &scriptedPrompt{selections: []string{"claude-fable-5"}}
	app.Interactive = true
	app.Prompt = selector
	if err := cli.Execute(app, []string{"use", "--for", "claude"}); err != nil {
		t.Fatal(err)
	}
	labels := map[string]string{}
	for _, choice := range selector.choices {
		labels[choice.Value] = choice.Label
	}
	if len(selector.choices) != 2 || labels["claude-fable-5"] != "Claude Fable 5 · Default agent" || labels["current"] != "Team Gateway" {
		t.Fatalf("interactive choices = %#v", selector.choices)
	}

	out.Reset()
	if err := cli.Execute(app, []string{"route", "list"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Default agent") {
		t.Fatalf("route list lacks purpose:\n%s", out.String())
	}

	out.Reset()
	if err := cli.Execute(app, []string{"route", "show", "claude-fable-5", "--json"}); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Purpose string `json:"purpose"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Purpose != "Default agent" {
		t.Fatalf("route JSON lacks purpose:\n%s", out.String())
	}

	if err := cli.Execute(app, []string{"route", "edit", "claude-fable-5", "--purpose", "Deep agent"}); err != nil {
		t.Fatal(err)
	}
	got, err = app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes["claude-fable-5"].Purpose != "Deep agent" {
		t.Fatalf("edited purpose = %q", got.Routes["claude-fable-5"].Purpose)
	}
	if err := cli.Execute(app, []string{"route", "edit", "claude-fable-5", "--purpose", ""}); err != nil {
		t.Fatal(err)
	}
	got, err = app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes["claude-fable-5"].Purpose != "" {
		t.Fatalf("cleared purpose = %q", got.Routes["claude-fable-5"].Purpose)
	}
}

func TestRouteListUsesChineseProductLabelsWithoutRewritingPurpose(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "gpt", "team", "Team Gateway", domain.Endpoints{OpenAIResponses: "https://team.test/v1"}, codex.ID, "gpt-test")
	cfg.Routes["gpt"] = qualifiedRoute("GPT Test", "team", "gpt-test", domain.ProtocolOpenAIResponses)
	route := cfg.Routes["gpt"]
	route.Purpose = "native Codex picker-aligned daily default"
	cfg.Routes["gpt"] = route
	cfg.SetSelectedRoute(codex.ID, "gpt", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "team-token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"route", "list"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"Routes", "Available routes", "Route  gpt", "GPT Test · native Codex picker-aligned daily default · Selected for Codex · Account team · Clients Codex"} {
		if !strings.Contains(text, want) {
			t.Fatalf("route list lacks %q:\n%s", want, text)
		}
	}
	for _, retired := range []string{"Service routes\n", "Profile  gpt"} {
		if strings.Contains(text, retired) {
			t.Fatalf("route list retained product label %q:\n%s", retired, text)
		}
	}
}
