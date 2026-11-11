package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"aigw-cli/internal/app"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"

	"github.com/spf13/cobra"
)

// providerCatalogs answers each provider's catalog requests by host: the IDs
// that a host lists, or a status, a body or a transport failure set for one
// request URL and authentication. It records each request, which may arrive
// at once.
type providerCatalogs struct {
	mu       sync.Mutex
	lists    map[string][]string
	status   map[string]int
	bodies   map[string]string
	failures map[string]error
	requests []string
}

func (catalogs *providerCatalogs) Do(request *http.Request) (*http.Response, error) {
	catalogs.mu.Lock()
	defer catalogs.mu.Unlock()
	scheme := "bearer"
	if request.Header.Get("X-Api-Key") != "" {
		scheme = "x-api-key"
	}
	key := scheme + " " + request.URL.String()
	catalogs.requests = append(catalogs.requests, key)
	if err := catalogs.failures[key]; err != nil {
		return nil, err
	}
	status, set := catalogs.status[key]
	if !set {
		status = http.StatusOK
	}
	body, set := catalogs.bodies[key]
	if !set {
		items := []map[string]string{}
		for _, id := range catalogs.lists[request.URL.Host] {
			items = append(items, map[string]string{"id": id})
		}
		data, _ := json.Marshal(map[string]any{"data": items})
		body = string(data)
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
}

// searchFixture is a configuration of four providers. full serves Anthropic,
// Responses and Chat Completions, the last two on one URL; its catalog lists
// full-a, full-old and full-new, and it has added full-a, full-old and
// full-gone-old, which are deprecated, and full-gone. partial lists partial-a
// on its Anthropic endpoint, has a Chat Completions endpoint too, and has added
// partial-a and partial-gone. unreadable serves Responses. notoken has no
// Token.
type searchFixture struct {
	runtime  invocation.Context
	out      *bytes.Buffer
	catalogs *providerCatalogs
}

// run executes `aigw model <args>` below an aigw root, as the CLI does, so
// that fixes name aigw.
func (f searchFixture) run(args ...string) error {
	f.out.Reset()
	root := &cobra.Command{Use: "aigw", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(NewCommand(f.runtime))
	root.SetOut(f.out)
	root.SetArgs(append([]string{"model"}, args...))
	return root.Execute()
}

// text is the last command's human output with runs of spaces joined.
func (f searchFixture) text() string { return strings.Join(strings.Fields(f.out.String()), " ") }

func newSearchFixture(t *testing.T) searchFixture {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["full"] = domain.Provider{Label: "Full", Endpoints: domain.Endpoints{
		Anthropic: "https://full.test", Responses: "https://full.test/v1", Chat: "https://full.test/v1",
	}}
	cfg.Providers["partial"] = domain.Provider{Label: "Partial", Endpoints: domain.Endpoints{Anthropic: "https://partial.test", Chat: "https://partial.test/v1"}}
	cfg.Providers["unreadable"] = domain.Provider{Label: "Unreadable", Endpoints: domain.Endpoints{Responses: "https://unreadable.test/v1"}}
	cfg.Providers["notoken"] = domain.Provider{Label: "No Token", Endpoints: domain.Endpoints{Anthropic: "https://notoken.test"}}
	anthropic := map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}
	for _, model := range []domain.Model{
		{Provider: "full", ID: "full-a", Label: "Full Sonnet"},
		{Provider: "full", ID: "full-old", Deprecated: true},
		{Provider: "full", ID: "full-gone"},
		{Provider: "full", ID: "full-gone-old", Deprecated: true},
		{Provider: "partial", ID: "partial-a"},
		{Provider: "partial", ID: "partial-gone"},
	} {
		model.Protocols = anthropic
		cfg.Models[model.Ref()] = model
	}
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	secrets := secret.NewMemoryStore()
	for _, id := range []string{"full", "partial", "unreadable"} {
		if err := secrets.Set(id, id+"-token"); err != nil {
			t.Fatal(err)
		}
	}
	catalogs := &providerCatalogs{
		lists:    map[string][]string{"full.test": {"full-new", "full-old", "full-a"}, "partial.test": {"partial-a"}, "unreadable.test": {"other"}},
		status:   map[string]int{"bearer https://partial.test/v1/models": http.StatusServiceUnavailable, "bearer https://unreadable.test/v1/models": http.StatusNotFound},
		bodies:   map[string]string{},
		failures: map[string]error{},
	}
	out := &bytes.Buffer{}
	return searchFixture{
		runtime: invocation.Context{Clients: clients.Registry(), Config: configStore, Secrets: secrets, HTTP: catalogs, Out: out},
		out:     out, catalogs: catalogs,
	}
}

// search runs `aigw model search <args> --json` and decodes its document.
func (f searchFixture) search(t *testing.T, args ...string) (searchDocument, error) {
	t.Helper()
	err := f.run(append([]string{"search", "--json"}, args...)...)
	var document searchDocument
	if decodeErr := json.Unmarshal(f.out.Bytes(), &document); decodeErr != nil {
		t.Fatalf("output is not one JSON document: %v\n%s", decodeErr, f.out)
	}
	return document, err
}

// provider returns the result of one provider.
func (document searchDocument) provider(t *testing.T, id string) searchProvider {
	t.Helper()
	for _, provider := range document.Providers {
		if provider.ID == id {
			return provider
		}
	}
	t.Fatalf("no result for %s in %+v", id, document)
	return searchProvider{}
}

// rows renders a provider's models as id=state[listed by], for comparison.
func (provider searchProvider) rows() string {
	rows := make([]string, 0, len(provider.Models))
	for _, model := range provider.Models {
		rows = append(rows, model.ID+"="+model.State+strings.Join(model.ListedBy, "+"))
	}
	return strings.Join(rows, " ")
}

func TestSearchMergesEveryProtocolSourceIntoOneRowPerModel(t *testing.T) {
	f := newSearchFixture(t)
	document, err := f.search(t, "--provider", "full")
	if err != nil {
		t.Fatal(err)
	}
	full := document.provider(t, "full")
	// Added models come in configuration order, then the others by ID. A
	// model that the provider no longer lists is missing even when it is
	// deprecated; one that it lists and that is deprecated is deprecated.
	want := "full/full-a=addedanthropic+responses+chat full/full-gone=missing full/full-gone-old=missing full/full-old=deprecatedanthropic+responses+chat " +
		"full/full-new=availableanthropic+responses+chat"
	if full.rows() != want || full.Problem != "" || len(full.Sources) != 3 || full.Sources[2].Status != "ok" || document.NextDetail != "aigw model add <provider/model>" {
		t.Fatalf("full = %s\n%+v", full.rows(), document)
	}
	if label := full.Models[0].Label; label != "Full Sonnet" || full.Models[1].Label != "Full · full-gone" || full.Models[4].Label != "" {
		t.Fatalf("labels = %q, %q", label, full.Models[4].Label)
	}
	// Responses and Chat Completions share one URL, so one read serves both.
	if requests := strings.Join(f.catalogs.requests, ","); len(f.catalogs.requests) != 2 ||
		!strings.Contains(requests, "x-api-key https://full.test/v1/models") || !strings.Contains(requests, "bearer https://full.test/v1/models") {
		t.Fatalf("requests = %s", requests)
	}
	if document.Providers[0].ID != "full" || len(document.Providers) != 1 {
		t.Fatalf("providers = %+v", document.Providers)
	}
	// Every word of the query matches the ID or the label, ignoring case.
	document, _ = f.search(t, "--provider", "full", "SONNET")
	if rows := document.provider(t, "full").rows(); rows != "full/full-a=addedanthropic+responses+chat" {
		t.Fatalf("a label query = %s", rows)
	}
	document, _ = f.search(t, "--provider", "full", "full", "new")
	if rows := document.provider(t, "full").rows(); rows != "full/full-new=availableanthropic+responses+chat" {
		t.Fatalf("a two-word query = %s", rows)
	}
}

func TestSearchShowsAPartialReadAndMarksNothingMissing(t *testing.T) {
	f := newSearchFixture(t)
	document, err := f.search(t, "--provider", "partial")
	if err == nil {
		t.Fatal("a partial read succeeded")
	}
	partial := document.provider(t, "partial")
	if partial.rows() != "partial/partial-a=addedanthropic" || partial.Problem != "OpenAI Chat Completions: HTTP 503" ||
		partial.NextAction != "aigw model search --json --provider partial" || document.NextAction != partial.NextAction {
		t.Fatalf("partial = %s\n%+v", partial.rows(), partial)
	}
	if !strings.Contains(f.out.String(), `"ok": false`) {
		t.Fatalf("a partial read's document = %s", f.out)
	}
}

func TestSearchGivesEachUnreadableCatalogItsFix(t *testing.T) {
	retry := "aigw model search --json --provider unreadable"
	edit := "aigw provider edit unreadable --endpoint responses=<url>"
	request := "bearer https://unreadable.test/v1/models"
	for _, test := range []struct {
		name, problem, fix string
		status             int
		body               string
		failure            error
	}{
		{name: "wrong endpoint", status: http.StatusNotFound, problem: "OpenAI Responses: HTTP 404", fix: edit},
		{name: "rejected Token", status: http.StatusUnauthorized, problem: "OpenAI Responses: the provider rejected the Token (HTTP 401)", fix: "aigw provider rotate unreadable"},
		{name: "server error", status: http.StatusInternalServerError, problem: "OpenAI Responses: HTTP 500", fix: retry},
		{name: "not a catalog", status: http.StatusOK, body: `{}`, problem: "OpenAI Responses: the response is not a model catalog", fix: edit},
		{name: "no answer", failure: errors.New("offline"), problem: "OpenAI Responses: the request did not complete", fix: retry},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newSearchFixture(t)
			f.catalogs.status[request] = test.status
			if test.body != "" {
				f.catalogs.bodies[request] = test.body
			}
			f.catalogs.failures[request] = test.failure
			document, err := f.search(t, "--provider", "unreadable")
			unreadable := document.provider(t, "unreadable")
			if err == nil || unreadable.Problem != test.problem || unreadable.NextAction+unreadable.NextDetail != test.fix || len(unreadable.Models) != 0 {
				t.Fatalf("unreadable = %+v, %v", unreadable, err)
			}
		})
	}
}

