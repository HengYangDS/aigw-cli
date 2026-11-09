package check

import (
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/domain"
)

// namedModelRuntime is a configured home whose Claude Code is enabled on
// one/claude-test, with a spare model of the same provider.
func namedModelRuntime(t *testing.T) runtimeFixture {
	t.Helper()
	ctx, cfg, out := configuredRuntime(t)
	cfg.Models["spare"] = domain.Model{Label: "Spare", Provider: "one", Family: "spare-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	enable(t, ctx, &cfg, claude.ID)
	return runtimeFixture{ctx: ctx, cfg: cfg, out: out}
}

// A check of a named model sends its requests with that model's ID and
// leaves every selection and the configuration unchanged; the row says that
// the client still uses its own model.
func TestCheckTriesANamedModelWithoutSelectingIt(t *testing.T) {
	fixture := namedModelRuntime(t)
	before := fixture.configuration(t)
	var bodies []string
	fixture.ctx.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := []byte{}
		if request.Body != nil {
			body, _ = io.ReadAll(request.Body)
		}
		bodies = append(bodies, request.Method+" "+string(body))
		return answer(request)
	})
	if err := execute(fixture.ctx, claude.ID, "--model", "one/spare-test", "--json"); err != nil {
		t.Fatalf("check of a named model: %v\n%s", err, fixture.out)
	}
	doc := decode(t, fixture.out)
	client := doc.client(t, claude.ID)
	if doc.Model != "one/spare-test" || client.Model != "one/spare-test" || client.Depth != depthModel || len(bodies) != 2 || !strings.Contains(bodies[1], `"model":"spare-test"`) {
		t.Fatalf("a check of a named model = %+v; requests %q", doc, bodies)
	}
	if after := fixture.configuration(t); after != before {
		t.Fatal("a check of a named model changed the configuration")
	}
	fixture.out.Reset()
	if err := execute(fixture.ctx, claude.ID, "--model", "one/spare-test", "--depth", "local"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fixture.out.String(), "Tried with --model; Claude still uses one/claude-test") {
		t.Fatalf("the human check does not say what Claude still uses:\n%s", fixture.out)
	}
}

// A one-time Token from standard input authenticates the requests of the
// check, once read, and the stored Token is neither read nor replaced; the
// local depth skips its stored-Token check and says why.
func TestCheckUsesAOneTimeTokenOnce(t *testing.T) {
	fixture := namedModelRuntime(t)
	fixture.ctx.In = strings.NewReader("sk-once-0001\n")
	var keys []string
	fixture.ctx.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		keys = append(keys, request.Header.Get("X-Api-Key"))
		return answer(request)
	})
	if err := execute(fixture.ctx, claude.ID, "--model", "one/spare-test", "--token-stdin", "--depth", "endpoint", "--json"); err != nil {
		t.Fatalf("check with a one-time Token: %v\n%s", err, fixture.out)
	}
	client := decode(t, fixture.out).client(t, claude.ID)
	token := client.Checks[slices.IndexFunc(client.Checks, func(check localCheck) bool { return check.Name == "claude:token" })]
	store := secrets(t, fixture.ctx)
	if client.Depth != depthEndpoint || !slices.Equal(keys, []string{"sk-once-0001"}) || token.State != checkSkipped ||
		token.Detail != "The one-time Token from standard input replaces the stored Token for this check" || len(store.getProviders) != 0 || len(store.writes) != 0 {
		t.Fatalf("a one-time Token = %+v; keys %q; reads %q; writes %q", client, keys, store.getProviders, store.writes)
	}
	if strings.Contains(fixture.out.String(), "sk-once-0001") {
		t.Fatal("the document shows the one-time Token")
	}
}

// --model and --token-stdin are refused before standard input is read or a
// request is sent: --model without exactly one named client, a malformed or
// unknown model, --token-stdin without --model or at the local or client
// depth, and a one-time Token for a client that sends none from AIGW.
func TestCheckRefusesANamedModelBeforeReadingInput(t *testing.T) {
	fixture := namedModelRuntime(t)
	binding := fixture.cfg.Clients[codex.ID]
	binding.ModelProvider, binding.Authentication = "amazon-bedrock", domain.AuthenticationClientNative
	fixture.cfg.Clients[codex.ID] = binding
	save(t, fixture.ctx, fixture.cfg)
	input := strings.NewReader("sk-never-read\n")
	fixture.ctx.In = input
	fixture.ctx.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		t.Fatalf("a refused check sent %s", request.URL)
		return nil, nil
	})
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"--model", "one/spare-test"}, want: "--model needs exactly one named client, the one that tries it, not 0"},
		{args: []string{claude.ID, codex.ID, "--model", "one/spare-test"}, want: "--model needs exactly one named client, the one that tries it, not 2"},
		{args: []string{claude.ID, "--model", "one/"}, want: "has an empty provider or model ID"},
		{args: []string{claude.ID, "--model", "spare-test"}, want: `"spare-test" names no provider; it could mean one/spare-test`},
		{args: []string{claude.ID, "--model", "onex/spare-test"}, want: `unknown provider "onex"; did you mean one?`},
		{args: []string{claude.ID, "--token-stdin"}, want: "--token-stdin needs --model"},
		{args: []string{claude.ID, "--model", "one/spare-test", "--token-stdin", "--depth", "local"}, want: "--token-stdin works at the endpoint or model depth, not at local"},
		{args: []string{claude.ID, "--model", "one/spare-test", "--token-stdin", "--depth", "client"}, want: "--token-stdin works at the endpoint or model depth, not at client"},
		{args: []string{claude.ID, "--model", "one/spare-test", "--model", "one/claude-test"}, want: "this flag takes one value"},
		{args: []string{codex.ID, "--model", "one/gpt-test", "--token-stdin"}, want: "--token-stdin needs a client that sends a Token from AIGW, and Codex uses its own authentication"},
	} {
		if err := execute(fixture.ctx, test.args...); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("check %q = %v, want %q", test.args, err, test.want)
		}
	}
	if input.Len() != len("sk-never-read\n") {
		t.Fatal("a refused check read standard input")
	}
}

