package use

import (
	"bytes"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
)

func tokenAcquisitionRuntime(t *testing.T) (invocation.Context, domain.Config, secret.Store, *bytes.Buffer) {
	t.Helper()
	run, cfg, out := configuredRuntime(t)
	store := secret.NewMemoryStore()
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
		if err := execute(runtime, codex.ID, "gateway/gpt-test"); err != nil {
			t.Fatal(err)
		}
		if token, err := store.Get("gateway"); err != nil || token != "new-token" {
			t.Fatalf("token = %q, %v", token, err)
		}
		out := buffer.String()
		for _, want := range []string{"Model selected", "Token", "Validated and stored", "Install Codex if needed, then run `aigw sync`"} {
			if !strings.Contains(out, want) {
				t.Fatalf("credential acquisition output lacks %q: %q", want, out)
			}
		}
	})

	for _, test := range []struct {
		name    string
		prepare func(invocation.Context, domain.Config) invocation.Context
		want    string
	}{
		{name: "noninteractive", prepare: func(value invocation.Context, _ domain.Config) invocation.Context {
			value.Interactive = false
			return value
		}, want: "has no Token, and aigw use asks for one only in a terminal"},
		{name: "prompt", prepare: func(value invocation.Context, _ domain.Config) invocation.Context {
			value.Prompt = &promptStub{err: errors.New("cancelled")}
			return value
		}, want: "cancelled"},
		{name: "validation", prepare: func(value invocation.Context, _ domain.Config) invocation.Context {
			value.HTTP = doerFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusUnauthorized, Body: http.NoBody}, nil
			})
			return value
		}, want: "Token validation failed"},
		{name: "client convergence", prepare: func(value invocation.Context, _ domain.Config) invocation.Context {
			value.Discovery = nil
			return value
		}, want: "client discovery is unavailable"},
		{name: "planning", prepare: func(value invocation.Context, cfg domain.Config) invocation.Context {
			cfg.SetClientActivation(codex.ID, true, "", []string{filepath.Join(t.TempDir(), "missing", "configuration.toml")})
			binding := cfg.Clients[codex.ID]
			binding.CredentialCommand = filepath.Join(t.TempDir(), "codex-reader")
			cfg.Clients[codex.ID] = binding
			if err := value.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			value.Prompt = &promptStub{err: errors.New("a selection that cannot be planned asked for a Token")}
			return value
		}, want: "Codex config does not exist"},
		{name: "commit", prepare: func(value invocation.Context, cfg domain.Config) invocation.Context {
			value.Discovery = changedDuringDiscovery(t, value, cfg, nil)
			return value
		}, want: "preimage changed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, cfg, store, _ := tokenAcquisitionRuntime(t)
			runtime = test.prepare(runtime, cfg)
			command := NewCommand(runtime)
			command.SilenceErrors = true
			command.SilenceUsage = true
			command.SetArgs([]string{codex.ID, "gateway/gpt-test"})
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
	runtime.Secrets = secret.NewEnvironmentStore(func(string) string { return "" })
	runtime.Interactive = true
	runtime.Prompt = &promptStub{err: errors.New("prompt must not run")}

	err := execute(runtime, codex.ID, "gateway/gpt-test")
	problem := terminal.ProblemFor(err)
	if err == nil || problem.Fix != "Set "+secret.EnvironmentKey("gateway")+", then run `aigw use codex gateway/gpt-test`." {
		t.Fatalf("error = %v, fix %q", err, problem.Fix)
	}
	if strings.Contains(problem.Fix, "rotate") || strings.Contains(err.Error(), "prompt must not run") {
		t.Fatalf("read-only environment backend followed a writable remediation path: %v", err)
	}
}

// With the read-only environment backend, a preview names the variable that
// supplies the missing Token, in JSON and for a person, and never a prompt.
func TestUsePreviewNamesTheVariableOfAMissingEnvironmentToken(t *testing.T) {
	runtime, _, _ := configuredRuntime(t)
	runtime.Secrets = secret.NewEnvironmentStore(func(string) string { return "" })
	key := secret.EnvironmentKey("gateway")
	preview := useDocument(t, runtime, codex.ID, "gateway/gpt-test", "--dry-run")
	if !preview.TokenMissing || preview.NextAction != "" || preview.NextDetail != "Set "+key+", then run `aigw use codex gateway/gpt-test`." {
		t.Fatalf("a preview without the environment Token = %#v", preview)
	}
	out := &bytes.Buffer{}
	runtime.Out = out
	if err := execute(runtime, codex.ID, "gateway/gpt-test", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Missing; set "+key+" to supply it") || strings.Contains(out.String(), "terminal") {
		t.Fatalf("human preview of a missing environment Token:\n%s", out)
	}
}

// Without a terminal, and with --json, which never prompts, a missing Token
// is refused with the step that stores it from standard input, as guidance
// that names the selection again; the refusal says why nothing asks for it.
func TestUseNamesTheTokenStepThatNeedsNoPrompt(t *testing.T) {
	for _, test := range []struct {
		name        string
		interactive bool
		flags       []string
		want        string
	}{
		{name: "no terminal", want: "provider gateway has no Token, and aigw use asks for one only in a terminal"},
		{name: "json", interactive: true, flags: []string{"--json"}, want: "provider gateway has no Token, and aigw use --json never prompts for one"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, _, store, _ := tokenAcquisitionRuntime(t)
			runtime.Interactive = test.interactive
			runtime.Prompt = &promptStub{err: errors.New("prompt must not run")}
			problem := terminal.ProblemFor(execute(runtime, append([]string{codex.ID, "gateway/gpt-test"}, test.flags...)...))
			action, detail := terminal.NextSteps(problem.Fix)
			if problem.Title != test.want || action != "" ||
				detail != "Pipe the Token into `aigw provider rotate gateway --token-stdin`, then run `aigw use codex gateway/gpt-test`." {
				t.Fatalf("a missing Token without a prompt = %#v", problem)
			}
			if secretExists(t, store, "gateway") {
				t.Fatal("a refused selection stored a Token")
			}
		})
	}
}
