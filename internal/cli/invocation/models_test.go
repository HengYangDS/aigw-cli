package invocation

import (
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"
)

func TestClosestMatchesNamesWhatLooksLikeTheInput(t *testing.T) {
	providers := []string{"dmxapi", "ucloud", "aihubmix"}
	for _, test := range []struct {
		typed string
		want  []string
	}{
		{"dmxap", []string{"dmxapi"}},
		{"DMXAPI", []string{"dmxapi"}},
		{"ucloude", []string{"ucloud"}},
		{"hub", []string{"aihubmix"}},
		{"zzz", []string{}},
	} {
		if got := closestMatches(test.typed, providers); !slices.Equal(got, test.want) {
			t.Errorf("closestMatches(%q) = %q, want %q", test.typed, got, test.want)
		}
	}
	// The closest come first and at most three are named.
	got := closestMatches("gpt-6", []string{"gpt-6.1-sol", "gpt-5", "gpt-6", "gpt-6-mini", "gpt-6.2"})
	if !slices.Equal(got, []string{"gpt-6", "gpt-5", "gpt-6.2"}) {
		t.Fatalf("ranked matches = %q", got)
	}
	if editDistance("kitten", "sitting") != 3 || editDistance("", "abc") != 3 || editDistance("über", "uber") != 1 {
		t.Fatal("edit distance is not the Levenshtein distance in runes")
	}
}

// modelConfig holds two providers that both serve gpt-6.1-sol, and a UCloud
// model whose ID differs from its key only in case.
func modelConfig() domain.Config {
	cfg := domain.NewConfig(clients.Admission())
	responses := map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}
	cfg.Providers["dmxapi"] = domain.Provider{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmxapi.test/v1"}}
	cfg.Providers["ucloud"] = domain.Provider{Label: "UCloud", Endpoints: domain.Endpoints{OpenAIResponses: "https://ucloud.test/v1"}}
	cfg.Models["dmxapi-sol"] = domain.Model{Provider: "dmxapi", Family: "gpt-6.1-sol", Protocols: responses}
	cfg.Models["ucloud-sol"] = domain.Model{Provider: "ucloud", Family: "gpt-6.1-sol", Protocols: responses}
	cfg.Models["ucloud-minimax"] = domain.Model{Provider: "ucloud", Family: "minimax-m3", UpstreamModel: "MiniMax-M3", Protocols: responses}
	return cfg
}

func TestResolveModelNamesTheClosestMatches(t *testing.T) {
	cfg := modelConfig()
	parse := func(text string) domain.ModelRef {
		ref, err := ParseModelArgument(cfg, text)
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}
	key, model, err := ResolveModel(cfg, parse("ucloud/MiniMax-M3"), "", nil)
	if err != nil || key != "ucloud-minimax" || model.Family != "minimax-m3" {
		t.Fatalf("a unique match = %q %#v, %v", key, model, err)
	}
	for text, want := range map[string]struct{ problem, fix string }{
		"ucloud/minimax-m3": {"unknown model ucloud/minimax-m3; did you mean ucloud/MiniMax-M3?", "aigw model list --provider ucloud"},
		"ucloud/gpt-6.1-sl": {"unknown model ucloud/gpt-6.1-sl; did you mean ucloud/gpt-6.1-sol?", "aigw model list --provider ucloud"},
		"ucloud/zzz":        {"unknown model ucloud/zzz", "aigw model list --provider ucloud"},
		"dmxap/gpt-6.1-sol": {`unknown provider "dmxap"; did you mean dmxapi?`, "aigw provider list"},
	} {
		_, _, err := ResolveModel(cfg, parse(text), "", nil)
		if problem := terminal.ProblemFor(err); problem.Title != want.problem || problem.Fix != want.fix {
			t.Errorf("ResolveModel(%s) = %q, fix %q; want %q, fix %q", text, problem.Title, problem.Fix, want.problem, want.fix)
		}
	}
}

