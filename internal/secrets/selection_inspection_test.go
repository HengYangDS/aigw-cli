package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectReportsExplicitBackendCapabilities(t *testing.T) {
	tests := []struct {
		name      string
		selection Selection
		want      BackendSelection
	}{
		{
			name:      "environment",
			selection: Selection{Backend: "env"},
			want: BackendSelection{
				Kind:         "env",
				Availability: "available",
				Mutability:   "read_only",
				Persistence:  "explicit",
			},
		},
		{
			name:      "file",
			selection: Selection{Backend: "file", GOOS: "linux", Root: t.TempDir()},
			want: BackendSelection{
				Kind:         "file",
				Availability: "available",
				Mutability:   "read_write",
				Persistence:  "explicit",
			},
		},
		{
			name: "keyring",
			selection: Selection{
				Backend:      "keyring",
				GOOS:         "linux",
				Root:         t.TempDir(),
				KeyringProbe: func(Store) error { return nil },
			},
			want: BackendSelection{
				Kind:         "keyring",
				Availability: "available",
				Mutability:   "read_write",
				Persistence:  "explicit",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, err := Select(test.selection)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Inspect(store)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("Inspect() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestInspectReportsNilMemoryAndUnknownStores(t *testing.T) {
	tests := []struct {
		name    string
		store   Store
		want    BackendSelection
		wantErr bool
	}{
		{
			name:    "nil",
			want:    BackendSelection{Kind: "unknown", Availability: "unavailable", Mutability: "unknown", Persistence: "unknown", RecoveryAction: "aigw doctor"},
			wantErr: true,
		},
		{
			name:  "memory",
			store: NewMemoryStore(),
			want:  BackendSelection{Kind: "memory", Availability: "available", Mutability: "read_write", Persistence: "ephemeral"},
		},
		{
			name:  "unknown",
			store: untypedStore{},
			want:  BackendSelection{Kind: "unknown", Availability: "available", Mutability: "read_write", Persistence: "explicit"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Inspect(test.store)
			if (err != nil) != test.wantErr {
				t.Fatalf("Inspect() error = %v, wantErr %v", err, test.wantErr)
			}
			if got != test.want {
				t.Fatalf("Inspect() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestInspectAutomaticSelectionIsValueFreeAndDoesNotPersist(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	probeCalls := 0
	store, err := Select(Selection{
		GOOS: "linux",
		Root: root,
		KeyringProbe: func(Store) error {
			probeCalls++
			return errors.New("service unavailable")
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := Inspect(store)
	if err != nil {
		t.Fatal(err)
	}
	want := BackendSelection{
		Kind:         "file",
		Availability: "available",
		Mutability:   "read_write",
		Persistence:  "deferred",
	}
	if got != want {
		t.Fatalf("Inspect() = %#v, want %#v", got, want)
	}
	if probeCalls != 1 {
		t.Fatalf("Inspect() keyring probe calls = %d, want 1", probeCalls)
	}
	if _, err := os.Stat(filepath.Join(root, "backend")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Inspect() persisted automatic selection: %v", err)
	}
}

func TestInspectAutomaticPersistedSelectionDoesNotProbeAnotherBackend(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	if _, _, err := newBackendChoice(root).Persist("file"); err != nil {
		t.Fatal(err)
	}
	store, err := Select(Selection{
		GOOS: "linux",
		Root: root,
		KeyringProbe: func(Store) error {
			t.Fatal("Inspect() probed keyring after file selection was persisted")
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := Inspect(store)
	if err != nil {
		t.Fatal(err)
	}
	want := BackendSelection{Kind: "file", Availability: "available", Mutability: "read_write", Persistence: "persisted"}
	if got != want {
		t.Fatalf("Inspect() = %#v, want %#v", got, want)
	}
}

func TestInspectPreservesBackendIdentityThroughTypedViews(t *testing.T) {
	backend, err := Select(Selection{Backend: "file", GOOS: "linux", Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	view, err := ForKind(backend, ProviderDiagnostic)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Inspect(view)
	if err != nil {
		t.Fatal(err)
	}
	want := BackendSelection{Kind: "file", Availability: "available", Mutability: "read_write", Persistence: "explicit"}
	if got != want {
		t.Fatalf("Inspect(typed view) = %#v, want %#v", got, want)
	}
}

func TestInspectReportsUnavailableBackendAndOneRecoveryAction(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "backend"), []byte("retired\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := Select(Selection{GOOS: "linux", Root: root})
	if err != nil {
		t.Fatal(err)
	}

	got, err := Inspect(store)
	if err == nil || !strings.Contains(err.Error(), "invalid persisted") {
		t.Fatalf("Inspect() error = %v, want invalid persisted selection", err)
	}
	if got.Availability != "unavailable" || got.Mutability != "unknown" || got.Persistence != "unknown" || got.Kind != "unknown" {
		t.Fatalf("Inspect() = %#v, want unavailable unresolved selection", got)
	}
	if got.RecoveryAction != "aigw doctor" {
		t.Fatalf("recovery action = %q, want aigw doctor", got.RecoveryAction)
	}
}
