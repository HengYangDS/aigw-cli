package model

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

// TestEditRefusesToTakeARecommendedProtocol takes chat, on which Hermes's
// recommendation names the mixed model, from that model: the edit and its
// preview refuse it and write nothing, and the fix names the export of the team
// file that owns the recommendation.
func TestEditRefusesToTakeARecommendedProtocol(t *testing.T) {
	f := newFixture(t)
	before, err := os.ReadFile(f.runtime.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"edit", "mixed/Chat-Only", "--protocol", "anthropic"},
		{"edit", "mixed/Chat-Only", "--protocol", "anthropic", "--dry-run", "--json"},
	} {
		p := problem(f.run(args...))
		if p.Title != "Hermes's recommendation names mixed/Chat-Only on OpenAI Chat Completions, which this edit would remove" ||
			p.Fix != "aigw config export" || !strings.Contains(p.Evidence, "aigw setup --from <file>") {
			t.Fatalf("model %q = %+v", args, p)
		}
	}
	if after, err := os.ReadFile(f.runtime.Config.Path()); err != nil || !bytes.Equal(after, before) {
		t.Fatalf("a refused edit changed the configuration: %v", err)
	}
}

// TestEditKeepsRecommendationsOnTheirProtocols gives the mixed model a second
// protocol that Hermes speaks, while Hermes's recommendation names the model
// without a protocol: the edit records the protocol that the choice reached,
// reports the client, and moves no choice.
func TestEditKeepsRecommendationsOnTheirProtocols(t *testing.T) {
	f := newFixture(t)
	cfg := f.load(t)
	cfg.Recommendations["hermes"] = domain.ClientRecommendation{
		Primary:      domain.ClientSelection{Model: "team"},
		Alternatives: []domain.ClientSelection{{Model: "mixed-chat"}},
	}
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := f.run("edit", "mixed/Chat-Only", "--protocol", "chat", "--protocol", "anthropic", "--dry-run", "--json"); err != nil {
		t.Fatal(err)
	}
	if got := f.document(t).PinnedRecommendations; !slices.Equal(got, []string{"hermes"}) {
		t.Fatalf("preview pinned recommendations = %q", got)
	}
	if got := f.load(t).Recommendations["hermes"].Alternatives[0]; got.Protocol != "" {
		t.Fatalf("a preview pinned a recommendation: %#v", got)
	}
	if err := f.run("edit", "mixed/Chat-Only", "--protocol", "chat", "--protocol", "anthropic"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.text(), "Recommendations Kept on their current protocols for Hermes") {
		t.Fatalf("edit report:\n%s", f.out)
	}
	got := f.load(t).Recommendations["hermes"]
	want := domain.ClientRecommendation{
		Primary:      domain.ClientSelection{Model: "team"},
		Alternatives: []domain.ClientSelection{{Model: "mixed-chat", Protocol: domain.ProtocolOpenAIChatCompletions}},
	}
	if got.Primary != want.Primary || !slices.Equal(got.Alternatives, want.Alternatives) {
		t.Fatalf("recommendation = %#v, want %#v", got, want)
	}
	// A choice that already records its protocol stays as it was.
	if err := f.run("edit", "mixed/Chat-Only", "--protocol", "chat", "--protocol", "anthropic", "--json"); err != nil {
		t.Fatal(err)
	}
	if got := f.document(t).PinnedRecommendations; len(got) != 0 {
		t.Fatalf("an unchanged recommendation was reported as pinned: %q", got)
	}
}
