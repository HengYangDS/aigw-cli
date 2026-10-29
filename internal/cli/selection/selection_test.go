package selection

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

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/claudedesktop"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/clients"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/prompt"
	"aigw-cli/internal/secrets"
	surfaceidentity "aigw-cli/internal/surface"
)

func TestUseForwardingPreviewIsCredentialFreeAndDoesNotWrite(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	before, err := runtime.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	runtime.HTTP = doerFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("selection preview contacted a service")
		return nil, errors.New("unexpected request")
	})
	command := NewUseCommand(runtime)
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetArgs([]string{"--for", codex.ID, "codex", "--forwarding-endpoint", "http://127.0.0.1:8792/v1", "--dry-run", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var result useResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || !result.DryRun || result.Endpoint != "http://127.0.0.1:8792/v1" || result.UpstreamEndpoint != cfg.Accounts["gateway"].Endpoints.OpenAIResponses {
		t.Fatalf("selection preview = %s, %v", out.Bytes(), err)
	}
	out.Reset()
	command = NewUseCommand(runtime)
	command.SetArgs([]string{"--for", codex.ID, "codex", "--forwarding-endpoint", result.Endpoint, "--dry-run"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"Selection preview", "Client", "Route", result.RouteLabel, "Endpoint", result.Endpoint, "Account upstream", result.UpstreamEndpoint, "Credentials, configuration and client files were unchanged"} {
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

func TestUsePreservesForwardingChangedDuringDiscovery(t *testing.T) {
	run, cfg, _ := configuredRuntime(t)
	credentials := secrets.NewMemoryStore()
	run.Secrets = credentials
	if err := credentials.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetForwardingEndpoint(codex.ID, "http://127.0.0.1:8792/original/v1"); err != nil {
		t.Fatal(err)
	}
	if err := run.Config.Save(cfg); err != nil {
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
	command := NewUseCommand(run)
	command.SetArgs([]string{"codex", "--for", codex.ID})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "preimage changed") {
		t.Fatalf("selection must reject a newer assembled configuration: %v", err)
	}
	got, err := run.Config.Load()
	if err != nil || !reflect.DeepEqual(got, newer) {
		t.Fatalf("selection overwrote the operator forwarding destination: %v", err)
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
	run.Secrets = secrets.NewMemoryStore()
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
	if err := codex.SyncConfig(target, selected); err != nil {
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
		ID: string(codex.HomeDefault), Authority: string(surfaceidentity.AuthorityAIGW),
		ConfigPath: target, Present: true, AutoManaged: true,
	}}}}
	before, err := run.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	command := NewUseCommand(run)
	command.SetArgs([]string{"codex", "--for", codex.ID, "--dry-run"})
	if err := command.Execute(); err != nil {
		t.Fatalf("explicit selection preview rejected an authorized root selection: %v", err)
	}
	after, err := run.Config.CaptureSnapshot()
	current, readErr := os.ReadFile(target)
	if err != nil || readErr != nil || !reflect.DeepEqual(before, after) || !bytes.Equal(current, drifted) || !strings.Contains(out.String(), filepath.Base(target)) {
		t.Fatal("selection preview changed owned configuration or native user selections")
	}
}

func TestUseForwardingApplyNoOpAndDirectPreserveAccount(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	credentials := secrets.NewMemoryStore()
	if err := credentials.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runtime.Secrets = credentials
	for index, extra := range [][]string{
		{"--forwarding-endpoint", "http://127.0.0.1:8792/v1"},
		{},
		{"--direct"},
	} {
		out.Reset()
		command := NewUseCommand(runtime)
		command.SilenceErrors, command.SilenceUsage = true, true
		command.SetArgs(append([]string{"--for", codex.ID, "codex"}, extra...))
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		current, err := runtime.Config.Load()
		if err != nil || !reflect.DeepEqual(current.Accounts, cfg.Accounts) {
			t.Fatalf("forwarding selection changed its Account: %v", err)
		}
		want := "http://127.0.0.1:8792/v1"
		if index == 2 {
			want = ""
		}
		if current.Clients[codex.ID].ForwardingEndpoint != want {
			t.Fatalf("selection %d destination = %q, want %q", index, current.Clients[codex.ID].ForwardingEndpoint, want)
		}
	}
}

type staticDiscovery struct {
	result     discovery.Result
	onDiscover func()
}

func (source staticDiscovery) Discover() discovery.Result {
	if source.onDiscover != nil {
		source.onDiscover()
	}
	return source.result
}

func secretExists(t testing.TB, store secrets.Store, account string) bool {
	t.Helper()
	present, err := store.Exists(account)
	if err != nil {
		t.Fatalf("observe credential for %q: %v", account, err)
	}
	return present
}

type promptStub struct {
	secret   string
	selected string
	choices  []prompt.Choice
	err      error
}

func (stub *promptStub) Secret(string) (string, error) { return stub.secret, stub.err }
func (stub *promptStub) Text(string) (string, error)   { return "", stub.err }
func (stub *promptStub) Select(_ string, choices []prompt.Choice) (string, error) {
	stub.choices = append([]prompt.Choice(nil), choices...)
	return stub.selected, stub.err
}

type doerFunc func(*http.Request) (*http.Response, error)

func (do doerFunc) Do(request *http.Request) (*http.Response, error) { return do(request) }

func configuredRuntime(t *testing.T) (invocation.Context, domain.Config, *bytes.Buffer) {
	t.Helper()
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["gateway"] = domain.Account{
		Label: "Gateway",
		Endpoints: domain.Endpoints{
			Anthropic:       "https://gateway.example.test",
			OpenAIResponses: "https://gateway.example.test/v1",
		},
	}
	cfg.Routes["codex"] = domain.Route{
		Label:   "Codex",
		Account: "gateway",
		Model:   "gpt-test",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolOpenAIResponses: {},
		},
	}
	cfg.SetSelectedRoute(codex.ID, "codex", "")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	return invocation.Context{Clients: clients.Registry(), Config: store, Out: out, RenderOut: out, Width: 120, Discovery: staticDiscovery{}}, cfg, out
}

func TestUseSelectsOnlyTheRoutesDeclaredClient(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	secretStore := secrets.NewMemoryStore()
	runtime.Secrets = secretStore
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	cfg.Routes["claude"] = domain.Route{
		Label:   "Claude",
		Purpose: "Team reviewer",
		Account: "gateway",
		Model:   "claude-next",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolAnthropic: {},
		},
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	command := NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", claude.ID, "claude"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	got, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectedRoute(claude.ID) != "claude" || got.SelectedRoute(codex.ID) != "codex" || len(got.Clients) != 2 {
		t.Fatalf("client bindings = %#v", got.Clients)
	}
	for _, want := range []string{"Route selected", "Claude", "Team reviewer", "Projection", "Install Claude if needed, then run `aigw sync`"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output lacks %q: %q", want, out.String())
		}
	}
}

