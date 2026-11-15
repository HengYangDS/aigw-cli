//go:build darwin && cgo

package native

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Security -framework LocalAuthentication -framework Foundation
#include <Security/Security.h>
#include <LocalAuthentication/LocalAuthentication.h>
#include <stdlib.h>
#include <string.h>

// aigwOpenKeychains opens the keychain files that an isolated test names,
// separated by colons, as the list to search in that order, whose first
// keychain receives a new item. Without a name *keychains stays NULL, which
// selects the user's search list and default keychain. A name that is not an
// absolute path is refused: macOS would resolve it in the user's own keychain
// folder.
static OSStatus aigwOpenKeychains(const char *paths, CFArrayRef *keychains) {
    *keychains = NULL;
    if (paths == NULL || paths[0] == '\0') return errSecSuccess;
    CFMutableArrayRef opened = CFArrayCreateMutable(NULL, 0, &kCFTypeArrayCallBacks);
    char *names = strdup(paths);
    OSStatus status = opened == NULL || names == NULL ? errSecAllocate : errSecSuccess;
    char *rest = names;
    while (status == errSecSuccess && rest != NULL) {
        const char *name = strsep(&rest, ":");
        SecKeychainRef keychain = NULL;
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
        status = name[0] == '/' ? SecKeychainOpen(name, &keychain) : errSecParam;
#pragma clang diagnostic pop
        if (status == errSecSuccess) CFArrayAppendValue(opened, keychain);
        if (keychain != NULL) CFRelease(keychain);
    }
    free(names);
    if (status == errSecSuccess) {
        *keychains = opened;
    } else if (opened != NULL) {
        CFRelease(opened);
    }
    return status;
}

