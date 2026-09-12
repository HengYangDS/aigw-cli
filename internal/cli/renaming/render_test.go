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
		{"Profile rename plan", renaming.Plan{Resource: "profile", OldID: "old", NewID: "new", Status: "planned", Profile: configuration.Profile{Account: "account"}, AffectedReferences: []string{"routes.codex"}}},
		{"Account renamed", renaming.Plan{Resource: "account", OldID: "old", NewID: "new", Status: "applied", Account: configuration.Account{Label: "New"}, AffectedReferences: []string{"profiles.codex.account"}}},
		{"Profile renamed", renaming.Plan{Resource: "profile", OldID: "old", NewID: "new", Status: "applied", Profile: configuration.Profile{Account: "account"}}},
	}
	for _, test := range statuses {
		t.Run(test.title+"/"+test.plan.Status, func(t *testing.T) {
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
