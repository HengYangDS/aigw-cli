package setup

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// guidedPrompt answers each prompt with the next scripted answer of its kind,
// records the labels that it was asked, and fails the prompt whose label
// starts with fail.
type guidedPrompt struct {
	texts, selections, secrets []string
	asked                      []string
	choices                    [][]terminal.Choice
	fail                       string
}

func (p *guidedPrompt) answer(label string, answers *[]string) (string, error) {
	p.asked = append(p.asked, label)
	if p.fail != "" && strings.HasPrefix(label, p.fail) {
		return "", fmt.Errorf("%w: no input received", terminal.ErrInputCancelled)
	}
	if len(*answers) == 0 {
		return "", fmt.Errorf("no answer for %q", label)
	}
	answer := (*answers)[0]
	*answers = (*answers)[1:]
	return answer, nil
}

func (p *guidedPrompt) Secret(label string) (string, error) { return p.answer(label, &p.secrets) }
func (p *guidedPrompt) Text(label string) (string, error)   { return p.answer(label, &p.texts) }
func (p *guidedPrompt) Select(label string, choices []terminal.Choice) (string, error) {
	p.choices = append(p.choices, slices.Clone(choices))
	return p.answer(label, &p.selections)
}

// guidedHome is a home for guided setup: its configuration, Tokens, the
// installed clients, and a provider whose catalog lists models and whose
// endpoints answer with status.
type guidedHome struct {
	runtime  invocation.Context
	out      *bytes.Buffer
	secrets  secret.Store
	checked  []string
	status   int
	models   string
	requests []string
}

func newGuidedHome(t *testing.T, installed ...string) *guidedHome {
	t.Helper()
	home := &guidedHome{out: &bytes.Buffer{}, secrets: secret.NewMemoryStore(), status: http.StatusOK, models: `{"data":[{"id":"team-model"},{"id":"spare-model"}]}`}
	executables := map[string]string{}
	for _, client := range installed {
		executables[client] = filepath.Join(t.TempDir(), client)
	}
	home.runtime = invocation.Context{Clients: clients.Registry(),
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Config:     store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission()),
		Secrets:    home.secrets,
		Discovery:  setupDiscovery{result: discovery.Result{Executables: executables}},
		Host:       hostAt(t.TempDir()),
		HTTP: setupHTTPClient(func(request *http.Request) (*http.Response, error) {
			home.requests = append(home.requests, request.Method+" "+request.URL.Path)
			body := "{}"
			if request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/models") {
				body = home.models
			}
			return &http.Response{StatusCode: home.status, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
		}),
		Interactive: true,
		Out:         home.out,
	}
	return home
}