static OSStatus aigwQueryKeychainItem(const char *service, const char *account,
                                     CFArrayRef keychains, CFTypeRef *result,
                                     Boolean returnData) {
    @autoreleasepool {
        CFStringRef serviceName = CFStringCreateWithCString(NULL, service, kCFStringEncodingUTF8);
        CFStringRef accountName = CFStringCreateWithCString(NULL, account, kCFStringEncodingUTF8);
        if (serviceName == NULL || accountName == NULL) {
            if (serviceName != NULL) CFRelease(serviceName);
            if (accountName != NULL) CFRelease(accountName);
            return errSecParam;
        }
        CFMutableDictionaryRef query = CFDictionaryCreateMutable(
            NULL, 0, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
        if (query == NULL) {
            CFRelease(serviceName);
            CFRelease(accountName);
            return errSecAllocate;
        }
        CFDictionarySetValue(query, kSecClass, kSecClassGenericPassword);
        CFDictionarySetValue(query, kSecAttrService, serviceName);
        CFDictionarySetValue(query, kSecAttrAccount, accountName);
        CFDictionarySetValue(query, returnData ? kSecReturnData : kSecReturnAttributes, kCFBooleanTrue);
        CFDictionarySetValue(query, kSecMatchLimit, kSecMatchLimitOne);
        LAContext *context = [[LAContext alloc] init];
        context.interactionNotAllowed = YES;
        CFDictionarySetValue(query, kSecUseAuthenticationContext, (CFTypeRef)context);
        if (keychains != NULL) CFDictionarySetValue(query, kSecMatchSearchList, keychains);
        OSStatus status = SecItemCopyMatching(query, result);
        [context release];
        CFRelease(query);
        CFRelease(serviceName);
        CFRelease(accountName);
        return status;
    }
}

static OSStatus aigwReadKeychainItem(const char *service, const char *account,
                                    CFArrayRef keychains, CFDataRef *result) {
    // The go-keyring provider writes legacy Keychain items. LAContext alone
    // cannot suppress their access prompt, so disable UI only in the
    // dedicated, single-operation credential worker during this call.
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
    Boolean wasAllowed = false;
    OSStatus status = SecKeychainGetUserInteractionAllowed(&wasAllowed);
    if (status != errSecSuccess) return status;
    status = SecKeychainSetUserInteractionAllowed(false);
    if (status != errSecSuccess) return status;
    status = aigwQueryKeychainItem(service, account, keychains, (CFTypeRef *)result, true);
    OSStatus restore = SecKeychainSetUserInteractionAllowed(wasAllowed);
#pragma clang diagnostic pop
    return status == errSecSuccess ? restore : status;
}

static OSStatus aigwObserveKeychainItem(const char *service, const char *account,
                                       CFArrayRef keychains) {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
    Boolean wasAllowed = false;
    OSStatus status = SecKeychainGetUserInteractionAllowed(&wasAllowed);
    if (status != errSecSuccess) return status;
    status = SecKeychainSetUserInteractionAllowed(false);
    if (status != errSecSuccess) return status;
    CFTypeRef attributes = NULL;
    status = aigwQueryKeychainItem(service, account, keychains, &attributes, false);
    if (attributes != NULL) CFRelease(attributes);
    OSStatus restore = SecKeychainSetUserInteractionAllowed(wasAllowed);
#pragma clang diagnostic pop
    return status == errSecSuccess ? restore : status;
}

#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"

// aigwDisableKeychainInteraction turns Keychain user interaction off for the
// whole of a write or a delete, whose steps Go orders, and reports what it was.
// Go may run consecutive cgo calls of one goroutine on different threads, so
// the caller keeps the operation on one thread: the setting then holds for
// every step, whether macOS keeps it for the process or for the thread.
static OSStatus aigwDisableKeychainInteraction(Boolean *wasAllowed) {
    OSStatus status = SecKeychainGetUserInteractionAllowed(wasAllowed);
    if (status != errSecSuccess) return status;
    return SecKeychainSetUserInteractionAllowed(false);
}

static OSStatus aigwRestoreKeychainInteraction(Boolean wasAllowed) {
    return SecKeychainSetUserInteractionAllowed(wasAllowed);
}

// aigwFindKeychainItem looks a slot up by its attributes. It reads no value,
// so it finds an item that this program may not read.
static OSStatus aigwFindKeychainItem(const char *service, const char *account,
                                    CFArrayRef keychains, SecKeychainItemRef *item) {
    return SecKeychainFindGenericPassword(keychains, (UInt32)strlen(service), service,
                                          (UInt32)strlen(account), account, NULL, NULL, item);
}

// aigwCreateKeychainItem creates a slot's item with its label in one call: no
// other writer meets it without one, and macOS names it by that label, not by
// the service, when it asks a person for access. The item goes to the first
// keychain of a test's list, or to the default keychain.
static OSStatus aigwCreateKeychainItem(const char *service, const char *account,
                                      const char *label, CFArrayRef keychains,
                                      const void *value, UInt32 length) {
    SecKeychainRef keychain = keychains == NULL
                                  ? NULL
                                  : (SecKeychainRef)CFArrayGetValueAtIndex(keychains, 0);
    SecKeychainAttribute fields[] = {
        {kSecServiceItemAttr, (UInt32)strlen(service), (void *)service},
        {kSecAccountItemAttr, (UInt32)strlen(account), (void *)account},
        {kSecLabelItemAttr, (UInt32)strlen(label), (void *)label},
    };
    SecKeychainAttributeList attributes = {3, fields};
    return SecKeychainItemCreateFromContent(kSecGenericPasswordItemClass, &attributes,
                                            length, value, keychain, NULL, NULL);
}

// aigwReplaceKeychainItem replaces the value of the item that a lookup found.
static OSStatus aigwReplaceKeychainItem(SecKeychainItemRef item, const char *service,
                                       const char *account, CFArrayRef keychains,
                                       const void *value, UInt32 length) {
    // Do not replace a retained item whose value this identity cannot
    // read: the mutation would commit a new Token without usable access.
    CFDataRef previous = NULL;
    OSStatus status = aigwQueryKeychainItem(service, account, keychains,
                                            (CFTypeRef *)&previous, true);
    if (previous != NULL) CFRelease(previous);
    if (status != errSecSuccess) return status;
    return SecKeychainItemModifyAttributesAndData(item, NULL, length, value);
}

static OSStatus aigwDeleteKeychainItem(SecKeychainItemRef item) {
    return SecKeychainItemDelete(item);
}

// aigwSlotKeychainUnlocked reports whether the keychain that holds a slot's
// item is unlocked, or, for a slot without an item, the keychain that would
// receive one: the first of a test's list, or the default keychain. It reads
// the item's attributes and the keychain's status, neither of which needs the
// keychain unlocked, with user interaction off as every lookup has it.
static OSStatus aigwSlotKeychainUnlocked(const char *service, const char *account,
                                        CFArrayRef keychains, Boolean *unlocked) {
    Boolean wasAllowed = false;
    OSStatus status = aigwDisableKeychainInteraction(&wasAllowed);
    if (status != errSecSuccess) return status;
    SecKeychainItemRef item = NULL;
    SecKeychainRef keychain = NULL;
    status = aigwFindKeychainItem(service, account, keychains, &item);
    if (status == errSecSuccess) {
        status = SecKeychainItemCopyKeychain(item, &keychain);
    } else if (status == errSecItemNotFound && keychains != NULL) {
        keychain = (SecKeychainRef)CFRetain(CFArrayGetValueAtIndex(keychains, 0));
        status = errSecSuccess;
    } else if (status == errSecItemNotFound) {
        status = SecKeychainCopyDefault(&keychain);
    }
    if (status == errSecSuccess) {
        SecKeychainStatus keychainStatus = 0;
        status = SecKeychainGetStatus(keychain, &keychainStatus);
        if (status == errSecSuccess) {
            *unlocked = (keychainStatus & kSecUnlockStateStatus) != 0;
        }
    }
    // The reference goes before any later lookup, which would otherwise
    // return it again after another writer replaced the item.
    if (item != NULL) CFRelease(item);
    if (keychain != NULL) CFRelease(keychain);
    OSStatus restore = aigwRestoreKeychainInteraction(wasAllowed);
    return status == errSecSuccess ? restore : status;
}

#pragma clang diagnostic pop

static void aigwClearAndFree(void *value, size_t length) {
    if (value != NULL) {
        (void)memset_s(value, length, 0, length);
        free(value);
    }
}
*/
import "C"

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"runtime"
	"strings"
	"unsafe"
)

