package secrets

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	keyring "github.com/zalando/go-keyring"
)

func TestInspectReportsExplicitBackendCapabilities(t *testing.T) {
	keyring.MockInit()
	tests := []struct {
		name      string
		selection Selection
		store     Store
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
			name:  "keyring",
			store: scopedView{store: mockKeyringStore()},
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
			store := test.store
			if store == nil {
				selected, err := Select(test.selection)
				if err != nil {
					t.Fatal(err)
				}
				store = selected
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
			store: &faultStore{Store: NewMemoryStore()},
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
	for _, platform := range []string{"darwin", "linux", "windows"} {
		for _, backend := range []string{"keyring", "file"} {
			t.Run(platform+"/"+backend, func(t *testing.T) {
				root := filepath.Join(t.TempDir(), "secrets")
				probeCalls := 0
				store, err := Select(Selection{
					GOOS: platform,
					Root: root,
					KeyringProbe: func(Store) error {
						probeCalls++
						if backend == "keyring" && probeCalls == 1 {
							return nil
						}
						return errors.New("service unavailable")
					},
				})
				if err != nil {
					t.Fatal(err)
				}
				if probeCalls != 0 {
					t.Fatalf("Select() probe calls = %d, want lazy observation", probeCalls)
				}
				want := BackendSelection{
					Kind:         backend,
					Availability: "available",
					Mutability:   "read_write",
					Persistence:  "deferred",
				}
				for range 2 {
					got, err := Inspect(store)
					if err != nil {
						t.Fatal(err)
					}
					if got != want {
						t.Fatalf("Inspect() = %#v, want %#v", got, want)
					}
				}
				if probeCalls != 1 {
					t.Fatalf("Inspect() keyring probe calls = %d, want 1", probeCalls)
				}
				if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("Inspect() mutated credential storage: %v", err)
				}
			})
		}
	}
}

func TestInspectObservesExternallyPersistedMatchingChoice(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	probes := 0
	store, err := Select(Selection{
		GOOS: "linux", Root: root,
		KeyringProbe: func(Store) error {
			probes++
			return errors.New("isolated file backend")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := BackendSelection{Kind: "file", Availability: "available", Mutability: "read_write", Persistence: "deferred"}
	if got, err := Inspect(store); err != nil || got != want {
		t.Fatalf("initial observation = %+v, %v; want %+v", got, err, want)
	}
	if _, _, err := newBackendChoice(root).Persist("file"); err != nil {
		t.Fatal(err)
	}
	want.Persistence = "persisted"
	if got, err := Inspect(store); err != nil || got != want {
		t.Fatalf("persisted observation = %+v, %v; want %+v", got, err, want)
	}
	if probes != 1 {
		t.Fatalf("backend capability probes = %d; want one", probes)
	}
	if _, err := os.Stat(filepath.Join(root, "tokens")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inspection created credential storage: %v", err)
	}
}

func TestInspectRechecksChangedChoiceWithoutAdoptingIt(t *testing.T) {
	for _, test := range []struct {
		name        string
		backend     string
		persistence string
		problem     string
	}{
		{"matching choice", "file", "persisted", ""},
		{"removed choice", "", "deferred", ""},
		{"different choice", "keyring", "", "selection changed"},
		{"invalid choice", "invalid", "", "invalid persisted"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "secrets")
			choice := newBackendChoice(root)
			if _, _, err := choice.Persist("file"); err != nil {
				t.Fatal(err)
			}
			store, err := Select(Selection{GOOS: "linux", Root: root, KeyringProbe: func(Store) error {
				t.Fatal("inspection must not select or probe another backend")
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
			want := BackendSelection{Kind: "file", Availability: "available", Mutability: "read_write", Persistence: "persisted"}
			if got, err := Inspect(store); err != nil || got != want {
				t.Fatalf("initial observation = %+v, %v; want %+v", got, err, want)
			}
			marker := filepath.Join(root, backendChoiceName)
			if test.backend == "" {
				err = os.Remove(marker)
			} else {
				err = replaceBackendChoice(choice, test.backend)
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := Inspect(store)
			if test.problem != "" {
				want = BackendSelection{Kind: "unknown", Availability: "unavailable", Mutability: "unknown", Persistence: "unknown", RecoveryAction: "aigw doctor"}
				if err == nil || !strings.Contains(err.Error(), test.problem) || got != want {
					t.Fatalf("conflicting observation = %+v, %v", got, err)
				}
			} else {
				want.Persistence = test.persistence
				if err != nil || got != want {
					t.Fatalf("observation = %+v, %v; want %+v", got, err, want)
				}
			}
			data, readErr := os.ReadFile(marker)
			if test.backend == "" {
				if !errors.Is(readErr, os.ErrNotExist) {
					t.Fatalf("inspection recreated selection: %q, %v", data, readErr)
				}
			} else if readErr != nil || string(data) != test.backend+"\n" {
				t.Fatalf("inspection changed selection: %q, %v", data, readErr)
			}
			if _, err := os.Stat(filepath.Join(root, "tokens")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("inspection created credential storage: %v", err)
			}
		})
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

func TestExplicitKeyringSelectionUsesOnlyTheRequestedNativeOperation(t *testing.T) {
	root := t.TempDir()
	store, err := Select(Selection{
		Backend: "keyring", GOOS: runtime.GOOS, Root: root,
		Executable: filepath.Join(root, "unavailable-native-program"),
	})
	if err != nil {
		t.Fatalf("explicit backend selection performed an unrelated native probe: %v", err)
	}
	if _, ok := backendStore(store).(keyringStore); !ok {
		t.Fatalf("explicit selection = %T, want keyringStore", backendStore(store))
	}
	if _, err := store.Exists("selected-account"); err == nil {
		t.Fatal("unavailable exact native operation did not fail closed")
	}
	inspected, err := Inspect(store)
	if err == nil || inspected.Kind != "keyring" || inspected.Availability != "unavailable" {
		t.Fatalf("explicit native inspection = %+v, %v", inspected, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("explicit native failure created fallback state: %v, %v", entries, err)
	}
}

func TestAutomaticNativeSelectionObservesAvailabilityAndPreservesPersistedChoice(t *testing.T) {
	for _, persisted := range []bool{false, true} {
		t.Run(fmt.Sprintf("persisted=%t", persisted), func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "secrets")
			if persisted {
				if _, _, err := newBackendChoice(root).Persist("keyring"); err != nil {
					t.Fatal(err)
				}
			}
			store, err := Select(Selection{
				GOOS: runtime.GOOS, Root: root,
				Executable: filepath.Join(root, "unavailable-native-program"),
			})
			if err != nil {
				t.Fatalf("automatic selection was not deferred: %v", err)
			}
			observed, err := Inspect(store)
			if persisted {
				if err == nil || observed.Kind != "keyring" || observed.Availability != "unavailable" || observed.Persistence != "persisted" {
					t.Fatalf("persisted native inspection = %+v, %v", observed, err)
				}
				if err := store.Set("selected-account", "fixture-token"); err == nil {
					t.Fatal("persisted native failure silently selected another backend")
				}
				if _, err := os.Stat(filepath.Join(root, "tokens")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("persisted native failure created fallback storage: %v", err)
				}
				return
			}
			if err != nil || observed.Kind != "file" || observed.Availability != "available" || observed.Persistence != "deferred" {
				t.Fatalf("initial automatic inspection = %+v, %v", observed, err)
			}
			if _, err := os.Stat(filepath.Join(root, backendChoiceName)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("inspection persisted a backend choice: %v", err)
			}
			if err := store.Set("selected-account", "fixture-token"); err != nil {
				t.Fatalf("initial native unavailability blocked permitted file storage: %v", err)
			}
			if backend, err := newBackendChoice(root).Read(); err != nil || backend != "file" {
				t.Fatalf("persisted fallback choice = %q, %v", backend, err)
			}
			if value, err := store.Get("selected-account"); err != nil || value != "fixture-token" {
				t.Fatalf("selected fallback credential = %q, %v", value, err)
			}
		})
	}
}
