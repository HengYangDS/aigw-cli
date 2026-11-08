package secret

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// listingKeyring is the system keyring's double, whose reads fail the test: a
// listing reads names only.
func listingKeyring(t *testing.T, names []string, err error) keyringStore {
	t.Helper()
	return keyringStore{
		read: func(string, string) (string, error) {
			t.Fatal("a listing read a value")
			return "", nil
		},
		enumerate: func(service string) ([]string, error) {
			if service != Service {
				t.Fatalf("listed service %q", service)
			}
			return names, err
		},
	}
}

// The file backend names its Tokens from the directory alone: a file it
// cannot read is still named, and a balance credential's slot, a staging
// file and a directory are not Tokens.
func TestFileStoreListsTokenNamesWithoutReadingThem(t *testing.T) {
	root := filepath.Join(t.TempDir(), "tokens")
	store := newFileStore(root)
	for _, name := range []string{"orphan", "team"} {
		if err := store.Set(name, "sk-"+name+"-0001"); err != nil {
			t.Fatal(err)
		}
	}
	for _, extra := range []string{slotName(ProviderDiagnostic, "team"), credentialStagingPrefix + "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"} {
		if err := os.WriteFile(filepath.Join(root, extra), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "folder"), 0o700); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(filepath.Join(root, "orphan"), 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "orphan"), 0o600) })
	}
	unused, err := UnusedTokens(store, []string{"team"})
	if err != nil || !slices.Equal(unused, []string{"orphan"}) {
		t.Fatalf("UnusedTokens() = %q, %v", unused, err)
	}
	if unused, err := UnusedTokens(newFileStore(filepath.Join(t.TempDir(), "absent")), nil); err != nil || len(unused) != 0 {
		t.Fatalf("UnusedTokens() of an absent directory = %q, %v", unused, err)
	}
}

// The environment backend names its Tokens from the variables' names, with
// the inverse of their encoding: a set variable only, and only a name that
// the encoding writes. Two IDs that differ in case share one variable, so a
// configured provider of either case uses it.
func TestEnvironmentStoreListsTokenVariables(t *testing.T) {
	variables := map[string]string{
		"AIGW_TOKEN_TEAM":         "sk-team-0001",
		"AIGW_TOKEN_ORPHAN_2D1":   "sk-orphan-0001",
		"AIGW_TOKEN_A_2EB":        "sk-dotted-0001",
		"AIGW_TOKEN_UNSET":        "",
		"AIGW_TOKEN_BAD_ZZ":       "sk-bad-0001",
		"AIGW_TOKEN_LOWER_2d":     "sk-lower-0001",
		"AIGW_TOKEN_":             "sk-empty-name-0001",
		"AIGW_DIAGNOSTIC_USER_ID": "not a Token",
	}
	store, err := Select(Selection{
		Backend: "env", GOOS: runtime.GOOS, Getenv: func(key string) string {
			t.Fatalf("a listing looked up %s", key)
			return ""
		},
		Environ: func() []string {
			environ := []string{"PATH=/bin", "NO_EQUALS"}
			for name, value := range variables {
				environ = append(environ, name+"="+value)
			}
			return environ
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// A configured provider's variable is compared by name, so its case
	// reaches the same variable.
	unused, err := UnusedTokens(store, []string{"Team"})
	if err != nil || !slices.Equal(unused, []string{"a.b", "orphan-1"}) {
		t.Fatalf("UnusedTokens() = %q, %v", unused, err)
	}
	// A store built without the environment's names cannot list them, and
	// says so rather than claim that there are none.
	if _, err := UnusedTokens(NewEnvironmentStore(func(string) string { return "" }), nil); err == nil {
		t.Fatal("an environment without its names listed no Tokens")
	}
}

// The system keyring's names come from the bounded native worker. A backend
// that cannot list reports it, and a store outside AIGW's typed backends
// cannot name its Tokens either.
func TestKeyringListsThroughItsWorkerAndReportsARefusal(t *testing.T) {
	store := scopedView{store: listingKeyring(t, []string{"team", "orphan", "orphan"}, nil)}
	unused, err := UnusedTokens(store, []string{"team"})
	if err != nil || !slices.Equal(unused, []string{"orphan"}) {
		t.Fatalf("UnusedTokens() = %q, %v", unused, err)
	}
	refused := errors.New("the keychain would need a person")
	if _, err := UnusedTokens(scopedView{store: listingKeyring(t, nil, refused)}, nil); !errors.Is(err, refused) {
		t.Fatalf("a keyring that refuses = %v", err)
	}
	if _, err := UnusedTokens(plainStore{}, nil); err == nil {
		t.Fatal("a store without a typed backend listed no Tokens")
	}
}

type plainStore struct{ Store }

// The memory backend names the Tokens that hold a value, and no other
// purpose's slot.
func TestMemoryStoreListsItsTokens(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Set("team", "sk-team-0001"); err != nil {
		t.Fatal(err)
	}
	balance, err := ForKind(store, ProviderDiagnostic)
	if err != nil {
		t.Fatal(err)
	}
	if err := balance.Set("balance-only", "credential"); err != nil {
		t.Fatal(err)
	}
	if unused, err := UnusedTokens(store, nil); err != nil || !slices.Equal(unused, []string{"team"}) {
		t.Fatalf("UnusedTokens() = %q, %v", unused, err)
	}
}

// A name that differs only in case from a configured provider is that
// provider's slot on a file system that ignores case, so it is never listed:
// check would print the command that deletes a live Token.
func TestUnusedTokensNeverListsACaseVariantOfAProvider(t *testing.T) {
	store := NewMemoryStore()
	for _, id := range []string{"TEAM", "orphan"} {
		if err := store.Set(id, "value"); err != nil {
			t.Fatal(err)
		}
	}
	unused, err := UnusedTokens(store, []string{"team"})
	if err != nil || !slices.Equal(unused, []string{"orphan"}) {
		t.Fatalf("UnusedTokens() = %q, %v", unused, err)
	}
}
