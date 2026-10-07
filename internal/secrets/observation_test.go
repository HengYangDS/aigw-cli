package secrets

import (
	"errors"
	"testing"
)

func mustExist(t testing.TB, store Store, account string) bool {
	t.Helper()
	present, err := store.Exists(account)
	if err != nil {
		t.Fatalf("Exists(%q): %v", account, err)
	}
	return present
}

func TestKeyringExistsUsesMetadataObserver(t *testing.T) {
	observedService := ""
	observedSlot := ""
	store := scopedView{store: keyringStore{observe: func(service, slot string) (bool, error) {
		observedService = service
		observedSlot = slot
		return true, nil
	}}}

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
	store := scopedView{store: keyringStore{observe: func(string, string) (bool, error) {
		return false, want
	}}}

	present, err := store.Exists("team")
	if present || !errors.Is(err, want) {
		t.Fatalf("Exists() = %v, %v; want false and wrapped observation error", present, err)
	}
}

func TestExistsRejectsInvalidAccountIdentifiers(t *testing.T) {
	stores := []Store{
		NewMemoryStore(),
		NewEnvironmentStore(func(string) string { return "value" }),
		scopedView{store: keyringStore{observe: func(string, string) (bool, error) { return true, nil }}},
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

func TestAvailabilityObservationPreservesPurposeAndLiveReads(t *testing.T) {
	values := map[string]string{
		EnvironmentKey("team"):                      "first-token",
		EnvironmentKey("other"):                     "other-token",
		DiagnosticSystemTokenEnvironmentKey("team"): "system-token",
		DiagnosticUserIDEnvironmentKey("team"):      "user-id",
	}
	lookups := map[string]int{}
	backend := NewEnvironmentStore(func(key string) string {
		lookups[key]++
		return values[key]
	})
	store := ObserveAvailability(backend)
	diagnostics, err := ForKind(store, ProviderDiagnostic)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if !mustExist(t, store, "team") || !mustExist(t, store, "other") || !mustExist(t, diagnostics, "team") {
			t.Fatal("selected credential purpose lost its metadata")
		}
	}
	for key := range values {
		if lookups[key] != 1 {
			t.Fatalf("metadata observations for %q = %d; want one", key, lookups[key])
		}
	}
	for _, value := range []string{"second-token", "third-token"} {
		values[EnvironmentKey("team")] = value
		if got, err := store.Get("team"); err != nil || got != value {
			t.Fatalf("live credential read = %q, %v; want %q", got, err, value)
		}
	}
	for _, view := range []Store{store, diagnostics} {
		observed, err := Inspect(view)
		original, originalErr := Inspect(backend)
		if err != nil || originalErr != nil || observed != original || !IsReadOnly(view) {
			t.Fatalf("metadata scope changed backend identity or read-only policy: %+v, %v", observed, err)
		}
	}
	if err := store.Set("team", "refused"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("environment mutation = %v; want read-only refusal", err)
	}
}

func TestAvailabilityObservationRefreshesAndPreservesFailure(t *testing.T) {
	want := errors.New("metadata authorization denied")
	for _, state := range []struct {
		name    string
		present bool
		err     error
	}{{"present", true, nil}, {"absent", false, nil}, {"error", false, want}} {
		t.Run(state.name, func(t *testing.T) {
			calls := 0
			backend := scopedView{store: keyringStore{observe: func(string, string) (bool, error) {
				calls++
				return state.present, state.err
			}}}
			store := ObserveAvailability(backend)
			for range 2 {
				present, err := store.Exists("team")
				if present != state.present || !errors.Is(err, state.err) {
					t.Fatalf("metadata = %v, %v; want %v, %v", present, err, state.present, state.err)
				}
			}
			if calls != 1 {
				t.Fatalf("one operation queried metadata %d times", calls)
			}
			if _, err := store.Exists("invalid account"); err == nil || calls != 1 {
				t.Fatalf("invalid Account queried backend: %v, calls=%d", err, calls)
			}
			state.present, state.err = true, nil
			if !mustExist(t, ObserveAvailability(store), "team") || calls != 2 {
				t.Fatalf("new operation reused an old observation: calls=%d", calls)
			}
		})
	}
}

func TestAvailabilityObservationInvalidatesBeforeMutation(t *testing.T) {
	want := errors.New("native mutation returned an error")
	for _, operation := range []string{"set", "delete"} {
		for _, outcome := range []struct {
			name string
			err  error
		}{{"success", nil}, {"failure", want}} {
			t.Run(operation+"/"+outcome.name, func(t *testing.T) {
				present := operation == "delete"
				calls := 0
				mutate := func(value bool) error {
					present = value
					return outcome.err
				}
				store := ObserveAvailability(scopedView{store: keyringStore{
					observe: func(string, string) (bool, error) { calls++; return present, nil },
					write:   func(string, string, string) error { return mutate(true) },
					remove:  func(string, string) error { return mutate(false) },
				}})
				for range 2 {
					if mustExist(t, store, "team") != (operation == "delete") {
						t.Fatal("initial observation differs from native metadata")
					}
				}
				var err error
				if operation == "set" {
					err = store.Set("team", "synthetic-token")
				} else {
					err = store.Delete("team")
				}
				if !errors.Is(err, outcome.err) || mustExist(t, store, "team") != present || calls != 2 {
					t.Fatalf("mutation retained stale metadata: present=%v calls=%d err=%v", present, calls, err)
				}
			})
		}
	}
}

func TestAvailabilityObservationSerializesConcurrentMetadata(t *testing.T) {
	calls := 0
	store := ObserveAvailability(scopedView{store: keyringStore{observe: func(string, string) (bool, error) {
		calls++
		return true, nil
	}}})
	results := make(chan bool, 16)
	for range cap(results) {
		go func() {
			present, err := store.Exists("team")
			results <- present && err == nil
		}()
	}
	for range cap(results) {
		if !<-results {
			t.Fatal("concurrent metadata observation failed")
		}
	}
	if calls != 1 {
		t.Fatalf("concurrent operation queried native metadata %d times", calls)
	}
}

func TestAvailabilityObservationRetainsUntypedAndAbsentStores(t *testing.T) {
	if ObserveAvailability(nil) != nil {
		t.Fatal("absent backend gained a metadata scope")
	}
	store := &faultStore{Store: NewMemoryStore()}
	if ObserveAvailability(store) != store {
		t.Fatal("untyped backend was wrapped or replaced")
	}
}
