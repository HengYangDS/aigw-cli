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
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
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
	runtime.Secrets = secrets.NewMemoryStore()
	runtime.HTTP = doerFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("selection preview contacted a service")
		return nil, errors.New("unexpected request")
	})
	command := NewUseCommand(runtime)
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetArgs([]string{"--for", configuration.ClientCodex, "codex", "--forwarding-endpoint", "http://127.0.0.1:8792/v1", "--dry-run", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		DryRun           bool   `json:"dry_run"`
		Endpoint         string `json:"endpoint"`
		UpstreamEndpoint string `json:"upstream_endpoint"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || !result.DryRun || result.Endpoint != "http://127.0.0.1:8792/v1" || result.UpstreamEndpoint != cfg.Accounts["gateway"].Endpoints.OpenAIResponses {
		t.Fatalf("selection preview = %s, %v", out.Bytes(), err)
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
	binding := cfg.Clients[configuration.ClientCodex]
	binding.ForwardingEndpoint = "http://127.0.0.1:8792/original/v1"
	cfg.Clients[configuration.ClientCodex] = binding
	if err := run.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	newer := cfg.Clone()
	binding.ForwardingEndpoint = "http://127.0.0.1:8792/operator/v1"
	newer.Clients[configuration.ClientCodex] = binding
	run.Discovery = staticDiscovery{onDiscover: func() {
		if err := run.Config.Save(newer); err != nil {
			t.Fatal(err)
		}
	}}
	command := NewUseCommand(run)
	command.SetArgs([]string{"codex", "--for", configuration.ClientCodex})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "preimage changed") {
		t.Fatalf("selection must reject a newer assembled configuration: %v", err)
	}
	got, err := run.Config.Load()
	if err != nil || !reflect.DeepEqual(got, newer) {
		t.Fatalf("selection overwrote the operator forwarding destination: %v", err)
	}
}

func TestUsePreviewSharesExplicitCodexSelectionAuthority(t *testing.T) {
	run, cfg, _ := configuredRuntime(t)
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
	cfg.SetClientActivation(configuration.ClientCodex, true, "/opt/codex", []string{target})
	if err := run.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	selected, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
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
		ID: string(surfaceidentity.CodexHomeDefault), Authority: string(surfaceidentity.AuthorityAIGW),
		ConfigPath: target, Present: true, AutoManaged: true,
	}}}}
	before, err := run.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	command := NewUseCommand(run)
	command.SetArgs([]string{"codex", "--for", configuration.ClientCodex, "--dry-run", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatalf("explicit selection preview rejected an authorized root selection: %v", err)
	}
	after, err := run.Config.CaptureSnapshot()
	current, readErr := os.ReadFile(target)
	if err != nil || readErr != nil || !reflect.DeepEqual(before, after) || !bytes.Equal(current, drifted) {
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
		command.SetArgs(append([]string{"--for", configuration.ClientCodex, "codex"}, extra...))
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
		if current.Clients[configuration.ClientCodex].ForwardingEndpoint != want {
			t.Fatalf("selection %d destination = %q, want %q", index, current.Clients[configuration.ClientCodex].ForwardingEndpoint, want)
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

func configuredRuntime(t *testing.T) (invocation.Context, configuration.Config, *bytes.Buffer) {
	t.Helper()
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{
		Label: "Gateway",
		Endpoints: configuration.Endpoints{
			Anthropic:       "https://gateway.example.test",
			OpenAIResponses: "https://gateway.example.test/v1",
		},
	}
	cfg.Routes["codex"] = configuration.Route{
		Label:   "Codex",
		Account: "gateway",
		Model:   "gpt-test",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{
			configuration.ProtocolOpenAIResponses: {},
		},
	}
	cfg.SetSelectedRoute(configuration.ClientCodex, "codex", "")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	return invocation.Context{Config: store, Out: out, RenderOut: out, Width: 120, Discovery: staticDiscovery{}}, cfg, out
}

func TestUseSelectsOnlyTheRoutesDeclaredClient(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	secretStore := secrets.NewMemoryStore()
	runtime.Secrets = secretStore
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	cfg.Routes["claude"] = configuration.Route{
		Label:   "Claude",
		Purpose: "Team reviewer",
		Account: "gateway",
		Model:   "claude-next",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{
			configuration.ProtocolAnthropic: {},
		},
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	command := NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", configuration.ClientClaude, "claude"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	got, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectedRoute(configuration.ClientClaude) != "claude" || got.SelectedRoute(configuration.ClientCodex) != "codex" || len(got.Clients) != 2 {
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
		cmd.SetArgs([]string{"--for", configuration.ClientCodex, "codex"})
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
		{name: "restart", installed: true, want: []string{"Route selected", "Restart required", "Restart Claude Desktop, then run `aigw check`"}},
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
			cfg.Routes["desktop"] = configuration.Route{
				Label: "Desktop", Account: "gateway", Model: "claude-test",
				Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}},
			}
			var executable, library string
			if test.installed {
				executable = filepath.Join(t.TempDir(), "Claude")
				if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
					t.Fatal(err)
				}
				library = filepath.Join(t.TempDir(), "Claude-3p", "configLibrary")
				runtime.Discovery = staticDiscovery{result: discovery.Result{
					Executables: map[string]string{configuration.ClientClaudeDesktop: executable},
					Surfaces:    []discovery.Surface{{ID: "claude-desktop-config-library", ConfigPath: library, Present: true, AutoManaged: true}},
				}}
			}
			if test.preselected {
				cfg.SetSelectedRoute(configuration.ClientClaudeDesktop, "desktop", "")
				cfg.SetClientActivation(configuration.ClientClaudeDesktop, true, executable, []string{library})
			}
			if err := runtime.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			command := NewUseCommand(runtime)
			command.SilenceErrors = true
			command.SilenceUsage = true
			args := []string{"--for", configuration.ClientClaudeDesktop, "desktop"}
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

func tokenAcquisitionRuntime(t *testing.T) (invocation.Context, configuration.Config, secrets.Store, *bytes.Buffer) {
	t.Helper()
	run, cfg, out := configuredRuntime(t)
	store := secrets.NewMemoryStore()
	run.Secrets = store
	run.Interactive = true
	run.HTTP = doerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})
	run.Prompt = &promptStub{secret: "new-token"}
	return run, cfg, store, out
}

func TestUseAcquiresMissingTokenAndCompensatesFailures(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		runtime, _, store, buffer := tokenAcquisitionRuntime(t)
		command := NewUseCommand(runtime)
		command.SetArgs([]string{"--for", configuration.ClientCodex, "codex"})
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		if token, err := store.Get("gateway"); err != nil || token != "new-token" {
			t.Fatalf("token = %q, %v", token, err)
		}
		out := buffer.String()
		for _, want := range []string{"Route selected", "Account Token", "Validated and stored", "Install Codex if needed, then run `aigw sync`"} {
			if !strings.Contains(out, want) {
				t.Fatalf("credential acquisition output lacks %q: %q", want, out)
			}
		}
	})

	for _, test := range []struct {
		name    string
		prepare func(invocation.Context, configuration.Config) invocation.Context
		want    string
	}{
		{name: "noninteractive", prepare: func(value invocation.Context, _ configuration.Config) invocation.Context {
			value.Interactive = false
			return value
		}, want: "missing a token"},
		{name: "prompt", prepare: func(value invocation.Context, _ configuration.Config) invocation.Context {
			value.Prompt = &promptStub{err: errors.New("cancelled")}
			return value
		}, want: "cancelled"},
		{name: "validation", prepare: func(value invocation.Context, _ configuration.Config) invocation.Context {
			value.HTTP = doerFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusUnauthorized, Body: http.NoBody}, nil
			})
			return value
		}, want: "token validation failed"},
		{name: "client convergence", prepare: func(value invocation.Context, _ configuration.Config) invocation.Context {
			value.Discovery = nil
			return value
		}, want: "client discovery is unavailable"},
		{name: "commit", prepare: func(value invocation.Context, cfg configuration.Config) invocation.Context {
			cfg.Routes["next"] = configuration.Route{
				Label: "Next", Account: "gateway", Model: "gpt-next",
				Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
			}
			cfg.SetClientActivation(configuration.ClientCodex, true, "", []string{filepath.Join(t.TempDir(), "missing-configuration.toml")})
			if err := value.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			return value
		}, want: "synchronization preflight failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, cfg, store, _ := tokenAcquisitionRuntime(t)
			runtime = test.prepare(runtime, cfg)
			command := NewUseCommand(runtime)
			command.SilenceErrors = true
			command.SilenceUsage = true
			args := []string{"--for", configuration.ClientCodex, "codex"}
			if test.name == "commit" {
				args = []string{"--for", configuration.ClientCodex, "next"}
			}
			command.SetArgs(args)
			err := command.Execute()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if secretExists(t, store, "gateway") {
				t.Fatalf("failed %s retained newly acquired token", test.name)
			}
		})
	}
}

func TestUseNamesMissingEnvironmentTokenWithoutPrompting(t *testing.T) {
	runtime, _, _ := configuredRuntime(t)
	runtime.Secrets = secrets.NewEnvironmentStore(func(string) string { return "" })
	runtime.Interactive = true
	runtime.Prompt = &promptStub{err: errors.New("prompt must not run")}

	command := NewUseCommand(runtime)
	command.SetArgs([]string{"--for", configuration.ClientCodex, "codex"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), secrets.EnvironmentKey("gateway")) || !strings.Contains(err.Error(), "aigw use --for codex codex") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "aigw rotate") || strings.Contains(err.Error(), "prompt must not run") {
		t.Fatalf("read-only environment backend followed a writable remediation path: %v", err)
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
			command.SetArgs([]string{"--for", configuration.ClientCodex, "codex"})
			if err := command.ExecuteContext(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation error = %v", err)
			}
			if secretExists(t, store, "gateway") {
				t.Fatal("cancelled selection retained its acquired token")
			}
		})
	}
}
