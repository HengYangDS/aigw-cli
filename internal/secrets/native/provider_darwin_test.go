//go:build darwin

package native

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestKeychainReadUsesPrivateFixtureWithoutAuthorizationUI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.keychain-db")
	runKeychainFixtureCommand(t, "create-keychain", "-p", "", path)
	t.Cleanup(func() { runKeychainFixtureCommand(t, "delete-keychain", path) })
	runKeychainFixtureCommand(t, "unlock-keychain", "-p", "", path)

	const service, account, token = "aigw-private-test", "authorized", "synthetic\nvalue"
	for _, item := range []struct {
		account string
		stored  string
	}{
		{account, "go-keyring-base64:" + base64.StdEncoding.EncodeToString([]byte(token))},
		{"legacy-hex", "go-keyring-encoded:" + hex.EncodeToString([]byte(token))},
		{"legacy-plain", token},
	} {
		runKeychainFixtureCommand(t, "add-generic-password", "-s", service, "-a", item.account, "-w", item.stored, "-A", path)
		value, err := readCredentialFromKeychain(service, item.account, path)
		if err != nil || string(value) != token {
			t.Fatalf("private item %q read = %q, %v", item.account, value, err)
		}
	}
	if present, err := observeCredentialInKeychain(service, account, path); err != nil || !present {
		t.Fatalf("authorized item metadata = %t, %v", present, err)
	}
	if value, err := readPrivateKeychainFixture(t, service, account, path); err != nil || string(value) != token {
		t.Fatalf("native subprocess lost the retained Account identity: %v", err)
	}
	if present, err := observeCredentialInKeychain(service, "absent", path); err != nil || present {
		t.Fatalf("missing item metadata = %t, %v", present, err)
	}
	assertKeychainMutationPreservesDeniedItem(t, service, path, token)

	if value, err := readCredentialFromKeychain(service, "absent", path); len(value) != 0 || !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing private item = %q, %v", value, err)
	}
	for _, item := range []struct {
		account string
		stored  string
	}{
		{"invalid-base64", "go-keyring-base64:***"},
		{"invalid-hex", "go-keyring-encoded:ZZ"},
	} {
		runKeychainFixtureCommand(t, "add-generic-password", "-s", service, "-a", item.account, "-w", item.stored, "-A", path)
		if value, err := readCredentialFromKeychain(service, item.account, path); len(value) != 0 || !errors.Is(err, ErrUnavailable) {
			t.Fatalf("invalid private item %q = %q, %v", item.account, value, err)
		}
	}
	runKeychainFixtureCommand(t, "add-generic-password", "-s", service, "-a", "denied", "-w", "synthetic", "-T", "", path)
	if present, err := observeCredentialInKeychain(service, "denied", path); err != nil || !present {
		t.Fatalf("denied item metadata = %t, %v", present, err)
	}
	if value, err := readPrivateKeychainFixture(t, service, "denied", path); len(value) != 0 || fixtureExitCode(err) != failureExit {
		t.Fatalf("unauthorized private item = %q, %v", value, err)
	}
}

func TestNewNativeKeychainItemsHaveReadablePurposeLabels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "labels.keychain-db")
	runKeychainFixtureCommand(t, "create-keychain", "-p", "", path)
	t.Cleanup(func() { runKeychainFixtureCommand(t, "delete-keychain", path) })
	runKeychainFixtureCommand(t, "unlock-keychain", "-p", "", path)

	const service = "aigw-private-test"
	// security renders kSecLabelItemAttr with its legacy numeric tag.
	const labelAttribute = "0x00000007 <blob>="
	for _, item := range []struct {
		account string
		label   string
	}{
		{"team", "AIGW Account Token: team"},
		{"diagnostic@team", "AIGW Provider Diagnostic: team"},
		{"native@team", "AIGW Account Token: native@team"},
	} {
		t.Run(item.account, func(t *testing.T) {
			if err := writeCredentialToKeychain(service, item.account, path, []byte("synthetic")); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			output, err := exec.CommandContext(ctx, "/usr/bin/security", "find-generic-password", "-s", service, "-a", item.account, path).CombinedOutput()
			if err != nil {
				t.Fatal(err)
			}
			observed := ""
			for line := range strings.SplitSeq(string(output), "\n") {
				if strings.Contains(line, labelAttribute) {
					observed = strings.TrimSpace(line)
					break
				}
			}
			if observed != labelAttribute+`"`+item.label+`"` {
				t.Fatalf("new native item label = %q, want %q", observed, item.label)
			}
		})
	}
}

