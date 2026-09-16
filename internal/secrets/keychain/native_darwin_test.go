//go:build darwin

package keychain

import (
	"bytes"
	"errors"
	"slices"
	"testing"
	"unsafe"
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