// slotLookups is how often one write or delete looks its slot up. Another
// writer that changes the slot between a lookup and the operation on what it
// found makes macOS refuse that operation, and the mutation then looks the
// slot up again. In every round measured on macOS 27.2.1, 200 rounds of three
// writers that replace one slot at once and 150 of five, at least one writer
// succeeded, and each refused writer got a status that slotChanged names.
// Five lookups are a bound, which the suite pins with two creators and with
// three replacers of one slot, and which ends a writer that keeps losing.
const slotLookups = 5

// keychainStep is where a test lets another process act while an operation
// runs: after each lookup of a write or a delete and before the operation on
// what the lookup found ("looked-up"), and after macOS failed the
// authentication of a read or a write and before the worker reads the lock
// state again ("refused"). The product leaves it a no-op.
var keychainStep = func(point string) {}

// keychainSlot is one credential slot in the keychains of one operation.
type keychainSlot struct {
	service, account *C.char
	// keychains is the search list of an isolated test, whose first keychain
	// receives a new item, or 0 for the user's search list and default
	// keychain.
	keychains C.CFArrayRef
}

// openKeychains opens the keychain files that an isolated test names in path,
// separated by colons. The product names none, and 0 then selects the user's
// search list and default keychain.
func openKeychains(path string) (C.CFArrayRef, error) {
	keychainPath := C.CString(path)
	defer C.free(unsafe.Pointer(keychainPath))

	var keychains C.CFArrayRef
	//nolint:gocritic // cgo maps a generated comparison to this call, which has none.
	if status := C.aigwOpenKeychains(keychainPath, &keychains); status != C.errSecSuccess {
		return 0, &KeychainError{Status: int32(status)}
	}
	return keychains, nil
}

func releaseKeychains(keychains C.CFArrayRef) {
	if keychains != 0 {
		C.CFRelease(C.CFTypeRef(keychains))
	}
}

// openKeychainSlot names one slot of the keychains that path selects, for one
// operation, which closes it.
func openKeychainSlot(service, account, path string) (*keychainSlot, error) {
	keychains, err := openKeychains(path)
	if err != nil {
		return nil, err
	}
	return &keychainSlot{service: C.CString(service), account: C.CString(account), keychains: keychains}, nil
}

func (slot *keychainSlot) close() {
	C.free(unsafe.Pointer(slot.service))
	C.free(unsafe.Pointer(slot.account))
	releaseKeychains(slot.keychains)
}

func observeCredentialInKeychain(service, account, path string) (bool, error) {
	slot, err := openKeychainSlot(service, account, path)
	if err != nil {
		return false, err
	}
	defer slot.close()
	return slot.observe()
}

