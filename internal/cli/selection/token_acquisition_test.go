package selection

import (
	"bytes"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

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