// An ambiguous catalog establishes no membership, so the provider's added
// models are not reported missing, while an empty list is a complete read.
func TestSearchTrustsOnlyAnUnambiguousCatalog(t *testing.T) {
	request := "bearer https://unreadable.test/v1/models"
	for body, want := range map[string]string{
		`{"data":null}`:                        "",
		`{"data":[{"id":"a","id":"b"}]}`:       "",
		`{"data":[],"data":[]}`:                "",
		`{"data":[],"object":"list"}`:          "ok",
		`{"data":[{"id":"x","owned_by":"y"}]}`: "ok",
	} {
		f := newSearchFixture(t)
		f.catalogs.status[request], f.catalogs.bodies[request] = http.StatusOK, body
		document, _ := f.search(t, "--provider", "unreadable")
		unreadable := document.provider(t, "unreadable")
		if got := unreadable.Sources[0].Status; want == "ok" && got != "ok" || want == "" && (got != "request_failed" || unreadable.Problem == "") {
			t.Fatalf("%s: source = %+v", body, unreadable)
		}
	}
	f := newSearchFixture(t)
	f.catalogs.bodies["x-api-key https://partial.test/v1/models"] = `{"data":null}`
	document, _ := f.search(t, "--provider", "partial")
	if partial := document.provider(t, "partial"); partial.rows() != "" || !strings.Contains(partial.Problem, "Anthropic Messages: the response is not a model catalog") {
		t.Fatalf("an ambiguous catalog marked a model = %+v", partial)
	}
}