// observe reports whether the slot holds an item, from its attributes alone.
func (slot *keychainSlot) observe() (bool, error) {
	status := C.aigwObserveKeychainItem(slot.service, slot.account, slot.keychains)
	switch status {
	case C.errSecSuccess:
		return true, nil
	case C.errSecItemNotFound:
		return false, nil
	default:
		return false, &KeychainError{Status: int32(status)}
	}
}

// withLockState adds to the failure of a read or a write whether the slot's
// keychain was unlocked. macOS fails authentication alike for an item that
// this program may not use and for every item of a locked keychain, so that
// status alone is told apart, by the lock state before the operation began,
// which the caller read, and after its refusal: a keychain that was locked at
// either time may have refused for that reason alone. The refused operation
// does not run again.
func (slot *keychainSlot) withLockState(err error, unlockedBefore bool) error {
	if failure, ok := errors.AsType[*KeychainError](err); ok && failure.Status == authenticationFailed {
		keychainStep("refused")
		unlockedAfter := slot.unlocked()
		failure.Unlocked = unlockedBefore && unlockedAfter
	}
	return err
}

// unlocked reports whether the keychain that holds the slot's item, or that
// would receive one, is unlocked. A lock state that cannot be read is not an
// unlocked keychain.
func (slot *keychainSlot) unlocked() bool {
	var unlocked C.Boolean
	status := C.aigwSlotKeychainUnlocked(slot.service, slot.account, slot.keychains, &unlocked)
	return status == C.errSecSuccess && unlocked != 0
}

func readCredentialFromKeychain(service, account, path string) ([]byte, error) {
	slot, err := openKeychainSlot(service, account, path)
	if err != nil {
		return nil, err
	}
	defer slot.close()

	unlocked := slot.unlocked()
	var data C.CFDataRef
	//nolint:gocritic // cgo maps a generated comparison to this call, which has none.
	status := C.aigwReadKeychainItem(slot.service, slot.account, slot.keychains, &data)
	if data != 0 {
		defer C.CFRelease(C.CFTypeRef(data))
	}
	if status == C.errSecItemNotFound {
		return nil, ErrNotFound
	}
	if status == C.errSecAuthFailed {
		// A value query can fail authentication even when the exact slot is absent.
		if present, err := slot.observe(); err == nil && !present {
			return nil, ErrNotFound
		}
	}
	if status != C.errSecSuccess {
		return nil, slot.withLockState(&KeychainError{Status: int32(status)}, unlocked)
	}
	if data == 0 {
		return nil, ErrUnavailable
	}
	if C.CFGetTypeID(C.CFTypeRef(data)) != C.CFDataGetTypeID() {
		return nil, ErrUnavailable
	}
	length := C.CFDataGetLength(data)
	if length < 0 || length > C.CFIndex(2*maxStoredValue+64) {
		return nil, ErrUnavailable
	}
	raw := C.GoBytes(unsafe.Pointer(C.CFDataGetBytePtr(data)), C.int(length))
	defer clear(raw)
	return decodeStoredCredential(raw)
}

// decodeStoredCredential reverses go-keyring's storage envelope, which AIGW's
// writer applies.
func decodeStoredCredential(raw []byte) ([]byte, error) {
	stored := bytes.TrimSpace(raw)
	if encoded, ok := bytes.CutPrefix(stored, []byte("go-keyring-base64:")); ok {
		value := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
		n, err := base64.StdEncoding.Decode(value, encoded)
		if err != nil || n > maxStoredValue {
			clear(value)
			return nil, ErrUnavailable
		}
		return value[:n], nil
	}
	if encoded, ok := bytes.CutPrefix(stored, []byte("go-keyring-encoded:")); ok {
		value := make([]byte, hex.DecodedLen(len(encoded)))
		n, err := hex.Decode(value, encoded)
		if err != nil || n > maxStoredValue {
			clear(value)
			return nil, ErrUnavailable
		}
		return value[:n], nil
	}
	if len(stored) > maxStoredValue {
		return nil, ErrUnavailable
	}
	return bytes.Clone(stored), nil
}

// storedCredential applies go-keyring's storage envelope exactly once.
func storedCredential(value []byte) []byte {
	const prefix = "go-keyring-base64:"
	stored := make([]byte, len(prefix)+base64.StdEncoding.EncodedLen(len(value)))
	copy(stored, prefix)
	base64.StdEncoding.Encode(stored[len(prefix):], value)
	return stored
}

