package use

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// execute runs `aigw use` with args, below the aigw root as the program runs
// it, and returns its error.
func execute(runtime invocation.Context, args ...string) error {
	root := &cobra.Command{Use: "aigw", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(NewCommand(runtime))
	root.SetArgs(append([]string{"use"}, args...))
	return root.Execute()
}

// A preview reads no Token value, sends no request and writes nothing; it
// names the client by its label and the model by its name.
func TestUsePreviewIsCredentialFreeAndWritesNothing(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	runtime.Secrets = credentialFreeStore{secret.NewMemoryStore()}
	before, err := runtime.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	runtime.HTTP = doerFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("selection preview contacted a service")
		return nil, errors.New("unexpected request")
	})
	if err := execute(runtime, codex.ID, "gateway/gpt-test", "--dry-run", "--json"); err != nil {
		t.Fatal(err)
	}
	var result document
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || !result.DryRun || result.Model != "gateway/gpt-test" ||
		result.Provider != "gateway" || result.Protocol != "responses" || result.Endpoint != cfg.Providers["gateway"].Endpoints.OpenAIResponses ||
		!result.TokenMissing || result.NextAction != "" || result.NextDetail != "Run `aigw use codex gateway/gpt-test` in a terminal, which asks for the provider's Token." {
		t.Fatalf("selection preview = %s, %v", out.Bytes(), err)
	}
	out.Reset()
	if err := execute(runtime, codex.ID, "gateway/gpt-test", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"Model selected (preview)", "gateway/gpt-test", "Missing; aigw use asks for it in a terminal", "Nothing was written"} {
		if !strings.Contains(out.String(), value) {
			t.Fatalf("human preview omitted %q: %s", value, out.String())
		}
	}
	if !regexp.MustCompile(`(?m)^  Client +Codex$`).MatchString(out.String()) {
		t.Fatalf("human preview did not name the client by its label: %s", out.String())
	}
	after, err := runtime.Config.CaptureSnapshot()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("selection preview wrote configuration: %v", err)
	}
}

// A configuration that another command saved while this selection discovered
// its client is newer, so the selection refuses to overwrite it.
func TestUseRefusesAConfigurationChangedDuringDiscovery(t *testing.T) {
	run, cfg, _ := configuredRuntime(t)
	credentials := secret.NewMemoryStore()
	run.Secrets = credentials
	if err := credentials.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	newer := cfg.Clone()
	if err := newer.SetForwardingEndpoint(codex.ID, "http://127.0.0.1:8792/operator/v1"); err != nil {
		t.Fatal(err)
	}
	run.Discovery = staticDiscovery{onDiscover: func() {
		if err := run.Config.Save(newer); err != nil {
			t.Fatal(err)
		}
	}}
	if err := execute(run, codex.ID, "gateway/gpt-test"); err == nil || !strings.Contains(err.Error(), "preimage changed") {
		t.Fatalf("selection must reject a newer assembled configuration: %v", err)
	}
	got, err := run.Config.Load()
	if err != nil || !reflect.DeepEqual(got, newer) {
		t.Fatalf("selection overwrote the operator's configuration: %v", err)
	}
}

