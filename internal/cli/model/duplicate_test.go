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
		OpenAIResponses: "https://ucloud.test/v1", OpenAIChatCompletions: "https://ucloud.test/chat/v1",
	}}
	cfg.Models["ucloud-sol-chat"] = domain.Model{Provider: "ucloud", Family: "sol", UpstreamModel: "sol", Label: "Sol",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIChatCompletions: {domain.CapabilityText}}}
	cfg.Models["ucloud-sol-responses"] = domain.Model{Provider: "ucloud", Family: "sol", UpstreamModel: "sol",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
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
		"show ucloud/sol":   "model show ucloud/sol --protocol <chat|responses>",
		"remove ucloud/sol": "model remove ucloud/sol --protocol <chat|responses>",
		// edit's --protocol replaces the protocols, so it cannot select one.
		"edit ucloud/sol --label x": "aigw route show ucloud-sol-chat",
	} {
		p := problem(f.run(strings.Fields(args)...))
		if p.Title != "ucloud/sol names 2 models, one for each of the protocols chat, responses" || p.Fix != fix {
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

func TestRemoveRefusesTheLastModel(t *testing.T) {
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
	for _, args := range [][]string{{"remove", "team/team-model"}, {"remove", "team/team-model", "--dry-run", "--json"}} {
		p := problem(f.run(args...))
		if p.Title != "team/team-model is the only model, and aigw needs at least one" || p.Fix != "aigw model add <provider/model>" {
			t.Fatalf("model %q = %q, fix %q", args, p.Title, p.Fix)
		}
	}
	if after, err := os.ReadFile(f.runtime.Config.Path()); err != nil || !bytes.Equal(after, before) {
		t.Fatalf("a refused removal changed the configuration: %v", err)
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

// TestTheCatalogFixSkipsAddedModels makes the closest listed ID an added
// model: the fix adds the closest one that is not added yet.
func TestTheCatalogFixSkipsAddedModels(t *testing.T) {
	f := newFixture(t)
	f.catalog.ids = []string{"team-model", "team-model-2"}
	p := problem(f.run("add", "team/team-modl"))
	if p.Title != "the team catalog does not list team-modl; did you mean team/team-model-2?" || p.Fix != "model add team/team-model-2" {
		t.Fatalf("a close match that is added = %q, fix %q", p.Title, p.Fix)
	}
	f.catalog.ids = []string{"team-model"}
	if p := problem(f.run("add", "team/team-modl")); p.Fix != "model add team/team-modl --unlisted" {
		t.Fatalf("a catalog whose only close match is added = %q", p.Fix)
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