func TestPrivateKeychainCopiedReaderUsesExactExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "copied-reader.keychain-db")
	runKeychainFixtureCommand(t, "create-keychain", "-p", "", path)
	t.Cleanup(func() { runKeychainFixtureCommand(t, "delete-keychain", path) })
	runKeychainFixtureCommand(t, "unlock-keychain", "-p", "", path)
	const service, account, token = "aigw-private-test", "copied-reader", "synthetic-copied-reader-token"
	if err := writeCredentialToKeychain(service, account, path, []byte(token)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	copied := filepath.Join(t.TempDir(), "copied-aigw-test")
	if err := os.WriteFile(copied, data, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Run("missing copied reader", func(t *testing.T) {
		value, err := runPrivateKeychainChild(t, copied+".absent", service, account, path, "TestPrivateKeychainReadChild")
		if err == nil || len(value) != 0 {
			t.Fatal("missing copied reader silently used another executable")
		}
	})
	for _, executable := range []string{os.Args[0], copied} {
		t.Run(executable, func(t *testing.T) {
			got, err := os.ReadFile(executable)
			if err != nil || sha256.Sum256(got) != sha256.Sum256(data) {
				t.Fatalf("credential reader copy changed executable identity: %v", err)
			}
			value, err := runPrivateKeychainChild(t, executable, service, account, path, "TestPrivateKeychainReadChild")
			if err != nil || string(value) != token {
				t.Fatalf("same-byte native reader cannot read its creator-authorized item: %v", err)
			}
		})
	}
	if err := writeCredentialToKeychain(service, account, path, []byte("rotated-synthetic-token")); err != nil {
		t.Fatal(err)
	}
	value, err := runPrivateKeychainChild(t, copied, service, account, path, "TestPrivateKeychainReadChild")
	if err != nil || string(value) != "rotated-synthetic-token" {
		t.Fatalf("copied reader lost native item rotation: %v", err)
	}
	if err := deleteCredentialFromKeychain(service, account, path); err != nil {
		t.Fatal(err)
	}
	value, err = runPrivateKeychainChild(t, copied, service, account, path, "TestPrivateKeychainReadChild")
	if len(value) != 0 || fixtureExitCode(err) != missingExit {
		t.Fatalf("copied reader retained a deleted native item: %v", err)
	}
}

func TestLockedKeychainMetadataDoesNotPrompt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locked.keychain-db")
	runKeychainFixtureCommand(t, "create-keychain", "-p", "", path)
	t.Cleanup(func() { runKeychainFixtureCommand(t, "delete-keychain", path) })
	runKeychainFixtureCommand(t, "unlock-keychain", "-p", "", path)
	runKeychainFixtureCommand(t, "add-generic-password", "-s", "aigw-private-test", "-a", "locked", "-w", "synthetic", "-A", path)
	runKeychainFixtureCommand(t, "lock-keychain", path)

	value, err := observePrivateKeychainFixture(t, "aigw-private-test", "locked", path)
	if err == nil && string(value) == "1" || fixtureExitCode(err) == failureExit && len(value) == 0 {
		return
	}
	t.Fatalf("locked Keychain metadata was reported absent or stalled: %q, %v", value, err)
}

func assertKeychainMutationPreservesDeniedItem(t *testing.T, service, path, token string) {
	t.Helper()
	runKeychainFixtureCommand(t, "add-generic-password", "-s", service, "-a", "security-writer", "-w", "go-keyring-base64:"+base64.StdEncoding.EncodeToString([]byte(token)), path)
	if value, err := readPrivateKeychainFixture(t, service, "security-writer", path); len(value) != 0 || fixtureExitCode(err) != failureExit {
		t.Fatalf("old writer's item unexpectedly bypassed authorization: %q, %v", value, err)
	}
	if err := writeCredentialToKeychain(service, "security-writer", path, []byte("replacement")); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unauthorized legacy item accepted a replacement: %v", err)
	}
	const nativeAccount = "native-writer"
	for _, value := range [][]byte{nil, make([]byte, maxStoredValue+1)} {
		if err := writeCredentialToKeychain(service, nativeAccount, path, value); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("invalid native value was accepted: %v", err)
		}
	}
	if err := writeCredentialToKeychain(service, nativeAccount, path, []byte(token)); err != nil {
		t.Fatal(err)
	}
	if value, err := readCredentialFromKeychain(service, nativeAccount, path); err != nil || string(value) != token {
		t.Fatalf("native writer's item = %q, %v", value, err)
	}
	if err := writeCredentialToKeychain(service, nativeAccount, path, []byte("replacement")); err != nil {
		t.Fatal(err)
	}
	if value, err := readCredentialFromKeychain(service, nativeAccount, path); err != nil || string(value) != "replacement" {
		t.Fatalf("updated native item = %q, %v", value, err)
	}
	if err := deleteCredentialFromKeychain(service, nativeAccount, path); err != nil {
		t.Fatal(err)
	}
	if value, err := readCredentialFromKeychain(service, nativeAccount, path); len(value) != 0 || !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted native item = %q, %v", value, err)
	}
	if value, err := readPrivateKeychainFixture(t, service, "security-writer", path); len(value) != 0 || fixtureExitCode(err) != failureExit {
		t.Fatalf("legacy item changed during native-slot lifecycle: %q, %v", value, err)
	}
}