func TestUsePreviewSharesExplicitCodexSelectionAuthority(t *testing.T) {
	run, cfg, out := configuredRuntime(t)
	root := t.TempDir()
	target := filepath.Join(root, "codex.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\nmodel = \"native-model\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run.Executable = filepath.Join(root, "aigw")
	run.Host.Home = root
	run.Secrets = secret.NewMemoryStore()
	if err := run.Secrets.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{target})
	if err := run.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	selected, err := cfg.ResolveRuntime(codex.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	selected.CredentialCommand = run.Executable
	if _, err := codex.ReconcileConfigs(nil, []codex.TargetRef{codex.HomeTarget(target)}, selected, selected, false); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	drifted := bytes.Replace(projected, []byte(`model = "gpt-test" # managed by AIGW`), []byte(`model = "user-model"`), 1)
	if bytes.Equal(drifted, projected) {
		t.Fatal("fixture did not change the native root model selection")
	}
	if err := os.WriteFile(target, drifted, 0o600); err != nil {
		t.Fatal(err)
	}
	run.Discovery = staticDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{
		ID:         string(codex.HomeDefault),
		ConfigPath: target, Present: true, AutoManaged: true,
	}}}}
	before, err := run.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	command := NewCommand(run)
	command.SetArgs([]string{codex.ID, "gateway/gpt-test", "--dry-run"})
	if err := command.Execute(); err != nil {
		t.Fatalf("explicit selection preview rejected an authorized root selection: %v", err)
	}
	after, err := run.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("selection preview changed owned configuration")
	}
	if current, err := os.ReadFile(target); err != nil || !bytes.Equal(current, drifted) {
		t.Fatalf("selection preview changed native user selections: %v", err)
	}
	// The preview names the file relative to the home directory.
	if !regexp.MustCompile(`(?m)^  Codex ~/codex\.toml +update$`).MatchString(out.String()) {
		t.Fatalf("selection preview does not name ~/codex.toml:\n%s", out)
	}
}

// A selection of another model of the provider keeps the client's forwarding
// and retry budgets; one of another provider's model clears both and says
// so, in human output and JSON.
func TestUseClearsForwardingAndRetriesOnlyForAnotherProvider(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	credentials := secret.NewMemoryStore()
	runtime.Secrets = credentials
	for _, provider := range []string{"gateway", "other"} {
		if err := credentials.Set(provider, "token"); err != nil {
			t.Fatal(err)
		}
	}
	three := uint64(3)
	binding := cfg.Clients[codex.ID]
	binding.RetryPolicy = &domain.RetryPolicy{StreamMaxRetries: &three}
	cfg.Clients[codex.ID] = binding
	if err := cfg.SetForwardingEndpoint(codex.ID, "http://127.0.0.1:8792/v1"); err != nil {
		t.Fatal(err)
	}
	cfg.Models["next"] = domain.Model{Label: "Next", Provider: "gateway", Family: "gpt-next", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.Providers["other"] = domain.Provider{Label: "Other", Endpoints: domain.Endpoints{OpenAIResponses: "https://other.example.test/v1"}}
	cfg.Models["other"] = domain.Model{Label: "Other", Provider: "other", Family: "gpt-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := execute(runtime, codex.ID, "gateway/gpt-next", "--json"); err != nil {
		t.Fatal(err)
	}
	var kept document
	if err := json.Unmarshal(out.Bytes(), &kept); err != nil || kept.ForwardingCleared || kept.RetriesCleared || kept.Retries == nil ||
		kept.Endpoint != "http://127.0.0.1:8792/v1" || kept.ProviderEndpoint != "https://gateway.example.test/v1" {
		t.Fatalf("selection within the provider = %s, %v", out.Bytes(), err)
	}
	out.Reset()
	if err := execute(runtime, codex.ID, "other/gpt-test"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Forwarding", "Cleared, because the client's upstream endpoint changed", "Retries", "Cleared, because they belonged to the previous provider"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("selection of another provider lacks %q:\n%s", want, out)
		}
	}
	saved, err := runtime.Config.Load()
	if binding := saved.Clients[codex.ID]; err != nil || binding.ForwardingEndpoint != "" || binding.RetryPolicy != nil {
		t.Fatalf("another provider kept the connection settings: %#v, %v", binding, err)
	}
}

// A selection changes only its client's selection, and names the step that
// writes the files of a client that is not installed.
func TestUseSelectsOnlyItsClient(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	secretStore := secret.NewMemoryStore()
	runtime.Secrets = secretStore
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	cfg.Models["claude"] = domain.Model{Label: "Claude", Provider: "gateway", Family: "claude-next", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := execute(runtime, claude.ID, "gateway/claude-next"); err != nil {
		t.Fatal(err)
	}
	got, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectedModel(claude.ID) != "claude" || got.SelectedModel(codex.ID) != "codex" || len(got.Clients) != 2 {
		t.Fatalf("client selections = %#v", got.Clients)
	}
	for _, want := range []string{"Model selected", "Claude", "gateway/claude-next", "Install Claude if needed, then run `aigw sync`"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output lacks %q: %q", want, out.String())
		}
	}
}

