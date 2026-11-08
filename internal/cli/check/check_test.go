package check

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
)

// A check climbs to the depth that it asks for and no further: local sends
// nothing and reads no Token, endpoint sends one request without a model, and
// model, the default, adds one inference request, with one Token read for
// both.
func TestCheckClimbsToTheRequestedDepth(t *testing.T) {
	for _, test := range []struct {
		args     []string
		reached  depth
		requests []string
		reads    int
	}{
		{args: []string{"--depth", "local"}, reached: depthLocal},
		{args: []string{"--depth", "endpoint"}, reached: depthEndpoint, requests: []string{"GET codex.example.test/v1/models"}, reads: 1},
		{reached: depthModel, requests: []string{"GET codex.example.test/v1/models", "POST codex.example.test/v1/responses"}, reads: 1},
	} {
		t.Run(string(test.reached), func(t *testing.T) {
			runtime, cfg, out := configuredRuntime(t)
			enable(t, runtime, &cfg, codex.ID)
			provider := &recorder{respond: answer}
			runtime.HTTP = provider
			if err := execute(runtime, append(test.args, "--json")...); err != nil {
				t.Fatalf("check: %v\n%s", err, out)
			}
			doc := decode(t, out)
			client := doc.client(t, codex.ID)
			if doc.SchemaVersion != 3 || !doc.OK || doc.EnabledClients != 1 || client.Depth != test.reached || client.Failure != nil || len(client.Skipped) != 0 {
				t.Fatalf("check to %s = %+v", test.reached, doc)
			}
			if !slices.Equal(provider.requests, test.requests) {
				t.Fatalf("requests = %q, want %q", provider.requests, test.requests)
			}
			if reads := len(secrets(t, runtime).getProviders); reads != test.reads {
				t.Fatalf("Token reads = %d, want %d", reads, test.reads)
			}
		})
	}
}

// A client stops at its first failed depth, after exactly one rejected
// request, with the fix that acts on the cause, while the other client still
// climbs, and the check exits 1.
func TestCheckStopsAClientAtItsFirstFailedDepth(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	enable(t, runtime, &cfg, claude.ID, codex.ID)
	provider := &recorder{respond: func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "claude.example.test" {
			return status(http.StatusUnauthorized)(request)
		}
		return answer(request)
	}}
	runtime.HTTP = provider
	if err := execute(runtime, "--json"); err == nil {
		t.Fatal("a check with a rejected Token passed")
	}
	doc := decode(t, out)
	rejected, passed := doc.client(t, claude.ID), doc.client(t, codex.ID)
	if doc.OK || rejected.Failure == nil || rejected.Failure.Depth != depthEndpoint || rejected.Failure.HTTPStatus != http.StatusUnauthorized ||
		rejected.Failure.Problem != "HTTP 401: the provider rejected the Token" || rejected.State != domainreadiness.Invalid || rejected.Depth != depthLocal {
		t.Fatalf("the rejected client = %+v", rejected)
	}
	if rejected.NextDetail != "aigw provider rotate one" || doc.NextDetail != "aigw provider rotate one" {
		t.Fatalf("the rejected client's fix = %q, %q", rejected.NextDetail, doc.NextDetail)
	}
	if passed.Depth != depthModel || passed.Failure != nil {
		t.Fatalf("the other client = %+v", passed)
	}
	claudeRequests := slices.DeleteFunc(slices.Clone(provider.requests), func(request string) bool { return !strings.Contains(request, "claude.example.test") })
	if len(claudeRequests) != 1 {
		t.Fatalf("the rejected client sent %q", claudeRequests)
	}
	out.Reset()
	if err := execute(runtime); err == nil {
		t.Fatal("the human check passed")
	}
	for _, want := range []string{"✗ Claude", "one/claude-test · endpoint", "HTTP 401: the provider rejected the Token", "Fix: aigw provider rotate one", "✓ Codex", "one/gpt-test · model"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("the human check lacks %q:\n%s", want, out)
		}
	}
}

