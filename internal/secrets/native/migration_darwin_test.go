//go:build darwin && cgo

package native

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

func TestPredecessorKeychainItemMigratesOnFirstRead(t *testing.T) {
	path := newPrivateKeychain(t)
	const service, account, token = "aigw-private-test", "predecessor", "synthetic-predecessor-token"
	addPredecessorItem(t, path, service, account, token)
	value, err := readPrivateKeychainFixture(t, service, account, path)
	if err != nil || string(value) != token {
		t.Fatalf("successor read of a predecessor item = %q, %v", value, err)
	}
	requireWorkerOwnedItem(t, path, service, account)
	if value, err := readPrivateKeychainFixture(t, service, account, path); err != nil || string(value) != token {
		t.Fatalf("migrated item read = %q, %v", value, err)
	}
}

func TestConcurrentReadsMigrateAPredecessorItemOnce(t *testing.T) {
	path := newPrivateKeychain(t)
	const service, account, token = "aigw-private-test", "concurrent", "synthetic-concurrent-token"
	addPredecessorItem(t, path, service, account, token)
	results := make(chan error, 6)
	for range cap(results) {
		go func() {
			value, err := privateKeychainChildOutput(t.Context(), os.Args[0], service, account, path, "TestPrivateKeychainReadChild")
			if err == nil && string(value) != token {
				err = errors.New("concurrent read returned another value")
			}
			results <- err
		}()
	}
	for range cap(results) {
		if err := <-results; err != nil {
			t.Fatalf("concurrent first read: %v", err)
		}
	}
	requireWorkerOwnedItem(t, path, service, account)
}

func TestInterruptedMigrationResumesFromItsStagedToken(t *testing.T) {
	const service, token = "aigw-private-test", "synthetic-staged-token"
	t.Run("retained item deleted", func(t *testing.T) {
		path := newPrivateKeychain(t)
		if err := writeCredentialToKeychain(service, "migration@resumed", path, []byte(token)); err != nil {
			t.Fatal(err)
		}
		if value, err := readPrivateKeychainFixture(t, service, "resumed", path); err != nil || string(value) != token {
			t.Fatalf("resumed migration read = %q, %v", value, err)
		}
		requireWorkerOwnedItem(t, path, service, "resumed")
	})
	t.Run("retained item still present", func(t *testing.T) {
		path := newPrivateKeychain(t)
		addPredecessorItem(t, path, service, "repeated", token)
		if err := writeCredentialToKeychain(service, "migration@repeated", path, []byte("synthetic-stale-staged-token")); err != nil {
			t.Fatal(err)
		}
		if value, err := readPrivateKeychainFixture(t, service, "repeated", path); err != nil || string(value) != token {
			t.Fatalf("repeated migration read = %q, %v", value, err)
		}
		requireWorkerOwnedItem(t, path, service, "repeated")
	})
	t.Run("final item recreated", func(t *testing.T) {
		path := newPrivateKeychain(t)
		for _, account := range []string{"completed", "migration@completed"} {
			if err := writeCredentialToKeychain(service, account, path, []byte(token)); err != nil {
				t.Fatal(err)
			}
		}
		if value, err := readPrivateKeychainFixture(t, service, "completed", path); err != nil || string(value) != token {
			t.Fatalf("completed migration read = %q, %v", value, err)
		}
		requireWorkerOwnedItem(t, path, service, "completed")
	})
}

func TestMigrationCustodySerializesMutations(t *testing.T) {
	const service, account, token = "aigw-private-test", "custody", "synthetic-custody-token"
	for _, operation := range []string{"write", "delete"} {
		t.Run(operation, func(t *testing.T) {
			path := newPrivateKeychain(t)
			if err := writeCredentialToKeychain(service, "migration@"+account, path, []byte(token)); err != nil {
				t.Fatal(err)
			}
			guard := flock.New(path + ".aigw-migration.lock")
			if err := guard.Lock(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = guard.Unlock() })
			mutate := func() error {
				if operation == "write" {
					return writeCredentialWithMigration(service, account, path, []byte("synthetic-rotated-token"))
				}
				return deleteCredentialWithMigration(service, account, path)
			}
			if err := mutate(); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("mutation during migration = %v", err)
			}
			if value, err := readCredentialFromKeychain(service, "migration@"+account, path); err != nil || string(value) != token {
				t.Fatalf("mutation changed the staged Token: %v", err)
			}
			if present, err := observeCredentialInKeychain(service, account, path); err != nil || present {
				t.Fatalf("mutation recreated the migrating item: %t, %v", present, err)
			}
			if err := guard.Unlock(); err != nil {
				t.Fatal(err)
			}
			if value, err := readPrivateKeychainFixture(t, service, account, path); err != nil || string(value) != token {
				t.Fatalf("migration resume: %v", err)
			}
			if err := mutate(); err != nil {
				t.Fatalf("mutation after migration: %v", err)
			}
			value, err := readPrivateKeychainFixture(t, service, account, path)
			if operation == "write" {
				if err != nil || string(value) != "synthetic-rotated-token" {
					t.Fatalf("rotation was overwritten: %v", err)
				}
				requireWorkerOwnedItem(t, path, service, account)
			} else if len(value) != 0 || fixtureExitCode(err) != missingExit {
				t.Fatalf("deleted Token was resurrected: %v", err)
			}
		})
	}
}

