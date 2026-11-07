package model

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"
)

// catalogServer answers a catalog request with the listed IDs, counting the
// requests that it receives.
type catalogServer struct {
	ids      []string
	status   int
	requests int
}

func (server *catalogServer) Do(request *http.Request) (*http.Response, error) {
	server.requests++
	status := server.status
	if status == 0 {
		status = http.StatusOK
	}
	items := make([]map[string]string, 0, len(server.ids))
	for _, id := range server.ids {
		items = append(items, map[string]string{"id": id})
	}
	body, _ := json.Marshal(map[string]any{"data": items})
	return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(body)), Request: request}, nil
}

// fixture is a configuration with two providers: team, with one Anthropic
// endpoint and a Token, whose team-model Claude selects, and mixed, with an
// Anthropic and a Chat Completions endpoint and no Token, whose chat-only
// model Hermes recommends.
type fixture struct {
	runtime invocation.Context
	out     *bytes.Buffer
	catalog *catalogServer
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Providers["mixed"] = domain.Provider{Label: "Mixed", Endpoints: domain.Endpoints{
		Anthropic: "https://mixed.test", OpenAIChatCompletions: "https://mixed.test/v1",
	}}
	cfg.Families["team-model"] = domain.Family{Label: "Team Model"}
	cfg.Models["team"] = domain.Model{Provider: "team", Family: "team-model", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Models["mixed-chat"] = domain.Model{
		Provider: "mixed", Family: "chat-only", UpstreamModel: "Chat-Only", Label: "Chat Only",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIChatCompletions: {}},
	}
	cfg.SetSelectedModel("claude", "team", "")
	cfg.Recommendations["hermes"] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: "mixed-chat", Protocol: domain.ProtocolOpenAIChatCompletions}}
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	secrets := secret.NewMemoryStore()
	if err := secrets.Set("team", "team-token"); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	catalog := &catalogServer{ids: []string{"team-model", "spare-model"}}
	return fixture{
		runtime: invocation.Context{Clients: clients.Registry(), Config: configStore, Secrets: secrets, HTTP: catalog, Out: out},
		out:     out, catalog: catalog,
	}
}

// run executes `aigw model <args>` against the fixture and returns its error.
func (f fixture) run(args ...string) error {
	f.out.Reset()
	command := NewCommand(f.runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetOut(f.out)
	command.SetArgs(args)
	return command.Execute()
}

// load returns the stored configuration.
func (f fixture) load(t *testing.T) domain.Config {
	t.Helper()
	cfg, err := f.runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// problem is the Problem and Fix that err presents.
func problem(err error) terminal.Problem {
	if err == nil {
		return terminal.Problem{}
	}
	return terminal.ProblemFor(err)
}

// document decodes the change document that the last command wrote.
func (f fixture) document(t *testing.T) changeDocument {
	t.Helper()
	var document changeDocument
	if err := json.Unmarshal(f.out.Bytes(), &document); err != nil {
		t.Fatalf("output is not one JSON document: %v\n%s", err, f.out)
	}
	return document
}

// list decodes the list document that the last command wrote.
func (f fixture) list(t *testing.T) listDocument {
	t.Helper()
	var document listDocument
	if err := json.Unmarshal(f.out.Bytes(), &document); err != nil {
		t.Fatalf("output is not one JSON document: %v\n%s", err, f.out)
	}
	return document
}

// text is the last command's human output with runs of spaces joined.
func (f fixture) text() string { return strings.Join(strings.Fields(f.out.String()), " ") }