// A depth that a client cannot run is skipped with its reason and fails
// nothing. A client that authenticates itself runs neither request and reads
// no Token, and the deeper check that proves its authentication is a session.
func TestCheckSkipsTheRequestsOfAClientWithItsOwnAuthentication(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	binding := cfg.Clients[codex.ID]
	binding.ModelProvider, binding.Authentication = "amazon-bedrock", domain.AuthenticationClientNative
	cfg.Clients[codex.ID] = binding
	enable(t, runtime, &cfg, codex.ID)
	provider := &recorder{respond: answer}
	runtime.HTTP = provider
	if err := execute(runtime, "--json"); err != nil {
		t.Fatalf("check: %v\n%s", err, out)
	}
	client := decode(t, out).client(t, codex.ID)
	if client.Depth != depthLocal || len(client.Skipped) != 2 || client.Skipped[0].Depth != depthEndpoint || client.Skipped[1].Depth != depthModel ||
		!strings.Contains(client.Skipped[0].Reason, "Codex uses its own authentication") || client.NextAction != "aigw check codex --depth client" {
		t.Fatalf("a client with its own authentication = %+v", client)
	}
	store := secrets(t, runtime)
	if len(provider.requests) != 0 || len(store.getProviders) != 0 || len(store.existsProviders) != 0 {
		t.Fatalf("a client with its own authentication sent %q and read %q, %q", provider.requests, store.getProviders, store.existsProviders)
	}
}

// Claude Code with its own model preference runs no inference, because AIGW
// does not choose the model that it sends; a session proves that model.
func TestCheckSkipsTheModelDepthOfAClientWithItsOwnModel(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	enable(t, runtime, &cfg, claude.ID)
	preferOwnModel(t, runtime)
	provider := &recorder{respond: answer}
	runtime.HTTP = provider
	if err := execute(runtime, "--json"); err != nil {
		t.Fatalf("check: %v\n%s", err, out)
	}
	client := decode(t, out).client(t, claude.ID)
	if client.Depth != depthEndpoint || !client.NativeModelPreference || len(client.Skipped) != 1 || client.Skipped[0].Depth != depthModel ||
		client.NextAction != "aigw check claude --depth client" || !slices.Equal(provider.requests, []string{"GET claude.example.test/v1/models"}) {
		t.Fatalf("Claude Code with its own model = %+v; requests %q", client, provider.requests)
	}
}

// An endpoint that lists no models for Claude Code proves no Token without a
// model, so the model depth proves it.
func TestCheckLeavesTheTokenToTheModelDepthWithoutAModelList(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	enable(t, runtime, &cfg, claude.ID)
	runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet {
			return status(http.StatusNotFound)(request)
		}
		return answer(request)
	})
	if err := execute(runtime, "--depth", "endpoint", "--json"); err != nil {
		t.Fatalf("check: %v\n%s", err, out)
	}
	client := decode(t, out).client(t, claude.ID)
	if client.Depth != depthLocal || len(client.Skipped) != 1 || client.Skipped[0].Depth != depthEndpoint || client.NextAction != "aigw check claude" {
		t.Fatalf("an endpoint without a model list = %+v", client)
	}
	out.Reset()
	if err := execute(runtime, "--json"); err != nil {
		t.Fatalf("check: %v\n%s", err, out)
	}
	if client := decode(t, out).client(t, claude.ID); client.Depth != depthModel || len(client.Skipped) != 1 || client.NextAction != "" {
		t.Fatalf("the model depth did not prove the Token: %+v", client)
	}
}