func TestLockedKeychainKeepsAPredecessorItemDenied(t *testing.T) {
	path := newPrivateKeychain(t)
	const service, account = "aigw-private-test", "locked-predecessor"
	addPredecessorItem(t, path, service, account, "synthetic-locked-token")
	runKeychainFixtureCommand(t, "lock-keychain", path)
	if value, err := readPrivateKeychainFixture(t, service, account, path); len(value) != 0 || fixtureExitCode(err) != failureExit {
		t.Fatalf("locked predecessor item read = %q, %v", value, err)
	}
	runKeychainFixtureCommand(t, "unlock-keychain", "-p", "", path)
	if owners := keychainDecryptOwners(t, path, account); !slices.Equal(owners, []string{"/usr/bin/security"}) {
		t.Fatalf("locked predecessor item access changed: %q", owners)
	}
}

func TestWriteMigratesAPredecessorItemBeforeReplacingIt(t *testing.T) {
	path := newPrivateKeychain(t)
	const service, account = "aigw-private-test", "rotated"
	addPredecessorItem(t, path, service, account, "synthetic-previous-token")
	if err := writeCredentialWithMigration(service, account, path, []byte("synthetic-rotated-token")); err != nil {
		t.Fatalf("replace predecessor item: %v", err)
	}
	if value, err := readPrivateKeychainFixture(t, service, account, path); err != nil || string(value) != "synthetic-rotated-token" {
		t.Fatalf("rotated item read = %q, %v", value, err)
	}
	requireWorkerOwnedItem(t, path, service, account)
}

func TestMutationRemovesStaleStagedTokens(t *testing.T) {
	const service = "aigw-private-test"
	path := newPrivateKeychain(t)
	for _, account := range []string{"owned", "migration@owned"} {
		if err := writeCredentialToKeychain(service, account, path, []byte("synthetic-"+account)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeCredentialWithMigration(service, "owned", path, []byte("synthetic-rotated-token")); err != nil {
		t.Fatal(err)
	}
	requireWorkerOwnedItem(t, path, service, "owned")
	addPredecessorItem(t, path, service, "removed", "synthetic-removed-token")
	if err := writeCredentialToKeychain(service, "migration@removed", path, []byte("synthetic-removed-token")); err != nil {
		t.Fatal(err)
	}
	if err := deleteCredentialWithMigration(service, "removed", path); err != nil {
		t.Fatalf("delete predecessor item: %v", err)
	}
	for _, account := range []string{"removed", "migration@removed"} {
		if present, err := observeCredentialInKeychain(service, account, path); err != nil || present {
			t.Fatalf("deletion left %q: %t, %v", account, present, err)
		}
	}
	if err := deleteCredentialWithMigration(service, "removed", path); !errors.Is(err, ErrNotFound) {
		t.Fatalf("repeated deletion = %v", err)
	}
	if value, err := readPrivateKeychainFixture(t, service, "removed", path); len(value) != 0 || fixtureExitCode(err) != missingExit {
		t.Fatalf("absent slot read = %q, %v", value, err)
	}
}

func TestUnusablePredecessorTokenStaysRetained(t *testing.T) {
	path := newPrivateKeychain(t)
	const service, account = "aigw-private-test", "undecodable"
	runKeychainFixtureCommand(t, "add-generic-password", "-s", service, "-a", account, "-w", "go-keyring-base64:***", path)
	if value, err := readCredentialWithMigration(service, account, path); len(value) != 0 || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("undecodable predecessor read = %q, %v", value, err)
	}
	if owners := keychainDecryptOwners(t, path, account); !slices.Equal(owners, []string{"/usr/bin/security"}) {
		t.Fatalf("undecodable predecessor item access changed: %q", owners)
	}
	if present, err := observeCredentialInKeychain(service, "migration@"+account, path); err != nil || present {
		t.Fatalf("failed migration left a staging item: %t, %v", present, err)
	}
	if err := writeCredentialToKeychain(service, "invalid", path, []byte("synthetic-token")); err != nil {
		t.Fatal(err)
	}
	runKeychainFixtureCommand(t, "add-generic-password", "-U", "-s", service, "-a", "invalid", "-w", "go-keyring-base64:***", path)
	if value, err := readCredentialWithMigration(service, "invalid", path); len(value) != 0 || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("invalid owned item read = %q, %v", value, err)
	}
}

