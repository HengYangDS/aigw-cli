//go:build darwin

package keychain

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

func TestNativeReadDisablesInteractionBeforeExactItemLookup(t *testing.T) {
	for _, status := range []int32{0, -25300, -25308, -25293, -25291} {
		var calls []string
		value := []byte("native-value")
		api := nativeAPI{
			interaction: func(allowed bool) int32 {
				if allowed {
					t.Fatal("Keychain UI enabled")
				}
				calls = append(calls, "disable-ui")
				return 0
			},
			find: func(chain uintptr, serviceSize uint32, service string, accountSize uint32, account string, size *uint32, data *unsafe.Pointer, item *uintptr) int32 {
				if chain != 42 || service != "service" || account != "account" || serviceSize != 7 || accountSize != 7 || item != nil {
					t.Fatal("native credential identity changed")
				}
				calls = append(calls, "find")
				if status == 0 {
					*size = 12
					// #nosec G103 -- Test owns this buffer through the synchronous fake native call.
					*data = unsafe.Pointer(&value[0])
				}
				return status
			},
			free: func(_ uintptr, data unsafe.Pointer) int32 {
				// #nosec G103 -- Verify release receives the same test-owned native allocation.
				if data != unsafe.Pointer(&value[0]) {
					t.Fatal("wrong native allocation freed")
				}
				calls = append(calls, "free")
				return 0
			},
		}
		got, err := api.read(42, "service", "account", false)
		want := []string{"disable-ui", "find"}
		if status == 0 {
			want = append(want, "free")
		}
		if !slices.Equal(calls, want) || status == 0 && (!bytes.Equal(got, value) || err != nil) || status != 0 && (got != nil || err == nil) {
			t.Fatalf("status %d: calls %v, result %q, %v", status, calls, got, err)
		}
	}
}

func TestNativeReadRequiresSuccessfulUISuppression(t *testing.T) {
	api := nativeAPI{interaction: func(bool) int32 { return -1 }}
	if value, err := api.read(0, "service", "account", false); value != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("failed UI suppression returned %q, %v", value, err)
	}
}

func TestNativeMutationSuppressesUIAndPreservesItemIdentity(t *testing.T) {
	for _, operation := range []string{writeCommand, deleteCommand} {
		for _, status := range []int32{0, -25308, -25293, -25291} {
			var calls []string
			api := nativeAPI{
				interaction: func(allowed bool) int32 {
					if allowed {
						t.Fatal("mutation enabled interaction")
					}
					calls = append(calls, "disable-ui")
					return 0
				},
				find: func(chain uintptr, serviceSize uint32, service string, accountSize uint32, account string, size *uint32, data *unsafe.Pointer, item *uintptr) int32 {
					if chain != 42 || service != "service" || account != "account" || serviceSize != 7 || accountSize != 7 || size != nil || data != nil || item == nil {
						t.Fatal("mutation lookup changed identity or requested password")
					}
					calls = append(calls, "find")
					*item = 99
					return 0
				},
				modify: func(item, attributes uintptr, size uint32, data unsafe.Pointer) int32 {
					// #nosec G103 -- Inspect the exact borrowed test buffer during this fake native call.
					if item != 99 || attributes != 0 || string(unsafe.Slice((*byte)(data), size)) != "replacement" {
						t.Fatal("update replaced item identity, attributes or bytes")
					}
					calls = append(calls, writeCommand)
					return status
				},
				remove: func(item uintptr) int32 {
					if item != 99 {
						t.Fatal("delete targeted another item")
					}
					calls = append(calls, deleteCommand)
					return status
				},
				release: func(item uintptr) {
					if item != 99 {
						t.Fatal("released another item")
					}
					calls = append(calls, "release")
				},
			}
			err := api.mutate(42, operation, "service", "account", []byte("replacement"))
			if !errors.Is(err, nativeStatus(status)) || !slices.Equal(calls, []string{"disable-ui", "find", operation, "release"}) {
				t.Fatalf("mutation %s status %d: calls=%v error=%v", operation, status, calls, err)
			}
		}
	}
}

func TestNativeMutationRejectsUnadmittedOperationsBeforeLookup(t *testing.T) {
	api := nativeAPI{interaction: func(bool) int32 { return -1 }}
	for _, operation := range []string{workerCommand, writeCommand, deleteCommand} {
		if err := api.mutate(0, operation, "service", "account", nil); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("unavailable mutation %s: %v", operation, err)
		}
	}
	if err := api.mutate(0, writeCommand, "service", "account", make([]byte, maxStoredValue+1)); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("oversized native write: %v", err)
	}
}

func TestNativeObservationRequestsNoValueOrItemAllocation(t *testing.T) {
	queried := false
	api := nativeAPI{
		interaction: func(bool) int32 { return 0 },
		find: func(_ uintptr, _ uint32, _ string, _ uint32, _ string, size *uint32, data *unsafe.Pointer, item *uintptr) int32 {
			queried = true
			if size != nil || data != nil || item != nil {
				t.Fatal("metadata probe requested secret bytes or an owned allocation")
			}
			return 0
		},
	}
	if value, err := api.read(0, "service", "account", true); err != nil || value != nil || !queried {
		t.Fatalf("metadata probe: %v", err)
	}
}