// preferOwnModel changes only the model of Claude Code's settings, as Claude
// Code does when the person picks another model in it.
func preferOwnModel(t *testing.T, runtime invocation.Context) {
	t.Helper()
	path, err := claude.SettingsPath(runtime.Host)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	settings["model"] = json.RawMessage(`"opus[1m]"`)
	changed, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(changed, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A named disabled client is checked like an enabled one without its client
// files, which it has none of, and a check that names no client while none is
// enabled fails without sending anything.
func TestCheckOfANamedDisabledClient(t *testing.T) {
	runtime, _, out := configuredRuntime(t)
	provider := &recorder{respond: answer}
	runtime.HTTP = provider
	if err := execute(runtime, "codex", "--json"); err != nil {
		t.Fatalf("a named disabled client: %v\n%s", err, out)
	}
	doc := decode(t, out)
	client := doc.client(t, codex.ID)
	files := client.Checks[slices.IndexFunc(client.Checks, func(check localCheck) bool { return check.Name == "codex:files" })]
	if !doc.OK || doc.EnabledClients != 0 || len(doc.Clients) != 1 || client.Enabled || client.Depth != depthModel ||
		files.State != checkSkipped || files.Detail != "Codex is disabled, so AIGW keeps no client files for it" {
		t.Fatalf("a named disabled client = %+v", doc)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("a named disabled client sent %q", provider.requests)
	}
	out.Reset()
	provider.requests = nil
	if err := execute(runtime, "--json"); err == nil {
		t.Fatal("a check without an enabled client passed")
	}
	doc = decode(t, out)
	if doc.OK || len(doc.Clients) != 0 || doc.EnabledClients != 0 || doc.State != domainreadiness.Deferred || doc.Problem != "No client is enabled" ||
		!strings.HasPrefix(doc.NextAction, "aigw use ") || len(provider.requests) != 0 {
		t.Fatalf("a check without an enabled client = %+v; requests %q", doc, provider.requests)
	}
}

// A local failure stops the client before it reads a Token or sends a
// request: a provider without a Token, a client file that no longer matches,
// and a variable in the environment that competes with AIGW's Token, which
// stops every client.
func TestCheckStopsAtALocalFailureBeforeAnyRequest(t *testing.T) {
	for _, test := range []struct {
		name, problem, fix string
		state              domainreadiness.State
		prepare            func(t *testing.T, runtime invocation.Context)
		environment        []string
	}{
		{name: "missing Token", problem: "Provider one has no Token", fix: "aigw provider rotate one", state: domainreadiness.Deferred,
			prepare: func(t *testing.T, runtime invocation.Context) {
				delete(secrets(t, runtime).tokens, "one")
			}},
		{name: "removed client file", problem: "Codex configuration target is unavailable", fix: "aigw client edit codex --target <file>", state: domainreadiness.Invalid,
			prepare: func(t *testing.T, runtime invocation.Context) {
				if err := os.Remove(filepath.Join(runtime.Host.Home, ".codex", "config.toml")); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "competing variable", problem: "OPENAI_API_KEY is set in the environment, which a client may send instead of the Token that AIGW gives it",
			fix: "Remove OPENAI_API_KEY from the environment that starts your clients.", state: domainreadiness.Invalid, environment: []string{"OPENAI_API_KEY=sk-other", "ANTHROPIC_API_KEY=", "HOME=/home"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, cfg, out := configuredRuntime(t)
			enable(t, runtime, &cfg, codex.ID)
			store := secrets(t, runtime)
			if test.prepare != nil {
				test.prepare(t, runtime)
			}
			runtime.Env = test.environment
			provider := &recorder{respond: answer}
			runtime.HTTP = provider
			if err := execute(runtime, "--json"); err == nil {
				t.Fatal("a local failure passed")
			}
			client := decode(t, out).client(t, codex.ID)
			if client.Failure == nil || client.Failure.Depth != depthLocal || !strings.Contains(client.Failure.Problem, test.problem) || client.State != test.state ||
				client.NextAction+client.NextDetail != test.fix || client.Depth != "" {
				t.Fatalf("the local failure = %+v", client)
			}
			if len(provider.requests) != 0 || len(store.getProviders) != 0 {
				t.Fatalf("a local failure sent %q and read the Token of %q", provider.requests, store.getProviders)
			}
		})
	}
}

// A check of named clients observes only their providers: another client's
// provider metadata, Token and endpoint stay untouched.
func TestCheckObservesOnlyTheCheckedClients(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	cfg.Providers["two"] = domain.Provider{Label: "Two", Endpoints: domain.Endpoints{Anthropic: "https://two.example.test"}}
	model := cfg.Models["claude"]
	model.Provider = "two"
	cfg.Models["claude"] = model
	enable(t, runtime, &cfg, claude.ID, codex.ID)
	provider := &recorder{respond: answer}
	runtime.HTTP = provider
	if err := execute(runtime, "codex", "--json"); err != nil {
		t.Fatalf("check codex: %v\n%s", err, out)
	}
	doc := decode(t, out)
	store := secrets(t, runtime)
	if len(doc.Clients) != 1 || doc.EnabledClients != 1 || !slices.Equal(store.existsProviders, []string{"one"}) || !slices.Equal(store.getProviders, []string{"one"}) ||
		slices.ContainsFunc(provider.requests, func(request string) bool { return strings.Contains(request, "two.example.test") }) {
		t.Fatalf("a check of Codex observed %q, read %q and sent %q: %+v", store.existsProviders, store.getProviders, provider.requests, doc)
	}
}

// The arguments are checked before anything is read: an unknown client with
// the closest one, a client named twice, an unknown depth and a second
// --depth.
func TestCheckRefusesArgumentsBeforeReadingAnything(t *testing.T) {
	runtime, _, _ := configuredRuntime(t)
	runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("no request may be sent") })
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"codx"}, want: `unknown client "codx"; did you mean codex?`},
		{args: []string{"codex", "codex"}, want: "codex is named more than once"},
		{args: []string{"--depth", "inference"}, want: "--depth must be local, endpoint, model or client"},
		{args: []string{"--depth", "local", "--depth", "model"}, want: "this flag takes one value"},
	} {
		if err := execute(runtime, test.args...); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("check %q = %v, want %q", test.args, err, test.want)
		}
	}
	store := secrets(t, runtime)
	if len(store.existsProviders)+len(store.getProviders) != 0 {
		t.Fatalf("a refused check read %q, %q", store.existsProviders, store.getProviders)
	}
}

