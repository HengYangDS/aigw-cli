package model

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

func TestEditReplacesEachSetItIsGiven(t *testing.T) {
	f := newFixture(t)
	// Without Hermes's recommendation of the model on chat, the edit may take
	// chat away.
	cfg := f.load(t)
	delete(cfg.Recommendations, "hermes")
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	// The given capabilities go to each protocol that admits them.
	if err := f.run("edit", "mixed/Chat-Only", "--protocol", "chat", "--protocol", "anthropic", "--protocol", "chat", "--capability", "tool_search", "--capability", "text"); err != nil {
		t.Fatal(err)
	}
	model := f.load(t).Models["mixed-chat"]
	if got := model.Protocols; len(got) != 2 || !slices.Equal(got[domain.ProtocolAnthropic], []domain.Capability{domain.CapabilityText, domain.CapabilityToolSearch}) ||
		!slices.Equal(got[domain.ProtocolOpenAIChatCompletions], []domain.Capability{domain.CapabilityText}) {
		t.Fatalf("edited protocols = %v", got)
	}
	if model.Label != "Chat Only" || model.Family != "chat-only" {
		t.Fatalf("an edit of the protocols changed the label or family: %#v", model)
	}
	// A protocol change alone keeps each kept protocol's capabilities, and a
	// new protocol declares none: no capability moves to a protocol where it
	// was not qualified.
	if err := f.run("edit", "mixed/Chat-Only", "--protocol", "anthropic", "--protocol", "responses"); err == nil {
		t.Fatal("a protocol without an endpoint was accepted")
	}
	if err := f.run("edit", "mixed/Chat-Only", "--protocol", "anthropic"); err != nil {
		t.Fatal(err)
	}
	if got := f.load(t).Models["mixed-chat"].Protocols; len(got) != 1 || !slices.Equal(got[domain.ProtocolAnthropic], []domain.Capability{domain.CapabilityText, domain.CapabilityToolSearch}) {
		t.Fatalf("protocols after dropping chat = %v", got)
	}
	if err := f.run("edit", "mixed/Chat-Only", "--protocol", "anthropic", "--protocol", "chat"); err != nil {
		t.Fatal(err)
	}
	if got := f.load(t).Models["mixed-chat"].Protocols; len(got[domain.ProtocolOpenAIChatCompletions]) != 0 || len(got[domain.ProtocolAnthropic]) != 2 {
		t.Fatalf("a regained protocol declared capabilities: %v", got)
	}
	// An edit of another field changes no capability.
	if err := f.run("edit", "mixed/Chat-Only", "--label", "Renamed"); err != nil {
		t.Fatal(err)
	}
	if got := f.load(t).Models["mixed-chat"].Protocols; len(got[domain.ProtocolOpenAIChatCompletions]) != 0 || len(got[domain.ProtocolAnthropic]) != 2 {
		t.Fatalf("a label edit moved capabilities: %v", got)
	}
	// --capability "" clears them, and --label "" clears the label.
	if err := f.run("edit", "mixed/Chat-Only", "--capability", "", "--label", "", "--json"); err != nil {
		t.Fatal(err)
	}
	cleared := f.load(t).Models["mixed-chat"]
	if len(cleared.Protocols[domain.ProtocolAnthropic]) != 0 || cleared.Label != "" || f.document(t).Label != "Mixed · chat-only" {
		t.Fatalf("cleared model = %#v, document label %q", cleared, f.document(t).Label)
	}
	if f.document(t).NextAction != "aigw model show mixed/Chat-Only" {
		t.Fatalf("edit continuation = %q", f.document(t).NextAction)
	}
}

// TestEditPinsTheProtocolOfAnImplicitSelection gives a model that Claude
// selects without an explicit protocol a second protocol that Claude does not
// speak: the selection keeps its protocol and stays valid.
func TestEditPinsTheProtocolOfAnImplicitSelection(t *testing.T) {
	f := newFixture(t)
	cfg := f.load(t)
	cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1"}}
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := f.run("edit", "team/team-model", "--protocol", "anthropic", "--protocol", "responses"); err != nil {
		t.Fatal(err)
	}
	if got := f.load(t).Clients["claude"]; got.Model != "team" || got.Protocol != domain.ProtocolAnthropic {
		t.Fatalf("Claude's selection = %#v", got)
	}
}

func TestEditLinksAFamilyAndCreatesAnUnknownOne(t *testing.T) {
	f := newFixture(t)
	if err := f.run("edit", "mixed/Chat-Only", "--family", "team-model", "--json"); err != nil || f.document(t).Family != "team-model" || f.document(t).FamilyCreated != "" {
		t.Fatalf("a known family = %v:\n%s", err, f.out)
	}
	if err := f.run("edit", "mixed/Chat-Only", "--family", "chat-family"); err != nil || !strings.Contains(f.text(), "New family chat-family") {
		t.Fatalf("an unknown family = %v:\n%s", err, f.out)
	}
	if family := f.load(t).Families["chat-family"]; family.Label != "Chat-Only" {
		t.Fatalf("created family = %#v", family)
	}
}