// TestResolveModelRefusesANameThatSeveralModelsCarry holds today's
// configuration, which can give one name a Route per protocol: a protocol
// selects one, and without one the name is refused, naming each protocol, with
// the command rerun with --protocol, or the first Route without that command.
func TestResolveModelRefusesANameThatSeveralModelsCarry(t *testing.T) {
	cfg := modelConfig()
	cfg.Providers["ucloud"] = domain.Provider{Label: "UCloud", Endpoints: domain.Endpoints{OpenAIResponses: "https://ucloud.test/v1", OpenAIChatCompletions: "https://ucloud.test/v1"}}
	cfg.Models["ucloud-sol-chat"] = domain.Model{Provider: "ucloud", Family: "gpt-6.1-sol", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIChatCompletions: {}}}
	ref, _ := domain.ParseModelRef("ucloud/gpt-6.1-sol")
	if keys := cfg.ResolveModelRef(ref); !slices.Equal(keys, []string{"ucloud-sol", "ucloud-sol-chat"}) {
		t.Fatalf("ResolveModelRef = %q", keys)
	}
	rerun := func(choice string) string { return "aigw model show ucloud/gpt-6.1-sol --protocol " + choice }
	_, _, err := ResolveModel(cfg, ref, "", rerun)
	if problem := terminal.ProblemFor(err); problem.Title != "ucloud/gpt-6.1-sol names 2 models; each of the protocols responses, chat selects one" ||
		problem.Fix != "aigw model show ucloud/gpt-6.1-sol --protocol <responses|chat>" {
		t.Fatalf("an ambiguous name = %q, fix %q", problem.Title, problem.Fix)
	}
	// A command without a protocol filter names show, which has one.
	if _, _, err := ResolveModel(cfg, ref, "", nil); terminal.ProblemFor(err).Fix != "aigw model show ucloud/gpt-6.1-sol --protocol <responses|chat>" {
		t.Fatalf("an ambiguous name without a protocol filter = %q", terminal.ProblemFor(err).Fix)
	}
	if _, _, err := ResolveModel(cfg, ref, "", func(string) string { return "" }); terminal.ProblemFor(err).Fix != "Run the command for one of them with --protocol <responses|chat>." {
		t.Fatalf("an ambiguous name whose rerun needs quoting = %q", terminal.ProblemFor(err).Fix)
	}
	if key, _, err := ResolveModel(cfg, ref, domain.ProtocolOpenAIChatCompletions, rerun); err != nil || key != "ucloud-sol-chat" {
		t.Fatalf("a protocol that selects one = %q, %v", key, err)
	}
	_, _, err = ResolveModel(cfg, ref, domain.ProtocolAnthropic, rerun)
	if problem := terminal.ProblemFor(err); problem.Title != "ucloud/gpt-6.1-sol declares no Anthropic Messages" || problem.Fix != "aigw model list --provider ucloud" {
		t.Fatalf("a protocol that selects none of several = %q, fix %q", problem.Title, problem.Fix)
	}
	// A given protocol also applies to a unique name: a model that does not
	// declare it is refused, never selected anyway.
	minimax := must(domain.ParseModelRef("ucloud/MiniMax-M3"))
	_, _, err = ResolveModel(cfg, minimax, domain.ProtocolAnthropic, rerun)
	if problem := terminal.ProblemFor(err); problem.Title != "ucloud/MiniMax-M3 declares no Anthropic Messages" || problem.Fix != "aigw model show ucloud/MiniMax-M3" {
		t.Fatalf("a unique name with another protocol = %q, fix %q", problem.Title, problem.Fix)
	}
	if key, _, err := ResolveModel(cfg, minimax, domain.ProtocolOpenAIResponses, rerun); err != nil || key != "ucloud-minimax" {
		t.Fatalf("a unique name with its protocol = %q, %v", key, err)
	}
}

// TestAmbiguousModelOffersOnlyProtocolsThatSelect gives one name a Route on
// anthropic and chat and another on chat: chat selects neither, so the
// refusal offers anthropic only when it selects each, and otherwise names
// the models' configuration IDs.
func TestAmbiguousModelOffersOnlyProtocolsThatSelect(t *testing.T) {
	cfg := modelConfig()
	cfg.Providers["ucloud"] = domain.Provider{Label: "UCloud", Endpoints: domain.Endpoints{
		Anthropic: "https://ucloud.test", OpenAIResponses: "https://ucloud.test/v1", OpenAIChatCompletions: "https://ucloud.test/v1",
	}}
	cfg.Models["ucloud-sol"] = domain.Model{Provider: "ucloud", Family: "gpt-6.1-sol", Protocols: map[domain.EndpointProtocol][]domain.Capability{
		domain.ProtocolAnthropic: {}, domain.ProtocolOpenAIChatCompletions: {},
	}}
	cfg.Models["ucloud-sol-chat"] = domain.Model{Provider: "ucloud", Family: "gpt-6.1-sol", Protocols: map[domain.EndpointProtocol][]domain.Capability{
		domain.ProtocolOpenAIChatCompletions: {},
	}}
	ref := must(domain.ParseModelRef("ucloud/gpt-6.1-sol"))
	rerun := func(choice string) string { return "aigw model show ucloud/gpt-6.1-sol --protocol " + choice }
	_, _, err := ResolveModel(cfg, ref, "", rerun)
	if problem := terminal.ProblemFor(err); problem.Title != "ucloud/gpt-6.1-sol names 2 models, ucloud-sol, ucloud-sol-chat, and no protocol selects each of them" ||
		problem.Fix != "aigw config path" {
		t.Fatalf("overlapping models = %q, fix %q", problem.Title, problem.Fix)
	}
	if key, _, err := ResolveModel(cfg, ref, domain.ProtocolAnthropic, rerun); err != nil || key != "ucloud-sol" {
		t.Fatalf("the protocol that only one declares = %q, %v", key, err)
	}
	// chat names both, so it selects neither.
	if _, _, err := ResolveModel(cfg, ref, domain.ProtocolOpenAIChatCompletions, rerun); !strings.Contains(terminal.ProblemFor(err).Title, "no protocol selects each") {
		t.Fatalf("a protocol that both declare = %v", err)
	}
	// With a responses model added, each of anthropic and responses selects
	// one of two models: chat, shared, is not offered.
	delete(cfg.Models, "ucloud-sol-chat")
	cfg.Models["ucloud-sol-responses"] = domain.Model{Provider: "ucloud", Family: "gpt-6.1-sol", Protocols: map[domain.EndpointProtocol][]domain.Capability{
		domain.ProtocolOpenAIResponses: {}, domain.ProtocolOpenAIChatCompletions: {},
	}}
	_, _, err = ResolveModel(cfg, ref, "", rerun)
	if problem := terminal.ProblemFor(err); problem.Title != "ucloud/gpt-6.1-sol names 2 models; each of the protocols anthropic, responses selects one" ||
		problem.Fix != "aigw model show ucloud/gpt-6.1-sol --protocol <anthropic|responses>" {
		t.Fatalf("models that share chat = %q, fix %q", problem.Title, problem.Fix)
	}
}

