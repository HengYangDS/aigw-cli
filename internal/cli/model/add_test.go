package model

import (
	"bytes"
	"net/http"
	"os"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

func TestAddChecksTheCatalogAndStoresAKeyThatOutputNeverShows(t *testing.T) {
	f := newFixture(t)
	if err := f.run("add", "team/spare-model", "--label", "Spare", "--json"); err != nil {
		t.Fatal(err)
	}
	document := f.document(t)
	if document.ID != "team/spare-model" || document.CatalogChecked == nil || !*document.CatalogChecked || document.Label != "Spare" || f.catalog.requests != 1 {
		t.Fatalf("added model = %v after %d requests", document, f.catalog.requests)
	}
	cfg := f.load(t)
	ref, _ := domain.ParseModelRef("team/spare-model")
	keys := cfg.ResolveModelRef(ref)
	if len(keys) != 1 || keys[0] != domain.ModelKey(ref) || strings.Contains(f.out.String(), keys[0]) {
		t.Fatalf("stored keys %q, in output %t", keys, len(keys) > 0 && strings.Contains(f.out.String(), keys[0]))
	}
	key := keys[0]
	if model := cfg.Models[key]; model.Family != "spare-model" || model.UpstreamModelID() != "spare-model" || len(model.Protocols[domain.ProtocolAnthropic]) != 0 {
		t.Fatalf("added model = %#v", model)
	}
	if cfg.SelectedModel("claude") != "team" || cfg.SelectedModel("hermes") != "" {
		t.Fatalf("adding a model changed a selection: %#v", cfg.Clients)
	}
}

func TestAddRefusesWhatTheCatalogDoesNotList(t *testing.T) {
	f := newFixture(t)
	err := f.run("add", "team/spare-mdl")
	// The fix adds the listed model that the user most likely meant.
	if p := problem(err); p.Title != "the team catalog does not list spare-mdl; did you mean team/spare-model?" || p.Fix != "model add team/spare-model" {
		t.Fatalf("an unlisted model = %q, fix %q", p.Title, p.Fix)
	}
	// Without a close listed ID, --unlisted adds the ID as typed, after the
	// user confirms it, so the fix is prose that JSON carries as next_detail.
	if p := problem(f.run("add", "team/zzz")); p.Title != "the team catalog does not list zzz" ||
		p.Fix != "If the provider serves this exact model ID, run `model add team/zzz --unlisted`." {
		t.Fatalf("an unlisted model without a close match = %q, fix %q", p.Title, p.Fix)
	}
	// A word that would need quoting leaves the fix in prose.
	if p := problem(f.run("add", "team/spare-mdl", "--label", "two words")); p.Fix != "If the provider serves this exact model ID, run the same command with --unlisted." {
		t.Fatalf("a close-match fix that would need quoting = %q", p.Fix)
	}
	f.catalog.ids = nil
	if p := problem(f.run("add", "team/spare-model", "--json")); p.Title != "the team catalog lists no model, so AIGW cannot check that it serves team/spare-model" || p.Fix != "model add team/spare-model --unlisted" {
		t.Fatalf("an empty catalog = %q, fix %q", p.Title, p.Fix)
	}
	f.catalog.status = http.StatusInternalServerError
	p := problem(f.run("add", "team/spare-model"))
	if p.Title != "the team catalog cannot be read, so AIGW cannot check that it serves team/spare-model" ||
		p.Evidence != "Anthropic Messages: Catalog request failed; configuration was not changed" || p.Fix != "model add team/spare-model --unlisted" {
		t.Fatalf("an unreadable catalog = %+v", p)
	}
	// The mixed provider has no Token, so no catalog of its can be read.
	p = problem(f.run("add", "mixed/new", "--protocol", "chat"))
	if p.Evidence != "Anthropic Messages: Token unavailable; OpenAI Chat Completions: Token unavailable" {
		t.Fatalf("a provider without a Token = %+v", p)
	}
	// A refused word keeps the fix in prose.
	if p := problem(f.run("add", "team/two words")); p.Fix != "Run the same command with --unlisted." {
		t.Fatalf("a fix that would need quoting = %q", p.Fix)
	}
	if cfg := f.load(t); len(cfg.Models) != 2 {
		t.Fatalf("a refused addition wrote a model: %v", cfg.Models)
	}
}

func TestAddWithoutTheCatalogCheck(t *testing.T) {
	f := newFixture(t)
	if err := f.run("add", "team/openai/gpt-oss", "--unlisted"); err != nil {
		t.Fatal(err)
	}
	if f.catalog.requests != 0 || !strings.Contains(f.text(), "! Catalog Not checked: --unlisted") || !strings.Contains(f.text(), "Next aigw use <client> team/openai/gpt-oss") {
		t.Fatalf("an unlisted addition sent %d requests:\n%s", f.catalog.requests, f.out)
	}
	model := f.load(t).Models[domain.ModelKey(must(domain.ParseModelRef("team/openai/gpt-oss")))]
	// A model ID that is not an identifier forms a family with a derived ID.
	if model.UpstreamModelID() != "openai/gpt-oss" || model.Family != domain.FamilyID("openai/gpt-oss") || !domain.ValidIdentifier(model.Family) {
		t.Fatalf("a model ID with a slash = %#v", model)
	}
}

func TestAddPreviewReadsNoCatalogAndWritesNothing(t *testing.T) {
	f := newFixture(t)
	before, err := os.ReadFile(f.runtime.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.run("add", "team/spare-model", "--family", "brand-new", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Model added (preview)", "· Catalog Not checked in a preview", "New family brand-new", "Nothing was written", "Next model add team/spare-model --family brand-new"} {
		if !strings.Contains(f.text(), want) {
			t.Fatalf("add preview lacks %q:\n%s", want, f.out)
		}
	}
	// A preview reads no catalog, and says so.
	if err := f.run("add", "team/spare-model", "--dry-run", "--json"); err != nil || !f.document(t).DryRun || f.document(t).CatalogChecked == nil || *f.document(t).CatalogChecked {
		t.Fatalf("add preview document = %v:\n%s", err, f.out)
	}
	after, err := os.ReadFile(f.runtime.Config.Path())
	if err != nil || !bytes.Equal(after, before) || f.catalog.requests != 0 {
		t.Fatalf("a preview wrote configuration or read the catalog %d times: %v", f.catalog.requests, err)
	}
}

func TestAddTakesTheProtocolFromTheProvidersEndpoints(t *testing.T) {
	f := newFixture(t)
	// The mixed provider has two endpoints, so --protocol is needed.
	p := problem(f.run("add", "mixed/new", "--unlisted"))
	if p.Title != "provider mixed serves Anthropic Messages and OpenAI Chat Completions, so mixed/new needs --protocol" || p.Fix != "model add mixed/new --unlisted --protocol <anthropic|chat>" {
		t.Fatalf("a provider with several endpoints = %q, fix %q", p.Title, p.Fix)
	}
	if p := problem(f.run("add", "mixed/two words", "--unlisted")); p.Fix != "Run the same command with --protocol <anthropic|chat>." {
		t.Fatalf("a protocol fix that would need quoting = %q", p.Fix)
	}
	p = problem(f.run("add", "team/new", "--protocol", "responses", "--unlisted"))
	if p.Title != "team/new cannot declare OpenAI Responses: provider team has no OpenAI Responses endpoint" ||
		p.Evidence != "Provider team serves Anthropic Messages." || p.Fix != "model add team/new --unlisted --protocol anthropic" {
		t.Fatalf("a protocol that the provider lacks = %+v", p)
	}
	if err := f.run("add", "mixed/new", "--protocol", "chat", "--unlisted", "--json"); err != nil {
		t.Fatal(err)
	}
	if protocols := f.document(t).Protocols; len(protocols) != 1 || protocols[0].Protocol != "chat" {
		t.Fatalf("the model's protocols = %v", protocols)
	}
}

func TestAddFollowsTheFamilyRule(t *testing.T) {
	f := newFixture(t)
	cfg := f.load(t)
	cfg.Families["minimax-m3"] = domain.Family{Label: "MiniMax M3"}
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	// An existing family whose ID equals the model ID ignoring case.
	if err := f.run("add", "team/MiniMax-M3", "--unlisted", "--json"); err != nil || f.document(t).Family != "minimax-m3" || f.document(t).FamilyCreated != "" {
		t.Fatalf("a model with a family's ID = %v:\n%s", err, f.out)
	}
	// A variant does not join by its prefix.
	if err := f.run("add", "team/team-model-cdx", "--unlisted", "--json"); err != nil || f.document(t).Family != "team-model-cdx" {
		t.Fatalf("a variant = %v:\n%s", err, f.out)
	}
	// An unknown --family is created, labelled from the model; a known one is joined.
	if err := f.run("add", "team/variant", "--family", "team-family", "--unlisted", "--json"); err != nil || f.document(t).FamilyCreated != "team-family" {
		t.Fatalf("an unknown family = %v:\n%s", err, f.out)
	}
	if family := f.load(t).Families["team-family"]; family.Label != "variant" {
		t.Fatalf("created family = %#v", family)
	}
	if err := f.run("add", "team/variant-2", "--family", "team-model", "--unlisted", "--json"); err != nil || f.document(t).Family != "team-model" || f.document(t).FamilyCreated != "" {
		t.Fatalf("a known family = %v:\n%s", err, f.out)
	}
}

func TestAddDeclaresCapabilitiesOnTheProtocolsThatAdmitThem(t *testing.T) {
	f := newFixture(t)
	if err := f.run("add", "mixed/tools", "--protocol", "anthropic", "--capability", "tool_search", "--capability", "text", "--capability", "text", "--unlisted"); err != nil {
		t.Fatal(err)
	}
	model := f.load(t).Models[domain.ModelKey(must(domain.ParseModelRef("mixed/tools")))]
	if got := model.Protocols[domain.ProtocolAnthropic]; len(got) != 2 || got[0] != domain.CapabilityText || got[1] != domain.CapabilityToolSearch {
		t.Fatalf("declared capabilities = %v", got)
	}
	if !strings.Contains(f.text(), "Protocols Anthropic Messages (text, tool_search)") {
		t.Fatalf("capabilities in output:\n%s", f.out)
	}
	p := problem(f.run("add", "mixed/chat-tools", "--protocol", "chat", "--capability", "tool_search", "--unlisted"))
	// The fix declares the protocol that admits the capability instead.
	if p.Title != "no protocol of mixed/chat-tools admits tool_search; it declares OpenAI Chat Completions" ||
		p.Fix != "model add mixed/chat-tools --capability tool_search --unlisted --protocol anthropic" {
		t.Fatalf("a capability that no protocol admits = %q, fix %q", p.Title, p.Fix)
	}
}

func TestAddRefusesBadArgumentsBeforeConfigurationAccess(t *testing.T) {
	f := newFixture(t)
	if err := os.WriteFile(f.runtime.Config.Path(), []byte("not [ toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "team/x", "--family", "bad family"},
		{"add", "team/x", "--label", " "},
		{"add", "team/x", "--capability", "telepathy"},
		{"add", "team/x", "--capability", "text", "--capability", ""},
		{"add", "/x"},
	} {
		err := f.run(args...)
		if err == nil || !strings.Contains(err.Error(), "run `model add --help`") {
			t.Errorf("model %q = %v, want a refusal that names the help", args, err)
		}
	}
}

func TestAddRefusesAnExistingModelOrAnUnknownProvider(t *testing.T) {
	f := newFixture(t)
	if p := problem(f.run("add", "team/team-model", "--unlisted")); p.Title != "team/team-model is already added; `aigw model edit` changes it" || p.Fix != "aigw model edit team/team-model --help" {
		t.Fatalf("an existing model = %q, fix %q", p.Title, p.Fix)
	}
	if p := problem(f.run("add", "temm/x", "--unlisted")); p.Title != `unknown provider "temm"; did you mean team?` {
		t.Fatalf("an unknown provider = %q", p.Title)
	}
	if p := problem(f.run("add", "spare-model")); p.Title != `"spare-model" names no provider; a model is named provider/model` {
		t.Fatalf("an argument without a slash = %q", p.Title)
	}
}

func TestModelKeyAndFamilyIDAreValidIdentifiers(t *testing.T) {
	for _, text := range []string{"team/spare-model", "team/openai/gpt-oss", "team/" + strings.Repeat("long-", 30), "team/模型", "team/a b", "a/....."} {
		ref := must(domain.ParseModelRef(text))
		key := domain.ModelKey(ref)
		if !domain.ValidIdentifier(key) || key != domain.ModelKey(ref) {
			t.Errorf("ModelKey(%s) = %q, not a stable identifier", text, key)
		}
		if family := domain.FamilyID(ref.ID()); !domain.ValidIdentifier(family) {
			t.Errorf("FamilyID(%q) = %q, not an identifier", ref.ID(), family)
		}
	}
	if domain.ModelKey(must(domain.ParseModelRef("team/a.b"))) == domain.ModelKey(must(domain.ParseModelRef("team/a/b"))) {
		t.Fatal("two references share a key")
	}
	if got := domain.FamilyID("gpt-6.1-sol"); got != "gpt-6.1-sol" {
		t.Fatalf("FamilyID of an identifier = %q", got)
	}
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
