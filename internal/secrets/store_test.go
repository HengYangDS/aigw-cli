package secrets

import (
	"errors"
	"strings"
	"testing"
)

func TestMemoryStoreLifecycle(t *testing.T) {
	store := NewMemoryStore()
	if store.Has("dmx") {
		t.Fatal("new store unexpectedly has secret")
	}
	if err := store.Set("dmx", "top-secret-value"); err != nil {
		t.Fatal(err)
	}
	if !store.Has("dmx") {
		t.Fatal("stored secret not found")
	}
	got, err := store.Get("dmx")
	if err != nil || got != "top-secret-value" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	if err := store.Delete("dmx"); err != nil {
		t.Fatal(err)
	}
	_, err = store.Get("dmx")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing error = %v", err)
	}
}

func TestStoresRejectInvalidProfileAndEmptySecret(t *testing.T) {
	store := NewMemoryStore()
	for _, name := range []string{"bad name", "", "../escape"} {
		if err := store.Set(name, "value"); err == nil {
			t.Errorf("Set(%q) succeeded", name)
		}
	}
	if err := store.Set("dmx", ""); err == nil {
		t.Fatal("empty secret accepted")
	}
}

func TestEnvironmentStoreUsesNormalizedReadOnlyVariable(t *testing.T) {
	env := map[string]string{"AIGW_TOKEN_DMX_TEAM_1": "from-environment"}
	store := NewEnvironmentStore(func(key string) string { return env[key] })
	got, err := store.Get("dmx-team.1")
	if err != nil || got != "from-environment" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	if err := store.Set("dmx-team.1", "new-secret"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Set error = %v", err)
	}
	if err := store.Delete("dmx-team.1"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Delete error = %v", err)
	}
}

func TestErrorsNeverContainSecret(t *testing.T) {
	secret := "sk-this-must-not-leak-anywhere"
	store := NewMemoryStore()
	err := store.Set("bad name", secret)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestSelectStoreRequiresExplicitSupportedBackend(t *testing.T) {
	store, err := Select("env", func(key string) string { return map[string]string{"AIGW_TOKEN_DMX": "ci-secret"}[key] })
	if err != nil {
		t.Fatal(err)
	}
	if value, err := store.Get("dmx"); err != nil || value != "ci-secret" {
		t.Fatalf("environment selection = %q, %v", value, err)
	}
	if _, err := Select("plaintext", func(string) string { return "" }); err == nil || !strings.Contains(err.Error(), "supported") {
		t.Fatalf("unsupported backend error = %v", err)
	}
}