func TestUseRenderingDoesNotReinspectCredentialMetadata(t *testing.T) {
	run, _, out := configuredRuntime(t)
	reads, renderingReads := 0, 0
	run.Secrets = secret.NewEnvironmentStore(func(string) string {
		reads++
		if out.Len() > 0 {
			renderingReads++
		}
		return "synthetic-token"
	})
	for invocation := 1; invocation <= 2; invocation++ {
		out.Reset()
		if err := execute(run, codex.ID, "gateway/gpt-test"); err != nil {
			t.Fatal(err)
		}
		if reads != invocation || renderingReads != 0 {
			t.Fatalf("invocation %d: credential observations total=%d during rendering=%d; want one fresh observation", invocation, reads, renderingReads)
		}
	}
}

func TestUseReportsClaudeDesktopActivationState(t *testing.T) {
	for _, test := range []struct {
		name        string
		installed   bool
		preselected bool
		attempts    int
		want        []string
		forbid      string
	}{
		{name: "restart", installed: true, want: []string{"Model selected", "Restart required", "Restart Claude Desktop, then run `aigw check claude-desktop`", "Anthropic Messages"}, forbid: "anthropic"},
		{name: "deferred", want: []string{"Model selected", "Client files", "Written once Claude Desktop is installed", "Install Claude Desktop if needed, then run `aigw sync`"}, forbid: "Restart required"},
		{name: "restart-json", installed: true, want: []string{"\"changed\": true", "\"restart_required\": true", "Restart Claude Desktop, then run `aigw check claude-desktop`"}},
		{name: "preselected-restart", installed: true, preselected: true, want: []string{"Model already selected", "Restart required", "Restart Claude Desktop, then run `aigw check claude-desktop`"}},
		{name: "preselected-restart-json", installed: true, preselected: true, want: []string{"\"changed\": false", "\"restart_required\": true", "Restart Claude Desktop, then run `aigw check claude-desktop`"}},
		{name: "converged", installed: true, preselected: true, attempts: 2, want: []string{"Model already selected", "aigw check claude-desktop"}, forbid: "Restart required"},
		{name: "converged-json", installed: true, preselected: true, attempts: 2, want: []string{"\"changed\": false", "\"next_action\": \"aigw check claude-desktop\""}, forbid: "restart_required"},
		{name: "deferred-json", want: []string{"\"files_deferred\": true", "Install Claude Desktop if needed, then run `aigw sync`"}, forbid: "restart_required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, cfg, out := configuredRuntime(t)
			store := secret.NewMemoryStore()
			runtime.Secrets = store
			runtime.Executable = filepath.Join(t.TempDir(), "aigw")
			if err := store.Set("gateway", "token"); err != nil {
				t.Fatal(err)
			}
			cfg.Models["desktop"] = domain.Model{
				Label: "Desktop", Provider: "gateway", Family: "claude-test",
				Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
			}
			var executable, library string
			if test.installed {
				executable = filepath.Join(t.TempDir(), "Claude")
				if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
					t.Fatal(err)
				}
				library = filepath.Join(t.TempDir(), "Claude-3p", "configLibrary")
				runtime.Discovery = staticDiscovery{result: discovery.Result{
					Executables: map[string]string{claudedesktop.ID: executable},
					Surfaces:    []discovery.Surface{{ID: "claude-desktop-config-library", ConfigPath: library, Present: true, AutoManaged: true}},
				}}
			}
			if test.preselected {
				cfg.SetSelectedModel(claudedesktop.ID, "desktop", "")
				cfg.SetClientActivation(claudedesktop.ID, true, executable, []string{library})
			}
			if err := runtime.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			command := NewCommand(runtime)
			command.SilenceErrors = true
			command.SilenceUsage = true
			args := []string{claudedesktop.ID, "gateway/claude-test"}
			if strings.HasSuffix(test.name, "-json") {
				args = append(args, "--json")
			}
			command.SetArgs(args)
			for range max(test.attempts, 1) {
				out.Reset()
				if err := command.Execute(); err != nil {
					t.Fatal(err)
				}
			}
			if entries, err := os.ReadDir(library); test.preselected && (err != nil || len(entries) == 0) {
				t.Fatalf("selected Desktop did not receive its first native projection: %v", err)
			}
			for _, want := range test.want {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("use output = %q, want %q", out.String(), want)
				}
			}
			if strings.Contains(out.String(), test.forbid) && test.forbid != "" {
				t.Fatalf("use output = %q, forbid %q", out.String(), test.forbid)
			}
		})
	}
}