func TestBusyMigrationLeavesThePredecessorItem(t *testing.T) {
	path := newPrivateKeychain(t)
	const service, account = "aigw-private-test", "busy"
	addPredecessorItem(t, path, service, account, "synthetic-busy-token")
	guard := flock.New(path + ".aigw-migration.lock")
	if err := guard.Lock(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guard.Unlock() })
	if value, err := readPrivateKeychainFixture(t, service, account, path); len(value) != 0 || fixtureExitCode(err) != failureExit {
		t.Fatalf("read during a held migration = %q, %v", value, err)
	}
	if err := restoreCredentialForPredecessor(service, account, path); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("restore during a held migration = %v", err)
	}
	if owners := keychainDecryptOwners(t, path, account); !slices.Equal(owners, []string{"/usr/bin/security"}) {
		t.Fatalf("busy migration changed the predecessor item: %q", owners)
	}
}

func TestMigrationLockUsesThePrivateTemporaryDirectory(t *testing.T) {
	name, err := migrationLockPath("")
	if err != nil || !filepath.IsAbs(name) || filepath.Base(name) != "aigw-credential-migration.lock" {
		t.Fatalf("default migration lock = %q, %v", name, err)
	}
	if info, err := os.Stat(filepath.Dir(name)); err != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("migration lock directory is not private: %v, %v", info, err)
	}
}

func TestRollbackRestoresThePredecessorReader(t *testing.T) {
	const service, token = "aigw-private-test", "synthetic-rollback-token"
	t.Run("migrated item", func(t *testing.T) {
		path := newPrivateKeychain(t)
		addPredecessorItem(t, path, service, "migrated", token)
		if value, err := readPrivateKeychainFixture(t, service, "migrated", path); err != nil || string(value) != token {
			t.Fatalf("migrate before rollback = %q, %v", value, err)
		}
		if err := restoreCredentialForPredecessor(service, "migrated", path); err != nil {
			t.Fatalf("restore migrated item: %v", err)
		}
		requirePredecessorItem(t, path, service, "migrated", token)
		if value, err := readPrivateKeychainFixture(t, service, "migrated", path); err != nil || string(value) != token {
			t.Fatalf("re-upgrade read = %q, %v", value, err)
		}
		requireWorkerOwnedItem(t, path, service, "migrated")
	})
	t.Run("interrupted migration", func(t *testing.T) {
		path := newPrivateKeychain(t)
		if err := writeCredentialToKeychain(service, "migration@interrupted", path, []byte(token)); err != nil {
			t.Fatal(err)
		}
		if err := restoreCredentialForPredecessor(service, "interrupted", path); err != nil {
			t.Fatalf("restore interrupted migration: %v", err)
		}
		requirePredecessorItem(t, path, service, "interrupted", token)
	})
	t.Run("predecessor item", func(t *testing.T) {
		path := newPrivateKeychain(t)
		addPredecessorItem(t, path, service, "retained", token)
		if err := writeCredentialToKeychain(service, "migration@retained", path, []byte("synthetic-stale-token")); err != nil {
			t.Fatal(err)
		}
		if err := restoreCredentialForPredecessor(service, "retained", path); err != nil {
			t.Fatalf("restore predecessor item: %v", err)
		}
		requirePredecessorItem(t, path, service, "retained", token)
	})
	t.Run("absent slot", func(t *testing.T) {
		if err := restoreCredentialForPredecessor(service, "absent", newPrivateKeychain(t)); err != nil {
			t.Fatalf("restore absent slot: %v", err)
		}
	})
	t.Run("refused items", func(t *testing.T) {
		path := newPrivateKeychain(t)
		runKeychainFixtureCommand(t, "add-generic-password", "-s", service, "-a", "foreign", "-w", "synthetic", "-T", "/usr/bin/true", path)
		if err := restoreCredentialForPredecessor(service, "foreign", path); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("restore foreign item = %v", err)
		}
		oversized := []byte(strings.Repeat("x", securityCommandLimit))
		if err := writeCredentialToKeychain(service, "oversized", path, oversized); err != nil {
			t.Fatal(err)
		}
		if err := restoreCredentialForPredecessor(service, "oversized", path); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("restore oversized Token = %v", err)
		}
		requireWorkerOwnedItem(t, path, service, "oversized")
	})
}

