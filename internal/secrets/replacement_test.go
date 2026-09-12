package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReplacementCompensationOwnsItsAutomaticBackend(t *testing.T) {
	for _, view := range []struct {
		name   string
		kind   Kind
		scoped bool
	}{
		{name: "default"},
		{name: "api-token", kind: APIToken, scoped: true},
		{name: "provider-diagnostic", kind: ProviderDiagnostic, scoped: true},
	} {
		t.Run(view.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "secrets")
			store, err := Select(Selection{
				GOOS: runtime.GOOS, Root: root,
				KeyringProbe: func(Store) error { return errors.New("isolated file backend") },
			})
			if err != nil {
				t.Fatal(err)
			}
			if view.scoped {
				store, err = ForKind(store, view.kind)
				if err != nil {
					t.Fatal(err)
				}
			}
			rollback, err := Replace(store, map[string]string{"one": "written-token"})
			if err != nil {
				t.Fatal(err)
			}
			if err := rollback(); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Get("one"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("credential survived compensation: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, backendChoiceName)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("backend selection survived credential compensation: %v", err)
			}
			if observed, err := Inspect(store); err != nil || observed.Persistence != "deferred" {
				t.Fatalf("compensated backend observation = %+v, %v", observed, err)
			}
			if err := store.Set("one", "later-token"); err != nil {
				t.Fatal(err)
			}
			if err := rollback(); err != nil {
				t.Fatal(err)
			}
			if got, err := store.Get("one"); err != nil || got != "later-token" {
				t.Fatalf("completed compensation changed later credential: %q, %v", got, err)
			}
			if observed, err := Inspect(store); err != nil || observed.Persistence != "persisted" {
				t.Fatalf("later backend observation = %+v, %v", observed, err)
			}
		})
	}
}

func TestReplacementCompensationRetainsItsBackendAndCredentialKind(t *testing.T) {
	for _, test := range []struct {
		name     string
		kind     Kind
		previous string
		readErr  error
	}{
		{name: "new API token", kind: APIToken, readErr: ErrNotFound},
		{name: "existing API token", kind: APIToken, previous: "original-token"},
		{name: "new diagnostic token", kind: ProviderDiagnostic, readErr: ErrNotFound},
		{name: "existing diagnostic token", kind: ProviderDiagnostic, previous: "original-token"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "secrets")
			backend := newFileStore(filepath.Join(root, "tokens"))
			original, err := ForKind(backend, test.kind)
			if err != nil {
				t.Fatal(err)
			}
			if test.previous != "" {
				if err := original.Set("team", test.previous); err != nil {
					t.Fatal(err)
				}
			}
			store, err := Select(Selection{
				GOOS: runtime.GOOS, Root: root,
				KeyringProbe: func(Store) error { return errors.New("isolated file backend") },
			})
			if err != nil {
				t.Fatal(err)
			}
			store, err = ForKind(store, test.kind)
			if err != nil {
				t.Fatal(err)
			}
			rollback, err := Replace(store, map[string]string{"team": "written-token"})
			if err != nil {
				t.Fatal(err)
			}
			if err := replaceBackendChoice(newBackendChoice(root), "keyring"); err != nil {
				t.Fatal(err)
			}
			err = rollback()
			if err == nil || !strings.Contains(err.Error(), "backend selection rollback") {
				t.Fatalf("compensation error = %v; want preserved backend conflict", err)
			}
			value, err := original.Get("team")
			if !errors.Is(err, test.readErr) || value != test.previous {
				t.Fatalf("compensated credential = %q, %v; want %q, %v", value, err, test.previous, test.readErr)
			}
			if observed, err := newBackendChoice(root).Read(); err != nil || observed != "keyring" {
				t.Fatalf("backend choice = %q, %v; want external keyring choice", observed, err)
			}
		})
	}
}

