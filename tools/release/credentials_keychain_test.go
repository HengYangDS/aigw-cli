package main

import (
	"aigw-cli/internal/secrets"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func requireUnoccupiedLegacyKeychainSlot(t *testing.T, account, keychain string) {
	t.Helper()
	if legacyKeychainSlot(t, "find-generic-password", account, keychain) {
		t.Fatalf("native credential test requires an unoccupied legacy %q slot", account)
	}
	t.Cleanup(func() {
		legacyKeychainSlot(t, "delete-generic-password", account, keychain)
		if legacyKeychainSlot(t, "find-generic-password", account, keychain) {
			t.Errorf("legacy credential slot %q remains after cleanup", account)
		}
	})
}

func legacyKeychainSlot(t *testing.T, operation, account, keychain string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	args := []string{operation, "-s", secrets.Service, "-a", account}
	if keychain != "" {
		args = append(args, keychain)
	}
	_, err := exec.CommandContext(ctx, "/usr/bin/security", args...).CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("legacy Keychain %s timed out for %q: %v", operation, account, ctx.Err())
	}
	if err == nil {
		return true
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 44 {
		return false
	}
	t.Fatalf("legacy Keychain %s failed for %q: %v", operation, account, err)
	return false
}

func TestLegacyKeychainSlotCleanupAfterEarlyExit(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("legacy Keychain slots exist only on macOS")
	}
	keychain := filepath.Join(t.TempDir(), "legacy.keychain-db")
	runPrivateKeychainCommand(t, "create-keychain", "-p", "", keychain)
	t.Cleanup(func() { runPrivateKeychainCommand(t, "delete-keychain", keychain) })
	runPrivateKeychainCommand(t, "unlock-keychain", "-p", "", keychain)
	const account = "owned-legacy-slot"
	t.Run("early exit", func(t *testing.T) {
		requireUnoccupiedLegacyKeychainSlot(t, account, keychain)
		runPrivateKeychainCommand(t, "add-generic-password", "-s", secrets.Service, "-a", account, "-w", "synthetic", "-A", keychain)
		t.SkipNow()
	})
	if legacyKeychainSlot(t, "find-generic-password", account, keychain) {
		t.Fatal("early exit left its legacy credential item behind")
	}
}

func runPrivateKeychainCommand(t *testing.T, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "/usr/bin/security", args...).CombinedOutput(); err != nil {
		t.Fatalf("private Keychain fixture command %q failed: %v; %s", args[0], err, output)
	}
}