func TestEditRefusesWhatWouldBreakItsModel(t *testing.T) {
	f := newFixture(t)
	for args, want := range map[string]struct{ title, fix string }{
		"edit team/team-model --protocol chat": {
			"team/team-model cannot declare OpenAI Chat Completions: provider team has no OpenAI Chat Completions endpoint",
			"model edit team/team-model --protocol anthropic",
		},
		// The fix keeps the model's own protocols beside the one that admits
		// tool search.
		"edit mixed/Chat-Only --capability tool_search": {
			"no protocol of mixed/Chat-Only admits tool_search; it declares OpenAI Chat Completions",
			"model edit mixed/Chat-Only --capability tool_search --protocol chat --protocol anthropic",
		},
		"edit mixed/Chat-Only --protocol chat --capability tool_search": {
			"no protocol of mixed/Chat-Only admits tool_search; it declares OpenAI Chat Completions",
			"model edit mixed/Chat-Only --capability tool_search --protocol chat --protocol anthropic",
		},
	} {
		if p := problem(f.run(strings.Fields(args)...)); p.Title != want.title || p.Fix != want.fix {
			t.Errorf("model %s = %q, fix %q; want %q, fix %q", args, p.Title, p.Fix, want.title, want.fix)
		}
	}
	// A provider without an Anthropic endpoint cannot declare tool search, so
	// the fix is the help.
	cfg := f.load(t)
	cfg.Providers["mixed"] = domain.Provider{Label: "Mixed", Endpoints: domain.Endpoints{OpenAIChatCompletions: "https://mixed.test/v1"}}
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if p := problem(f.run("edit", "mixed/Chat-Only", "--capability", "tool_search")); p.Fix != "model edit --help" {
		t.Fatalf("a capability that the provider cannot serve = %q", p.Fix)
	}
	// Of the given protocols, the served ones stay in the rerun; when none
	// is served, the rerun gives the one that the provider serves, as add's
	// does.
	p := problem(f.run("edit", "mixed/Chat-Only", "--protocol", "anthropic", "--protocol", "chat"))
	if p.Fix != "model edit mixed/Chat-Only --protocol chat" {
		t.Fatalf("a mixed protocol set = %q", p.Fix)
	}
	p = problem(f.run("edit", "mixed/Chat-Only", "--protocol", "responses", "--protocol", "anthropic"))
	if p.Fix != "model edit mixed/Chat-Only --protocol chat" {
		t.Fatalf("a set of unserved protocols = %q", p.Fix)
	}
}

func TestEditKeepsTheProtocolThatAClientUses(t *testing.T) {
	f := newFixture(t)
	cfg := f.load(t)
	// Hermes uses the mixed model on chat; it also speaks anthropic.
	cfg.Models["mixed-chat"] = domain.Model{
		Provider: "mixed", Family: "chat-only", UpstreamModel: "Chat-Only",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIChatCompletions: {}, domain.ProtocolAnthropic: {}},
	}
	cfg.SetSelectedModel("hermes", "mixed-chat", domain.ProtocolOpenAIChatCompletions)
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	p := problem(f.run("edit", "mixed/Chat-Only", "--protocol", "anthropic"))
	if p.Title != "Hermes uses mixed/Chat-Only on OpenAI Chat Completions, which this edit would remove" || p.Fix != "aigw use hermes mixed/Chat-Only --protocol anthropic" {
		t.Fatalf("an edit that takes a client's protocol = %q, fix %q", p.Title, p.Fix)
	}
	// Claude speaks only anthropic, so none of the remaining protocols serves it.
	cfg = f.load(t)
	cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1"}}
	cfg.Models["team"] = domain.Model{Provider: "team", Family: "team-model", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}, domain.ProtocolOpenAIResponses: {}}}
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if p := problem(f.run("edit", "team/team-model", "--protocol", "responses")); p.Fix != "aigw use claude <provider/model>" {
		t.Fatalf("a fix without a remaining protocol for the client = %q", p.Fix)
	}
	// Hermes speaks all three, so a choice is offered.
	cfg = f.load(t)
	cfg.Providers["mixed"] = domain.Provider{Label: "Mixed", Endpoints: domain.Endpoints{
		Anthropic: "https://mixed.test", OpenAIChatCompletions: "https://mixed.test/v1", OpenAIResponses: "https://mixed.test/v1",
	}}
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if p := problem(f.run("edit", "mixed/Chat-Only", "--protocol", "anthropic", "--protocol", "responses")); p.Fix != "aigw use hermes mixed/Chat-Only --protocol <anthropic|responses>" {
		t.Fatalf("a fix with a choice = %q", p.Fix)
	}
	if err := f.run("edit", "mixed/Chat-Only", "--protocol", "chat", "--protocol", "anthropic"); err != nil {
		t.Fatalf("an edit that keeps the client's protocol: %v", err)
	}
}

