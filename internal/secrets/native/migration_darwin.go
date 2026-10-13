//go:build darwin && cgo

package native

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aigw-cli/internal/process"

	"github.com/gofrs/flock"
)

const (
	// securityTool is the predecessor's credential reader. Releases up to 0.3.5
	// stored every item through go-keyring, which runs this tool, so a retained
	// item's access list trusts it alone.
	securityTool = "/usr/bin/security"
	// stagedAccountPrefix marks the AIGW-owned item that holds a Token while its
	// exact slot changes owner. No predecessor created such an account.
	stagedAccountPrefix = "migration@"
	// The waits stay inside the worker's five-second operation deadline.
	migrationLockWait   = 1500 * time.Millisecond
	securityToolTimeout = time.Second
	securityOutputLimit = 2*maxStoredValue + 64
	// securityCommandLimit is the tool's interactive command-line limit.
	securityCommandLimit = 4096
)

// readCredentialWithMigration returns the exact slot's Token. A denial that a
// retained predecessor item explains, or an interrupted migration, migrates the
// slot once to AIGW ownership.
func readCredentialWithMigration(service, account, path string) ([]byte, error) {
	value, err := readCredentialFromKeychain(service, account, path)
	if err == nil {
		return value, nil
	}
	if migrationEligible(service, account, path, err) {
		return migrateRetainedCredential(service, account, path)
	}
	// A concurrent worker may have completed a migration between the lookups.
	return readCredentialFromKeychain(service, account, path)
}

// writeCredentialWithMigration replaces the exact slot. A retained predecessor
// item migrates first, so an interrupted rotation keeps a readable Token.
func writeCredentialWithMigration(service, account, path string, value []byte) error {
	err := writeCredentialToKeychain(service, account, path, value)
	if err != nil && migrationEligible(service, account, path, err) {
		var previous []byte
		previous, err = migrateRetainedCredential(service, account, path)
		clear(previous)
		if err == nil {
			err = writeCredentialToKeychain(service, account, path, value)
		}
	}
	if err != nil {
		return err
	}
	return removeStagedCredential(service, account, path)
}