func TestRestoreRunsTheBoundedWorker(t *testing.T) {
	for _, test := range []struct {
		name string
		code int
	}{{name: "restored"}, {name: "failed", code: failureExit}} {
		t.Run(test.name, func(t *testing.T) {
			program := filepath.Join(t.TempDir(), "aigw")
			script := fmt.Sprintf("#!/bin/sh\n[ \"$1\" = %q ] && [ \"$2\" = aigw-private-test ] && [ \"$3\" = synthetic ] || exit 9\nexit %d\n", restoreCommand, test.code)
			if err := os.WriteFile(program, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			err := Restore(program, "aigw-private-test", "synthetic")
			if test.code == 0 && err != nil || test.code != 0 && !errors.Is(err, ErrUnavailable) {
				t.Fatalf("restore = %v", err)
			}
		})
	}
}

func requirePredecessorItem(t *testing.T, path, service, account, token string) {
	t.Helper()
	if owners := keychainDecryptOwners(t, path, account); !slices.Equal(owners, []string{"/usr/bin/security"}) {
		t.Fatalf("restored item %q decrypt owners = %q", account, owners)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/usr/bin/security", "find-generic-password", "-s", service, "-a", account, "-w", path).Output()
	if err != nil || strings.TrimSpace(string(output)) != "go-keyring-base64:"+base64.StdEncoding.EncodeToString([]byte(token)) {
		t.Fatalf("predecessor reader cannot read the restored item: %v", err)
	}
	if present, err := observeCredentialInKeychain(service, "migration@"+account, path); err != nil || present {
		t.Fatalf("restore of %q left a staging item: %t, %v", account, present, err)
	}
}

func newPrivateKeychain(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "migration.keychain-db")
	runKeychainFixtureCommand(t, "create-keychain", "-p", "", path)
	t.Cleanup(func() { runKeychainFixtureCommand(t, "delete-keychain", path) })
	runKeychainFixtureCommand(t, "set-keychain-settings", path)
	runKeychainFixtureCommand(t, "unlock-keychain", "-p", "", path)
	return path
}

// addPredecessorItem writes an item as go-keyring did through /usr/bin/security,
// so only that tool is trusted to decrypt it.
func addPredecessorItem(t *testing.T, path, service, account, token string) {
	t.Helper()
	runKeychainFixtureCommand(t, "add-generic-password", "-s", service, "-a", account, "-w", "go-keyring-base64:"+base64.StdEncoding.EncodeToString([]byte(token)), path)
	if owners := keychainDecryptOwners(t, path, account); !slices.Equal(owners, []string{"/usr/bin/security"}) {
		t.Fatalf("predecessor fixture decrypt owners = %q", owners)
	}
}

func requireWorkerOwnedItem(t *testing.T, path, service, account string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	worker, err := filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatal(err)
	}
	if owners := keychainDecryptOwners(t, path, account); !slices.Equal(owners, []string{worker}) {
		t.Fatalf("item %q decrypt owners = %q, want the worker %q", account, owners, worker)
	}
	if present, err := observeCredentialInKeychain(service, "migration@"+account, path); err != nil || present {
		t.Fatalf("migration of %q left a staging item: %t, %v", account, present, err)
	}
}

// keychainDecryptOwners lists the applications a private fixture item trusts to
// decrypt it. The access dump contains no secret values.
func keychainDecryptOwners(t *testing.T, path, account string) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/usr/bin/security", "dump-keychain", "-a", path).Output()
	if err != nil {
		t.Fatalf("dump private Keychain access: %v", err)
	}
	for item := range strings.SplitSeq(string(output), "keychain: ") {
		if !strings.Contains(item, `"acct"<blob>="`+account+`"`) {
			continue
		}
		owners := []string{}
		for entry := range strings.SplitSeq(item, "entry ") {
			authorizations, applications, _ := strings.Cut(entry, "applications")
			if !strings.Contains(authorizations, " decrypt") {
				continue
			}
			for line := range strings.SplitSeq(applications, "\n") {
				_, owner, found := strings.Cut(strings.TrimSpace(line), ": /")
				if found && !strings.HasPrefix(strings.TrimSpace(line), "requirement") {
					owners = append(owners, "/"+strings.TrimSuffix(owner, " (OK)"))
				}
			}
		}
		return owners
	}
	t.Fatalf("private Keychain has no %q item", account)
	return nil
}
