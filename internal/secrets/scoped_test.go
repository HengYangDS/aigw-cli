package secrets

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const diagnosticValue = `{"system_token":"platform-system","user_id":"42"}`

func TestCredentialKindSelectionIsIdempotent(t *testing.T) {
	for _, kind := range []Kind{APIToken, ProviderDiagnostic} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			backend := NewMemoryStore()
			first, err := ForKind(backend, kind)
			if err != nil {
				t.Fatal(err)
			}
			again, err := ForKind(first, kind)
			if err != nil {
				t.Fatalf("select the same credential purpose: %v", err)
			}
			if err := again.Set("team", "credential"); err != nil {
				t.Fatal(err)
			}
			if got, err := first.Get("team"); err != nil || got != "credential" {
				t.Fatalf("original view = %q, %v", got, err)
			}
			initial, err := Inspect(backend)
			if err != nil {
				t.Fatal(err)
			}
			if observed, err := Inspect(again); err != nil || observed != initial {
				t.Fatalf("backend observation = %#v, %v; want %#v", observed, err, initial)
			}
			if err := again.Delete("team"); err != nil {
				t.Fatal(err)
			}
			if present, err := first.Exists("team"); err != nil || present {
				t.Fatalf("original view after deletion = %v, %v", present, err)
			}
		})
	}
}

func TestCredentialViewValidatesBeforeBackendResolution(t *testing.T) {
	for _, operation := range []string{"get", "set", "delete", "exists"} {
		t.Run(operation, func(t *testing.T) {
			probes := 0
			store, err := Select(Selection{
				GOOS: runtime.GOOS, Root: filepath.Join(t.TempDir(), "secrets"),
				KeyringProbe: func(Store) error {
					probes++
					return errors.New("isolated file backend")
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			switch operation {
			case "get":
				_, err = store.Get("invalid account")
			case "set":
				err = store.Set("team", "")
			case "delete":
				err = store.Delete("invalid account")
			case "exists":
				_, err = store.Exists("invalid account")
			}
			if err == nil || probes != 0 {
				t.Fatalf("input admission = %v, backend probes = %d; want validation before resolution", err, probes)
			}
		})
	}
}

func TestTypedCredentialViewsShareBackendWithoutSharingSlots(t *testing.T) {
	backend := NewMemoryStore()
	diagnostics, err := ForKind(backend, ProviderDiagnostic)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Set("dmx", "api-token"); err != nil {
		t.Fatal(err)
	}
	if err := diagnostics.Set("dmx", diagnosticValue); err != nil {
		t.Fatal(err)
	}
	if got, err := backend.Get("dmx"); err != nil || got != "api-token" {
		t.Fatalf("API token = %q, %v", got, err)
	}
	if got, err := diagnostics.Get("dmx"); err != nil || got != diagnosticValue {
		t.Fatalf("diagnostic credential = %q, %v", got, err)
	}
	if err := diagnostics.Delete("dmx"); err != nil {
		t.Fatal(err)
	}
	if _, err := diagnostics.Get("dmx"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("diagnostic credential after delete = %v", err)
	}
	if got, err := backend.Get("dmx"); err != nil || got != "api-token" {
		t.Fatalf("diagnostic delete changed API token = %q, %v", got, err)
	}
}

func TestFileBackendPersistsBothCredentialKinds(t *testing.T) {
	backend, err := Select(Selection{Backend: "file", GOOS: runtime.GOOS, Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := ForKind(backend, ProviderDiagnostic)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Set("dmx", "api-token"); err != nil {
		t.Fatal(err)
	}
	if err := diagnostics.Set("dmx", diagnosticValue); err != nil {
		t.Fatal(err)
	}
	if got, err := backend.Get("dmx"); err != nil || got != "api-token" {
		t.Fatalf("API token = %q, %v", got, err)
	}
	if got, err := diagnostics.Get("dmx"); err != nil || got != diagnosticValue {
		t.Fatalf("diagnostic credential = %q, %v", got, err)
	}
}

func TestFileBackendCredentialKindsCannotCollideWithAccountIDs(t *testing.T) {
	root := t.TempDir()
	backend, err := Select(Selection{Backend: "file", GOOS: runtime.GOOS, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := ForKind(backend, ProviderDiagnostic)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Set("diagnostic-dmx", "api-token"); err != nil {
		t.Fatal(err)
	}
	if err := diagnostics.Set("dmx", diagnosticValue); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(root, "tokens", "diagnostic-dmx"),
		filepath.Join(root, "tokens", "diagnostic@dmx"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("credential slot %q: %v", path, err)
		}
	}
	if got, err := backend.Get("diagnostic-dmx"); err != nil || got != "api-token" {
		t.Fatalf("API token = %q, %v", got, err)
	}
	if got, err := diagnostics.Get("dmx"); err != nil || got != diagnosticValue {
		t.Fatalf("diagnostic credential = %q, %v", got, err)
	}
}

func TestForKindRejectsUntypedStoreAndUnknownKind(t *testing.T) {
	if _, err := NewDiagnosticCredentialStore(&faultStore{Store: NewMemoryStore()}); err == nil {
		t.Fatal("untyped diagnostic backend accepted")
	}
	if _, err := ForKind(&faultStore{Store: NewMemoryStore()}, APIToken); err == nil {
		t.Fatal("untyped store accepted")
	}
	if _, err := ForKind(NewMemoryStore(), Kind(255)); err == nil {
		t.Fatal("unknown credential kind accepted")
	}
}