func TestEditNeedsAChangeAndValidValues(t *testing.T) {
	f := newFixture(t)
	if err := os.WriteFile(f.runtime.Config.Path(), []byte("not [ toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"edit", "team/team-model"},
		{"edit", "team/team-model", "--family", "bad family"},
		{"edit", "team/team-model", "--capability", "telepathy"},
		{"edit", "team/"},
	} {
		if err := f.run(args...); err == nil || !strings.Contains(err.Error(), "run `model edit --help`") {
			t.Errorf("model %q = %v, want a refusal that names the help", args, err)
		}
	}
	// A protocol's old spelling fails flag parsing, whose refusal the CLI root
	// gives the command's help.
	if err := f.run("edit", "team/team-model", "--protocol", "responses", "--protocol", "openai_responses"); err == nil ||
		err.Error() != `invalid argument "openai_responses" for "--protocol" flag: expected anthropic, responses, or chat` {
		t.Errorf("an old protocol spelling = %v", err)
	}
}

func TestEditPreviewWritesNothing(t *testing.T) {
	f := newFixture(t)
	before, err := os.ReadFile(f.runtime.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.run("edit", "team/team-model", "--label", "Renamed", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.text(), "Model edited (preview)") || !strings.Contains(f.text(), "Label Renamed") || !strings.Contains(f.text(), "Next model edit team/team-model --label Renamed") {
		t.Fatalf("edit preview:\n%s", f.out)
	}
	if after, err := os.ReadFile(f.runtime.Config.Path()); err != nil || !bytes.Equal(after, before) {
		t.Fatalf("an edit preview wrote configuration: %v", err)
	}
}

func TestRemoveDropsTheModelAndItsRecommendations(t *testing.T) {
	f := newFixture(t)
	if err := f.run("remove", "mixed/Chat-Only", "--dry-run"); err != nil || !strings.Contains(f.text(), "Recommendations Dropped for Hermes") {
		t.Fatalf("removal preview = %v:\n%s", err, f.out)
	}
	if _, exists := f.load(t).Models["mixed-chat"]; !exists {
		t.Fatal("a removal preview removed the model")
	}
	if err := f.run("remove", "mixed/Chat-Only", "--json"); err != nil {
		t.Fatal(err)
	}
	if got := f.document(t).Recommendations; !slices.Equal(got, []string{"hermes"}) {
		t.Fatalf("removal recommendations = %v", got)
	}
	cfg := f.load(t)
	if _, exists := cfg.Models["mixed-chat"]; exists || len(cfg.Recommendations) != 0 || len(cfg.Providers) != 2 {
		t.Fatalf("after removal: models %v, recommendations %v, providers %v", cfg.Models, cfg.Recommendations, cfg.Providers)
	}
	if token, err := f.runtime.Secrets.Get("team"); err != nil || token != "team-token" {
		t.Fatalf("a removal changed a Token: %q, %v", token, err)
	}
}

func TestRemoveRefusesAModelThatAnyClientUses(t *testing.T) {
	f := newFixture(t)
	p := problem(f.run("remove", "team/team-model"))
	if p.Title != "Claude uses team/team-model, so it cannot be removed" || p.Fix != "aigw use claude <provider/model>" {
		t.Fatalf("removing a selected model = %q, fix %q", p.Title, p.Fix)
	}
	// A disabled client keeps its selection too.
	cfg := f.load(t)
	binding := cfg.Clients["claude"]
	binding.Enabled = false
	cfg.Clients["claude"] = binding
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := f.run("remove", "team/team-model", "--dry-run"); err == nil {
		t.Fatal("a model that a disabled client selects was removed")
	}
}

func TestRemoveRecommendedModelPromotesTheFirstAlternative(t *testing.T) {
	cfg := domain.NewConfig(domain.Admission{})
	choice := func(model string) domain.ClientSelection { return domain.ClientSelection{Model: model} }
	cfg.Recommendations["a"] = domain.ClientRecommendation{Primary: choice("gone"), Alternatives: []domain.ClientSelection{choice("next"), choice("last")}}
	cfg.Recommendations["b"] = domain.ClientRecommendation{Primary: choice("kept"), Alternatives: []domain.ClientSelection{choice("gone")}}
	cfg.Recommendations["c"] = domain.ClientRecommendation{Primary: choice("gone")}
	cfg.Recommendations["d"] = domain.ClientRecommendation{Primary: choice("kept")}
	if changed := cfg.RemoveRecommendedModel("gone"); !slices.Equal(changed, []string{"a", "b", "c"}) {
		t.Fatalf("changed recommendations = %q", changed)
	}
	if a := cfg.Recommendations["a"]; a.Primary.Model != "next" || len(a.Alternatives) != 1 || a.Alternatives[0].Model != "last" {
		t.Fatalf("a = %#v", a)
	}
	if b := cfg.Recommendations["b"]; b.Primary.Model != "kept" || len(b.Alternatives) != 0 {
		t.Fatalf("b = %#v", b)
	}
	if _, exists := cfg.Recommendations["c"]; exists {
		t.Fatal("a recommendation without a choice was kept")
	}
	if d := cfg.Recommendations["d"]; d.Primary.Model != "kept" {
		t.Fatalf("d = %#v", d)
	}
}
