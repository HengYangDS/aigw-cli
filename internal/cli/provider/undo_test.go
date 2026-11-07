package provider

import (
	"errors"
	"strings"
	"testing"

	"aigw-cli/internal/secret"

	"github.com/spf13/cobra"
)

// changeConfiguration returns a prompt hook that changes the stored
// configuration while a secret is typed, so that the command's save fails.
func (f *fixture) changeConfiguration(t *testing.T) func() {
	return func() {
		cfg := f.load(t)
		provider := cfg.Providers["mixed"]
		provider.Label = "Changed"
		cfg.Providers["mixed"] = provider
		if err := f.runtime.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAddReportsTheTokenThatAFailedSaveLeavesStored(t *testing.T) {
	f := newFixture(t)
	f.runtime.Interactive = true
	f.runtime.Prompt = &prompter{values: []string{"sk-fresh-0001"}, before: f.changeConfiguration(t)}
	p := problem(f.run("", "add", "fresh", "--endpoint", "anthropic=https://fresh.test"))
	if p.Impact != "Nothing was saved; the Token was deleted again." || f.token("fresh") != "" {
		t.Fatalf("an undone addition = %+v", p)
	}
	// A previous Token, which no provider used, is put back.
	f = newFixture(t)
	if err := f.runtime.Secrets.Set("fresh", "sk-stray-0001"); err != nil {
		t.Fatal(err)
	}
	f.runtime.Interactive = true
	f.runtime.Prompt = &prompter{values: []string{"sk-fresh-0001"}, before: f.changeConfiguration(t)}
	if p := problem(f.run("", "add", "fresh", "--endpoint", "anthropic=https://fresh.test")); p.Impact != "Nothing was saved; the previous Token was put back." || f.token("fresh") != "sk-stray-0001" {
		t.Fatalf("a previous Token = %+v, %q", p, f.token("fresh"))
	}
	// A backend that does not keep what it is given cannot take the Token
	// back out, so the failure says that it stays, and the same command
	// converges.
	f = newFixture(t)
	f.runtime.Secrets = &stubSecrets{}
	f.runtime.Interactive = true
	f.runtime.Prompt = &prompter{values: []string{"sk-fresh-0001"}, before: f.changeConfiguration(t)}
	p = problem(f.run("", "add", "fresh", "--endpoint", "anthropic=https://fresh.test"))
	if p.Title != "provider fresh was not added, and the Token stays stored" || p.Fix != "aigw provider add fresh --endpoint anthropic=https://fresh.test" ||
		!strings.HasPrefix(p.Evidence, "Saving the configuration failed: ") || !strings.Contains(p.Evidence, "The Token stays stored: ") {
		t.Fatalf("an unrestored Token = %+v", p)
	}
}

func TestRemoveReportsWhatAFailedSavePutBackOrLeftDeleted(t *testing.T) {
	f := newFixture(t)
	f.credentialMixed(t)
	cfg := f.load(t)
	after := cfg.Clone()
	removeProvider(&after, "mixed")
	stale := cfg.Clone()
	provider := stale.Providers["mixed"]
	provider.Label = "Stale"
	stale.Providers["mixed"] = provider
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	document := removeDocument{Token: credentialPresent, BalanceCredential: credentialPresent}
	p := problem(commitRemoval(cmd, f.runtime, stale, after, "mixed", document))
	if p.Impact != "Nothing was saved; the balance credentials were put back and the Token was put back." {
		t.Fatalf("an undone removal = %+v", p)
	}
	// A Token that cannot be put back stays deleted, and removing the
	// provider again converges.
	stub := &stubSecrets{value: "sk-mixed-token-0001", setErr: errors.New("backend failed")}
	f.runtime.Secrets = stub
	document.BalanceCredential = credentialAbsent
	p = problem(commitRemoval(cmd, f.runtime, stale, after, "mixed", document))
	if p.Title != "provider mixed was not removed, and the Token stays deleted" || p.Fix != "aigw provider remove mixed" || !stub.deleted ||
		!strings.HasSuffix(p.Evidence, "The Token stays deleted: backend failed.") {
		t.Fatalf("an unrestored removal = %+v", p)
	}
	// A Token that could not be read before its deletion cannot be put back.
	f.runtime.Secrets = &stubSecrets{value: "sk-mixed-token-0001", getErr: errors.New("unreadable")}
	p = problem(commitRemoval(cmd, f.runtime, stale, after, "mixed", document))
	if !strings.HasSuffix(p.Evidence, "The Token stays deleted because it could not be read before the change.") {
		t.Fatalf("an unreadable Token = %+v", p)
	}
}

func TestEditReportsTheBalanceCredentialsThatAFailedSaveLeaves(t *testing.T) {
	f := newFixture(t)
	f.runtime.Interactive = true
	f.runtime.Prompt = &prompter{values: []string{"new-system"}, before: f.changeConfiguration(t)}
	p := problem(f.run("", "edit", "team", "--label", "Renamed", "--balance-token-stdin", "--balance-user-id", "7"))
	if p.Impact != "Nothing was saved; the previous balance credentials were put back." {
		t.Fatalf("an undone edit = %+v", p)
	}
	if credential, _ := f.runtime.Providers.Get("team"); credential.SystemToken != "system-token" {
		t.Fatal("the previous balance credentials were not put back")
	}
	// New balance credentials for a provider without any are deleted again,
	// and removed ones are put back.
	f = newFixture(t)
	if err := f.runtime.Providers.Delete("team"); err != nil {
		t.Fatal(err)
	}
	f.runtime.Interactive = true
	f.runtime.Prompt = &prompter{values: []string{"new-system"}, before: f.changeConfiguration(t)}
	if p := problem(f.run("", "edit", "team", "--label", "Renamed", "--balance-token-stdin", "--balance-user-id", "7")); p.Impact != "Nothing was saved; the new balance credentials were deleted again." {
		t.Fatalf("new balance credentials = %+v", p)
	}
	// A backend that refuses to put the previous ones back leaves the new
	// ones, and the same command converges.
	f = newFixture(t)
	f.runtime.Providers = &onceDiagnostics{DiagnosticCredentialStore: f.runtime.Providers}
	f.runtime.Interactive = true
	f.runtime.Prompt = &prompter{values: []string{"new-system"}, before: f.changeConfiguration(t)}
	p = problem(f.run("", "edit", "team", "--label", "Renamed", "--balance-token-stdin", "--balance-user-id", "7"))
	if p.Title != "the edit of provider team was not saved, and the new balance credentials stay stored" ||
		p.Fix != "aigw provider edit team --balance-token-stdin --balance-user-id 7 --label Renamed" {
		t.Fatalf("unrestored balance credentials = %+v", p)
	}
	// Removed balance credentials that cannot be read cannot be put back.
	f = newFixture(t)
	f.runtime.Providers = &stubDiagnostics{getErr: errors.New("unreadable")}
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	changes, err := changeBalance(f.runtime, "team", editOptions{noBalanceToken: true}, secret.DiagnosticCredential{})
	if err != nil || len(changes) != 1 || changes[0].undo != nil {
		t.Fatalf("an unreadable removal = %+v, %v", changes, err)
	}
}

// onceDiagnostics stores balance credentials once and refuses every later
// write, as a backend that fails partway does.
type onceDiagnostics struct {
	secret.DiagnosticCredentialStore
	writes int
}

func (store *onceDiagnostics) Set(id string, credential secret.DiagnosticCredential) error {
	store.writes++
	if store.writes > 1 {
		return errors.New("backend failed")
	}
	return store.DiagnosticCredentialStore.Set(id, credential)
}

// A removal whose save fails puts back what it can and names what stays:
// here the Token comes back and the balance credentials stay deleted.
func TestRemoveReportsAPartlyUndoneRemoval(t *testing.T) {
	f := newFixture(t)
	f.credentialMixed(t)
	f.runtime.Providers = &unrestorableDiagnostics{credential: secret.DiagnosticCredential{SystemToken: "mixed-system", UserID: "9"}}
	cfg := f.load(t)
	after := cfg.Clone()
	removeProvider(&after, "mixed")
	stale := cfg.Clone()
	provider := stale.Providers["mixed"]
	provider.Label = "Stale"
	stale.Providers["mixed"] = provider
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	p := problem(commitRemoval(cmd, f.runtime, stale, after, "mixed", removeDocument{Token: credentialPresent, BalanceCredential: credentialPresent}))
	if p.Title != "provider mixed was not removed, and the balance credentials stay deleted" || p.Impact != "The Token was put back." || f.token("mixed") != "sk-mixed-token-0001" {
		t.Fatalf("a partly undone removal = %+v", p)
	}
	if sentence("") != "" || sentence("the Token") != "The Token" || clauses("; ", nil) != "" {
		t.Fatal("the clause helpers changed")
	}
}

// unrestorableDiagnostics holds one balance credential, deletes it, and
// refuses to store any.
type unrestorableDiagnostics struct {
	credential secret.DiagnosticCredential
	deleted    bool
}

func (store *unrestorableDiagnostics) Get(string) (secret.DiagnosticCredential, error) {
	if store.deleted {
		return secret.DiagnosticCredential{}, secret.ErrNotFound
	}
	return store.credential, nil
}

func (*unrestorableDiagnostics) Set(string, secret.DiagnosticCredential) error {
	return errors.New("backend failed")
}

func (store *unrestorableDiagnostics) Delete(string) error {
	store.deleted = true
	return nil
}

func (store *unrestorableDiagnostics) Exists(string) (bool, error) { return !store.deleted, nil }

// Removing balance credentials that cannot be read deletes them without an
// undo, and one that is already gone when it is deleted is no failure.
func TestChangeBalanceRemovesWhatItCannotRead(t *testing.T) {
	f := newFixture(t)
	f.runtime.Providers = &stubDiagnostics{getErr: errors.New("unreadable"), deleteErr: secret.ErrNotFound}
	changes, err := changeBalance(f.runtime, "team", editOptions{noBalanceToken: true}, secret.DiagnosticCredential{})
	if err != nil || len(changes) != 1 || changes[0].undo != nil {
		t.Fatalf("a removal of unreadable balance credentials = %+v, %v", changes, err)
	}
}
