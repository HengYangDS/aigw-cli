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
		if got := ClosestMatches(test.typed, providers); !slices.Equal(got, test.want) {
			t.Errorf("ClosestMatches(%q) = %q, want %q", test.typed, got, test.want)
		}
	}
	// The closest come first and at most three are named.
	got := ClosestMatches("gpt-6", []string{"gpt-6.1-sol", "gpt-5", "gpt-6", "gpt-6-mini", "gpt-6.2"})
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
	key, model, err := ResolveModel(cfg, parse("ucloud/MiniMax-M3"))
	if err != nil || key != "ucloud-minimax" || model.Family != "minimax-m3" {
		t.Fatalf("a unique match = %q %#v, %v", key, model, err)
	}
	for text, want := range map[string]struct{ problem, fix string }{
		"ucloud/minimax-m3": {"unknown model ucloud/minimax-m3; did you mean ucloud/MiniMax-M3?", "aigw model list --provider ucloud"},
		"ucloud/gpt-6.1-sl": {"unknown model ucloud/gpt-6.1-sl; did you mean ucloud/gpt-6.1-sol?", "aigw model list --provider ucloud"},
		"ucloud/zzz":        {"unknown model ucloud/zzz", "aigw model list --provider ucloud"},
		"dmxap/gpt-6.1-sol": {`unknown provider "dmxap"; did you mean dmxapi?`, "aigw model list"},
	} {
		_, _, err := ResolveModel(cfg, parse(text))
		if problem := terminal.ProblemFor(err); problem.Title != want.problem || problem.Fix != want.fix {
			t.Errorf("ResolveModel(%s) = %q, fix %q; want %q, fix %q", text, problem.Title, problem.Fix, want.problem, want.fix)
		}
	}
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
	if problem := terminal.ProblemFor(err); problem.Title != `unknown client "codx"; did you mean codex? Clients are claude, claude-desktop, codex, or hermes` || problem.Fix != "aigw status" {
		t.Fatalf("an unknown client = %q, fix %q", problem.Title, problem.Fix)
	}
	_, err = ResolveClient(admission, "zzzzzz")
	if problem := terminal.ProblemFor(err); problem.Title != `unknown client "zzzzzz"; clients are claude, claude-desktop, codex, or hermes` {
		t.Fatalf("a client without a close match = %q", problem.Title)
	}
}