func TestSearchNamesTheFixOfAMissingToken(t *testing.T) {
	f := newSearchFixture(t)
	document, err := f.search(t)
	if err == nil {
		t.Fatal("a search with a provider that has no Token succeeded")
	}
	notoken := document.provider(t, "notoken")
	if notoken.Problem != "Anthropic Messages: no Token is stored" || notoken.NextDetail != "aigw provider rotate notoken" || len(notoken.Models) != 0 {
		t.Fatalf("notoken = %+v", notoken)
	}
	// The other providers still print, in configuration order, and the first
	// failure's fix is the document's.
	if len(document.Providers) != 4 || document.provider(t, "full").rows() == "" || document.NextDetail != "aigw provider rotate notoken" {
		t.Fatalf("search = %+v", document)
	}
	f.runtime.Secrets = environmentBackend(func(string) string { return "" })
	document, _ = f.search(t, "--provider", "notoken")
	if notoken := document.provider(t, "notoken"); notoken.Problem != "Anthropic Messages: no Token: AIGW_TOKEN_NOTOKEN is not set" ||
		notoken.NextDetail != "Set AIGW_TOKEN_NOTOKEN in aigw's environment." {
		t.Fatalf("an environment without the Token = %+v", notoken)
	}
	f.runtime.Secrets = environmentBackend(func(string) string { return "unreadable-token" })
	f.catalogs.status["bearer https://unreadable.test/v1/models"] = http.StatusForbidden
	document, _ = f.search(t, "--provider", "unreadable")
	if unreadable := document.provider(t, "unreadable"); unreadable.NextDetail != "Set AIGW_TOKEN_UNREADABLE to a Token that the provider accepts, then run `aigw model search --provider unreadable`." {
		t.Fatalf("an environment Token that is rejected = %+v", unreadable)
	}
	f.runtime.Secrets = failingSecrets{}
	document, _ = f.search(t, "--provider", "notoken")
	if notoken := document.provider(t, "notoken"); notoken.Problem != "Anthropic Messages: the credential backend could not tell whether a Token is stored" ||
		notoken.NextAction != "aigw provider show notoken" {
		t.Fatalf("a failed backend = %+v", notoken)
	}
}