func TestNativeReadReleasesRejectedNativeBuffers(t *testing.T) {
	for _, test := range []struct {
		size       uint32
		freeStatus int32
	}{
		{size: maxStoredValue + 1}, {size: 0, freeStatus: -1}, {size: 1},
	} {
		freed := false
		api := nativeAPI{
			interaction: func(bool) int32 { return 0 },
			find: func(_ uintptr, _ uint32, _ string, _ uint32, _ string, size *uint32, _ *unsafe.Pointer, _ *uintptr) int32 {
				*size = test.size
				return 0
			},
			free: func(uintptr, unsafe.Pointer) int32 { freed = true; return test.freeStatus },
		}
		if value, err := api.read(0, "service", "account", false); value != nil || !freed || !errors.Is(err, ErrUnavailable) {
			t.Fatalf("rejected buffer: %v", err)
		}
	}
}

func TestNativeAbsentReadUsesTheRealBridgeAndBoundedWorker(t *testing.T) {
	// Only a unique absent item is queried; this process disables its own UI.
	account := "aigw-absent-" + filepath.Base(t.TempDir())
	if value, err := queryNative(workerCommand, "AIGW_TOKEN", account, nil); value != nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("native missing item classification: %v", err)
	}
	if value, err := Read("AIGW_TOKEN", account); value != "" || !errors.Is(err, ErrNotFound) {
		t.Fatalf("bounded native missing item classification: %v", err)
	}
	if exists, err := Exists("AIGW_TOKEN", account); exists || err != nil {
		t.Fatalf("bounded absent metadata: %v", err)
	}
}

func TestNativePrivateKeychainReadAndLockedFailure(t *testing.T) {
	if root := os.Getenv("AIGW_TEST_PRIVATE_KEYCHAIN"); root != "" {
		if expectation := os.Getenv("AIGW_TEST_PRIVATE_READER"); expectation != "" {
			testPrivateKeychainReader(t, root, expectation)
			return
		}
		testPrivateKeychain(t, root)
		return
	}
	before, err := exec.Command("/usr/bin/security", "list-keychains", "-d", "user").Output()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNativePrivateKeychainReadAndLockedFailure$")
	command.Env = append(os.Environ(), "AIGW_TEST_PRIVATE_KEYCHAIN="+t.TempDir())
	output, runErr := command.CombinedOutput()
	after, err := exec.Command("/usr/bin/security", "list-keychains", "-d", "user").Output()
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("Keychain search list changed: %v", err)
	}
	if runErr != nil {
		t.Fatalf("isolated native contract: %v\n%s", runErr, output)
	}
}

func testPrivateKeychain(t *testing.T, root string) {
	t.Helper()
	api, closeLibrary, err := loadNative()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := closeLibrary(); err != nil {
			t.Error(err)
		}
	})
	if code := api.interaction(false); code != 0 {
		t.Fatalf("disable UI: %d", code)
	}
	lib, err := purego.Dlopen(securityFramework, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := purego.Dlclose(lib); err != nil {
			t.Error(err)
		}
	})
	var create func(string, uint32, string, bool, uintptr, *uintptr) int32
	var remove func(uintptr) int32
	var lock func(uintptr) int32
	var add func(uintptr, uint32, string, uint32, string, uint32, string, *uintptr) int32
	purego.RegisterLibFunc(&create, lib, "SecKeychainCreate")
	purego.RegisterLibFunc(&remove, lib, "SecKeychainDelete")
	purego.RegisterLibFunc(&lock, lib, "SecKeychainLock")
	purego.RegisterLibFunc(&add, lib, "SecKeychainAddGenericPassword")
	var chain uintptr
	const password = "synthetic-keychain-password"
	if code := create(filepath.Join(root, "fixture.keychain"), uint32(len(password)), password, false, 0, &chain); code != 0 {
		t.Fatalf("create private Keychain: %d", code)
	}
	t.Cleanup(func() {
		defer api.release(chain)
		if code := remove(chain); code != 0 {
			t.Errorf("delete private Keychain: %d", code)
		}
	})
	const value = "go-keyring-base64:c3ludGhldGlj"
	if code := add(chain, 7, "service", 7, "account", uint32(len(value)), value, nil); code != 0 {
		t.Fatalf("create synthetic item: %d", code)
	}
	if got, err := api.read(chain, "service", "account", false); err != nil || string(got) != value {
		t.Fatalf("synthetic exact item read: %v", err)
	}
	testPrivateReaderExecutableIdentity(t, root)
	if got, err := api.read(chain, "service", "absent", false); got != nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing item: %v", err)
	}
	testNativeCredentialMutation(t, api, chain)
	command := exec.Command("/usr/bin/security", "-i")
	command.Stdin = strings.NewReader("add-generic-password -s service -a security-writer -w synthetic-fixture '" + filepath.Join(root, "fixture.keychain") + "'\n")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create security-owned private fixture: %v; output bytes=%d", err, len(output))
	}
	if got, err := api.read(chain, "service", "security-writer", true); err != nil || got != nil {
		t.Fatalf("security-owned metadata: %v", err)
	}
	if got, err := api.read(chain, "service", "security-writer", false); got != nil || !errors.Is(err, ErrDenied) {
		t.Fatalf("foreign writer authorization boundary: %v", err)
	}
	if code := lock(chain); code != 0 {
		t.Fatalf("lock private Keychain: %d", code)
	}
	if got, err := api.read(chain, "service", "account", false); got != nil || !errors.Is(err, ErrDenied) {
		t.Fatalf("locked item: %v", err)
	}
	if err := api.mutate(chain, writeCommand, "service", "account", []byte("replacement")); !errors.Is(err, ErrDenied) {
		t.Fatalf("locked write: %v", err)
	}
	// File-based Keychain deletion has its own authorization: this owned item
	// can be removed while locked without unlocking or reading its password.
	if err := api.mutate(chain, deleteCommand, "service", "account", nil); err != nil {
		t.Fatalf("owned locked-item deletion: %v", err)
	}
	if _, err := api.read(chain, "service", "account", true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted locked item remains: %v", err)
	}
}

