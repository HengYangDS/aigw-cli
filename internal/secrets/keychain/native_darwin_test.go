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
		{size: 128*1024 + 1}, {size: 0, freeStatus: -1}, {size: 1},
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
	if value, err := queryNative("AIGW_TOKEN", account, false); value != nil || !errors.Is(err, ErrNotFound) {
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
	if got, err := api.read(chain, "service", "absent", false); got != nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing item: %v", err)
	}
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
}
