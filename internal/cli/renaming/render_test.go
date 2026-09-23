package renaming

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/renaming"
	"bytes"
	"strings"
	"testing"
)

func TestWriteRenameResultHumanStatuses(t *testing.T) {
	statuses := []struct {
		title string
		plan  renaming.Plan
	}{
		{"Account finalization plan", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "blocked", Finalize: true, Account: configuration.Account{Label: "New"}, ExternalTODOs: []string{"external action"}}},
		{"Account finalization plan", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "planned", Finalize: true, Account: configuration.Account{Label: "New"}}},
		{"Account already finalized", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "already-finalized", Finalize: true, Account: configuration.Account{Label: "New"}}},
		{"Account finalization complete", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "finalized", Finalize: true, Account: configuration.Account{Label: "New"}}},
		{"Account rename plan", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "planned", Account: configuration.Account{Label: "New"}}},
		{"Route rename plan", renaming.Plan{Resource: "route", OldID: "old", NewID: "new", Status: "planned", Route: configuration.Route{Account: "account"}, AffectedReferences: []string{"routes.codex"}}},
		{"Account renamed", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "applied", Account: configuration.Account{Label: "New"}, AffectedReferences: []string{"routes.codex.account"}}},
		{"Route renamed", renaming.Plan{Resource: "route", OldID: "old", NewID: "new", Status: "applied", Route: configuration.Route{Account: "account"}}},
	}
	for _, test := range statuses {
		t.Run(test.title+"/"+string(test.plan.Status), func(t *testing.T) {
			out := &bytes.Buffer{}
			if err := writeResult(invocation.Context{Out: out}, test.plan, false); err != nil {
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
		cfg := configuration.NewConfig()
		cfg.SetClientActivation(configuration.ClientClaude, enabled, "", nil)
		var out bytes.Buffer
		plan := renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "applied", Config: cfg}
		if err := writeResult(invocation.Context{Out: &out}, plan, false); err != nil {
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