func TestSearchRefusesBeforeReadingAnyCatalog(t *testing.T) {
	f := newSearchFixture(t)
	if p := problem(f.run("search", "--provider", "fulx")); p.Title != `unknown provider "fulx"; did you mean full?` || p.Fix != "aigw provider list" {
		t.Fatalf("unknown provider = %+v", p)
	}
	f.runtime.Config = store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	if p := problem(f.run("search")); p.Title != "no provider is configured" || p.Fix != "aigw setup" {
		t.Fatalf("no provider = %+v", p)
	}
	if len(f.catalogs.requests) != 0 {
		t.Fatalf("a refusal sent %q", f.catalogs.requests)
	}
}

func TestSearchHumanOutput(t *testing.T) {
	f := newSearchFixture(t)
	if err := f.run("search", "--provider", "full", "--provider", "partial"); err == nil {
		t.Fatal("a partial read succeeded")
	}
	text := f.text()
	for _, want := range []string{"AIGW Model search", "Full ✓ full/full-a added · Full Sonnet", "! full/full-old deprecated", "! full/full-gone missing",
		"· full/full-new available", "Partial ✓ partial/partial-a added", "✗ Not read OpenAI Chat Completions: HTTP 503 Fix: aigw model search --provider full --provider partial",
		"6 models from 2 providers", "Next aigw model search --provider full --provider partial"} {
		if !strings.Contains(text, want) {
			t.Fatalf("output lacks %q: %s", want, text)
		}
	}
	if err := f.run("search", "nothing-matches", "--provider", "full"); err != nil {
		t.Fatal(err)
	}
	if text := f.text(); !strings.Contains(text, "Full · No model matches") || !strings.Contains(text, "0 models from 1 provider Next aigw model add <provider/model>") {
		t.Fatalf("no match = %s", text)
	}
}

// failingSecrets is a credential backend that fails every operation.
type failingSecrets struct{}

func (failingSecrets) Get(string) (string, error)  { return "", errors.New("backend failed") }
func (failingSecrets) Set(string, string) error    { return errors.New("backend failed") }
func (failingSecrets) Delete(string) error         { return errors.New("backend failed") }
func (failingSecrets) Exists(string) (bool, error) { return false, errors.New("backend failed") }

