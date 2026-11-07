package model

import (
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

func TestListFiltersByQueryProviderClientAndProtocol(t *testing.T) {
	f := newFixture(t)
	models := func(t *testing.T, args ...string) []string {
		t.Helper()
		if err := f.run(append([]string{"list", "--json"}, args...)...); err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, item := range f.list(t).Models {
			names = append(names, item.ID)
		}
		return names
	}
	for _, test := range []struct {
		args []string
		want []string
	}{
		{nil, []string{"mixed/Chat-Only", "team/team-model"}},
		{[]string{"--provider", "team"}, []string{"team/team-model"}},
		{[]string{"--provider", "team", "--provider", "mixed"}, []string{"mixed/Chat-Only", "team/team-model"}},
		{[]string{"--client", "codex"}, nil},
		{[]string{"--client", "claude"}, []string{"team/team-model"}},
		{[]string{"--client", "codex", "--client", "hermes"}, []string{"mixed/Chat-Only", "team/team-model"}},
		{[]string{"--protocol", "chat"}, []string{"mixed/Chat-Only"}},
		// Every query word must appear in the ID or the label, ignoring case.
		{[]string{"chat", "ONLY"}, []string{"mixed/Chat-Only"}},
		{[]string{"team model"}, []string{"team/team-model"}},
		{[]string{"chat", "team"}, nil},
	} {
		if got := models(t, test.args...); !slices.Equal(got, test.want) {
			t.Errorf("model list %q = %q, want %q", test.args, got, test.want)
		}
	}
	if f.catalog.requests != 0 {
		t.Fatalf("model list sent %d requests", f.catalog.requests)
	}
}

func TestListRefusesUnknownNamesWithTheirFix(t *testing.T) {
	f := newFixture(t)
	err := f.run("list", "--client", "codx")
	if p := problem(err); !strings.Contains(p.Title, "did you mean codex?") || p.Fix != "aigw status" {
		t.Fatalf("an unknown client = %q, fix %q", p.Title, p.Fix)
	}
	err = f.run("list", "--provider", "teem")
	if p := problem(err); p.Title != `unknown provider "teem"; did you mean team?` || p.Fix != "aigw provider list" {
		t.Fatalf("an unknown provider = %q, fix %q", p.Title, p.Fix)
	}
}

func TestListRendersEachProviderOnce(t *testing.T) {
	f := newFixture(t)
	if err := f.run("list"); err != nil {
		t.Fatal(err)
	}
	text := f.text()
	for _, want := range []string{"Mixed · mixed/Chat-Only Chat Only · OpenAI Chat Completions Team ✓ team/team-model Team · Team Model · Anthropic Messages · used by Claude", "aigw use <client> <provider/model>"} {
		if !strings.Contains(text, want) {
			t.Fatalf("model list lacks %q:\n%s", want, f.out)
		}
	}
	if err := f.run("list", "nothing-matches"); err != nil || !strings.Contains(f.text(), "No added model matches.") {
		t.Fatalf("an empty list = %v:\n%s", err, f.out)
	}
}

func TestListMarksADeprecatedModel(t *testing.T) {
	f := newFixture(t)
	cfg := f.load(t)
	model := cfg.Models["mixed-chat"]
	model.Lifecycle = domain.ModelDeprecated
	cfg.Models["mixed-chat"] = model
	delete(cfg.Recommendations, "hermes")
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := f.run("list"); err != nil || !strings.Contains(f.text(), "! mixed/Chat-Only Chat Only · OpenAI Chat Completions · deprecated") {
		t.Fatalf("a deprecated model = %v:\n%s", err, f.out)
	}
	if err := f.run("show", "mixed/Chat-Only"); err != nil || !strings.Contains(f.text(), "! Deprecated Kept for the clients that select it") {
		t.Fatalf("a deprecated model's details = %v:\n%s", err, f.out)
	}
	if err := f.run("list", "--json"); err != nil || !f.list(t).Models[0].Deprecated {
		t.Fatalf("a deprecated model's document = %v:\n%s", err, f.out)
	}
}

func TestShowReportsOneModel(t *testing.T) {
	f := newFixture(t)
	if err := f.run("show", "mixed/Chat-Only", "--json"); err != nil {
		t.Fatal(err)
	}
	document := f.document(t)
	if document.ID != "mixed/Chat-Only" || document.Family != "chat-only" || document.Label != "Chat Only" {
		t.Fatalf("model show = %+v", document)
	}
	if endpoints := document.Endpoints; len(endpoints) != 1 || endpoints[0] != (endpointDetail{Protocol: "chat", URL: "https://mixed.test/v1"}) {
		t.Fatalf("model show endpoints = %+v", endpoints)
	}
	if err := f.run("show", "team/team-model"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Model team/team-model", "Selected by Claude", "Protocol clients Claude, Claude Desktop, Hermes", "Anthropic Messages https://team.test"} {
		if !strings.Contains(f.text(), want) {
			t.Fatalf("model show lacks %q:\n%s", want, f.out)
		}
	}
}

func TestShowRefusesAMalformedOrUnknownModel(t *testing.T) {
	f := newFixture(t)
	for args, want := range map[string]struct{ title, fix string }{
		"show chat-only":         {`"chat-only" names no provider; it could mean mixed/Chat-Only`, "aigw model list chat-only"},
		"show team/missing":      {"unknown model team/missing", "aigw model list --provider team"},
		"show team/team-modl":    {"unknown model team/team-modl; did you mean team/team-model?", "aigw model list --provider team"},
		"show nobody/team-model": {`unknown provider "nobody"`, "aigw provider list"},
	} {
		if p := problem(f.run(strings.Fields(args)...)); p.Title != want.title || p.Fix != want.fix {
			t.Errorf("model %s = %q, fix %q; want %q, fix %q", args, p.Title, p.Fix, want.title, want.fix)
		}
	}
	// A malformed reference is refused before configuration access, with the
	// command's help.
	for _, argument := range []string{"/team-model", "team/", "bad provider/model"} {
		err := f.run("show", argument)
		if err == nil || !strings.Contains(err.Error(), "run `model show --help`") {
			t.Errorf("model show %q = %v, want the help", argument, err)
		}
	}
	if err := f.run("show"); err == nil {
		t.Fatal("model show without an argument succeeded")
	}
}

func TestProtocolListReadsAsProse(t *testing.T) {
	for protocols, want := range map[string]string{
		"":                         "no protocol",
		"anthropic":                "Anthropic Messages",
		"anthropic chat":           "Anthropic Messages and OpenAI Chat Completions",
		"anthropic responses chat": "Anthropic Messages, OpenAI Responses and OpenAI Chat Completions",
	} {
		var parsed []domain.EndpointProtocol
		for name := range strings.FieldsSeq(protocols) {
			protocol, _ := domain.ProtocolNamed(name)
			parsed = append(parsed, protocol)
		}
		if got := protocolLabels(parsed); got != want {
			t.Errorf("protocolLabels(%q) = %q, want %q", protocols, got, want)
		}
	}
}
