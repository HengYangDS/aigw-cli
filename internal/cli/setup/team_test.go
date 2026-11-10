package setup

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
)

// teamFileText is a team file of provider team, whose Anthropic endpoint
// differs from the configured one's, with a balance probe, a capability that
// was qualified on its endpoint, a model on a protocol that the configured
// team has no endpoint for, and a recommendation for Claude Code.
const teamFileText = `version = 7
[recommendations.claude.primary]
route = "team-route"
[accounts.team]
label = "Team Gateway Again"
[accounts.team.endpoints]
anthropic = "https://elsewhere.test"
openai_responses = "https://elsewhere.test/v1"
[accounts.team.account_probe]
kind = "dmxapi"
base_url = "https://elsewhere.test"
[accounts.spare]
label = "Spare"
[accounts.spare.endpoints]
anthropic = "https://spare.test"
[models.team-model]
label = "Team Model"
[routes.team-route]
account = "team"
model = "team-model"
upstream_model = "team-model"
interfaces = { anthropic = ["text", "tool_search"] }
[routes.team-responses]
account = "team"
model = "team-model"
upstream_model = "team-model"
interfaces = { openai_responses = [] }
`

func writeTeamFile(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "team.toml")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// configuredTeam is a home whose team points at a local proxy, with a
// balance probe of its own, a local-only model, and Claude Code selecting
// team-route.
func configuredTeam(t *testing.T, home *guidedHome) {
	t.Helper()
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "http://127.0.0.1:8792/team"},
		BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://team.test"}}
	cfg.Models["team-route"] = domain.Model{Provider: "team", Family: "team-model", UpstreamModel: "team-model", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {domain.CapabilityText}}}
	cfg.Models["local"] = domain.Model{Provider: "team", Family: "local", UpstreamModel: "local", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedModel(claude.ID, "team-route", "")
	cfg.SetClientActivation(claude.ID, true, "", nil)
	cfg.Normalize()
	if err := home.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := home.secrets.Set("team", "sk-stored"); err != nil {
		t.Fatal(err)
	}
}

func TestTeamSetupReportsWhatTheFileChangedAndWhatStayedLocal(t *testing.T) {
	home := newGuidedHome(t)
	home.runtime.Interactive = false
	configuredTeam(t, home)
	path := writeTeamFile(t, teamFileText)
	if err := home.run(t, &guidedPrompt{}, "--from", path, "--prune", "--dry-run"); err != nil {
		t.Fatalf("preview: %v\n%s", err, home.out)
	}
	for _, want := range []string{
		"Team file preview", "Pruned", "team/local", "team anthropic endpoint", "Kept http://127.0.0.1:8792/team; the team file has https://elsewhere.test",
		"team responses endpoint", "Not added", "team balance probe", "team/team-model anthropic", "Dropped tool_search",
		"team/team-model responses", "Unusable until the provider has the endpoint", "Stored", "Nothing was written",
	} {
		if !strings.Contains(home.out.String(), want) {
			t.Errorf("preview lacks %q:\n%s", want, home.out)
		}
	}
	home.out.Reset()
	if err := home.run(t, &guidedPrompt{}, "--from", path); err != nil {
		t.Fatalf("apply: %v\n%s", err, home.out)
	}
	for _, want := range []string{"Team file applied", "Changed", "team", "Stored", "team/team-model; Install Claude if needed, then run `aigw sync`"} {
		if !strings.Contains(home.out.String(), want) {
			t.Errorf("apply lacks %q:\n%s", want, home.out)
		}
	}
	home.out.Reset()
	if err := home.run(t, &guidedPrompt{}, "--from", path); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(home.out.String(), "Already holds everything that the team file sets") {
		t.Errorf("a second apply:\n%s", home.out)
	}
}