func writeCredentialToKeychain(service, account, path string, value []byte) error {
	if len(value) == 0 || len(value) > maxStoredValue {
		return ErrUnavailable
	}
	slot, err := openKeychainSlot(service, account, path)
	if err != nil {
		return err
	}
	defer slot.close()

	label := C.CString(keychainItemLabel(account))
	defer C.free(unsafe.Pointer(label))
	stored := storedCredential(value)
	defer clear(stored)
	data := C.CBytes(stored)
	defer C.aigwClearAndFree(data, C.size_t(len(stored)))
	length := C.UInt32(len(stored))
	unlocked := slot.unlocked()
	err = slot.mutate(func(item C.SecKeychainItemRef, found bool) C.OSStatus {
		if found {
			return C.aigwReplaceKeychainItem(item, slot.service, slot.account, slot.keychains, data, length)
		}
		return C.aigwCreateKeychainItem(slot.service, slot.account, label, slot.keychains, data, length)
	})
	return slot.withLockState(err, unlocked)
}

func deleteCredentialFromKeychain(service, account, path string) error {
	slot, err := openKeychainSlot(service, account, path)
	if err != nil {
		return err
	}
	defer slot.close()
	return slot.mutate(func(item C.SecKeychainItemRef, found bool) C.OSStatus {
		if found {
			return C.aigwDeleteKeychainItem(item)
		}
		return C.errSecItemNotFound
	})
}

func keychainItemLabel(account string) string {
	if provider, diagnostic := strings.CutPrefix(account, "diagnostic@"); diagnostic {
		return "AIGW Provider Diagnostic: " + provider
	}
	return "AIGW Account Token: " + account
}

// mutate looks the slot up and runs operation on the item that the lookup
// found, or on none when the slot is absent, with Keychain user interaction
// off for all of it. It looks the slot up again when another writer changed
// the slot between a lookup and its operation.
func (slot *keychainSlot) mutate(operation func(item C.SecKeychainItemRef, found bool) C.OSStatus) error {
	changed := false
	status := withoutKeychainInteraction(func() C.OSStatus {
		var result C.OSStatus
		for range slotLookups {
			var item C.SecKeychainItemRef
			//nolint:gocritic // cgo maps a generated comparison to this call, which has none.
			result = C.aigwFindKeychainItem(slot.service, slot.account, slot.keychains, &item)
			keychainStep("looked-up")
			found := result == C.errSecSuccess
			if found || result == C.errSecItemNotFound {
				result = operation(item, found)
			}
			// The reference goes before the next lookup, which would
			// otherwise return it again after another writer replaced the item.
			if found {
				C.CFRelease(C.CFTypeRef(item))
			}
			if changed = slotChanged(found, result); !changed {
				break
			}
		}
		return result
	})
	switch {
	case status == C.errSecSuccess:
		return nil
	case status == C.errSecItemNotFound && !changed:
		return ErrNotFound
	default:
		return &KeychainError{Status: int32(status)}
	}
}

// withoutKeychainInteraction runs operation with Keychain user interaction
// off and puts the setting back. A setting that cannot be changed fails the
// operation before it starts, as it could otherwise prompt. The goroutine
// stays on its thread from the change of the setting to its restoration,
// because Go may otherwise run the steps between them on other threads.
func withoutKeychainInteraction(operation func() C.OSStatus) C.OSStatus {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var wasAllowed C.Boolean
	status := C.aigwDisableKeychainInteraction(&wasAllowed)
	if status == C.errSecSuccess {
		status = operation()
		if restore := C.aigwRestoreKeychainInteraction(wasAllowed); status == C.errSecSuccess {
			status = restore
		}
	}
	return status
}

// slotChanged reports that another writer changed the slot between a lookup,
// which found an item or none, and the operation that ended with status.
func slotChanged(found bool, status C.OSStatus) bool {
	switch status {
	case C.errSecDuplicateItem:
		// A creation met the item that another writer added since the lookup
		// missed it, or a replacement collided with another writer's.
		return true
	case C.errSecItemNotFound, C.errSecInvalidRecord:
		// The lookup's reference names a record that another writer has since
		// replaced or deleted. A lookup that found nothing has no reference:
		// a deletion's answer for an absent slot is its result.
		return found
	default:
		return false
	}
}