// deleteCredentialWithMigration removes the exact slot, through the
// predecessor's tool when only that tool may delete it, and any staged Token
// left by an interrupted migration.
func deleteCredentialWithMigration(service, account, path string) error {
	err := deleteCredentialFromKeychain(service, account, path)
	var failure *KeychainError
	if errors.As(err, &failure) && predecessorItem(service, account, path) {
		err = deleteRetainedCredential(service, account, path)
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if stagedErr := removeStagedCredential(service, account, path); stagedErr != nil {
		return stagedErr
	}
	return err
}

// migrationEligible reports whether a failed operation on the exact slot is
// explained by an interrupted migration or by a retained predecessor item that
// the predecessor's tool can use without UI.
func migrationEligible(service, account, path string, err error) bool {
	var failure *KeychainError
	if strings.HasPrefix(account, stagedAccountPrefix) || !errors.As(err, &failure) && !errors.Is(err, ErrNotFound) {
		return false
	}
	if staged, stagedErr := observeCredentialInKeychain(service, stagedAccount(account), path); stagedErr == nil && staged {
		return keychainUnlocked(path)
	}
	return failure != nil && predecessorItem(service, account, path)
}

// predecessorItem reports whether the exact slot holds a retained item that the
// predecessor's tool may read or delete without UI: its keychain is unlocked and
// its access list trusts the tool to decrypt it.
func predecessorItem(service, account, path string) bool {
	return keychainUnlocked(path) && itemTrustsApplication(service, account, path, securityTool)
}

// migrateRetainedCredential stages the slot's Token, removes the predecessor's
// item and recreates the slot as an AIGW-owned item. Every intermediate state
// keeps the Token in the retained item, the staging item, or both.
func migrateRetainedCredential(service, account, path string) ([]byte, error) {
	release, err := acquireMigrationLock(path)
	if err != nil {
		return nil, err
	}
	defer release()
	// Another worker may have completed the migration while this one waited.
	if value, err := readCredentialFromKeychain(service, account, path); err == nil {
		return value, nil
	}
	value, retained, err := migrationSource(service, account, path)
	defer clear(value)
	if err == nil {
		err = writeCredentialToKeychain(service, stagedAccount(account), path, value)
	}
	if err == nil && retained {
		err = deleteRetainedCredential(service, account, path)
	}
	if err == nil {
		err = writeCredentialToKeychain(service, account, path, value)
	}
	if err == nil {
		err = verifyCredential(service, account, path, value, readCredentialFromKeychain)
	}
	if err == nil {
		err = removeStagedCredential(service, account, path)
	}
	if err != nil {
		return nil, err
	}
	return bytes.Clone(value), nil
}

// migrationSource selects the Token to migrate. A retained item is
// authoritative, because a staged Token can predate a predecessor rotation.
func migrationSource(service, account, path string) ([]byte, bool, error) {
	if predecessorItem(service, account, path) {
		value, err := readRetainedCredential(service, account, path)
		return value, true, err
	}
	value, err := readCredentialFromKeychain(service, stagedAccount(account), path)
	return value, false, err
}

// restoreCredentialForPredecessor returns an AIGW-owned slot to the
// predecessor's tool before a portable rollback activates that program. An
// absent slot, or an item the tool already owns, needs no conversion.
func restoreCredentialForPredecessor(service, account, path string) error {
	release, err := acquireMigrationLock(path)
	if err != nil {
		return err
	}
	defer release()
	value, err := readCredentialFromKeychain(service, account, path)
	if errors.Is(err, ErrNotFound) {
		// An interrupted migration keeps its Token only in the staging item.
		value, err = readCredentialFromKeychain(service, stagedAccount(account), path)
	}
	defer clear(value)
	switch {
	case errors.Is(err, ErrNotFound):
		return nil
	case err != nil && predecessorItem(service, account, path):
		return removeStagedCredential(service, account, path)
	case err != nil:
		return err
	}
	command, err := retainedCredentialCommand(service, account, path, value)
	if err == nil {
		err = writeCredentialToKeychain(service, stagedAccount(account), path, value)
	}
	if err == nil {
		err = deleteCredentialFromKeychain(service, account, path)
		if errors.Is(err, ErrNotFound) {
			err = nil
		}
	}
	if err == nil {
		err = runSecurityCommand(command)
	}
	if err == nil {
		err = verifyCredential(service, account, path, value, readRetainedCredential)
	}
	if err == nil {
		err = removeStagedCredential(service, account, path)
	}
	return err
}

func verifyCredential(service, account, path string, want []byte, read func(string, string, string) ([]byte, error)) error {
	got, err := read(service, account, path)
	defer clear(got)
	if err == nil && !bytes.Equal(got, want) {
		err = ErrUnavailable
	}
	return err
}

// removeStagedCredential deletes the slot's AIGW-owned staging item, if any.
func removeStagedCredential(service, account, path string) error {
	err := deleteCredentialFromKeychain(service, stagedAccount(account), path)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func stagedAccount(account string) string { return stagedAccountPrefix + account }

// readRetainedCredential reads an item through the predecessor's tool, which a
// retained or restored item trusts.
func readRetainedCredential(service, account, path string) (value []byte, err error) {
	output, err := runSecurityTool(keychainArguments([]string{"find-generic-password", "-s", service, "-a", account, "-w"}, path), "")
	defer clear(output)
	if err == nil {
		value, err = decodeStoredCredential(output)
	}
	return value, err
}

// deleteRetainedCredential removes a retained item through the predecessor's
// tool, because AIGW may not delete an item that it cannot decrypt.
func deleteRetainedCredential(service, account, path string) error {
	output, err := runSecurityTool(keychainArguments([]string{"delete-generic-password", "-s", service, "-a", account}, path), "")
	clear(output)
	return err
}

// retainedCredentialCommand builds the tool's interactive command that writes
// the slot with go-keyring's storage envelope. The Token reaches the tool on
// standard input, never in argv, exactly as the predecessor's writer did.
func retainedCredentialCommand(service, account, path string, value []byte) (string, error) {
	stored := storedCredential(value)
	defer clear(stored)
	command := "add-generic-password -U -s '" + service + "' -a '" + account + "' -w '" + string(stored) + "'"
	if path != "" {
		command += " '" + path + "'"
	}
	if strings.ContainsAny(service+account+path, "'\n\x00") || len(command) >= securityCommandLimit {
		return "", ErrUnavailable
	}
	return command + "\n", nil
}

func runSecurityCommand(command string) error {
	output, err := runSecurityTool([]string{"-i"}, command)
	clear(output)
	return err
}

func keychainArguments(args []string, path string) []string {
	if path != "" {
		args = append(args, path)
	}
	return args
}

// runSecurityTool runs one bounded invocation of the predecessor's tool. Its
// diagnostics can quote the item, so callers discard them with the output.
func runSecurityTool(args []string, input string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), securityToolTimeout)
	defer cancel()
	output, err := (process.Runner{StdoutLimit: securityOutputLimit}).RunCapture(ctx, process.Plan{
		Executable: securityTool, Args: args, Stdin: input, Env: nativeEnvironment(os.Getenv),
	})
	if err != nil {
		return output, errors.Join(ErrUnavailable, err)
	}
	return output, nil
}

// acquireMigrationLock serializes migrations across concurrent workers. The
// kernel releases the lock if a worker exits before its migration completes.
func acquireMigrationLock(path string) (func(), error) {
	name, err := migrationLockPath(path)
	guard := flock.New(name)
	locked := false
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), migrationLockWait)
		locked, err = guard.TryLockContext(ctx, 20*time.Millisecond)
		cancel()
	}
	if err != nil || !locked {
		return nil, errors.Join(ErrUnavailable, err)
	}
	return func() { _ = guard.Unlock() }, nil
}

// migrationLockPath places the lock in the user's private temporary directory,
// or beside an explicitly selected keychain.
func migrationLockPath(path string) (string, error) {
	if path != "" {
		return path + ".aigw-migration.lock", nil
	}
	directory, err := userTemporaryDirectory()
	return filepath.Join(directory, "aigw-credential-migration.lock"), err
}