func TestTeamSetupReadsTheTokensThatItNeeds(t *testing.T) {
	path := writeTeamFile(t, teamFileText)
	// In a terminal it asks for the Token that the recommendation needs.
	home := newGuidedHome(t, claude.ID)
	prompt := &guidedPrompt{secrets: []string{"sk-team"}}
	if err := home.run(t, prompt, "--from", path); err != nil {
		t.Fatalf("terminal setup: %v\n%s", err, home.out)
	}
	if len(prompt.asked) != 1 || prompt.asked[0] != "Paste the Team Gateway Again Token: " || !strings.Contains(home.out.String(), "Accepted by the provider and stored") {
		t.Fatalf("asked %q:\n%s", prompt.asked, home.out)
	}
	// The read-only environment backend supplies a Token, or refuses a named
	// provider whose variable is not set.
	home = newGuidedHome(t)
	home.runtime.Interactive = false
	home.runtime.Secrets = secret.NewEnvironmentStore(func(key string) string {
		if key == secret.EnvironmentKey("team") {
			return "sk-env"
		}
		return ""
	})
	if err := home.run(t, &guidedPrompt{}, "--from", path, "--json"); err != nil || !strings.Contains(home.out.String(), `"state": "environment"`) {
		t.Fatalf("environment setup: %v\n%s", err, home.out)
	}
	if !strings.Contains(home.out.String(), "Set "+secret.EnvironmentKey("spare")) && strings.Contains(home.out.String(), `"provider": "spare"`) {
		t.Errorf("a missing environment Token names no variable:\n%s", home.out)
	}
	err := home.run(t, &guidedPrompt{}, "--from", path, "--provider", "spare")
	if err == nil || !strings.Contains(err.Error(), "setup cannot store one") {
		t.Fatalf("a named provider without its variable: %v", err)
	}
	// A rejected Token of a new provider names the team file's endpoint.
	home = newGuidedHome(t)
	home.runtime.Interactive = false
	home.status = http.StatusNotFound
	home.runtime.In = strings.NewReader("sk-spare\n")
	err = home.run(t, &guidedPrompt{}, "--from", path, "--provider", "spare", "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "Token validation failed") {
		t.Fatalf("a rejected Token: %v", err)
	}
}

func TestTeamSetupRefusesWhatItCannotRead(t *testing.T) {
	home := newGuidedHome(t)
	home.runtime.Interactive = false
	for _, test := range []struct {
		text, want string
	}{
		{"version = 6\n", "Export the team file again"},
		{"version = 7\n[clients.claude]\nroute = \"team\"\n", "Remove the [clients] section"},
	} {
		err := home.run(t, &guidedPrompt{}, "--from", writeTeamFile(t, test.text), "--json")
		if problem := terminal.ProblemFor(err); !strings.Contains(problem.Fix, test.want) || problem.Impact != "Nothing changed." {
			t.Errorf("%q: %#v", test.text, problem)
		}
	}
	if err := home.run(t, &guidedPrompt{}, "--from", filepath.Join(t.TempDir(), "missing.toml")); err == nil || !strings.Contains(err.Error(), "Cannot read the team file") {
		t.Errorf("a missing file: %v", err)
	}
	if err := home.run(t, &guidedPrompt{}, "--from", writeTeamFile(t, teamFileText), "--provider", "nobody", "--token-stdin"); err == nil || !strings.Contains(err.Error(), `unknown provider "nobody"`) {
		t.Errorf("an unknown provider: %v", err)
	}
}

func TestSetupRefusesFlagsThatCannotRun(t *testing.T) {
	path := writeTeamFile(t, teamFileText)
	for _, test := range []struct {
		args        []string
		interactive bool
		environment bool
		want        string
	}{
		{[]string{"--from", " "}, false, false, "--from requires a team file path"},
		{[]string{"--from", path, "--provider", "Team"}, false, false, `--provider "Team" is not a provider ID`},
		{[]string{"--from", path, "--provider", "team", "--provider", "spare", "--token-stdin"}, false, false, "needs exactly one --provider"},
		{[]string{"--from", path, "--provider", "team", "--token-stdin"}, false, true, "the environment backend reads the team Token"},
		{[]string{"--from", path, "--provider", "team"}, false, false, "pipe the Token to `aigw setup --from " + path + " --provider team --token-stdin`"},
		{[]string{"--from", path, "--provider", "team", "--provider", "spare"}, false, false, "one provider at a time"},
		{[]string{"--prune"}, true, false, "need --from"},
		{nil, false, false, "without one, apply a team file"},
		{[]string{"--json"}, true, false, "--json never prompts"},
		{[]string{"--dry-run"}, true, false, "the one write without a preview"},
	} {
		home := newGuidedHome(t)
		home.runtime.Interactive = test.interactive
		if test.environment {
			home.runtime.Secrets = secret.NewEnvironmentStore(func(string) string { return "" })
		}
		if err := home.run(t, &guidedPrompt{}, test.args...); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%q: %v, want %q", test.args, err, test.want)
		}
	}
}

