package check

import (
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
)

// A check of every enabled client lists the stored Tokens that no provider
// uses by their providers' IDs, never their values, with the removal that
// cleans each one up, and that changes neither its result nor its next step.
func TestCheckListsStoredTokensThatNoProviderUses(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	enable(t, runtime, &cfg, codex.ID)
	store := secret.NewMemoryStore()
	for id, token := range map[string]string{"one": "sk-one-0001", "orphan": "sk-orphan-0001"} {
		if err := store.Set(id, token); err != nil {
			t.Fatal(err)
		}
	}
	runtime.Secrets = store
	if err := execute(runtime, "--depth", "local", "--json"); err != nil {
		t.Fatalf("check: %v\n%s", err, out)
	}
	doc := decode(t, out)
	if !doc.OK || doc.NextAction+doc.NextDetail != "" || doc.UnusedTokens == nil || doc.UnusedTokens.State != unusedListed ||
		len(doc.UnusedTokens.Providers) != 1 || doc.UnusedTokens.Providers[0] != (unusedToken{Provider: "orphan", NextAction: "aigw provider remove orphan"}) {
		t.Fatalf("the document = %+v, unused %+v", doc, doc.UnusedTokens)
	}
	if strings.Contains(out.String(), "sk-orphan-0001") {
		t.Fatalf("the document shows a Token: %s", out)
	}
	out.Reset()
	if err := execute(runtime, "--depth", "local"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Stored Tokens that no provider uses", "orphan", "aigw provider remove orphan"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("the human check lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out.String(), "sk-orphan-0001") {
		t.Fatalf("the human check shows a Token: %s", out)
	}
}

// A check of named clients observes no other provider's credentials, so it
// lists no stored Token.
func TestCheckOfNamedClientsListsNoStoredToken(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	enable(t, runtime, &cfg, codex.ID)
	store := secret.NewMemoryStore()
	for _, id := range []string{"one", "orphan"} {
		if err := store.Set(id, "sk-fixture-0001"); err != nil {
			t.Fatal(err)
		}
	}
	runtime.Secrets = store
	if err := execute(runtime, codex.ID, "--depth", "local", "--json"); err != nil {
		t.Fatalf("check: %v\n%s", err, out)
	}
	if doc := decode(t, out); doc.UnusedTokens != nil {
		t.Fatalf("a check of named clients listed %+v", doc.UnusedTokens)
	}
}

// A backend that cannot name its Tokens is reported as such, and the rest of
// the check stands: no listing is claimed, and the check still passes.
func TestCheckSaysWhenTheBackendCannotListItsTokens(t *testing.T) {
	runtime, cfg, out := configuredRuntime(t)
	enable(t, runtime, &cfg, codex.ID)
	if err := execute(runtime, "--depth", "local", "--json"); err != nil {
		t.Fatalf("check: %v\n%s", err, out)
	}
	doc := decode(t, out)
	if !doc.OK || doc.UnusedTokens == nil || doc.UnusedTokens.State != unusedCannotList || len(doc.UnusedTokens.Providers) != 0 {
		t.Fatalf("a backend that cannot list = %+v, unused %+v", doc, doc.UnusedTokens)
	}
	out.Reset()
	if err := execute(runtime, "--depth", "local"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Cannot list its Tokens") {
		t.Fatalf("the human check does not say so:\n%s", out)
	}
}

// A check that stops because no client is enabled still names the stored
// Tokens that no provider uses, in its document and its evidence.
func TestCheckWithoutAClientListsStoredTokens(t *testing.T) {
	runtime, _, out := configuredRuntime(t)
	store := secret.NewMemoryStore()
	if err := store.Set("orphan", "sk-orphan-0001"); err != nil {
		t.Fatal(err)
	}
	runtime.Secrets = store
	if err := execute(runtime, "--json"); err == nil {
		t.Fatal("a check without an enabled client passed")
	}
	doc := decode(t, out)
	if doc.UnusedTokens == nil || len(doc.UnusedTokens.Providers) != 1 || doc.UnusedTokens.Providers[0].Provider != "orphan" {
		t.Fatalf("the document = %+v", doc.UnusedTokens)
	}
	want := "A Token is stored for orphan, which no provider uses; `aigw provider remove orphan` removes it."
	if problem := terminal.ProblemFor(execute(runtime)); !strings.Contains(problem.Evidence, want) {
		t.Fatalf("the evidence = %q, want %q", problem.Evidence, want)
	}
	several := unusedReport{Providers: []unusedToken{{Provider: "a"}, {Provider: "b"}}}
	if got := several.evidence(); !strings.Contains(got, "a and b") || !strings.Contains(got, "aigw provider remove <id>") {
		t.Fatalf("the evidence for several = %q", got)
	}
}
