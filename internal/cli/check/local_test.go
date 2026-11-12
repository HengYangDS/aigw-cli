package check

import (
	"errors"
	"io/fs"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	domainreadiness "aigw-cli/internal/app/readiness"
)

// recordingDiscovery records the clients that each discovery observes, and
// finds none of them installed.
type recordingDiscovery struct{ calls *[][]string }

func (d recordingDiscovery) Discover(clients ...string) discovery.Result {
	*d.calls = append(*d.calls, slices.Clone(clients))
	return discovery.Result{}
}

// A variable that competes with AIGW's Token fails the local depth by the
// name that the environment spells it, in any case where names ignore case,
// as on Windows; where they do not, another spelling hands a client nothing.
func TestEnvironmentCheckNamesACompetingVariableAsTheClientReadsIt(t *testing.T) {
	environment := []string{"PATH=/usr/bin", "anthropic_api_key=sk-other", "Openai_Api_Key=sk-other", "ANTHROPIC_AUTH_TOKEN="}
	defer func(previous bool) { namesIgnoreCase = previous }(namesIgnoreCase)
	namesIgnoreCase = true
	got := environmentCheck(environment)
	if got.State != checkFailed || got.Detail != "Openai_Api_Key and anthropic_api_key are set in the environment, which a client may send instead of the Token that AIGW gives it" ||
		got.fix != "Remove Openai_Api_Key and anthropic_api_key from the environment that starts your clients." {
		t.Fatalf("where names ignore case: %+v", got)
	}
	namesIgnoreCase = false
	if got := environmentCheck(environment); got.State != checkPassed {
		t.Fatalf("where names keep their case: %+v", got)
	}
	if got := environmentCheck([]string{"ANTHROPIC_API_KEY=sk-other"}); got.State != checkFailed || !strings.HasPrefix(got.Detail, "ANTHROPIC_API_KEY is set") {
		t.Fatalf("the exact name: %+v", got)
	}
}

// A check of Codex discovers Codex alone, whether its files wait for it to be
// installed or one of its configuration files is missing: it looks for no
// other client's executable or files, although Claude Code is enabled too,
// and sends no request for a client whose local depth failed.
func TestOneClientCheckDiscoversThatClientAlone(t *testing.T) {
	for _, executable := range []string{"", "/opt/codex"} {
		t.Run("executable="+executable, func(t *testing.T) {
			runtime, cfg, _ := configuredRuntime(t)
			cfg.SetClientActivation(codex.ID, true, executable, []string{filepath.Join(t.TempDir(), "missing", "config.toml")})
			cfg.SetClientActivation(claude.ID, true, "", nil)
			save(t, runtime, cfg)
			var calls [][]string
			runtime.Discovery = recordingDiscovery{calls: &calls}
			runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
				t.Fatalf("a check of a client whose files wait sent %s", request.URL)
				return nil, nil
			})
			if err := execute(runtime, codex.ID, "--json"); err == nil {
				t.Fatal("a check of a client without its files passed")
			}
			if len(calls) == 0 {
				t.Fatal("the check did not look for Codex")
			}
			for _, call := range calls {
				if !slices.Equal(call, []string{codex.ID}) {
					t.Fatalf("a one-client check discovered %q; calls = %q", call, calls)
				}
			}
		})
	}
}

// With no client enabled and a credential backend that cannot say which
// provider has a Token, the fix acts on the backend: activation's own step,
// this check's local depth, would fail the same way.
func TestCheckWithoutAClientNamesTheBackendItCannotObserve(t *testing.T) {
	runtime, _, out := configuredRuntime(t)
	secrets(t, runtime).existsErr = errors.New("locked")
	if err := execute(runtime, "--json"); err == nil {
		t.Fatal("a check without an enabled client passed")
	}
	doc := decode(t, out)
	if doc.State != domainreadiness.Unavailable || doc.NextAction != "" || doc.NextDetail != backendFix || len(doc.Clients) != 0 {
		t.Fatalf("a check without a client and an unobservable backend = %+v", doc)
	}
}

// A sandbox that denies the read of AIGW's Token folder gets the generic
// denial fix naming the folder, not a request to unlock a credential store.
func TestCheckNamesADeniedTokenFolder(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	cfg.SetClientActivation(codex.ID, true, "/opt/codex", nil)
	save(t, runtime, cfg)
	runtime.Discovery = recordingDiscovery{calls: new([][]string)}
	tokens := filepath.Join(runtime.Host.Home, "aigw", "secrets", "tokens")
	secrets(t, runtime).existsErr = &fs.PathError{Op: "lstat", Path: tokens, Err: syscall.EPERM}
	if err := execute(runtime, codex.ID, "--json"); err == nil {
		t.Fatal("a check that cannot read the Token folder passed")
	}
	text := out.String()
	for _, want := range []string{"~/aigw/secrets/tokens cannot be read", "forbids this operation on ~/aigw/secrets/tokens; allow it"} {
		if !strings.Contains(text, want) {
			t.Errorf("the document lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "unlock") || strings.Contains(text, runtime.Host.Home) {
		t.Errorf("the document names an unlock or the absolute home:\n%s", text)
	}
}