// A retry or a deeper check that a check of a named model names keeps the
// client and the model, and the one-time Token at the depths that send it.
func TestCheckOfANamedModelNamesItsRetry(t *testing.T) {
	fixture := namedModelRuntime(t)
	fixture.ctx.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return status(http.StatusServiceUnavailable)(request)
	})
	fixture.ctx.In = strings.NewReader("sk-once-0001\n")
	if err := execute(fixture.ctx, claude.ID, "--model", "one/spare-test", "--token-stdin", "--depth", "endpoint", "--json"); err == nil {
		t.Fatal("a failed check passed")
	}
	if got := decode(t, fixture.out).client(t, claude.ID).NextAction; got != "aigw check claude --model one/spare-test --token-stdin --depth endpoint" {
		t.Fatalf("the retry = %q", got)
	}
	fixture.out.Reset()
	fixture.ctx.HTTP = roundTripFunc(answer)
	preferOwnModel(t, fixture.ctx)
	if err := execute(fixture.ctx, claude.ID, "--model", "one/spare-test", "--json"); err != nil {
		t.Fatal(err)
	}
	if got := decode(t, fixture.out).client(t, claude.ID).NextAction; got != "aigw check claude --model one/spare-test --depth client" {
		t.Fatalf("the deeper check = %q", got)
	}
}

// A one-time Token that the provider rejects was never stored, so its fix is
// not to rotate the stored Token: it pipes a Token that the provider accepts
// into the same check, as guidance, since a script cannot run it as it stands.
func TestCheckOfARejectedOneTimeTokenNamesTheSameCheck(t *testing.T) {
	fixture := namedModelRuntime(t)
	fixture.ctx.In = strings.NewReader("sk-once-0001\n")
	fixture.ctx.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return status(http.StatusUnauthorized)(request)
	})
	if err := execute(fixture.ctx, claude.ID, "--model", "one/spare-test", "--token-stdin", "--depth", "endpoint", "--json"); err == nil {
		t.Fatal("a rejected one-time Token passed")
	}
	client := decode(t, fixture.out).client(t, claude.ID)
	want := "Pipe a Token that provider one accepts into `aigw check claude --model one/spare-test --token-stdin --depth endpoint`."
	if client.NextAction != "" || client.NextDetail != want {
		t.Fatalf("a rejected one-time Token's fix = %q, %q; want %q", client.NextAction, client.NextDetail, want)
	}
	if writes := secrets(t, fixture.ctx).writes; len(writes) != 0 {
		t.Fatalf("a rejected one-time Token wrote the store: %q", writes)
	}
}

// A model whose name needs quoting cannot appear in a command line, so the
// fix of its rejected one-time Token says the same check in plain words and
// nests no other sentence.
func TestCheckOfARejectedOneTimeTokenForAQuotedModelNamesTheSameCheck(t *testing.T) {
	fixture := namedModelRuntime(t)
	spare := fixture.cfg.Models["spare"]
	spare.UpstreamModel = "opus[1m]"
	fixture.cfg.Models["spare"] = spare
	if err := fixture.ctx.Config.Save(fixture.cfg); err != nil {
		t.Fatal(err)
	}
	fixture.ctx.In = strings.NewReader("sk-once-0001\n")
	fixture.ctx.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return status(http.StatusUnauthorized)(request)
	})
	if err := execute(fixture.ctx, claude.ID, "--model", "one/opus[1m]", "--token-stdin", "--depth", "endpoint", "--json"); err == nil {
		t.Fatal("a rejected one-time Token passed")
	}
	client := decode(t, fixture.out).client(t, claude.ID)
	if want := "Pipe a Token that provider one accepts into the same check."; client.NextAction != "" || client.NextDetail != want {
		t.Fatalf("the fix = %q, %q; want %q", client.NextAction, client.NextDetail, want)
	}
	if writes := secrets(t, fixture.ctx).writes; len(writes) != 0 {
		t.Fatalf("a rejected one-time Token wrote the store: %q", writes)
	}
}
