package secrets

import (
	"errors"
	"testing"
)

func mustExist(t testing.TB, store Store, profile string) bool {
	t.Helper()
	present, err := store.Exists(profile)
	if err != nil {
		t.Fatalf("Exists(%q): %v", profile, err)
	}
	return present
}

func TestKeyringExistsUsesMetadataObserver(t *testing.T) {
	observedService := ""
	observedSlot := ""
	store := KeyringStore{observe: func(service, slot string) (bool, error) {
		observedService = service
		observedSlot = slot
		return true, nil
	}}

	present, err := store.Exists("team")
	if err != nil || !present {
		t.Fatalf("Exists() = %v, %v", present, err)
	}
	if observedService != Service || observedSlot != "team" {
		t.Fatalf("observed %q/%q", observedService, observedSlot)
	}
}

func TestKeyringExistsPreservesObservationFailure(t *testing.T) {
	want := errors.New("credential metadata unavailable")
	store := KeyringStore{observe: func(string, string) (bool, error) {
		return false, want
	}}

	present, err := store.Exists("team")
	if present || !errors.Is(err, want) {
		t.Fatalf("Exists() = %v, %v; want false and wrapped observation error", present, err)
	}
}

func TestExistsRejectsInvalidAccountIdentifiers(t *testing.T) {
	stores := []Store{
		NewMemoryStore(),
		NewEnvironmentStore(func(string) string { return "value" }),
		KeyringStore{observe: func(string, string) (bool, error) { return true, nil }},
		newFileStore(t.TempDir()),
	}
	for _, store := range stores {
		if present, err := store.Exists("invalid account"); err == nil || present {
			t.Errorf("%T Exists() = %v, %v; want validation error", store, present, err)
		}
	}
}

func TestEnvironmentDiagnosticExistsRequiresBothValues(t *testing.T) {
	values := map[string]string{
		DiagnosticSystemTokenEnvironmentKey("team"): "system-token",
	}
	store := NewEnvironmentStore(func(key string) string { return values[key] })
	diagnostics, err := ForKind(store, ProviderDiagnostic)
	if err != nil {
		t.Fatal(err)
	}
	if present, err := diagnostics.Exists("team"); err != nil || present {
		t.Fatalf("partial diagnostic Exists() = %v, %v", present, err)
	}
	values[DiagnosticUserIDEnvironmentKey("team")] = "user-id"
	if present, err := diagnostics.Exists("team"); err != nil || !present {
		t.Fatalf("complete diagnostic Exists() = %v, %v", present, err)
	}
}

func TestMemoryExistsDistinguishesAbsenceFromFailure(t *testing.T) {
	store := NewMemoryStore()
	present, err := store.Exists("team")
	if err != nil || present {
		t.Fatalf("missing Exists() = %v, %v", present, err)
	}
	if err := store.Set("team", "secret"); err != nil {
		t.Fatal(err)
	}
	present, err = store.Exists("team")
	if err != nil || !present {
		t.Fatalf("present Exists() = %v, %v", present, err)
	}
}
