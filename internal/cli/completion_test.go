package cli

import (
	"bytes"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
)

// lockedStore is a credential store that fails the test on any use, as a
// locked or failing store would block or fail completion if it read one.
type lockedStore struct{ t *testing.T }

func (s lockedStore) Get(string) (string, error) {
	s.t.Error("completion read a Token")
	return "", nil
}

func (s lockedStore) Set(string, string) error {
	s.t.Error("completion stored a Token")
	return nil
}

func (s lockedStore) Delete(string) error {
	s.t.Error("completion deleted a Token")
	return nil
}

func (s lockedStore) Exists(string) (bool, error) {
	s.t.Error("completion read credential metadata")
	return false, nil
}

// refusingHTTP fails the test on any request.
type refusingHTTP struct{ t *testing.T }

func (c refusingHTTP) Do(*http.Request) (*http.Response, error) {
	c.t.Error("completion sent a request")
	return nil, http.ErrHandlerTimeout
}

// TestShellCompletionReadsNoTokenAndSendsNoRequest runs the shell's
// completion requests through the command tree, as a completion script does,
// with a credential store and an HTTP client that fail on any use. Each
// returns at once with suggestions from the local configuration alone, and
// a new name or a query offers nothing.
func TestShellCompletionReadsNoTokenAndSendsNoRequest(t *testing.T) {
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["messages"] = domain.Provider{Label: "Messages", Endpoints: domain.Endpoints{Anthropic: "https://messages.test"}}
	cfg.Providers["responses"] = domain.Provider{Label: "Responses", Endpoints: domain.Endpoints{OpenAIResponses: "https://responses.test/v1"}}
	cfg.Models["messages-opus"] = domain.Model{Provider: "messages", Family: "opus", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Models["responses-sol"] = domain.Model{Provider: "responses", Family: "sol", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args []string
		want []string
	}{
		{[]string{"model", "show", ""}, []string{"messages/opus", "responses/sol"}},
		{[]string{"model", "list", "--provider", ""}, []string{"messages", "responses"}},
		{[]string{"model", "list", "--client", ""}, []string{"claude", "claude-desktop", "codex", "hermes"}},
		{[]string{"model", "list", "--protocol", ""}, []string{"anthropic", "responses", "chat"}},
		{[]string{"model", "add", "--family", ""}, []string{"opus", "sol"}},
		{[]string{"model", "list", ""}, nil},
		{[]string{"model", "add", ""}, nil},
	} {
		out := &bytes.Buffer{}
		app := &App{Clients: clients.Registry(), Config: configStore, Secrets: lockedStore{t}, HTTP: refusingHTTP{t}, Out: out, Err: &bytes.Buffer{}}
		started := time.Now()
		if err := Execute(app, append([]string{"__complete"}, test.args...)); err != nil {
			t.Fatalf("completion of %q: %v", test.args, err)
		}
		if elapsed := time.Since(started); elapsed > 5*time.Second {
			t.Fatalf("completion of %q took %s", test.args, elapsed)
		}
		lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
		if directive := lines[len(lines)-1]; directive != ":4" {
			t.Fatalf("completion of %q ended with %q, want the no-file directive:\n%s", test.args, directive, out)
		}
		var got []string
		for _, line := range lines[:len(lines)-1] {
			value, _, _ := strings.Cut(line, "\t")
			got = append(got, value)
		}
		if strings.Join(got, " ") != strings.Join(test.want, " ") {
			t.Fatalf("completion of %q = %q, want %q", test.args, got, test.want)
		}
	}
}
