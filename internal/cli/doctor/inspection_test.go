package doctor

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// inspectedAdapter is a built-in client adapter whose projection inspection
// reports status, as the test decides, without reading native files.
type inspectedAdapter struct {
	adapter.Adapter
	status adapter.Status
}

func (clientAdapter inspectedAdapter) Inspect(context.Context, adapter.Dependencies, domain.Config, domain.Runtime) adapter.Status {
	return clientAdapter.status
}

// inspectClaude gives runtime the built-in client adapters, of which Claude
// Code's inspects its projection as status reports.
func inspectClaude(t *testing.T, runtime *invocation.Context, status adapter.Status) {
	t.Helper()
	registry, err := adapter.NewRegistry(inspectedAdapter{Adapter: claude.New(), status: status}, claudedesktop.New(), codex.New(), hermes.New())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(registry.IDs(), runtime.Clients.IDs()) {
		t.Fatalf("inspected clients = %v, want the built-in clients %v", registry.IDs(), runtime.Clients.IDs())
	}
	runtime.Clients = registry
}

// unobservableSecrets is a credential store whose Token metadata cannot be
// read.
type unobservableSecrets struct{ secret.Store }

func (unobservableSecrets) Exists(string) (bool, error) {
	return false, errors.New("credential metadata is unavailable")
}

// Without a readable configuration, doctor reports no client and fails on the
// configuration.
func TestCommandReportsNoClientWithoutConfiguration(t *testing.T) {
	deps, out, _ := doctorDependencies(t, validDoctorConfig())
	if err := os.Remove(deps.Config.Path()); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(deps.Config.Path(), 0o700); err != nil {
		t.Fatal(err)
	}
	result := executeJSON(t, NewCommand(deps), out)
	if result.OK || len(result.Clients) != 0 || findCheck(t, result.Checks, "config").OK {
		t.Fatalf("doctor result = %#v", result)
	}
}