// The document carries every local check, the shared ones and each client's,
// with or without --verbose, and the human check lists them only with it.
func TestCheckCarriesEveryLocalCheck(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	enable(t, runtime, &cfg, codex.ID)
	if err := execute(runtime, "--depth", "local", "--json"); err != nil {
		t.Fatalf("check: %v\n%s", err, out)
	}
	doc := decode(t, out)
	var shared, own []string
	for _, check := range doc.Checks {
		shared = append(shared, check.Name+"="+check.State)
	}
	client := doc.client(t, codex.ID)
	for _, check := range client.Checks {
		own = append(own, check.Name+"="+check.State+" "+check.Target)
	}
	if !slices.Equal(shared, []string{"environment=passed", "configuration=passed", "credential_backend=passed"}) ||
		!slices.Equal(own, []string{"codex:model=passed ", "codex:token=passed ", "codex:files=passed ", "codex:target-1=passed ~/.codex/config.toml"}) {
		t.Fatalf("local checks = %q, %q", shared, own)
	}
	for _, verbose := range []bool{false, true} {
		out.Reset()
		args := []string{"--depth", "local"}
		if verbose {
			args = append(args, "--verbose")
		}
		if err := execute(runtime, args...); err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(out.String(), "Codex configuration ~/.codex/config.toml"); got != verbose {
			t.Fatalf("verbose=%t lists the local checks: %t\n%s", verbose, got, out)
		}
	}
}

// A request that may succeed later names the same check again: a timeout
// (HTTP 408), a rate limit (429) and a provider failure (5xx), each one a
// retryable failure of the endpoint depth.
func TestCheckRetriesATransientEndpointFailure(t *testing.T) {
	for _, code := range []int{http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusBadGateway} {
		runtime, cfg, out := configuredRuntime(t)
		enable(t, runtime, &cfg, codex.ID)
		runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) { return status(code)(request) })
		if err := execute(runtime, "--depth", "endpoint", "--json"); err == nil {
			t.Fatalf("HTTP %d passed", code)
		}
		client := decode(t, out).client(t, codex.ID)
		if client.Failure == nil || !client.Failure.Retryable || client.NextAction != "aigw check codex --depth endpoint" || client.State != domainreadiness.Degraded {
			t.Fatalf("HTTP %d = %+v", code, client)
		}
	}
}
