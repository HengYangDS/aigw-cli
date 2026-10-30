package invocation

import (
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/renaming"
	"aigw-cli/internal/store"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestInteractiveRenamePreservesConfigurationLoadFailure(t *testing.T) {
	runtime := renameRuntime(t)
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte("invalid TOML = ["), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime.Config = store.NewStore(path, clients.Admission())
	runtime.Interactive = true
	runtime.Prompt = selectionPrompt{}
	if _, _, err := ResolveRenameIDs(runtime, "account", nil); err == nil {
		t.Fatal("invalid configuration was accepted for interactive selection")
	}
}

func TestWriteRenameResultHumanStatuses(t *testing.T) {
	statuses := []struct {
		title string
		plan  renaming.Plan
	}{
		{"Account finalization plan", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "blocked", Finalize: true, Account: domain.Account{Label: "New"}, ExternalTODOs: []string{"external action"}}},
		{"Account finalization plan", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "planned", Finalize: true, Account: domain.Account{Label: "New"}}},
		{"Account already finalized", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "already-finalized", Finalize: true, Account: domain.Account{Label: "New"}}},
		{"Account finalization complete", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "finalized", Finalize: true, Account: domain.Account{Label: "New"}}},
		{"Account rename plan", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "planned", Account: domain.Account{Label: "New"}}},
		{"Route rename plan", renaming.Plan{Resource: "route", OldID: "old", NewID: "new", Status: "planned", Route: domain.Route{Account: "account"}, AffectedReferences: []string{"routes.codex"}}},
		{"Account renamed", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "applied", Account: domain.Account{Label: "New"}, AffectedReferences: []string{"routes.codex.account"}}},
		{"Route renamed", renaming.Plan{Resource: "route", OldID: "old", NewID: "new", Status: "applied", Route: domain.Route{Account: "account"}}},
	}
	for _, test := range statuses {
		t.Run(test.title+"/"+string(test.plan.Status), func(t *testing.T) {
			out := &bytes.Buffer{}
			if err := WriteRenamePlan(Context{Clients: clients.Registry(), Out: out}, test.plan, false); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), test.title) {
				t.Fatalf("rename result lacks %q: %s", test.title, out)
			}
		})
	}
}

func TestAccountRenameContinuationFollowsEnabledClients(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		cfg := domain.NewConfig(clients.Admission())
		cfg.SetClientActivation(claude.ID, enabled, "", nil)
		var out bytes.Buffer
		plan := renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "applied", Config: cfg}
		if err := WriteRenamePlan(Context{Clients: clients.Registry(), Out: &out}, plan, false); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "aigw verify --for all") != enabled {
			t.Fatalf("enabled=%t, continuation=%s", enabled, &out)
		}
		if !strings.Contains(out.String(), "aigw account rename old new --finalize") {
			t.Fatalf("retirement continuation missing: %s", &out)
		}
	}
}

func TestResolveRenameIDsErrorBranches(t *testing.T) {
	if err := RenameArguments(Context{Clients: clients.Registry()}, "route")(&cobra.Command{Use: "rename"}, nil); err == nil {
		t.Fatal("expected non-interactive error")
	}
	if err := RenameArguments(Context{Clients: clients.Registry(), Interactive: true}, "route")(&cobra.Command{Use: "rename"}, nil); err == nil {
		t.Fatal("expected missing prompt error")
	}
	if _, _, err := ResolveRenameIDs(Context{Clients: clients.Registry(), Config: store.NewStore(filepath.Join(t.TempDir(), "empty.toml"), clients.Admission()), Interactive: true, Prompt: selectionPrompt{}}, "route", nil); err == nil || !strings.Contains(err.Error(), "No routes") {
		t.Fatalf("error = %v", err)
	}
	want := errors.New("select failed")
	deps := renameRuntime(t)
	deps.Interactive = true
	deps.Prompt = selectionPrompt{selectErr: want}
	if _, _, err := ResolveRenameIDs(deps, "route", nil); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	want = errors.New("text failed")
	deps.Prompt = selectionPrompt{selected: "old", textErr: want}
	if _, _, err := ResolveRenameIDs(deps, "route", nil); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
