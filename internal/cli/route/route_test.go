package route

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/prompt"
	"aigw-cli/internal/secrets"
)

type staticDiscovery struct{}

func (staticDiscovery) Discover() discovery.Result { return discovery.Result{} }

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
	cfg.Profiles["codex"] = configuration.Profile{
		Label:   "Codex",
		Account: "gateway",
		Client:  configuration.ClientCodex,
		Model:   "gpt-test",
	}
	cfg.Routes[configuration.ClientCodex] = "codex"
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	return invocation.Context{Config: store, Out: out, RenderOut: out, Width: 120, Discovery: staticDiscovery{}}, cfg, out
}

func execute(commandArgs []string, runtime invocation.Context) error {
	command := NewCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs(commandArgs)
	return command.Execute()
}

func TestCommandTreeAndList(t *testing.T) {
	runtime, _, out := configuredRuntime(t)
	command := NewCommand(runtime)
	if command.Use != "route" || len(command.Commands()) != 1 {
		t.Fatalf("route command = %q with %d children", command.Use, len(command.Commands()))
	}
	if err := execute([]string{"list"}, runtime); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "Current routes") || !strings.Contains(got, "Codex") {
		t.Fatalf("route list = %q", got)
	}
}

func TestListCoversLoadEmptySuggestedAndFallbackViews(t *testing.T) {
	badRuntime := invocation.Context{Config: configuration.NewStore(t.TempDir()), Out: io.Discard}
	if err := runList(badRuntime); err == nil {
		t.Fatal("malformed configuration was accepted by route list")
	}

	problem := errors.New("structured problem")
	emptyRuntime := invocation.Context{
		Config: configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml")),
		Out:    io.Discard,
		Problem: func(string, string, string, string, error) error {
			return problem
		},
	}
	if err := runList(emptyRuntime); !errors.Is(err, problem) {
		t.Fatalf("empty route error = %v", err)
	}

	runtime, cfg, out := configuredRuntime(t)
	delete(cfg.Routes, configuration.ClientCodex)
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := runList(runtime); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "aigw use codex") {
		t.Fatalf("fallback route view = %q", got)
	}

	cfg.Profiles["codex-only"] = configuration.Profile{Label: "Codex only", Account: "gateway", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude only", Account: "gateway", Client: configuration.ClientClaude, Model: "claude-test"}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runList(runtime); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "aigw use claude") || !strings.Contains(got, "aigw use codex") {
		t.Fatalf("suggested route view = %q", got)
	}
}

func TestUseSelectsOnlyTheProfilesDeclaredClient(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	secretStore := secrets.NewMemoryStore()
	runtime.Secrets = secretStore
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	cfg.Profiles["claude"] = configuration.Profile{
		Label:   "Claude",
		Purpose: "Team reviewer",
		Account: "gateway",
		Client:  configuration.ClientClaude,
		Model:   "claude-next",
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	command := NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"claude"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	got, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes[configuration.ClientClaude] != "claude" || got.Routes[configuration.ClientCodex] != "codex" || len(got.Routes) != 2 {
		t.Fatalf("routes = %#v", got.Routes)
	}
	for _, want := range []string{"Service switched", "Claude", "Team reviewer", "Client configuration synchronized", "aigw check"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output lacks %q: %q", want, out.String())
		}
	}
}

