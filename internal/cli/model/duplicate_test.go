package model

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

// duplicateFixture gives the name ucloud/sol two Routes, one on responses and
// one on chat, as today's configuration can until task 6.1's import merges
// them.
func duplicateFixture(t *testing.T) fixture {
	t.Helper()
	f := newFixture(t)
	cfg := f.load(t)
	cfg.Providers["ucloud"] = domain.Provider{Label: "UCloud", Endpoints: domain.Endpoints{
		Responses: "https://ucloud.test/v1", Chat: "https://ucloud.test/chat/v1",
	}}
	cfg.Models["ucloud-sol-chat"] = domain.Model{Provider: "ucloud", Family: "sol", UpstreamModel: "sol", Label: "Sol",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolChat: {domain.CapabilityText}}}
	cfg.Models["ucloud-sol-responses"] = domain.Model{Provider: "ucloud", Family: "sol", UpstreamModel: "sol",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}}
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestListShowsANameWithSeveralModelsOnce(t *testing.T) {
	f := duplicateFixture(t)
	if err := f.run("list", "--provider", "ucloud", "--json"); err != nil {
		t.Fatal(err)
	}
	models := f.list(t).Models
	if len(models) != 1 {
		t.Fatalf("listed models = %+v", models)
	}
	item := models[0]
	var protocols []string
	for _, protocol := range item.Protocols {
		protocols = append(protocols, protocol.Protocol)
	}
	if item.ID != "ucloud/sol" || !item.Duplicate || !slices.Equal(protocols, []string{"responses", "chat"}) || item.Protocols[1].Capabilities[0] != "text" {
		t.Fatalf("merged model = %+v", item)
	}
	// The first model in display order, on responses, gives the label.
	if err := f.run("list", "--provider", "ucloud"); err != nil || !strings.Contains(f.text(), "ucloud/sol UCloud · sol · OpenAI Responses, OpenAI Chat Completions · one model per protocol") {
		t.Fatalf("listed text = %v:\n%s", err, f.out)
	}
	if err := f.run("list", "--protocol", "chat", "--provider", "ucloud", "--json"); err != nil || len(f.list(t).Models) != 1 {
		t.Fatalf("a protocol filter on a merged model = %v:\n%s", err, f.out)
	}
}

func TestShowEditAndRemoveRefuseANameWithSeveralModels(t *testing.T) {
	f := duplicateFixture(t)
	for args, fix := range map[string]string{
		"show ucloud/sol":   "model show ucloud/sol --protocol <responses|chat>",
		"remove ucloud/sol": "model remove ucloud/sol --protocol <responses|chat>",
		// edit's --protocol replaces the protocols, so it cannot select one;
		// show tells the models apart.
		"edit ucloud/sol --label x": "aigw model show ucloud/sol --protocol <responses|chat>",
	} {
		p := problem(f.run(strings.Fields(args)...))
		if p.Title != "ucloud/sol names 2 models; each of the protocols responses, chat selects one" || p.Fix != fix {
			t.Errorf("model %s = %q, fix %q; want fix %q", args, p.Title, p.Fix, fix)
		}
	}
	if err := f.run("show", "ucloud/sol", "--protocol", "chat", "--json"); err != nil || f.document(t).Label != "Sol" || len(f.document(t).Protocols) != 1 {
		t.Fatalf("show with a protocol = %v:\n%s", err, f.out)
	}
	if err := f.run("remove", "ucloud/sol", "--protocol", "responses"); err != nil {
		t.Fatal(err)
	}
	cfg := f.load(t)
	if _, kept := cfg.Models["ucloud-sol-chat"]; !kept || len(cfg.ResolveModelRef(domain.NewModelRef("ucloud", "sol"))) != 1 {
		t.Fatalf("removal with a protocol removed the wrong model: %v", cfg.Models)
	}
}