func readPrivateKeychainFixture(t *testing.T, service, account, path string) ([]byte, error) {
	return runPrivateKeychainChild(t, os.Args[0], service, account, path, "TestPrivateKeychainReadChild")
}

func observePrivateKeychainFixture(t *testing.T, service, account, path string) ([]byte, error) {
	return runPrivateKeychainChild(t, os.Args[0], service, account, path, "TestPrivateKeychainObserveChild")
}

func runPrivateKeychainChild(t *testing.T, executable, service, account, path, testName string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^"+testName+"$")
	command.Env = append(os.Environ(),
		"AIGW_TEST_KEYCHAIN_PATH="+path,
		"AIGW_TEST_KEYCHAIN_SERVICE="+service,
		"AIGW_TEST_KEYCHAIN_ACCOUNT="+account,
	)
	output, err := command.Output()
	if ctx.Err() != nil {
		t.Fatalf("private Keychain read did not return without UI: %v", ctx.Err())
	}
	return output, err
}

func TestPrivateKeychainObserveChild(t *testing.T) {
	path := os.Getenv("AIGW_TEST_KEYCHAIN_PATH")
	if path == "" {
		return
	}
	present, err := observeCredentialInKeychain(
		os.Getenv("AIGW_TEST_KEYCHAIN_SERVICE"),
		os.Getenv("AIGW_TEST_KEYCHAIN_ACCOUNT"), path,
	)
	if err != nil {
		os.Exit(failureExit)
	}
	if present {
		_, _ = os.Stdout.WriteString("1")
	} else {
		_, _ = os.Stdout.WriteString("0")
	}
	os.Exit(0)
}

func fixtureExitCode(err error) int {
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return exit.ExitCode()
	}
	return 0
}

func TestPrivateKeychainReadChild(t *testing.T) {
	path := os.Getenv("AIGW_TEST_KEYCHAIN_PATH")
	if path == "" {
		return
	}
	value, err := readCredentialFromKeychain(
		os.Getenv("AIGW_TEST_KEYCHAIN_SERVICE"),
		os.Getenv("AIGW_TEST_KEYCHAIN_ACCOUNT"), path,
	)
	switch {
	case errors.Is(err, ErrNotFound):
		os.Exit(missingExit)
	case err != nil:
		os.Exit(failureExit)
	}
	_, _ = os.Stdout.Write(value)
	os.Exit(0)
}

func TestNativePresenceCommandClassifiesExactOutput(t *testing.T) {
	if _, err := queryCredential("unknown", "aigw-private-test", "synthetic", nil); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unknown credential operation = %v", err)
	}
	for _, test := range []struct {
		output  string
		present bool
		wantErr error
	}{
		{output: "1", present: true},
		{output: "0"},
		{output: "?", wantErr: ErrUnavailable},
	} {
		t.Run(test.output, func(t *testing.T) {
			program := filepath.Join(t.TempDir(), "synthetic-presence")
			if err := os.WriteFile(program, []byte("#!/bin/sh\nprintf '"+test.output+"'"), 0o700); err != nil {
				t.Fatal(err)
			}
			present, err := Exists(program, "aigw-private-test", "synthetic")
			if present != test.present || !errors.Is(err, test.wantErr) {
				t.Fatalf("presence = %t, %v", present, err)
			}
		})
	}
}

func runKeychainFixtureCommand(t *testing.T, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "/usr/bin/security", args...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("private Keychain fixture command %q failed: %v: %s", args[0], err, output)
	}
}

func TestNativeEnvironmentRetainsOnlyKeychainHome(t *testing.T) {
	values := map[string]string{"HOME": "/Users/runner", "PATH": "/untrusted"}
	want := []string{"HOME=/Users/runner"}
	if got := nativeEnvironment(func(name string) string { return values[name] }); !slices.Equal(got, want) {
		t.Fatalf("native environment = %q, want %q", got, want)
	}
}