func TestReplacementBatchCompensationRetriesOnlyUnrestoredAccounts(t *testing.T) {
	store := NewMemoryStore()
	for account, token := range map[string]string{"one": "old-one", "two": "old-two"} {
		if err := store.Set(account, token); err != nil {
			t.Fatal(err)
		}
	}
	rollback, err := Replace(store, map[string]string{"one": "new-one", "two": "new-two"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set("two", "foreign-two"); err != nil {
		t.Fatal(err)
	}
	if err := rollback(); err == nil || !strings.Contains(err.Error(), "postimage changed") {
		t.Fatalf("compensation did not preserve drift: %v", err)
	}
	if got, err := store.Get("one"); err != nil || got != "old-one" {
		t.Fatalf("independent Account was not restored: %q, %v", got, err)
	}
	if err := store.Set("one", "later-one"); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("two", "new-two"); err != nil {
		t.Fatal(err)
	}
	if err := rollback(); err != nil {
		t.Fatal(err)
	}
	for account, want := range map[string]string{"one": "later-one", "two": "old-two"} {
		if got, err := store.Get(account); err != nil || got != want {
			t.Fatalf("compensated Account %s = %q, %v", account, got, err)
		}
	}
}

func TestEmptyReplacementDoesNotObserveCredentials(t *testing.T) {
	rollback, err := Replace(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rollback(); err != nil {
		t.Fatal(err)
	}
}

func TestReplacementCompensationOwnsOnlyItsWrittenToken(t *testing.T) {
	for _, test := range []struct {
		name, previous, action, want string
		readErr                      error
		conflict                     bool
	}{
		{name: "restore absent", action: "restore", readErr: ErrNotFound},
		{name: "restore existing", previous: "old-token", action: "restore", want: "old-token"},
		{name: "preserve replacement of new token", action: "replace", want: "newer-token", conflict: true},
		{name: "preserve replacement of old token", previous: "old-token", action: "replace", want: "newer-token", conflict: true},
		{name: "preserve removal of new token", action: "remove", readErr: ErrNotFound, conflict: true},
		{name: "preserve removal of old token", previous: "old-token", action: "remove", readErr: ErrNotFound, conflict: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := NewMemoryStore()
			if test.previous != "" {
				if err := store.Set("one", test.previous); err != nil {
					t.Fatal(err)
				}
			}
			rollback, err := Replace(store, map[string]string{"one": "written-token"})
			if err != nil {
				t.Fatal(err)
			}
			switch test.action {
			case "replace":
				err = store.Set("one", "newer-token")
			case "remove":
				err = store.Delete("one")
			}
			if err != nil {
				t.Fatal(err)
			}
			err = rollback()
			if (err != nil) != test.conflict {
				t.Fatalf("compensation = %v; want conflict=%t", err, test.conflict)
			}
			if err != nil && !strings.Contains(err.Error(), "postimage changed") {
				t.Fatalf("compensation did not diagnose its ownership conflict: %v", err)
			}
			got, err := store.Get("one")
			if got != test.want || !errors.Is(err, test.readErr) {
				t.Fatalf("Token = %q, %v; want %q, %v", got, err, test.want, test.readErr)
			}
			if test.conflict {
				return
			}
			if err := store.Set("one", "later-token"); err != nil {
				t.Fatal(err)
			}
			if err := rollback(); err != nil {
				t.Fatal(err)
			}
			if got, err := store.Get("one"); err != nil || got != "later-token" {
				t.Fatalf("completed compensation changed later Token: %q, %v", got, err)
			}
		})
	}
}

func TestReplacementPartialFailurePreservesCompensationBoundaries(t *testing.T) {
	for _, phase := range []string{"restore", "restore fails", "newer replacement"} {
		t.Run(phase, func(t *testing.T) {
			writeErr := errors.New("second write failed")
			restoreErr := errors.New("restore failed")
			store := &faultStore{Store: NewMemoryStore(), setErrors: map[int]error{2: writeErr}}
			if err := store.Store.Set("first", "original"); err != nil {
				t.Fatal(err)
			}
			want := "original"
			switch phase {
			case "restore fails":
				store.setErrors[3] = restoreErr
				want = "prepared"
			case "newer replacement":
				want = "newer"
				store.onGet = func(store *faultStore, account string) {
					if token, err := store.Store.Get(account); err == nil && token == "prepared" {
						if err := store.Store.Set(account, "newer"); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			rollback, err := Replace(store, map[string]string{"first": "prepared", "second": "new"})
			if rollback != nil || !errors.Is(err, writeErr) {
				t.Fatalf("replacement = %v, want failed batch without rollback receipt", err)
			}
			if phase == "restore fails" && !errors.Is(err, restoreErr) {
				t.Fatalf("compensation failure lost its cause: %v", err)
			}
			if phase == "newer replacement" && !strings.Contains(err.Error(), "postimage changed") {
				t.Fatalf("compensation drift was not reported: %v", err)
			}
			if token, err := store.Store.Get("first"); err != nil || token != want {
				t.Fatalf("first Token = %q, %v; want %q", token, err, want)
			}
			if _, err := store.Store.Get("second"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("failed second write left a Token: %v", err)
			}
		})
	}
}

func TestReplacementReportsStorageFailures(t *testing.T) {
	failure := errors.New("storage unavailable")
	for _, phase := range []string{"read", "write", "observe", "restore", "delete"} {
		t.Run(phase, func(t *testing.T) {
			store := &faultStore{Store: NewMemoryStore()}
			if phase == "restore" {
				if err := store.Set("one", "old-token"); err != nil {
					t.Fatal(err)
				}
			}
			switch phase {
			case "read":
				store.getErr = failure
			case "write":
				store.setErr = failure
			}
			rollback, err := Replace(store, map[string]string{"one": "new-token"})
			if phase == "read" || phase == "write" {
				if !errors.Is(err, failure) || rollback != nil {
					t.Fatalf("replacement = %v, want storage failure without receipt", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			switch phase {
			case "observe":
				store.getErr = failure
			case "restore":
				store.setErr = failure
			case "delete":
				store.deleteErr = failure
			}
			if err := rollback(); !errors.Is(err, failure) {
				t.Fatalf("compensation = %v, want storage failure", err)
			}
		})
	}
}