func TestUseInteractiveSelectionAndValidationFailures(t *testing.T) {
	runtime, cfg, _ := configuredRuntime(t)
	secretStore := secrets.NewMemoryStore()
	runtime.Secrets = secretStore
	selector := &promptStub{selected: "codex"}
	runtime.Prompt = selector
	runtime.Interactive = true
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	cfg.Profiles["purpose"] = configuration.Profile{Label: "Purpose", Purpose: "Research", Account: "gateway", Client: configuration.ClientClaude, Model: "claude-research"}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	command := NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(selector.choices) != 2 || selector.choices[0].Value != "codex" || selector.choices[0].Label != "Codex" || selector.choices[1].Value != "purpose" || selector.choices[1].Label != "Purpose · Research" {
		t.Fatalf("choices = %#v", selector.choices)
	}

	for _, test := range []struct {
		name    string
		args    []string
		runtime func(invocation.Context) invocation.Context
		want    string
	}{
		{name: "profile required", runtime: func(value invocation.Context) invocation.Context { value.Interactive = false; return value }, want: "requires a profile"},
		{name: "unknown profile", args: []string{"missing"}, want: "Unknown profile"},
		{name: "load", args: []string{"codex"}, runtime: func(value invocation.Context) invocation.Context {
			value.Config = configuration.NewStore(t.TempDir())
			return value
		}, want: "read"},
		{name: "selection", runtime: func(value invocation.Context) invocation.Context {
			value.Prompt = &promptStub{err: errors.New("cancelled")}
			return value
		}, want: "cancelled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := runtime
			if test.runtime != nil {
				value = test.runtime(value)
			}
			command := NewUseCommand(value)
			command.SilenceErrors = true
			command.SilenceUsage = true
			command.SetArgs(test.args)
			err := command.Execute()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestUseAcquiresMissingTokenAndCompensatesFailures(t *testing.T) {
	newRuntime := func(t *testing.T) (invocation.Context, configuration.Config, *secrets.MemoryStore) {
		t.Helper()
		runtime, cfg, _ := configuredRuntime(t)
		store := secrets.NewMemoryStore()
		runtime.Secrets = store
		runtime.Interactive = true
		runtime.HTTP = doerFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
		})
		runtime.Prompt = &promptStub{secret: "new-token"}
		return runtime, cfg, store
	}

	t.Run("success", func(t *testing.T) {
		runtime, _, store := newRuntime(t)
		command := NewUseCommand(runtime)
		command.SetArgs([]string{"codex"})
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		if token, err := store.Get("gateway"); err != nil || token != "new-token" {
			t.Fatalf("token = %q, %v", token, err)
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
		}, want: "Token validation failed"},
		{name: "client convergence", prepare: func(value invocation.Context, _ configuration.Config) invocation.Context {
			value.Discovery = nil
			return value
		}, want: "client discovery is unavailable"},
		{name: "commit", prepare: func(value invocation.Context, cfg configuration.Config) invocation.Context {
			cfg.Profiles["next"] = configuration.Profile{Label: "Next", Account: "gateway", Client: configuration.ClientCodex, Model: "gpt-next"}
			cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Targets: []string{filepath.Join(t.TempDir(), "missing-configuration.toml")}}
			if err := value.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			return value
		}, want: "synchronization failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, cfg, store := newRuntime(t)
			runtime = test.prepare(runtime, cfg)
			command := NewUseCommand(runtime)
			command.SilenceErrors = true
			command.SilenceUsage = true
			args := []string{"codex"}
			if test.name == "commit" {
				args = []string{"next"}
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
	command.SetArgs([]string{"codex"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), secrets.EnvironmentKey("gateway")) || !strings.Contains(err.Error(), "aigw use codex") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "aigw rotate") || strings.Contains(err.Error(), "prompt must not run") {
		t.Fatalf("read-only environment backend followed a writable remediation path: %v", err)
	}
}

func TestUseSurfacesTokenStoreAndOutputFailures(t *testing.T) {
	runtime, _, _ := configuredRuntime(t)
	runtime.Secrets = failingSecretStore{setErr: errors.New("store failed")}
	runtime.Interactive = true
	runtime.Prompt = &promptStub{secret: "token"}
	runtime.HTTP = doerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})
	command := NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"codex"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "store failed") {
		t.Fatalf("store error = %v", err)
	}

	runtime, _, _ = configuredRuntime(t)
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runtime.Secrets = store
	runtime.RenderOut = failingWriter{err: errors.New("write failed")}
	command = NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"codex"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("output error = %v", err)
	}
}

type failingSecretStore struct{ setErr error }

func (failingSecretStore) Get(string) (string, error)     { return "", secrets.ErrNotFound }
func (store failingSecretStore) Set(string, string) error { return store.setErr }
func (failingSecretStore) Delete(string) error            { return nil }
func (failingSecretStore) Exists(string) (bool, error)    { return false, nil }

type failingWriter struct{ err error }

func (writer failingWriter) Write([]byte) (int, error) { return 0, writer.err }
