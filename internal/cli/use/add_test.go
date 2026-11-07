package use

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
)

// catalogServer answers each provider's catalog request with the model IDs
// listed, and records the Token of each request in tokens when it is given.
// The catalog check reads its protocols concurrently, so the record is
// locked. Any other request fails the test.
func catalogServer(t *testing.T, tokens *[]string, listed ...string) doerFunc {
	t.Helper()
	var recorded sync.Mutex
	return func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || !strings.HasSuffix(request.URL.Path, "/models") {
			t.Errorf("a selection sent %s %s", request.Method, request.URL)
			return nil, errors.New("unexpected request")
		}
		if tokens != nil {
			token := request.Header.Get("X-Api-Key")
			if token == "" {
				token = strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
			}
			recorded.Lock()
			*tokens = append(*tokens, token)
			recorded.Unlock()
		}
		data := make([]map[string]string, 0, len(listed))
		for _, id := range listed {
			data = append(data, map[string]string{"id": id})
		}
		body, _ := json.Marshal(map[string]any{"data": data})
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(strings.NewReader(string(body))), Request: request}, nil
	}
}

// addingRuntime is a runtime whose gateway provider has a Token and serves
// Anthropic Messages and OpenAI Responses.
func addingRuntime(t *testing.T) (invocation.Context, domain.Config) {
	t.Helper()
	runtime, cfg, _ := configuredRuntime(t)
	credentials := secret.NewMemoryStore()
	if err := credentials.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runtime.Secrets = credentials
	return runtime, cfg
}

func useDocument(t *testing.T, runtime invocation.Context, args ...string) document {
	t.Helper()
	out := &bytes.Buffer{}
	runtime.Out = out
	if err := execute(runtime, append(args, "--json")...); err != nil {
		t.Fatal(err)
	}
	var result document
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("use document: %v\n%s", err, out.Bytes())
	}
	return result
}

// A model that the provider's catalog lists is added and selected in one
// transaction, with the client's protocol, as its own family and declaring
// no capability; a model ID that the catalog does not list is refused before
// any write with the closest listed ID, and an unreadable catalog with the
// same command and --unlisted.
func TestUseAddsAModelThatTheCatalogLists(t *testing.T) {
	runtime, _ := addingRuntime(t)
	runtime.HTTP = catalogServer(t, nil, "gpt-test", "gpt-fresh")
	before, err := runtime.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	err = execute(runtime, codex.ID, "gateway/gpt-frsh")
	if problem := terminal.ProblemFor(err); !strings.Contains(problem.Title, "the gateway catalog does not list gpt-frsh; did you mean gateway/gpt-fresh?") ||
		problem.Fix != "aigw use codex gateway/gpt-fresh" {
		t.Fatalf("an unlisted model = %#v", problem)
	}
	runtime.HTTP = doerFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: http.NoBody, Request: request}, nil
	})
	err = execute(runtime, codex.ID, "gateway/gpt-fresh")
	if problem := terminal.ProblemFor(err); !strings.Contains(problem.Title, "the gateway catalog cannot be read") || problem.Fix != "aigw use codex gateway/gpt-fresh --unlisted" {
		t.Fatalf("an unreadable catalog = %#v", problem)
	}
	if after, err := runtime.Config.CaptureSnapshot(); err != nil || !after.Config.Equal(before.Config) {
		t.Fatalf("a refused addition wrote the configuration: %v", err)
	}

	runtime.HTTP = catalogServer(t, nil, "gpt-test", "gpt-fresh")
	result := useDocument(t, runtime, codex.ID, "gateway/gpt-fresh")
	if !result.Added || result.CatalogChecked == nil || !*result.CatalogChecked || result.Family != "gpt-fresh" || result.Protocol != "responses" ||
		!result.FilesDeferred || result.NextDetail != "Install Codex if needed, then run `aigw sync`" {
		t.Fatalf("an added model = %#v", result)
	}
	saved, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	key := saved.SelectedModel(codex.ID)
	if model := saved.Models[key]; model.Ref().String() != "gateway/gpt-fresh" || len(model.Protocols) != 1 || len(model.Protocols[domain.ProtocolOpenAIResponses]) != 0 {
		t.Fatalf("the added model = %#v", model)
	}
}

// --unlisted adds a model without reading the catalog, and a preview reads
// no catalog, reports the addition and says that the listing was not
// checked, writing nothing.
func TestUseAddsAnUnlistedModelAndPreviewsAnAddition(t *testing.T) {
	runtime, _ := addingRuntime(t)
	runtime.HTTP = doerFunc(func(request *http.Request) (*http.Response, error) {
		t.Errorf("an unlisted addition or a preview sent %s", request.URL)
		return nil, errors.New("unexpected request")
	})
	before, err := runtime.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	preview := useDocument(t, runtime, codex.ID, "gateway/gpt-private", "--dry-run")
	if !preview.DryRun || !preview.Added || preview.CatalogChecked == nil || *preview.CatalogChecked {
		t.Fatalf("an addition preview = %#v", preview)
	}
	if after, err := runtime.Config.CaptureSnapshot(); err != nil || !after.Config.Equal(before.Config) {
		t.Fatalf("an addition preview wrote the configuration: %v", err)
	}
	result := useDocument(t, runtime, codex.ID, "gateway/gpt-private", "--unlisted")
	if !result.Added || result.CatalogChecked == nil || *result.CatalogChecked {
		t.Fatalf("an unlisted addition = %#v", result)
	}
}

