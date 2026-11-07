package provider

import (
	"errors"
	"os"
	"strings"
	"testing"

	"aigw-cli/internal/secret"
)

func TestEverySubcommandFailsWhenTheConfigurationDoesNotLoad(t *testing.T) {
	f := newFixture(t)
	if err := os.WriteFile(f.path, []byte("version = [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"list"}, {"show", "team"}, {"add", "fresh", "--endpoint", "chat=https://x.test", "--dry-run"}, {"edit", "team", "--label", "x"},
		{"rotate", "team", "--token-stdin"}, {"balance"}, {"remove", "team"},
	} {
		if err := f.run("sk-x-0001\n", args...); err == nil || !strings.Contains(err.Error(), "parse") {
			t.Fatalf("%q with a configuration that does not parse = %v", args, err)
		}
	}
}

func TestAProviderWithoutModelsIsShownAndRemoved(t *testing.T) {
	f := newFixture(t)
	cfg := f.load(t)
	cfg.Providers["spare"] = cfg.Providers["mixed"]
	if err := f.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := f.run("", "show", "spare"); err != nil || !strings.Contains(f.text(), "Models None added") {
		t.Fatalf("show = %v %s", err, f.text())
	}
	if err := f.run("", "remove", "spare"); err != nil || !strings.Contains(f.text(), "Models None") || !strings.Contains(f.text(), "Token None stored") {
		t.Fatalf("remove = %v %s", err, f.text())
	}
}

func TestBalanceShowsADisabledTokenAndUnlimitedQuotas(t *testing.T) {
	f := newFixture(t)
	f.http.tokens = `{"success":true,"data":{"page_size":100,"items":[{"name":"team","key":"team**********0001","status":2,"unlimited_quota":true,"unlimited_count":true}]}}`
	if err := f.run("", "balance", "team"); err != nil {
		t.Fatal(err)
	}
	if text := f.text(); !strings.Contains(text, "✗ Token status Disabled") || !strings.Contains(text, "Remaining quota Unlimited") ||
		!strings.Contains(text, "Remaining requests Unlimited requests") {
		t.Fatalf("balance = %s", text)
	}
}

func TestCredentialBackendsThatFailPartway(t *testing.T) {
	// A backend that cannot store a Token stores nothing and saves nothing.
	f := newFixture(t)
	f.runtime.Secrets = &stubSecrets{getErr: errors.New("backend failed")}
	if err := f.run("sk-fresh-0001\n", "add", "fresh", "--endpoint", "anthropic=https://fresh.test", "--token-stdin"); err == nil {
		t.Fatal("an addition whose Token was not stored succeeded")
	}
	if _, added := f.load(t).Providers["fresh"]; added {
		t.Fatal("an addition whose Token was not stored saved the provider")
	}
	if err := f.run("sk-team-token-0002\n", "rotate", "team", "--token-stdin"); err == nil {
		t.Fatal("a rotation whose Token was not stored succeeded")
	}
	// A prompt that fails reads no Token.
	f = newFixture(t)
	f.runtime.Interactive, f.runtime.Prompt = true, &prompter{}
	if err := f.run("", "rotate", "team"); err == nil || f.token("team") != teamToken {
		t.Fatalf("a failed prompt = %v", err)
	}
	// Balance credentials that are already gone are removed.
	f = newFixture(t)
	f.runtime.Providers = &stubDiagnostics{deleteErr: secret.ErrNotFound}
	if err := f.run("", "edit", "team", "--no-balance-token"); err != nil {
		t.Fatal(err)
	}
}

func TestRemovalCompensationReportsWhatItCouldNotRestore(t *testing.T) {
	f := newFixture(t)
	stub := &stubSecrets{value: "sk-mixed-token-0001", setErr: errors.New("backend failed")}
	f.runtime.Secrets, f.runtime.Providers = stub, failingDiagnostics{}
	p := problem(f.run("", "remove", "mixed"))
	if p.Impact != "The provider was not removed, and a credential that it deleted first could not be restored." || !stub.deleted {
		t.Fatalf("an unrestored Token = %+v", p)
	}
	// A slot that is gone when it is read needs no restore, and a deletion
	// that fails deletes nothing.
	for _, stub := range []*stubSecrets{{getErr: secret.ErrNotFound}, {value: "sk-mixed-token-0001", deleteErr: errors.New("backend failed")}} {
		restore, err := deleteSlot(stub.Get, stub.Set, stub.Delete, "mixed")
		switch {
		case stub.getErr != nil && (err != nil || restore() != nil):
			t.Fatalf("an absent slot = %v", err)
		case stub.deleteErr != nil && err == nil:
			t.Fatal("a failed deletion returned a restore")
		}
	}
	f = newFixture(t)
	f.withEnvironment(map[string]string{"AIGW_DIAGNOSTIC_SYSTEM_TOKEN_MIXED": "system", "AIGW_DIAGNOSTIC_USER_ID_MIXED": "9"})
	if err := f.run("", "remove", "mixed", "--json"); err != nil {
		t.Fatal(err)
	}
	var document removeDocument
	f.decode(t, &document)
	if strings.Join(document.Unset, ",") != "AIGW_DIAGNOSTIC_SYSTEM_TOKEN_MIXED,AIGW_DIAGNOSTIC_USER_ID_MIXED" {
		t.Fatalf("variables to unset = %q", document.Unset)
	}
}

// stubSecrets is a Token backend that holds one value for every provider and
// fails as configured.
type stubSecrets struct {
	value                     string
	getErr, setErr, deleteErr error
	deleted                   bool
}

func (s *stubSecrets) Get(string) (string, error) {
	if s.getErr != nil {
		return "", s.getErr
	}
	if s.deleted || s.value == "" {
		return "", secret.ErrNotFound
	}
	return s.value, nil
}

func (s *stubSecrets) Set(string, string) error { return s.setErr }

func (s *stubSecrets) Delete(string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleted = true
	return nil
}

func (s *stubSecrets) Exists(string) (bool, error) { return s.value != "" && !s.deleted, s.getErr }

// stubDiagnostics is a balance credential backend that fails as configured.
type stubDiagnostics struct{ deleteErr error }

func (*stubDiagnostics) Get(string) (secret.DiagnosticCredential, error) {
	return secret.DiagnosticCredential{}, secret.ErrNotFound
}
func (*stubDiagnostics) Set(string, secret.DiagnosticCredential) error { return nil }
func (s *stubDiagnostics) Delete(string) error                         { return s.deleteErr }
func (*stubDiagnostics) Exists(string) (bool, error)                   { return true, nil }
