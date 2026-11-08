package invocation

import (
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/app/renaming"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"
	"bytes"
	"errors"
	"fmt"
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
	if _, _, err := ResolveRenameIDs(&cobra.Command{Use: "rename"}, runtime, "account", nil); err == nil {
		t.Fatal("invalid configuration was accepted for interactive selection")
	}
}

func TestWriteRenameResultHumanStatuses(t *testing.T) {
	statuses := []struct {
		title string
		plan  renaming.Plan
	}{
		{"Account finalization plan", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "blocked", Finalize: true, Provider: domain.Provider{Label: "New"}, ExternalTODOs: []string{"external action"}}},
		{"Account finalization plan", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "planned", Finalize: true, Provider: domain.Provider{Label: "New"}}},
		{"Account already finalized", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "already_finalized", Finalize: true, Provider: domain.Provider{Label: "New"}}},
		{"Account finalization complete", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "finalized", Finalize: true, Provider: domain.Provider{Label: "New"}}},
		{"Account rename plan", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "planned", Provider: domain.Provider{Label: "New"}}},
		{"Route rename plan", renaming.Plan{Resource: "route", OldID: "old", NewID: "new", Status: "planned", Model: domain.Model{Provider: "account"}, AffectedReferences: []string{"routes.codex"}}},
		{"Account renamed", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "applied", Provider: domain.Provider{Label: "New"}, AffectedReferences: []string{"routes.codex.account"}}},
		{"Route renamed", renaming.Plan{Resource: "route", OldID: "old", NewID: "new", Status: "applied", Model: domain.Model{Provider: "account"}}},
	}
	for _, test := range statuses {
		t.Run(test.title+"/"+string(test.plan.Status), func(t *testing.T) {
			out := &bytes.Buffer{}
			// A blocked plan reports itself and then fails.
			if err := WriteRenamePlan(Context{Clients: clients.Registry(), Out: out}, test.plan, false, false); (err != nil) != (test.plan.Status == renaming.StatusBlocked) {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), test.title) {
				t.Fatalf("rename result lacks %q: %s", test.title, out)
			}
		})
	}
}

func TestProviderRenameContinuationFollowsEnabledClients(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		cfg := domain.NewConfig(clients.Admission())
		cfg.SetClientActivation(claude.ID, enabled, "", nil)
		var out bytes.Buffer
		plan := renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "applied", Config: cfg}
		if err := WriteRenamePlan(Context{Clients: clients.Registry(), Out: &out}, plan, false, false); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "aigw check --depth client") != enabled {
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
	if _, _, err := ResolveRenameIDs(&cobra.Command{Use: "rename"}, Context{Clients: clients.Registry(), Config: store.NewStore(filepath.Join(t.TempDir(), "empty.toml"), clients.Admission()), Interactive: true, Prompt: selectionPrompt{}}, "route", nil); err == nil || !strings.Contains(err.Error(), "No routes") {
		t.Fatalf("error = %v", err)
	}
	want := errors.New("select failed")
	deps := renameRuntime(t)
	deps.Interactive = true
	deps.Prompt = selectionPrompt{selectErr: want}
	if _, _, err := ResolveRenameIDs(&cobra.Command{Use: "rename"}, deps, "route", nil); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	want = errors.New("text failed")
	deps.Prompt = selectionPrompt{selected: "old", textErr: want}
	if _, _, err := ResolveRenameIDs(&cobra.Command{Use: "rename"}, deps, "route", nil); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

// A rename selection or new-ID prompt that ends without an answer, cancelled
// or at the end of its input, names that prompt, says that nothing changed and
// names the same rename command again, which asks again.
func TestResolveRenameIDsNamesACancelledPrompt(t *testing.T) {
	cancelled := fmt.Errorf("%w: no input received", terminal.ErrInputCancelled)
	for _, test := range []struct {
		prompt     selectionPrompt
		args       []string
		title, fix string
	}{
		{prompt: selectionPrompt{selectErr: cancelled}, title: "The route selection was cancelled", fix: "aigw route rename"},
		{prompt: selectionPrompt{selectErr: terminal.ErrNoChoice}, title: "The route selection ended without a choice", fix: "aigw route rename"},
		{prompt: selectionPrompt{textErr: cancelled}, args: []string{"old"}, title: "The new route ID prompt was cancelled", fix: "aigw route rename old"},
	} {
		t.Run(test.title, func(t *testing.T) {
			deps := renameRuntime(t)
			deps.Interactive, deps.Prompt = true, test.prompt
			root := &cobra.Command{Use: "aigw"}
			route := &cobra.Command{Use: "route"}
			rename := &cobra.Command{Use: "rename"}
			root.AddCommand(route)
			route.AddCommand(rename)
			_, _, err := ResolveRenameIDs(rename, deps, "route", test.args)
			problem := terminal.ProblemFor(err)
			if problem.Title != test.title || problem.Impact != "Nothing changed." || problem.Fix != test.fix {
				t.Fatalf("a cancelled rename prompt = %#v", problem)
			}
		})
	}
}
