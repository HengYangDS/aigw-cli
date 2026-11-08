package check

import (
	"net/http"
	"path/filepath"
	"slices"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
)

// recordingDiscovery records the clients that each discovery observes, and
// finds none of them installed.
type recordingDiscovery struct{ calls *[][]string }

func (d recordingDiscovery) Discover(clients ...string) discovery.Result {
	*d.calls = append(*d.calls, slices.Clone(clients))
	return discovery.Result{}
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
