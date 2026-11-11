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
	if p.Title != "provider mixed was not removed, and the Token stays deleted" || p.Fix != "aigw provider remove mixed" ||
		p.Evidence != "The balance credentials could not be deleted: backend failed. The Token stays deleted: backend failed." || !stub.deleted {
		t.Fatalf("an unrestored Token = %+v", p)
	}
	// A slot that holds nothing is left alone, a deletion that fails deletes
	// nothing, and a value that cannot be read is deleted without an undo.
	for _, test := range []struct {
		stub    *stubSecrets
		deleted bool
		fails   bool
		undo    bool
	}{
		{&stubSecrets{getErr: secret.ErrNotFound}, false, false, false},
		{&stubSecrets{value: "sk-mixed-token-0001", deleteErr: errors.New("backend failed")}, false, true, false},
		{&stubSecrets{value: "sk-mixed-token-0001"}, true, false, true},
		{&stubSecrets{value: "sk-mixed-token-0001", getErr: errors.New("unreadable")}, true, false, false},
	} {
		change, deleted, err := deleteSlot(test.stub.Get, test.stub.Set, test.stub.Delete, "mixed", "the Token was put back", "the Token stays deleted")
		if deleted != test.deleted || (err != nil) != test.fails || (change.undo != nil) != test.undo {
			t.Fatalf("deleteSlot(%+v) = %+v, %t, %v", test.stub, change, deleted, err)
		}
	}
	// The environment backend's variables are each checked on their own.
	for _, test := range []struct {
		variables map[string]string
		unset     string
		row       string
	}{
		{map[string]string{"AIGW_BALANCE_TOKEN_MIXED": "system", "AIGW_BALANCE_USER_ID_MIXED": "9"},
			"AIGW_BALANCE_TOKEN_MIXED,AIGW_BALANCE_USER_ID_MIXED", "Balance token Still set in the environment; unset AIGW_BALANCE_TOKEN_MIXED and AIGW_BALANCE_USER_ID_MIXED"},
		{map[string]string{"AIGW_BALANCE_TOKEN_MIXED": "system"},
			"AIGW_BALANCE_TOKEN_MIXED", "Balance token Still set in the environment; unset AIGW_BALANCE_TOKEN_MIXED"},
	} {
		f = newFixture(t)
		f.withEnvironment(test.variables)
		if err := f.run("", "remove", "mixed", "--dry-run", "--json"); err != nil {
			t.Fatal(err)
		}
		var document removeDocument
		f.decode(t, &document)
		if err := f.run("", "remove", "mixed", "--dry-run"); err != nil {
			t.Fatal(err)
		}
		if strings.Join(document.Unset, ",") != test.unset || !strings.Contains(f.text(), test.row) || !strings.Contains(f.text(), "Token None stored") {
			t.Fatalf("variables to unset = %q, output %s", document.Unset, f.text())
		}
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

// stubDiagnostics is a balance credential backend that fails as configured,
// and holds nothing otherwise.
type stubDiagnostics struct{ getErr, deleteErr error }

func (s *stubDiagnostics) Get(string) (secret.DiagnosticCredential, error) {
	if s.getErr != nil {
		return secret.DiagnosticCredential{}, s.getErr
	}
	return secret.DiagnosticCredential{}, secret.ErrNotFound
}
func (*stubDiagnostics) Set(string, secret.DiagnosticCredential) error { return nil }
func (s *stubDiagnostics) Delete(string) error                         { return s.deleteErr }
func (*stubDiagnostics) Exists(string) (bool, error)                   { return true, nil }