// run runs `aigw setup` with prompt under a root whose check records the
// client that it checks.
func (home *guidedHome) run(t *testing.T, prompt *guidedPrompt, args ...string) error {
	t.Helper()
	home.runtime.Prompt = prompt
	root := &cobra.Command{Use: "aigw", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(NewCommand(home.runtime), &cobra.Command{Use: "check", RunE: func(_ *cobra.Command, args []string) error {
		home.checked = append(home.checked, args...)
		return nil
	}})
	root.SetArgs(append([]string{"setup"}, args...))
	return root.Execute()
}

func (home *guidedHome) config(t *testing.T) domain.Config {
	t.Helper()
	cfg, err := home.runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// newTeamAnswers adds provider team on Anthropic Messages with a Token.
func newTeamAnswers() *guidedPrompt {
	return &guidedPrompt{texts: []string{"team", "Team Gateway", "https://team.test/"}, selections: []string{string(domain.ProtocolAnthropic)}, secrets: []string{"sk-team"}}
}

func TestGuidedSetupSavesAProviderClientAndCatalogModelThenChecksTheClient(t *testing.T) {
	home := newGuidedHome(t, claude.ID)
	prompt := newTeamAnswers()
	prompt.selections = append(prompt.selections, "team-model")
	if err := home.run(t, prompt); err != nil {
		t.Fatalf("guided setup: %v\n%s", err, home.out)
	}
	cfg := home.config(t)
	selected, err := cfg.ResolveRuntime(claude.ID, "")
	if err != nil || selected.ProviderID != "team" || selected.UpstreamModel != "team-model" || cfg.Providers["team"].Endpoints.Anthropic != "https://team.test" {
		t.Fatalf("saved %+v, %v", selected, err)
	}
	if token, err := home.secrets.Get("team"); err != nil || token != "sk-team" {
		t.Fatalf("the Token = %q, %v", token, err)
	}
	if !slices.Equal(home.checked, []string{claude.ID}) || !strings.Contains(home.out.String(), "Claude               team/team-model on Anthropic Messages") {
		t.Fatalf("checked %q:\n%s", home.checked, home.out)
	}
	// The catalog's models are offered in order, with a typed ID last.
	if models := home.lastChoices(prompt); !slices.Equal(models, []string{"spare-model", "team-model", typeModelID}) {
		t.Fatalf("model choices %q", models)
	}
}

func (home *guidedHome) lastChoices(prompt *guidedPrompt) []string {
	var values []string
	for _, choice := range prompt.choices[len(prompt.choices)-1] {
		values = append(values, choice.Value)
	}
	return values
}

func TestGuidedSetupStopsAfterTheProviderWithoutAnInstalledClient(t *testing.T) {
	home := newGuidedHome(t)
	if err := home.run(t, newTeamAnswers()); err != nil {
		t.Fatal(err)
	}
	cfg := home.config(t)
	if _, added := cfg.Providers["team"]; !added || len(cfg.Models) != 0 || len(home.checked) != 0 {
		t.Fatalf("stopped setup saved %+v", cfg)
	}
	if !strings.Contains(home.out.String(), "Setup stopped before a model") || !strings.Contains(home.out.String(), "aigw client list") {
		t.Fatalf("output:\n%s", home.out)
	}
}

func TestGuidedSetupRunsAgainWithAStoredTokenAndATypedModel(t *testing.T) {
	home := newGuidedHome(t, claude.ID, hermes.ID)
	if err := home.run(t, newTeamAnswers()); err == nil {
		t.Fatal("two installed clients took no client answer")
	}
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["team"] = domain.Provider{Label: "Team Gateway", Endpoints: domain.Endpoints{Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1"}}
	if err := home.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := home.secrets.Set("team", "sk-stored"); err != nil {
		t.Fatal(err)
	}
	// The catalog lists no model, so the model ID is typed; Hermes speaks
	// both of team's protocols, so it asks which.
	home.models = `{"data":[]}`
	prompt := &guidedPrompt{selections: []string{"team", hermes.ID, string(domain.ProtocolOpenAIResponses)}, texts: []string{"typed-model"}}
	if err := home.run(t, prompt); err != nil {
		t.Fatalf("guided setup again: %v\n%s", err, home.out)
	}
	cfg = home.config(t)
	selected, err := cfg.ResolveRuntime(hermes.ID, "")
	if err != nil || selected.UpstreamModel != "typed-model" || selected.Protocol != domain.ProtocolOpenAIResponses || slices.Contains(prompt.asked, "Paste the Team Gateway Token: ") {
		t.Fatalf("saved %+v, %v; asked %q", selected, err, prompt.asked)
	}
	if token, _ := home.secrets.Get("team"); token != "sk-stored" || !strings.Contains(home.out.String(), "Stored before") {
		t.Fatalf("the stored Token changed to %q:\n%s", token, home.out)
	}
	// A model that the catalog lists on one of Hermes's protocols takes it
	// without asking, and an added model gains the protocol that it lacks.
	home.models = `{"data":[{"id":"typed-model"}]}`
	home.checked = nil
	prompt = &guidedPrompt{selections: []string{"team", hermes.ID, typeModelID, string(domain.ProtocolAnthropic)}, texts: []string{"typed-model"}}
	if err := home.run(t, prompt); err != nil {
		t.Fatalf("guided setup a third time: %v\n%s", err, home.out)
	}
	cfg = home.config(t)
	if keys := cfg.ResolveModelRef(domain.NewModelRef("team", "typed-model")); len(keys) != 1 || len(cfg.Models[keys[0]].Protocols) != 2 {
		t.Fatalf("models %+v", cfg.Models)
	}
}

func TestGuidedSetupRefusesWhatItCannotSave(t *testing.T) {
	for _, test := range []struct {
		name   string
		prompt *guidedPrompt
		status int
		want   string
	}{
		{"an invalid ID", &guidedPrompt{texts: []string{"Team"}}, http.StatusOK, `"Team" is not a provider ID`},
		{"an invalid URL", &guidedPrompt{texts: []string{"team", "Team", "ftp://team.test"}, selections: []string{string(domain.ProtocolAnthropic)}}, http.StatusOK, "the Anthropic Messages URL"},
		{"an empty Token", &guidedPrompt{texts: []string{"team", "Team", "https://team.test"}, selections: []string{string(domain.ProtocolAnthropic)}, secrets: []string{" "}}, http.StatusOK, "Token input must contain one visible ASCII value"},
		{"a rejected Token", newTeamAnswers(), http.StatusUnauthorized, "Token validation failed"},
		{"an endpoint that is not one", newTeamAnswers(), http.StatusNotFound, "Token validation failed"},
	} {
		home := newGuidedHome(t, claude.ID)
		home.status = test.status
		err := home.run(t, test.prompt)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: %v", test.name, err)
		}
		if present, _ := home.secrets.Exists("team"); present {
			t.Errorf("%s stored a Token", test.name)
		}
	}
	home := newGuidedHome(t, claude.ID)
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	if err := home.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err := home.run(t, &guidedPrompt{selections: []string{addProvider}, texts: []string{"team"}})
	if err == nil || !strings.Contains(err.Error(), "provider team already exists") {
		t.Errorf("an existing ID: %v", err)
	}
	home.runtime.Secrets = secret.NewEnvironmentStore(func(string) string { return "" })
	err = home.run(t, &guidedPrompt{selections: []string{"team"}})
	if err == nil || !strings.Contains(err.Error(), secret.EnvironmentKey("team")) {
		t.Errorf("an environment without the Token: %v", err)
	}
}

// Each prompt that ends without an answer names itself, says that nothing
// changed, and names `aigw setup`, which asks again.
func TestGuidedSetupNamesTheCancelledPrompt(t *testing.T) {
	for _, test := range []struct {
		fail, title string
		configured  bool
	}{
		{"Provider ID", "The provider ID prompt was cancelled", false},
		{"Provider name", "The provider name prompt was cancelled", false},
		{"Endpoint protocol", "The protocol prompt was cancelled", false},
		{"Anthropic Messages URL", "The endpoint prompt was cancelled", false},
		{"Paste the", "The Token prompt for provider team was cancelled", false},
		{"Provider: ", "The provider prompt was cancelled", true},
		{"Client: ", "The client prompt was cancelled", true},
		{"Model for", "The model prompt was cancelled", true},
		{"Protocol for", "The protocol prompt was cancelled", true},
	} {
		home := newGuidedHome(t, claude.ID, hermes.ID)
		prompt := newTeamAnswers()
		if test.configured {
			cfg := domain.NewConfig(clients.Admission())
			cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1"}}
			if err := home.runtime.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			_ = home.secrets.Set("team", "sk-stored")
			prompt = &guidedPrompt{selections: []string{"team", hermes.ID, "team-model"}}
		}
		prompt.fail = test.fail
		problem := terminal.ProblemFor(home.run(t, prompt))
		if problem.Title != test.title || problem.Impact != "Nothing changed." || problem.Fix != "aigw setup" {
			t.Errorf("%s: %#v", test.fail, problem)
		}
	}
	home := newGuidedHome(t, claude.ID)
	home.models = `{"data":[]}`
	prompt := newTeamAnswers()
	prompt.fail = "Model ID"
	if problem := terminal.ProblemFor(home.run(t, prompt)); problem.Title != "The model ID prompt was cancelled" {
		t.Errorf("the model ID prompt: %#v", problem)
	}
}

// A command tree without check names it as the next step, and the pending
// Token answers only for its own provider.
func TestGuidedSetupWithoutACheckCommandNamesIt(t *testing.T) {
	home := newGuidedHome(t, claude.ID)
	prompt := newTeamAnswers()
	prompt.selections = append(prompt.selections, "team-model")
	home.runtime.Prompt = prompt
	cmd := NewCommand(home.runtime)
	cmd.SetArgs(nil)
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	if err := cmd.Execute(); err != nil {
		t.Fatalf("guided setup: %v\n%s", err, home.out)
	}
	if !strings.HasSuffix(strings.TrimSpace(home.out.String()), "aigw check claude") {
		t.Fatalf("output:\n%s", home.out)
	}
	pending := pendingToken{Store: secret.NewMemoryStore(), provider: "team", token: "sk-pending"}
	if present, _ := pending.Exists("other"); present {
		t.Error("the pending Token answers for another provider")
	}
	if _, err := pending.Get("other"); !errors.Is(err, secret.ErrNotFound) {
		t.Errorf("another provider's Token = %v", err)
	}
}

// A configuration, discovery or credential store that fails stops setup with
// its failure before anything is written.
func TestSetupStopsAtAFailedDependency(t *testing.T) {
	path := writeTeamFile(t, teamFileText)
	failures := map[string]func(*guidedHome){
		"configuration": func(home *guidedHome) {
			home.runtime.Config = store.NewStore(t.TempDir(), clients.Admission())
		},
		"discovery": func(home *guidedHome) { home.runtime.Discovery = nil },
		"credentials": func(home *guidedHome) {
			home.runtime.Secrets = &scriptedSecretStore{existsErr: errors.New("keychain unavailable")}
		},
	}
	for name, fail := range failures {
		for _, args := range [][]string{nil, {"--from", path}} {
			home := newGuidedHome(t, claude.ID)
			fail(home)
			if err := home.run(t, newTeamAnswers(), args...); err == nil {
				t.Errorf("%s %q: setup passed", name, args)
			}
		}
	}
}

// An existing provider without a Token asks for one, and with no installed
// client stores it alone; the environment backend's Token is named by its
// variable.
func TestGuidedSetupStoresAnAskedTokenOfAnExistingProvider(t *testing.T) {
	home := newGuidedHome(t)
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	if err := home.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := home.run(t, &guidedPrompt{selections: []string{"team"}, secrets: []string{"sk-asked"}}); err != nil {
		t.Fatalf("guided setup: %v\n%s", err, home.out)
	}
	if token, err := home.secrets.Get("team"); err != nil || token != "sk-asked" || !strings.Contains(home.out.String(), "kept as it was") {
		t.Fatalf("the asked Token = %q, %v\n%s", token, err, home.out)
	}
	home = newGuidedHome(t, claude.ID)
	if err := home.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	home.runtime.Secrets = secret.NewEnvironmentStore(func(key string) string {
		if key == secret.EnvironmentKey("team") {
			return "sk-env"
		}
		return ""
	})
	if err := home.run(t, &guidedPrompt{selections: []string{"team", "team-model"}}); err != nil {
		t.Fatalf("guided setup with the environment backend: %v\n%s", err, home.out)
	}
	if !strings.Contains(home.out.String(), "Supplied by "+secret.EnvironmentKey("team")) {
		t.Fatalf("output:\n%s", home.out)
	}
}