// An added model without any of the client's protocols gains the client's
// protocol after the catalog check, and the result says so; a model takes
// the existing family whose ID equals its model ID, ignoring case, and never
// one whose ID only begins its model ID.
func TestUseGivesAModelTheClientsProtocolAndTakesOnlyTheFamilyWithItsID(t *testing.T) {
	runtime, cfg := addingRuntime(t)
	cfg.Models["chat"] = domain.Model{Label: "Chat", Provider: "gateway", Family: "chat-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Families["minimax-m3"] = domain.Family{Label: "MiniMax M3"}
	cfg.Families["gpt-test"] = domain.Family{Label: "GPT Test"}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runtime.HTTP = catalogServer(t, nil, "chat-test", "MiniMax-M3", "gpt-test-cdx")
	gained := useDocument(t, runtime, codex.ID, "gateway/chat-test")
	if gained.Added || gained.GainedProtocol != "responses" || gained.CatalogChecked == nil || !*gained.CatalogChecked {
		t.Fatalf("a gained protocol = %#v", gained)
	}
	saved, err := runtime.Config.Load()
	if model := saved.Models["chat"]; err != nil || len(model.Protocols) != 2 {
		t.Fatalf("the model after its gain = %#v, %v", model, err)
	}
	if named := useDocument(t, runtime, claude.ID, "gateway/MiniMax-M3"); named.Family != "minimax-m3" {
		t.Fatalf("a model with a family's ID = %#v", named)
	}
	if variant := useDocument(t, runtime, codex.ID, "gateway/gpt-test-cdx"); variant.Family != "gpt-test-cdx" {
		t.Fatalf("a variant joined a family by its prefix: %#v", variant)
	}
}

// In a terminal, a missing Token is asked for and validated first; the
// catalog check of the addition reads it before anything is stored.
func TestUseReadsTheCatalogWithTheTokenThatItAskedFor(t *testing.T) {
	runtime, _ := addingRuntime(t)
	runtime.Secrets = secret.NewMemoryStore()
	runtime.Interactive = true
	runtime.Prompt = &promptStub{secret: "asked-token"}
	var tokens []string
	catalog := catalogServer(t, &tokens, "gpt-fresh")
	runtime.HTTP = doerFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPost {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Request: request}, nil
		}
		return catalog(request)
	})
	if err := execute(runtime, codex.ID, "gateway/gpt-fresh"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(tokens, "asked-token") {
		t.Fatalf("the catalog was read with %q", tokens)
	}
	if stored, err := runtime.Secrets.Get("gateway"); err != nil || stored != "asked-token" {
		t.Fatalf("stored Token = %q, %v", stored, err)
	}
}

// A model that use adds for Claude Code is listed in Claude Desktop's profile
// too, so Desktop's file changes and its tool search, which every listed
// model declared, is off; the result names both, with the command that
// declares the capability.
func TestUseReportsOtherClientsAndALostToolSearch(t *testing.T) {
	runtime, cfg := addingRuntime(t)
	runtime.Executable = filepath.Join(t.TempDir(), "aigw")
	executable := filepath.Join(t.TempDir(), "Claude")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	library := filepath.Join(t.TempDir(), "Claude-3p", "configLibrary")
	runtime.Discovery = staticDiscovery{result: discovery.Result{
		Executables: map[string]string{claudedesktop.ID: executable},
		Surfaces:    []discovery.Surface{{ID: "claude-desktop-config-library", ConfigPath: library, Present: true, AutoManaged: true}},
	}}
	cfg.Models["desktop"] = domain.Model{Label: "Desktop", Provider: "gateway", Family: "claude-test",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {domain.CapabilityToolSearch}}}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := execute(runtime, claudedesktop.ID, "gateway/claude-test"); err != nil {
		t.Fatal(err)
	}
	runtime.HTTP = catalogServer(t, nil, "claude-test", "claude-fresh")
	result := useDocument(t, runtime, claude.ID, "gateway/claude-fresh")
	if !slices.Equal(result.OtherClients, []string{claudedesktop.ID}) || !slices.Equal(result.ToolSearchOff, []string{claudedesktop.ID}) ||
		result.ToolSearchFix != "aigw model edit gateway/claude-fresh --capability tool_search" {
		t.Fatalf("other clients and tool search = %#v", result)
	}
	out := &bytes.Buffer{}
	runtime.Out = out
	runtime.HTTP = catalogServer(t, nil, "claude-test", "claude-fresh", "claude-third")
	if err := execute(runtime, claude.ID, "gateway/claude-third"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Other clients", "Claude Desktop: their files change too"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("human result lacks %q:\n%s", want, out.String())
		}
	}
}