// The last model may go: its provider stays, without models, and no client
// selects anything.
func TestRemoveTakesTheLastModel(t *testing.T) {
	f := newFixture(t)
	cfg := f.load(t)
	delete(cfg.Models, "mixed-chat")
	delete(cfg.Recommendations, "hermes")
	delete(cfg.Clients, "claude")
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(f.runtime.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.run("remove", "team/team-model", "--dry-run", "--json"); err != nil {
		t.Fatal(err)
	}
	if after, err := os.ReadFile(f.runtime.Config.Path()); err != nil || !bytes.Equal(after, before) {
		t.Fatalf("a preview changed the configuration: %v", err)
	}
	if err := f.run("remove", "team/team-model"); err != nil {
		t.Fatal(err)
	}
	if cfg := f.load(t); len(cfg.Models) != 0 || len(cfg.Providers) != 2 {
		t.Fatalf("after the last model = %+v", cfg)
	}
}

func TestRemoveNamesEveryClientThatUsesTheModel(t *testing.T) {
	f := newFixture(t)
	cfg := f.load(t)
	cfg.SetSelectedModel("claude-desktop", "team", "")
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	p := problem(f.run("remove", "team/team-model"))
	if p.Title != "Claude, Claude Desktop use team/team-model, so it cannot be removed" || p.Fix != "aigw use claude <provider/model>" {
		t.Fatalf("a model that two clients use = %q, fix %q", p.Title, p.Fix)
	}
}

// TestTheCatalogFixNamesAnAddedCloseMatch makes the closest listed ID an
// added model, which the typo most likely meant: the refusal says that it is
// added and shows it, and never adds the typo.
func TestTheCatalogFixNamesAnAddedCloseMatch(t *testing.T) {
	f := newFixture(t)
	f.catalog.ids = []string{"team-model", "team-model-2"}
	p := problem(f.run("add", "team/team-modl", "--json"))
	if p.Title != "the team catalog does not list team-modl; team/team-model is already added" || p.Fix != "aigw model show team/team-model" {
		t.Fatalf("a close match that is added = %q, fix %q", p.Title, p.Fix)
	}
	// The closest listed ID that is not added is offered as before.
	f.catalog.ids = []string{"team-model", "team-model-two"}
	if p := problem(f.run("add", "team/team-model-tw")); p.Fix != "model add team/team-model-two" {
		t.Fatalf("a close match that is not added = %q", p.Fix)
	}
}

func TestAddTakesOneProtocol(t *testing.T) {
	f := newFixture(t)
	err := f.run("add", "mixed/new", "--protocol", "chat", "--protocol", "anthropic", "--unlisted")
	if err == nil || err.Error() != `invalid argument "anthropic" for "--protocol" flag: this flag takes one protocol` {
		t.Fatalf("a repeated protocol = %v", err)
	}
	if err := f.run("add"); err == nil || err.Error() != "model add needs a <provider/model> argument; run `model add --help`" {
		t.Fatalf("a missing argument = %v", err)
	}
	if err := f.run("show", "a/b", "c/d"); err == nil || err.Error() != "model show takes one <provider/model> argument, not 2; run `model show --help`" {
		t.Fatalf("two arguments = %v", err)
	}
}

// TestAProtocolFilterNeverSelectsAModelWithoutIt replays the two removals
// that once took the wrong model: a --protocol that a model does not declare
// refuses, whether the name has one model left or never had more.
func TestAProtocolFilterNeverSelectsAModelWithoutIt(t *testing.T) {
	f := newFixture(t)
	cfg := f.load(t)
	cfg.Models["mixed-chat-messages"] = domain.Model{Provider: "mixed", Family: "chat-only", UpstreamModel: "Chat-Only",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Models["mixed-solo"] = domain.Model{Provider: "mixed", Family: "solo", UpstreamModel: "solo-model",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	delete(cfg.Recommendations, "hermes")
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	// The first removal takes the chat model; the second finds only the
	// anthropic one, which chat does not select.
	if err := f.run("remove", "mixed/Chat-Only", "--protocol", "chat"); err != nil {
		t.Fatal(err)
	}
	p := problem(f.run("remove", "mixed/Chat-Only", "--protocol", "chat", "--dry-run", "--json"))
	if p.Title != "mixed/Chat-Only declares no OpenAI Chat Completions" || p.Fix != "aigw model show mixed/Chat-Only" {
		t.Fatalf("a second removal with chat = %q, fix %q", p.Title, p.Fix)
	}
	p = problem(f.run("remove", "mixed/solo-model", "--protocol", "chat"))
	if p.Title != "mixed/solo-model declares no OpenAI Chat Completions" {
		t.Fatalf("removing an anthropic-only model with chat = %q", p.Title)
	}
	if p := problem(f.run("show", "mixed/solo-model", "--protocol", "chat")); p.Title != "mixed/solo-model declares no OpenAI Chat Completions" {
		t.Fatalf("show with a protocol that the model lacks = %q", p.Title)
	}
	if _, kept := f.load(t).Models["mixed-chat-messages"]; !kept {
		t.Fatal("a removal took a model that does not declare its protocol")
	}
	if _, kept := f.load(t).Models["mixed-solo"]; !kept {
		t.Fatal("a removal took the anthropic-only model")
	}
}

// TestFixesThatWouldNeedQuotingAreProse gives the words of two fixes a
// space: a rebuilt command line would need quoting, so each fix is prose.
func TestFixesThatWouldNeedQuotingAreProse(t *testing.T) {
	f := newFixture(t)
	p := problem(f.run("add", "team/new", "--protocol", "responses", "--label", "two words", "--unlisted"))
	if p.Fix != "Run the same command with --protocol anthropic." {
		t.Fatalf("a missing-endpoint fix with a quoted word = %q", p.Fix)
	}
	cfg := f.load(t)
	cfg.Models["team-spaced"] = domain.Model{Provider: "team", Family: "spaced", UpstreamModel: "team model",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	f.catalog.ids = []string{"team model"}
	if p := problem(f.run("add", "team/team mode")); p.Fix != "Run `aigw model show` for team/team model." {
		t.Fatalf("an added close match whose name needs quoting = %q", p.Fix)
	}
}