func TestUseRespectsCommandCancellation(t *testing.T) {
	for _, phase := range []string{"before validation", "during validation", "after validation", "client convergence"} {
		t.Run(phase, func(t *testing.T) {
			run, _, store, _ := tokenAcquisitionRuntime(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if phase == "before validation" {
				cancel()
			}
			run.HTTP = doerFunc(func(request *http.Request) (*http.Response, error) {
				if phase == "during validation" {
					cancel()
					if err := request.Context().Err(); err != nil {
						return nil, err
					}
				}
				if phase == "after validation" {
					cancel()
				}
				return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
			})
			if phase == "client convergence" {
				run.Discovery = staticDiscovery{onDiscover: cancel}
			}
			command := NewCommand(run)
			command.SilenceErrors = true
			command.SilenceUsage = true
			command.SetArgs([]string{codex.ID, "gateway/gpt-test"})
			if err := command.ExecuteContext(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation error = %v", err)
			}
			if secretExists(t, store, "gateway") {
				t.Fatal("cancelled selection retained its acquired token")
			}
		})
	}
}

type staticDiscovery struct {
	result     discovery.Result
	onDiscover func()
}

func (source staticDiscovery) Discover(...string) discovery.Result {
	if source.onDiscover != nil {
		source.onDiscover()
	}
	return source.result
}

func secretExists(t testing.TB, store secret.Store, account string) bool {
	t.Helper()
	present, err := store.Exists(account)
	if err != nil {
		t.Fatalf("observe credential for %q: %v", account, err)
	}
	return present
}

// promptStub answers each selection prompt with the next of answers, or
// with selected once they run out, and keeps the choices of the last one.
type promptStub struct {
	secret   string
	selected string
	answers  []string
	choices  []terminal.Choice
	err      error
}

func (stub *promptStub) Secret(string) (string, error) { return stub.secret, stub.err }
func (stub *promptStub) Text(string) (string, error)   { return "", stub.err }
func (stub *promptStub) Select(_ string, choices []terminal.Choice) (string, error) {
	stub.choices = append([]terminal.Choice(nil), choices...)
	if len(stub.answers) > 0 {
		answer := stub.answers[0]
		stub.answers = stub.answers[1:]
		return answer, stub.err
	}
	return stub.selected, stub.err
}

type doerFunc func(*http.Request) (*http.Response, error)

func (do doerFunc) Do(request *http.Request) (*http.Response, error) { return do(request) }

func configuredRuntime(t *testing.T) (invocation.Context, domain.Config, *bytes.Buffer) {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["gateway"] = domain.Provider{
		Label: "Gateway",
		Endpoints: domain.Endpoints{
			Anthropic:       "https://gateway.example.test",
			OpenAIResponses: "https://gateway.example.test/v1",
		},
	}
	cfg.Models["codex"] = domain.Model{
		Label:    "Codex",
		Provider: "gateway",
		Family:   "gpt-test",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolOpenAIResponses: {},
		},
	}
	cfg.SetSelectedModel(codex.ID, "codex", "")
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	return invocation.Context{Clients: clients.Registry(), Config: configStore, Out: out, Width: 120, Discovery: staticDiscovery{}}, cfg, out
}