func testPrivateKeychainReader(t *testing.T, root, expectation string) {
	t.Helper()
	api, closeLibrary, err := loadNative()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := closeLibrary(); err != nil {
			t.Error(err)
		}
	})
	if code := api.interaction(false); code != 0 {
		t.Fatalf("disable reader interaction: %d", code)
	}
	lib, err := purego.Dlopen(securityFramework, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := purego.Dlclose(lib); err != nil {
			t.Error(err)
		}
	})
	var open func(string, *uintptr) int32
	purego.RegisterLibFunc(&open, lib, "SecKeychainOpen")
	var chain uintptr
	if code := open(filepath.Join(root, "fixture.keychain"), &chain); code != 0 {
		t.Fatalf("open private Keychain: %d", code)
	}
	defer api.release(chain)
	value, err := api.read(chain, "service", "account", false)
	defer clear(value)
	switch expectation {
	case "authorized":
		if err != nil || string(value) != "go-keyring-base64:c3ludGhldGlj" {
			t.Fatalf("same-image subprocess read: %v", err)
		}
	case "denied":
		if value != nil || !errors.Is(err, ErrDenied) {
			t.Fatalf("different-image authorization: %v", err)
		}
	default:
		t.Fatal("unknown reader expectation")
	}
}

func testPrivateReaderExecutableIdentity(t *testing.T, root string) {
	t.Helper()
	copyPath := filepath.Join(root, "successor-reader")
	program, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyPath, program, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, program := range []string{os.Args[0], copyPath} {
		testPrivateReaderProcess(t, program, root, "authorized")
	}
	metadata, err := exec.Command("/usr/bin/codesign", "--display", "--verbose=2", copyPath).CombinedOutput()
	if err != nil {
		t.Fatalf("inspect disposable code identity: %v", err)
	}
	_, identifier, found := strings.Cut(string(metadata), "Identifier=")
	identifier, _, _ = strings.Cut(identifier, "\n")
	if !found || identifier == "" {
		t.Fatal("disposable code has no signing identifier")
	}
	// Only this disposable copy is re-signed. No certificate, user keychain,
	// production executable or item access policy is changed. Both the path
	// and identifier stay the same; code-hash identity alone changes.
	command := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", "--identifier", identifier, copyPath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("different disposable code identity: %v\n%s", err, output)
	}
	testPrivateReaderProcess(t, copyPath, root, "denied")
}

func testPrivateReaderProcess(t *testing.T, program, root, expectation string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, program, "-test.run=^TestNativePrivateKeychainReadAndLockedFailure$")
	command.Env = append(os.Environ(), "AIGW_TEST_PRIVATE_KEYCHAIN="+root, "AIGW_TEST_PRIVATE_READER="+expectation)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("private reader process (%s): %v\n%s", expectation, err, output)
	}
}

func testNativeCredentialMutation(t *testing.T, api nativeAPI, chain uintptr) {
	t.Helper()
	for _, token := range []string{"created-token", "rotated-token"} {
		if err := api.mutate(chain, writeCommand, "service", "native-writer", []byte(token)); err != nil {
			t.Fatalf("native writer: %v", err)
		}
		if got, err := api.read(chain, "service", "native-writer", false); err != nil || string(got) != token {
			t.Fatalf("same-identity read after write: %v", err)
		}
	}
	for range 2 {
		if err := api.mutate(chain, deleteCommand, "service", "native-writer", nil); err != nil {
			t.Fatalf("native delete: %v", err)
		}
	}
	if _, err := api.read(chain, "service", "native-writer", true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted native item remains: %v", err)
	}
}
