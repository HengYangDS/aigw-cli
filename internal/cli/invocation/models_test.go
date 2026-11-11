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
	responses := map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}
	cfg.Providers["dmxapi"] = domain.Provider{Label: "DMXAPI", Endpoints: domain.Endpoints{Responses: "https://dmxapi.test/v1"}}
	cfg.Providers["ucloud"] = domain.Provider{Label: "UCloud", Endpoints: domain.Endpoints{Responses: "https://ucloud.test/v1"}}
	cfg.Models[domain.NewModelRef("dmxapi", "gpt-6.1-sol")] = domain.Model{Provider: "dmxapi", Family: "gpt-6.1-sol", ID: "gpt-6.1-sol", Protocols: responses}
	cfg.Models[domain.NewModelRef("ucloud", "gpt-6.1-sol")] = domain.Model{Provider: "ucloud", Family: "gpt-6.1-sol", ID: "gpt-6.1-sol", Protocols: responses}
	cfg.Models[domain.NewModelRef("ucloud", "MiniMax-M3")] = domain.Model{Provider: "ucloud", Family: "minimax-m3", ID: "MiniMax-M3", Protocols: responses}
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
	model, err := ResolveModel(cfg, parse("ucloud/MiniMax-M3"), "")
	if err != nil || model.Ref() != domain.NewModelRef("ucloud", "MiniMax-M3") || model.Family != "minimax-m3" {
		t.Fatalf("a match = %#v, %v", model, err)
	}
	for text, want := range map[string]struct{ problem, fix string }{
		"ucloud/minimax-m3": {"unknown model ucloud/minimax-m3; did you mean ucloud/MiniMax-M3?", "aigw model list --provider ucloud"},
		"ucloud/gpt-6.1-sl": {"unknown model ucloud/gpt-6.1-sl; did you mean ucloud/gpt-6.1-sol?", "aigw model list --provider ucloud"},
		"ucloud/zzz":        {"unknown model ucloud/zzz", "aigw model list --provider ucloud"},
		"dmxap/gpt-6.1-sol": {`unknown provider "dmxap"; did you mean dmxapi?`, "aigw provider list"},
	} {
		_, err := ResolveModel(cfg, parse(text), "")
		if problem := terminal.ProblemFor(err); problem.Title != want.problem || problem.Fix != want.fix {
			t.Errorf("ResolveModel(%s) = %q, fix %q; want %q, fix %q", text, problem.Title, problem.Fix, want.problem, want.fix)
		}
	}
}

// TestResolveModelRefusesAProtocolThatTheModelDoesNotDeclare: a given
// protocol always selects, so a model that does not declare it is refused,
// never selected anyway.
func TestResolveModelRefusesAProtocolThatTheModelDoesNotDeclare(t *testing.T) {
	cfg := modelConfig()
	minimax := must(domain.ParseModelRef("ucloud/MiniMax-M3"))
	_, err := ResolveModel(cfg, minimax, domain.ProtocolAnthropic)
	if problem := terminal.ProblemFor(err); problem.Title != "ucloud/MiniMax-M3 declares no Anthropic Messages" || problem.Fix != "aigw model show ucloud/MiniMax-M3" {
		t.Fatalf("a model with another protocol = %q, fix %q", problem.Title, problem.Fix)
	}
	if model, err := ResolveModel(cfg, minimax, domain.ProtocolResponses); err != nil || model.Ref() != minimax {
		t.Fatalf("a model with its protocol = %#v, %v", model, err)
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
	if got := Command("model", "show", "two words"); got != "" {
		t.Fatalf("Command with a word that needs quoting = %q", got)
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
