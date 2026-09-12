package secrets

import (
	"errors"
	"runtime"
	"testing"
)

func TestDiagnosticCredentialStoreLifecycle(t *testing.T) {
	fileBackend, err := Select(Selection{Backend: "file", GOOS: runtime.GOOS, Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	for kind, backend := range map[string]Store{"memory": NewMemoryStore(), "file": fileBackend} {
		t.Run(kind, func(t *testing.T) {
			store, err := NewDiagnosticCredentialStore(backend)
			if err != nil {
				t.Fatal(err)
			}
			want := DiagnosticCredential{SystemToken: "system-secret", UserID: "10000"}
			if err := backend.Set("dmx", "inference-token"); err != nil {
				t.Fatal(err)
			}
			if exists, err := store.Exists("dmx"); err != nil || exists {
				t.Fatalf("API Token substituted for diagnostic slot: %v, %v", exists, err)
			}
			if _, err := store.Get("dmx"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("empty diagnostic slot = %v", err)
			}
			if err := store.Set("dmx", want); err != nil {
				t.Fatal(err)
			}
			got, err := store.Get("dmx")
			if err != nil || got != want {
				t.Fatalf("Get = %#v, %v", got, err)
			}
			reopened, err := NewDiagnosticCredentialStore(backend)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := reopened.Get("dmx"); err != nil || got != want {
				t.Fatalf("shared backend credential = %#v, %v", got, err)
			}
			if err := store.Delete("dmx"); err != nil {
				t.Fatal(err)
			}
			if exists, err := reopened.Exists("dmx"); err != nil || exists {
				t.Fatalf("Exists after delete = %v, %v", exists, err)
			}
			if got, err := backend.Get("dmx"); err != nil || got != "inference-token" {
				t.Fatalf("diagnostic deletion changed API Token = %q, %v", got, err)
			}
		})
	}
}

func TestDiagnosticCredentialStorePreservesBackendErrors(t *testing.T) {
	backendMissing := ErrNotFound
	backend := &faultStore{Store: NewMemoryStore()}
	store := diagnosticCredentialStore{backend: backend}
	if _, err := store.Get("dmx"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing error = %v", err)
	}
	backend.existsErr = backendMissing
	if exists, err := store.Exists("dmx"); err != nil || exists {
		t.Fatalf("missing observation = %v, %v", exists, err)
	}
	backend.existsErr = nil
	want := DiagnosticCredential{SystemToken: "system", UserID: "42"}
	if err := store.Set("dmx", want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("dmx")
	exists, existsErr := store.Exists("dmx")
	if err != nil || got != want || existsErr != nil || !exists {
		t.Fatalf("credential = %#v, get=%v, exists=%v, observation=%v", got, err, exists, existsErr)
	}
	if err := backend.Store.Set("dmx", "not-json"); err != nil {
		t.Fatal(err)
	}
	if exists, err := store.Exists("dmx"); err != nil || !exists {
		t.Fatalf("present malformed slot = %v, %v", exists, err)
	}
	if _, err := store.Get("dmx"); err == nil {
		t.Fatal("malformed credential accepted")
	}
	if err := backend.Store.Set("dmx", `{"system_token":"","user_id":""}`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("dmx"); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("incomplete stored credential = %v, want invalid data rather than absence", err)
	}
	for _, incomplete := range []DiagnosticCredential{{SystemToken: "system"}, {UserID: "42"}} {
		if err := store.Set("dmx", incomplete); err == nil {
			t.Fatalf("incomplete credential accepted: %#v", incomplete)
		}
	}
	wantBackendErr := errors.New("backend failed")
	backend.getErr = wantBackendErr
	if _, err := store.Get("dmx"); !errors.Is(err, wantBackendErr) {
		t.Fatalf("backend Get error = %v", err)
	}
	backend.getErr = nil
	backend.setErr = wantBackendErr
	if err := store.Set("dmx", want); !errors.Is(err, wantBackendErr) {
		t.Fatalf("backend Set error = %v", err)
	}
	backend.setErr = nil
	backend.deleteErr = wantBackendErr
	if err := store.Delete("dmx"); !errors.Is(err, wantBackendErr) {
		t.Fatalf("backend Delete error = %v", err)
	}
	backend.existsErr = wantBackendErr
	if _, err := store.Exists("dmx"); !errors.Is(err, wantBackendErr) {
		t.Fatalf("backend Exists error = %v", err)
	}
}

func TestDiagnosticCredentialStoreValidatesAccountAndCompleteCredential(t *testing.T) {
	store := testDiagnosticCredentialStore(t)
	for _, name := range []string{"bad name", "", "../escape"} {
		if err := store.Set(name, DiagnosticCredential{SystemToken: "s", UserID: "1"}); err == nil {
			t.Errorf("Set(%q) succeeded", name)
		}
		if _, err := store.Get(name); err == nil || errors.Is(err, ErrNotFound) {
			t.Errorf("Get(%q) = %v, want invalid Account identifier", name, err)
		}
		if _, err := store.Exists(name); err == nil {
			t.Errorf("Exists(%q) accepted invalid Account identifier", name)
		}
		if err := store.Delete(name); err == nil {
			t.Errorf("Delete(%q) accepted invalid Account identifier", name)
		}
	}
	if err := store.Set("dmx", DiagnosticCredential{UserID: "10000"}); err == nil {
		t.Fatal("credential without system token accepted")
	}
	if err := store.Set("dmx", DiagnosticCredential{SystemToken: "system-secret"}); err == nil {
		t.Fatal("credential without user ID accepted")
	}
}

func testDiagnosticCredentialStore(t *testing.T) DiagnosticCredentialStore {
	t.Helper()
	store, err := NewDiagnosticCredentialStore(NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestEnvironmentDiagnosticCredentialUsesItsDeclaredPair(t *testing.T) {
	values := map[string]string{
		"AIGW_TOKEN_DMX":                   "api-token",
		"AIGW_DIAGNOSTIC_SYSTEM_TOKEN_DMX": "platform-system",
		"AIGW_DIAGNOSTIC_USER_ID_DMX":      "42",
	}
	backend, err := Select(Selection{Backend: "env", Getenv: func(key string) string { return values[key] }})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := NewDiagnosticCredentialStore(backend)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := diagnostics.Get("dmx"); err != nil || got != (DiagnosticCredential{SystemToken: "platform-system", UserID: "42"}) {
		t.Fatalf("diagnostic credential = %#v, %v", got, err)
	}
	if err := diagnostics.Set("dmx", DiagnosticCredential{SystemToken: "platform-system", UserID: "42"}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Set() error = %v, want ErrReadOnly", err)
	}
	if err := diagnostics.Delete("dmx"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Delete() error = %v, want ErrReadOnly", err)
	}
}

func TestEnvironmentDiagnosticRequiresCompletePair(t *testing.T) {
	for _, values := range []map[string]string{
		{"AIGW_DIAGNOSTIC_SYSTEM_TOKEN_DMX": "platform-system"},
		{"AIGW_DIAGNOSTIC_USER_ID_DMX": "42"},
	} {
		backend := NewEnvironmentStore(func(key string) string { return values[key] })
		diagnostics, err := NewDiagnosticCredentialStore(backend)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := diagnostics.Get("dmx"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("partial diagnostic variables returned %v", err)
		}
	}
}
