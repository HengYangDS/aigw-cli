package invocation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
)

func TestReadTokenUsesExplicitInvocationInput(t *testing.T) {
	for _, input := range []string{"secret-token", "secret-token\n", "secret-token\r\n"} {
		got, err := ReadToken(Context{In: strings.NewReader(input)}, true, false)
		if err != nil || got != "secret-token" {
			t.Fatalf("ReadToken() = %q, %v", got, err)
		}
	}
}

func TestReadTokenRequiresCompleteSingleBoundedInput(t *testing.T) {
	for _, input := range []string{"", "\n", "\r\n", " token", "token ", "token\tvalue", "token\x00value", "token\nextra", "token\r\nX-Injected: value", "token\n\n", "秘密", strings.Repeat("t", 65537)} {
		if token, err := ReadToken(Context{In: strings.NewReader(input)}, true, false); err == nil || token != "" {
			t.Errorf("malformed Token was admitted: length=%d", len(input))
		}
	}
	problem := errors.New("failure after first line")
	input := io.MultiReader(strings.NewReader("token\n"), failingReader{err: problem})
	if _, err := ReadToken(Context{In: input}, true, false); !errors.Is(err, problem) {
		t.Fatalf("input failure after first line = %v", err)
	}
	input = io.MultiReader(strings.NewReader("secret-"), strings.NewReader("token"))
	if token, err := ReadToken(Context{In: input}, true, false); err != nil || token != "secret-token" {
		t.Fatalf("fragmented complete input = %q, %v", token, err)
	}
}

func TestReadTokenRejectsUnavailableEmptyAndFailedExplicitInput(t *testing.T) {
	tests := []struct {
		name    string
		runtime Context
		want    string
	}{
		{name: "unavailable", runtime: Context{}, want: "unavailable"},
		{name: "empty", runtime: Context{In: strings.NewReader("\n")}, want: "Empty tokens"},
		{name: "failure", runtime: Context{In: failingReader{err: errors.New("broken pipe")}}, want: "Failed to read token"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ReadToken(test.runtime, true, false)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ReadToken() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestReadTokenRejectsImplicitInputOutsideInteractiveTerminal(t *testing.T) {
	_, err := ReadToken(Context{}, false, false)
	if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("ReadToken() error = %v, want interactive-terminal guidance", err)
	}
}

func TestRendererUsesTheBoundRenderWriter(t *testing.T) {
	var out bytes.Buffer
	runtime := Context{Out: &bytes.Buffer{}, RenderOut: &out, Width: 80}

	Renderer(runtime).Row("Account", "team-gateway")

	if got := out.String(); !strings.Contains(got, "team-gateway") {
		t.Fatalf("Renderer() output = %q, want bound render writer", got)
	}
}

func TestRendererFallsBackToOutputAndDiscard(t *testing.T) {
	var out bytes.Buffer
	Renderer(Context{Out: &out, Width: 80}).Row("Profile", "portable")
	if !strings.Contains(out.String(), "portable") {
		t.Fatalf("Renderer() output = %q, want output fallback", out.String())
	}
	Renderer(Context{}).Row("Profile", "discarded")
}

func TestDiscoverRequiresAndUsesTheBoundDiscoverer(t *testing.T) {
	if _, err := Discover(Context{}); err == nil {
		t.Fatal("Discover() error = nil, want unavailable error")
	}

	want := discovery.Result{Executables: map[string]string{configuration.ClientCodex: "/portable/codex"}}
	got, err := Discover(Context{Discovery: fixedDiscoverer{result: want}})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if got.Executable(configuration.ClientCodex) != want.Executable(configuration.ClientCodex) {
		t.Fatalf("Discover().Executable(codex) = %q, want %q", got.Executable(configuration.ClientCodex), want.Executable(configuration.ClientCodex))
	}
}

func TestProblemUsesStructuredProblemFactory(t *testing.T) {
	cause := errors.New("connection refused")
	runtime := Context{Problem: func(title, evidence, impact, fix string, got error) error {
		if title != "Unavailable" || evidence != "probe failed" || impact != "cannot test" || fix != "check endpoint" {
			t.Fatalf("unexpected problem fields: %q, %q, %q, %q", title, evidence, impact, fix)
		}
		if !errors.Is(got, cause) {
			t.Fatalf("cause = %v, want %v", got, cause)
		}
		return fmt.Errorf("structured: %w", got)
	}}

	err := Problem(runtime, "Unavailable", "probe failed", "cannot test", "check endpoint", cause)
	if err == nil || err.Error() != "structured: connection refused" {
		t.Fatalf("Problem() = %v, want structured error", err)
	}
}

func TestProblemFallsBackToCause(t *testing.T) {
	cause := errors.New("not configured")
	if got := Problem(Context{}, "ignored", "ignored", "ignored", "ignored", cause); !errors.Is(got, cause) {
		t.Fatalf("Problem() = %v, want original cause %v", got, cause)
	}
}

func TestTitle(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		want  string
	}{
		{name: "empty", value: "", want: ""},
		{name: "lowercase", value: "codex", want: "Codex"},
		{name: "already uppercase", value: "Claude", want: "Claude"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := Title(test.value); got != test.want {
				t.Fatalf("Title(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestSynchronizerPreservesInvocationCapabilities(t *testing.T) {
	config := configuration.NewStore("configuration.toml")
	secretStore := secrets.NewMemoryStore()
	runner := fakeRunner{}
	discoverer := fakeDiscoverer{}

	runtime := Context{
		Config:    config,
		Secrets:   secretStore,
		Runner:    runner,
		Discovery: discoverer,
	}
	got := Synchronizer(runtime)

	if got.Config != config {
		t.Fatalf("Synchronizer().Config = %#v, want invocation config", got.Config)
	}
	if got.Secrets != secretStore {
		t.Fatalf("Synchronizer().Secrets = %#v, want invocation secrets", got.Secrets)
	}
	if got.Runner != runner {
		t.Fatalf("Synchronizer().Runner = %#v, want invocation runner", got.Runner)
	}
	if got.Discovery != discoverer {
		t.Fatalf("Synchronizer().Discovery = %#v, want invocation discovery", got.Discovery)
	}
}

type fakeRunner struct{}

func (fakeRunner) RunCapture(context.Context, process.Plan) ([]byte, error) { return nil, nil }

type fakeDiscoverer struct{}

func (fakeDiscoverer) Discover() discovery.Result { return discovery.Result{} }

type fixedDiscoverer struct{ result discovery.Result }

func (d fixedDiscoverer) Discover() discovery.Result { return d.result }

type failingReader struct{ err error }

func (reader failingReader) Read([]byte) (int, error) { return 0, reader.err }
