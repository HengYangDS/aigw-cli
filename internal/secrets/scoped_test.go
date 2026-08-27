package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const diagnosticValue = `{"system_token":"platform-system","user_id":"42"}`

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

func TestEnvironmentBackendOwnsDiagnosticCredentialWithoutKeyringFallback(t *testing.T) {
	values := map[string]string{
		"AIGW_TOKEN_DMX":                   "api-token",
		"AIGW_DIAGNOSTIC_SYSTEM_TOKEN_DMX": "platform-system",
		"AIGW_DIAGNOSTIC_USER_ID_DMX":      "42",
	}
	backend, err := Select(Selection{Backend: "env", Getenv: func(key string) string { return values[key] }})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := ForKind(backend, ProviderDiagnostic)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := diagnostics.Get("dmx"); err != nil || got != diagnosticValue {
		t.Fatalf("diagnostic credential = %q, %v", got, err)
	}
	if err := diagnostics.Set("dmx", diagnosticValue); !errors.Is(err, ErrReadOnly) {
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
		diagnostics, err := ForKind(backend, ProviderDiagnostic)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := diagnostics.Get("dmx"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("partial diagnostic variables returned %v", err)
		}
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