func TestUseRenderingDoesNotReinspectCredentialMetadata(t *testing.T) {
	run, _, out := configuredRuntime(t)
	reads, renderingReads := 0, 0
	run.Secrets = secrets.NewEnvironmentStore(func(string) string {
		reads++
		if out.Len() > 0 {
			renderingReads++
		}
		return "synthetic-token"
	})
	for invocation := 1; invocation <= 2; invocation++ {
		out.Reset()
		cmd := NewUseCommand(run)
		cmd.SetArgs([]string{"--for", codex.ID, "codex"})
		if err := cmd.Execute(); err != nil {
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
		{name: "restart", installed: true, want: []string{"Route selected", "Restart required", "Restart Claude Desktop, then run `aigw check`", "Anthropic Messages"}, forbid: "anthropic"},
		{name: "deferred", want: []string{"Route selected", "Projection", "Deferred; native client projection is unavailable", "Install Claude Desktop if needed, then run `aigw sync`"}, forbid: "Restart required"},
		{name: "restart-json", installed: true, want: []string{"\"projection_changed\": true", "restart_required", "Restart Claude Desktop, then run `aigw check`"}},
		{name: "preselected-restart", installed: true, preselected: true, want: []string{"Route already selected", "Restart required", "Restart Claude Desktop, then run `aigw check`"}},
		{name: "preselected-restart-json", installed: true, preselected: true, want: []string{"\"changed\": false", "\"projection_changed\": true", "restart_required", "Restart Claude Desktop, then run `aigw check`"}},
		{name: "converged", installed: true, preselected: true, attempts: 2, want: []string{"Route already selected"}, forbid: "Restart required"},
		{name: "converged-json", installed: true, preselected: true, attempts: 2, want: []string{"\"changed\": false", "\"projection_changed\": false"}, forbid: "restart_required"},
		{name: "deferred-json", want: []string{"\"projection_changed\": false", "deferred", "Install Claude Desktop if needed, then run `aigw sync`"}, forbid: "restart_required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, cfg, out := configuredRuntime(t)
			store := secrets.NewMemoryStore()
			runtime.Secrets = store
			runtime.Executable = filepath.Join(t.TempDir(), "aigw")
			if err := store.Set("gateway", "token"); err != nil {
				t.Fatal(err)
			}
			cfg.Routes["desktop"] = domain.Route{
				Label: "Desktop", Account: "gateway", Model: "claude-test",
				Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
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
				cfg.SetSelectedRoute(claudedesktop.ID, "desktop", "")
				cfg.SetClientActivation(claudedesktop.ID, true, executable, []string{library})
			}
			if err := runtime.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			command := NewUseCommand(runtime)
			command.SilenceErrors = true
			command.SilenceUsage = true
			args := []string{"--for", claudedesktop.ID, "desktop"}
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
			command := NewUseCommand(run)
			command.SilenceErrors = true
			command.SilenceUsage = true
			command.SetArgs([]string{"--for", codex.ID, "codex"})
			if err := command.ExecuteContext(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation error = %v", err)
			}
			if secretExists(t, store, "gateway") {
				t.Fatal("cancelled selection retained its acquired token")
			}
		})
	}
}