// Setup from flags previews and reports in its own document until the
// command tree's last step removes it.
func TestFlagSetupDocuments(t *testing.T) {
	home := newGuidedHome(t, codex.ID)
	home.runtime.Interactive = false
	args := []string{"--account", "team", "--route", "team-route", "--openai-url", "https://team.test/v1", "--upstream-model", "team-model", "--for", codex.ID}
	if err := home.run(t, &guidedPrompt{}, append(args, "--dry-run", "--json")...); err != nil {
		t.Fatalf("preview: %v\n%s", err, home.out)
	}
	var preview struct {
		DryRun     bool   `json:"dry_run"`
		NextDetail string `json:"next_detail"`
		NextAction string `json:"next_action"`
	}
	if err := json.Unmarshal(home.out.Bytes(), &preview); err != nil || !preview.DryRun || !strings.Contains(preview.NextAction+preview.NextDetail, "aigw setup --account team") {
		t.Fatalf("preview document = %s, %v", home.out, err)
	}
	home.out.Reset()
	home.runtime.In = strings.NewReader("sk-team\n")
	if err := home.run(t, &guidedPrompt{}, append(args, "--token-stdin", "--json")...); err != nil {
		t.Fatalf("setup: %v\n%s", err, home.out)
	}
	var result struct {
		ConnectedProviders []string `json:"connected_accounts"`
	}
	if err := json.Unmarshal(home.out.Bytes(), &result); err != nil || len(result.ConnectedProviders) != 1 {
		t.Fatalf("setup document = %s, %v", home.out, err)
	}
	for protocol, flag := range map[domain.EndpointProtocol]string{
		domain.ProtocolAnthropic: "--anthropic-url", domain.ProtocolOpenAIChatCompletions: "--chat-url",
		domain.ProtocolOpenAIResponses: "--openai-url", "other": "--protocol",
	} {
		if got := setupEndpointFlag(protocol); got != flag {
			t.Errorf("%s: %s", protocol, got)
		}
	}
}

// The report names every kind of row that it can hold.
func TestTeamSetupRendersEveryRow(t *testing.T) {
	home := newGuidedHome(t)
	team := teamSetup{runtime: home.runtime, request: setupRequest{}}
	document := teamDocument{
		Added:   teamEntries{Providers: []string{"team"}, Models: []string{}, Families: []string{}},
		Changed: teamEntries{Providers: []string{}, Models: []string{}, Families: []string{}},
		KeptModels: []keptModel{
			{Model: "team/one", Client: claude.ID, Protocol: "anthropic", File: "team/one"},
			{Model: "team/two", Client: codex.ID, File: "team/two-new"},
		},
		Tokens: []tokenItem{
			{Provider: "env", State: tokenEnvironment}, {Provider: "gone", State: tokenMissing, NextDetail: "aigw provider rotate gone"},
			{Provider: "bare", State: tokenMissing}, {Provider: "new", State: tokenStored},
		},
		Clients:    []clientItem{{ID: claude.ID, State: clientWaitsForToken, Model: "team/one", NextDetail: "aigw provider rotate team"}},
		NextAction: "aigw check",
	}
	if err := team.write(document); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Kept, because claude selects it on anthropic, which the team file's model lacks", "and the team file names team/two-new",
		"Supplied by " + secret.EnvironmentKey("env"), "None; aigw provider rotate gone", "bare               None", "Accepted by the provider and stored",
		"team/one; aigw provider rotate team",
	} {
		if !strings.Contains(home.out.String(), want) {
			t.Errorf("the report lacks %q:\n%s", want, home.out)
		}
	}
	if got := tokenFix(secret.NewEnvironmentStore(func(string) string { return "" }), "team"); got != "Set "+secret.EnvironmentKey("team")+" to the provider's Token, then run `aigw sync`." {
		t.Errorf("the environment's Token fix = %q", got)
	}
	var endpoints domain.Endpoints
	for _, protocol := range []domain.EndpointProtocol{domain.ProtocolAnthropic, domain.ProtocolOpenAIResponses, domain.ProtocolOpenAIChatCompletions} {
		setEndpoint(&endpoints, protocol, "https://"+string(protocol)+".test")
	}
	if endpoints.Anthropic == "" || endpoints.OpenAIResponses == "" || endpoints.OpenAIChatCompletions == "" {
		t.Errorf("endpoints %+v", endpoints)
	}
}

