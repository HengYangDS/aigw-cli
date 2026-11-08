package use

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"
)

// Without a terminal, or with --json, a missing argument is refused and
// named, never asked for; an unknown client, a model argument without a
// provider and an unknown provider are refused with what they could mean, and
// a lone argument that names no client also with the model list that it may
// have meant, before any Token is read.
func TestUseRefusesArgumentsBeforeAnyChange(t *testing.T) {
	runtime, _, _ := configuredRuntime(t)
	runtime.Secrets = credentialFreeStore{}
	runtime.Prompt = &promptStub{selected: codex.ID}
	for _, test := range []struct {
		name        string
		interactive bool
		args        []string
		want, fix   string
	}{
		{"nothing without a terminal", false, nil, "aigw use without a terminal needs a <client> and a <provider/model> argument", "aigw use --help"},
		{"a model without a terminal", false, []string{codex.ID}, "aigw use without a terminal needs a <provider/model> argument after the client", "aigw use --help"},
		{"a client without a terminal", false, []string{"gateway/gpt-test"}, "aigw use without a terminal needs a <client> argument before the model", "aigw use --help"},
		{"a model with --json in a terminal", true, []string{codex.ID, "--json"}, "aigw use with --json needs a <provider/model> argument after the client", "aigw use --help"},
		{"too many arguments", true, []string{codex.ID, "gateway/gpt-test", "extra"}, "aigw use takes <client> and <provider/model>, not 3 arguments", "aigw use --help"},
		{"an unknown client", true, []string{"codx", "gateway/gpt-test"}, `unknown client "codx"; did you mean codex?`, "aigw client list"},
		{"a malformed model", true, []string{codex.ID, "/gpt-test"}, "has an empty provider or model ID", "aigw use --help"},
		{"a model without a provider", false, []string{codex.ID, "gpt-test"}, `"gpt-test" names no provider; it could mean gateway/gpt-test`, "aigw model list gpt-test"},
		{"an unknown provider", false, []string{codex.ID, "gatewy/gpt-test"}, `unknown provider "gatewy"; did you mean gateway?`, "aigw provider list"},
		{"a lone model ID", false, []string{"gpt-test"}, `unknown client "gpt-test"; clients are claude, claude-desktop, codex, or hermes, as ` +
			"`aigw client list` shows; a model is named provider/model", "aigw model list gpt-test"},
		{"a lone unknown client", true, []string{"codx", "--json"}, `unknown client "codx"; did you mean codex? Clients are claude, claude-desktop, codex, or hermes, ` +
			"as `aigw client list` shows; a model is named provider/model", "aigw model list codx"},
		{"a lone word that needs quoting", false, []string{"gpt test"}, `unknown client "gpt test"`, "aigw model list"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime.Interactive = test.interactive
			err := execute(runtime, test.args...)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if fix := terminal.ProblemFor(err).Fix; fix != test.fix {
				t.Fatalf("fix = %q, want %q", fix, test.fix)
			}
		})
	}
}

// A lone argument that names no client is refused from the arguments alone:
// in a fresh home it writes nothing, a configuration that does not parse is
// not read, and the closest clients lead the refusal even when an added model
// looks like the argument too.
func TestUseRefusesALoneClientTypoWithoutTheConfiguration(t *testing.T) {
	want := `unknown client "claud"; did you mean claude, claude-desktop? Clients are claude, claude-desktop, codex, or hermes, ` +
		"as `aigw client list` shows; a model is named provider/model"
	t.Run("fresh home", func(t *testing.T) {
		home := t.TempDir()
		runtime := invocation.Context{Clients: clients.Registry(), Config: store.NewStore(filepath.Join(home, "aigw", "config.toml"), clients.Admission()), Out: &bytes.Buffer{}}
		if problem := terminal.ProblemFor(execute(runtime, "claud", "--json")); problem.Title != want || problem.Fix != "aigw model list claud" {
			t.Fatalf("a client typo in a fresh home = %#v", problem)
		}
		if entries, err := os.ReadDir(home); err != nil || len(entries) != 0 {
			t.Fatalf("a client typo wrote %v, %v", entries, err)
		}
	})
	t.Run("unparsable configuration", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(path, []byte("version = [\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		runtime := invocation.Context{Clients: clients.Registry(), Config: store.NewStore(path, clients.Admission()), Out: &bytes.Buffer{}}
		if problem := terminal.ProblemFor(execute(runtime, "claud")); problem.Title != want {
			t.Fatalf("a client typo with an unparsable configuration = %#v", problem)
		}
	})
	t.Run("a model like it", func(t *testing.T) {
		runtime, cfg, _ := configuredRuntime(t)
		cfg.Providers["ucloud"] = domain.Provider{Label: "UCloud", Endpoints: domain.Endpoints{Anthropic: "https://ucloud.example.test"}}
		cfg.Models["sonnet"] = domain.Model{Provider: "ucloud", Family: "claude-sonnet-5-6", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
		if err := runtime.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		problem := terminal.ProblemFor(execute(runtime, "claud"))
		if problem.Title != want || strings.Contains(problem.Title, "ucloud/") {
			t.Fatalf("a client typo beside a claude model = %#v", problem)
		}
	})
}