// authentication passes each catalog request to catalogs and records the URL
// of every request whose authentication is not its provider's Token in
// exactly one header, the API key or the bearer, as its protocol sends it.
type authentication struct {
	catalogs *providerCatalogs
	mu       sync.Mutex
	wrong    []string
}

func (check *authentication) Do(request *http.Request) (*http.Response, error) {
	token := strings.TrimSuffix(request.URL.Hostname(), ".test") + "-token"
	key, bearer := request.Header.Get("X-Api-Key"), request.Header.Get("Authorization")
	if (key != token && bearer != "Bearer "+token) || (key != "" && bearer != "") {
		check.mu.Lock()
		check.wrong = append(check.wrong, request.URL.String())
		check.mu.Unlock()
	}
	return check.catalogs.Do(request)
}

// A search only reads: each catalog request carries its provider's Token in
// one header, the configuration keeps its bytes and every Token stays, and
// no output, human or JSON, holds a Token or an authorization header.
func TestSearchOnlyReads(t *testing.T) {
	f := newSearchFixture(t)
	check := &authentication{catalogs: f.catalogs}
	f.runtime.HTTP = check
	before, err := os.ReadFile(f.runtime.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{"full", "partial", "unreadable"}
	for _, args := range [][]string{{"search"}, {"search", "--json"}} {
		_ = f.run(args...)
		shown := strings.Contains(strings.ToLower(f.out.String()), "authorization")
		for _, id := range ids {
			shown = shown || strings.Contains(f.out.String(), id+"-token")
		}
		if shown {
			t.Fatalf("%v showed a Token or its header:\n%s", args, f.out)
		}
	}
	if len(f.catalogs.requests) == 0 || len(check.wrong) != 0 {
		t.Fatalf("requests %q; with the wrong authentication: %q", f.catalogs.requests, check.wrong)
	}
	if after, err := os.ReadFile(f.runtime.Config.Path()); err != nil || !bytes.Equal(after, before) {
		t.Fatalf("the search changed the configuration: %v", err)
	}
	for _, id := range ids {
		if token, err := f.runtime.Secrets.Get(id); err != nil || token != id+"-token" {
			t.Fatalf("the Token of %s = %q, %v", id, token, err)
		}
	}
}

func TestSearchReportsWhatItCannotLoadOrWrite(t *testing.T) {
	f := newSearchFixture(t)
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte("version = [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	broken := f
	broken.runtime.Config = store.NewStore(path, clients.Admission())
	if err := broken.run("search"); err == nil || !strings.Contains(err.Error(), "parse") {
		t.Fatalf("a configuration that does not parse = %v", err)
	}
	for _, args := range [][]string{{"search", "--provider", "full"}, {"search", "--provider", "full", "--json"}} {
		failing := f
		failing.runtime.Out = failingWriter{}
		root := &cobra.Command{Use: "aigw", SilenceErrors: true, SilenceUsage: true}
		root.AddCommand(NewCommand(failing.runtime))
		root.SetArgs(append([]string{"model"}, args...))
		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("%q with a failing output = %v", args, err)
		}
	}
}

// An observation without an endpoint names the provider's endpoint as its fix.
func TestSearchNamesAProviderWithoutAnEndpoint(t *testing.T) {
	f := newSearchFixture(t)
	command, _, _ := NewCommand(f.runtime).Find([]string{"search"})
	reason, fix := sourceProblem(command, f.runtime, "full", app.CatalogObservation{Status: app.CatalogEndpointUnavailable})
	if label := sourceLabel(""); label != "No endpoint" || reason != "no endpoint" || fix != "aigw provider edit full --endpoint <protocol>=<url>" {
		t.Fatalf("no endpoint = %q, %q, %q", label, reason, fix)
	}
}

// failingWriter refuses every write.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

// environmentBackend is the read-only environment backend that getenv
// supplies, as an invocation selects it.
func environmentBackend(getenv func(string) string) secret.Store {
	store, err := secret.Select(secret.Selection{Backend: "env", Getenv: getenv})
	if err != nil {
		panic(err)
	}
	return store
}