// Setup from flags refuses what it cannot plan, and previews in text.
func TestFlagSetupRefusalsAndTextPreview(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"--route", "bad id", "--for", claude.ID}, "Invalid account ID"},
		{[]string{"--route", "one", "--account", "bad id", "--for", claude.ID}, "Invalid account ID"},
		{[]string{"--route", "one", "--upstream-model", "m"}, "--for is required"},
		{[]string{"--route", "one", "--for", "other"}, "--for must be"},
		{[]string{"--route", "one", "--for", claude.ID, "--upstream-model", "m"}, "requires --anthropic-url"},
		{[]string{"--route", "one", "--for", claude.ID, "--anthropic-url", "https://one.test"}, "requires --upstream-model"},
		{[]string{"--route", "one", "--for", "hermes", "--upstream-model", "m", "--anthropic-url", "https://one.test", "--openai-url", "https://one.test/v1"}, "specify --protocol"},
		{[]string{"--route", "bad id", "--account", "one", "--for", claude.ID}, "Invalid route ID"},
		{[]string{"--for", claude.ID, "--json"}, "--json requires --route or --from"},
		{[]string{"--route", "one", "--for", claude.ID, "--anthropic-url", "https://one.test", "--upstream-model", "m", "--json"}, "--json reads the Token only from --token-stdin"},
	} {
		home := newGuidedHome(t)
		home.runtime.Interactive = false
		err := home.run(t, &guidedPrompt{}, test.args...)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%q: %v", test.args, err)
		}
		// The root makes an argument refusal a usage error, whose fix is
		// the help.
		if _, usage := errors.AsType[*invocation.ArgumentError](err); !usage && !strings.Contains(err.Error(), "--json") {
			t.Errorf("%q is no argument refusal: %v", test.args, err)
		}
	}
	home := newGuidedHome(t, claude.ID)
	home.runtime.Interactive = false
	if err := home.run(t, &guidedPrompt{}, "--account", "team", "--route", "team-route", "--anthropic-url", "https://team.test", "--upstream-model", "team-model", "--for", claude.ID, "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(home.out.String(), "First-time setup preview") || !strings.Contains(home.out.String(), "Not read in a preview") {
		t.Errorf("the text preview:\n%s", home.out)
	}
	if err := home.run(t, &guidedPrompt{}, "--account", "team", "--route", "team-route", "--anthropic-url", "https://team.test", "--upstream-model", "team-model", "--for", claude.ID, "--dry-run", "--token-stdin"); err != nil {
		t.Fatal(err)
	}
	home.runtime.Secrets = secret.NewEnvironmentStore(func(string) string { return "" })
	if err := home.run(t, &guidedPrompt{}, "--account", "team", "--route", "team-route", "--anthropic-url", "https://team.test", "--upstream-model", "team-model", "--for", claude.ID, "--dry-run", "--token-stdin"); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Errorf("a read-only preview with --token-stdin: %v", err)
	}
}

