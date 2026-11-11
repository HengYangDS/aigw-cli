package model

import (
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

// withoutEndpoint declares chat on team-model, whose provider has no Chat
// Completions endpoint, as a team file or an edit of the file can.
func withoutEndpoint(t *testing.T, f fixture) {
	t.Helper()
	cfg := f.load(t)
	ref := domain.NewModelRef("team", "team-model")
	model := cfg.Models[ref]
	model.Protocols = map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}, domain.ProtocolChat: {}}
	cfg.Models[ref] = model
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
}

// A protocol whose provider has no endpoint is marked, with the command that
// adds one, and never shown as an empty URL, by show and by list.
func TestAProtocolWithoutAnEndpointIsMarked(t *testing.T) {
	f := newFixture(t)
	withoutEndpoint(t, f)
	fix := "aigw provider edit team --endpoint chat=<url>"
	if err := f.run("show", "team/team-model"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Chat Completions No endpoint; run `" + fix + "`", "Anthropic Messages https://team.test"} {
		if !strings.Contains(f.text(), want) {
			t.Errorf("model show lacks %q:\n%s", want, f.out)
		}
	}
	if err := f.run("show", "team/team-model", "--json"); err != nil {
		t.Fatal(err)
	}
	endpoints := f.document(t).Endpoints
	if len(endpoints) != 2 || endpoints[0].URL != "https://team.test" || endpoints[1] != (endpointDetail{Protocol: "chat", Fix: fix}) {
		t.Fatalf("model show endpoints = %+v", endpoints)
	}
	if err := f.run("list", "--provider", "team"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Chat Completions (no endpoint)", fix} {
		if !strings.Contains(f.text(), want) {
			t.Errorf("model list lacks %q:\n%s", want, f.out)
		}
	}
	if err := f.run("list", "--provider", "team", "--json"); err != nil {
		t.Fatal(err)
	}
	protocols := f.list(t).Models[0].Protocols
	if len(protocols) != 2 || protocols[0].NoEndpoint || !protocols[1].NoEndpoint {
		t.Fatalf("model list protocols = %+v", protocols)
	}
}