// TestModelArgumentWithoutSlashNamesEachCandidateOnce gives one name two
// Routes: the refusal lists the name once.
func TestModelArgumentWithoutSlashNamesEachCandidateOnce(t *testing.T) {
	cfg := modelConfig()
	cfg.Providers["ucloud"] = domain.Provider{Label: "UCloud", Endpoints: domain.Endpoints{OpenAIResponses: "https://ucloud.test/v1", OpenAIChatCompletions: "https://ucloud.test/v1"}}
	cfg.Models["ucloud-sol-chat"] = domain.Model{Provider: "ucloud", Family: "gpt-6.1-sol", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIChatCompletions: {}}}
	_, err := ParseModelArgument(cfg, "gpt-6.1-sol")
	if problem := terminal.ProblemFor(err); problem.Title != `"gpt-6.1-sol" names no provider; it could mean dmxapi/gpt-6.1-sol, ucloud/gpt-6.1-sol` {
		t.Fatalf("a bare ID with two Routes of one name = %q", problem.Title)
	}
	if got := Command("model", "show", "two words"); got != "" {
		t.Fatalf("Command with a word that needs quoting = %q", got)
	}
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

func TestModelArgumentWithoutSlashListsWhatItCouldMean(t *testing.T) {
	cfg := modelConfig()
	_, err := ParseModelArgument(cfg, "gpt-6.1-sol")
	problem := terminal.ProblemFor(err)
	if problem.Title != `"gpt-6.1-sol" names no provider; it could mean dmxapi/gpt-6.1-sol, ucloud/gpt-6.1-sol` || problem.Fix != "aigw model list gpt-6.1-sol" {
		t.Fatalf("an argument without a slash = %q, fix %q", problem.Title, problem.Fix)
	}
	_, err = ParseModelArgument(cfg, "minimax-m3")
	if problem := terminal.ProblemFor(err); !strings.Contains(problem.Title, "ucloud/MiniMax-M3") {
		t.Fatalf("a case-only match was not listed: %q", problem.Title)
	}
	_, err = ParseModelArgument(cfg, "nothing")
	if problem := terminal.ProblemFor(err); problem.Title != `"nothing" names no provider; a model is named provider/model` || problem.Fix != "aigw model list nothing" {
		t.Fatalf("an unknown argument without a slash = %q, fix %q", problem.Title, problem.Fix)
	}
	_, err = ParseModelArgument(cfg, "two words")
	if problem := terminal.ProblemFor(err); problem.Fix != "aigw model list" {
		t.Fatalf("a fix that would need quoting = %q", problem.Fix)
	}
	if _, err := ParseModelArgument(cfg, "/gpt"); err == nil || terminal.StatesFix(err) {
		t.Fatalf("a malformed reference = %v, want the parser's refusal", err)
	}
}

func TestResolveClientNamesTheClosestClients(t *testing.T) {
	admission := clients.Admission()
	if spec, err := ResolveClient(admission, "codex"); err != nil || spec.ID != "codex" {
		t.Fatalf("a known client = %#v, %v", spec, err)
	}
	_, err := ResolveClient(admission, "codx")
	if problem := terminal.ProblemFor(err); problem.Title != `unknown client "codx"; did you mean codex? Clients are claude, claude-desktop, codex, or hermes` || problem.Fix != "aigw client list" {
		t.Fatalf("an unknown client = %q, fix %q", problem.Title, problem.Fix)
	}
	_, err = ResolveClient(admission, "zzzzzz")
	if problem := terminal.ProblemFor(err); problem.Title != `unknown client "zzzzzz"; clients are claude, claude-desktop, codex, or hermes` {
		t.Fatalf("a client without a close match = %q", problem.Title)
	}
}