// A named provider's piped Token is stored and names the next step for its
// models; one that its existing endpoint refuses names that endpoint's edit;
// a file whose model would take a selection's protocol keeps the local one.
func TestTeamSetupStoresANamedTokenAndKeepsASelectedModel(t *testing.T) {
	path := writeTeamFile(t, teamFileText)
	home := newGuidedHome(t)
	home.runtime.Interactive = false
	home.runtime.In = strings.NewReader("sk-team\n")
	if err := home.run(t, &guidedPrompt{}, "--from", path, "--provider", "team", "--token-stdin", "--json"); err != nil {
		t.Fatalf("setup: %v\n%s", err, home.out)
	}
	if token, err := home.secrets.Get("team"); err != nil || token != "sk-team" || !strings.Contains(home.out.String(), `"state": "stored"`) {
		t.Fatalf("the named Token = %q, %v\n%s", token, err, home.out)
	}
	home = newGuidedHome(t)
	configuredTeam(t, home)
	home.status = http.StatusNotFound
	prompt := &guidedPrompt{secrets: []string{"sk-new"}}
	err := home.run(t, prompt, "--from", path, "--provider", "team")
	if problem := terminal.ProblemFor(err); !strings.Contains(problem.Fix, "aigw provider edit team --endpoint anthropic=<url>") {
		t.Fatalf("a refused endpoint of an existing provider: %#v", problem)
	}
	prompt = &guidedPrompt{fail: "Paste the"}
	if problem := terminal.ProblemFor(home.run(t, prompt, "--from", path, "--provider", "team")); problem.Title != "The Token prompt for provider team was cancelled" {
		t.Fatalf("a cancelled Token prompt: %#v", problem)
	}
	home.status = http.StatusOK
	away := strings.Replace(teamFileText, `interfaces = { anthropic = ["text", "tool_search"] }`, `interfaces = { openai_responses = [] }`, 1)
	away = strings.Replace(away, "[recommendations.claude.primary]\nroute = \"team-route\"\n", "", 1)
	home.out.Reset()
	if err := home.run(t, &guidedPrompt{}, "--from", writeTeamFile(t, away), "--json"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(home.out.String(), `"kept_models"`) || !strings.Contains(home.out.String(), `"protocol": "anthropic"`) {
		t.Fatalf("the kept model:\n%s", home.out)
	}
}

// A preview observes each Token's presence, as the run does, and plans what
// the run would: with no Token, with one stored, and with a store that
// cannot tell, which leaves the Token unobserved rather than absent.
func TestTeamSetupPreviewMatchesItsRun(t *testing.T) {
	path := writeTeamFile(t, teamFileText)
	for _, test := range []struct {
		name  string
		store func() secret.Store
		token string
	}{
		{"no Token", func() secret.Store { return secret.NewMemoryStore() }, tokenMissing},
		{"a stored Token", func() secret.Store {
			store := secret.NewMemoryStore()
			_ = store.Set("team", "sk-team")
			return store
		}, tokenPresent},
		{"a store that cannot tell", func() secret.Store {
			return &scriptedSecretStore{existsErr: errors.New("keychain locked")}
		}, tokenUnobserved},
	} {
		var documents [2]teamDocument
		for index, args := range [][]string{{"--from", path, "--dry-run", "--json"}, {"--from", path, "--json"}} {
			home := newGuidedHome(t, claude.ID)
			home.runtime.Interactive = false
			home.runtime.Secrets = test.store()
			if err := home.run(t, &guidedPrompt{}, args...); err != nil {
				t.Fatalf("%s %q: %v\n%s", test.name, args, err, home.out)
			}
			if err := json.Unmarshal(home.out.Bytes(), &documents[index]); err != nil {
				t.Fatalf("%s: %v\n%s", test.name, err, home.out)
			}
		}
		preview, run := documents[0], documents[1]
		if !reflect.DeepEqual(preview.Tokens, run.Tokens) || !reflect.DeepEqual(preview.Clients, run.Clients) {
			t.Errorf("%s: the preview's Tokens %+v and clients %+v differ from the run's %+v and %+v", test.name, preview.Tokens, preview.Clients, run.Tokens, run.Clients)
		}
		if len(run.Tokens) != 1 || run.Tokens[0].State != test.token {
			t.Errorf("%s: Tokens %+v", test.name, run.Tokens)
		}
		if selected := run.Clients[0].State != clientNotSelected; selected != (test.token == tokenPresent) {
			t.Errorf("%s: Claude %+v", test.name, run.Clients[0])
		}
	}
}

// A provider that the team file adds with a protocol that it has no endpoint
// for is refused before any write.
func TestTeamSetupRefusesANewProviderWithoutTheModelsEndpoint(t *testing.T) {
	home := newGuidedHome(t)
	home.runtime.Interactive = false
	path := writeTeamFile(t, `version = 7
[accounts.fresh]
label = "Fresh"
[accounts.fresh.endpoints]
anthropic = "https://fresh.test"
[routes.fresh-chat]
account = "fresh"
model = "fresh-chat"
interfaces = { openai_chat_completions = [] }
`)
	problem := terminal.ProblemFor(home.run(t, &guidedPrompt{}, "--from", path, "--json"))
	if !strings.Contains(problem.Title, "fresh/fresh-chat") || !strings.Contains(problem.Title, "provider fresh") || problem.Fix != "Correct the team file, then run the same command again." {
		t.Fatalf("problem %#v", problem)
	}
	if _, err := os.Stat(home.runtime.Config.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the refused team file wrote the configuration: %v", err)
	}
}
