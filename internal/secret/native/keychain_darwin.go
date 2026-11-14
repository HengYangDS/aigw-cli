//go:build darwin && cgo

package native

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Security -framework LocalAuthentication -framework Foundation
#include <Security/Security.h>
#include <LocalAuthentication/LocalAuthentication.h>
#include <stdlib.h>
#include <string.h>

static OSStatus aigwQueryKeychainItem(const char *service, const char *account,
                                     const char *path, CFTypeRef *result,
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

        SecKeychainRef keychain = NULL;
        CFArrayRef searchList = NULL;
        OSStatus status = errSecSuccess;
        if (path != NULL && path[0] != '\0') {
            // The explicit search list is used only by isolated test keychains.
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
            status = SecKeychainOpen(path, &keychain);
#pragma clang diagnostic pop
            if (status == errSecSuccess) {
                const void *items[] = {keychain};
                searchList = CFArrayCreate(NULL, items, 1, &kCFTypeArrayCallBacks);
                if (searchList == NULL) {
                    status = errSecAllocate;
                } else {
                    CFDictionarySetValue(query, kSecMatchSearchList, searchList);
                }
            }
        }
        if (status == errSecSuccess) {
            status = SecItemCopyMatching(query, result);
        }
        if (searchList != NULL) CFRelease(searchList);
        if (keychain != NULL) CFRelease(keychain);
        [context release];
        CFRelease(query);
        CFRelease(serviceName);
        CFRelease(accountName);
        return status;
    }
}

static OSStatus aigwReadKeychainItem(const char *service, const char *account,
                                    const char *path, CFDataRef *result) {
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
    status = aigwQueryKeychainItem(service, account, path, (CFTypeRef *)result, true);
    OSStatus restore = SecKeychainSetUserInteractionAllowed(wasAllowed);
#pragma clang diagnostic pop
    return status == errSecSuccess ? restore : status;
}

static OSStatus aigwObserveKeychainItem(const char *service, const char *account,
                                       const char *path) {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
    Boolean wasAllowed = false;
    OSStatus status = SecKeychainGetUserInteractionAllowed(&wasAllowed);
    if (status != errSecSuccess) return status;
    status = SecKeychainSetUserInteractionAllowed(false);
    if (status != errSecSuccess) return status;
    CFTypeRef attributes = NULL;
    status = aigwQueryKeychainItem(service, account, path, &attributes, false);
    if (attributes != NULL) CFRelease(attributes);
    OSStatus restore = SecKeychainSetUserInteractionAllowed(wasAllowed);
#pragma clang diagnostic pop
    return status == errSecSuccess ? restore : status;
}

static OSStatus aigwMutateKeychainItem(const char *service, const char *account,
                                      const char *label, const char *path, const void *value,
                                      UInt32 length, Boolean removeItem) {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
    Boolean wasAllowed = false;
    OSStatus status = SecKeychainGetUserInteractionAllowed(&wasAllowed);
    if (status != errSecSuccess) return status;
    status = SecKeychainSetUserInteractionAllowed(false);
    if (status != errSecSuccess) return status;

    SecKeychainRef keychain = NULL;
    SecKeychainItemRef item = NULL;
    if (path != NULL && path[0] != '\0') {
        status = SecKeychainOpen(path, &keychain);
    }
    if (status == errSecSuccess) {
        UInt32 serviceLength = (UInt32)strlen(service);
        UInt32 accountLength = (UInt32)strlen(account);
        status = SecKeychainFindGenericPassword(keychain, serviceLength, service,
                                                accountLength, account,
                                                NULL, NULL, &item);
        Boolean created = false;
        if (!removeItem && status == errSecItemNotFound) {
            status = SecKeychainAddGenericPassword(keychain, serviceLength, service,
                                                   accountLength, account,
                                                   length, value, &item);
            created = status == errSecSuccess;
            if (status == errSecDuplicateItem) {
                // Another writer created the slot since the lookup: update the
                // item that it added.
                status = SecKeychainFindGenericPassword(keychain, serviceLength, service,
                                                        accountLength, account,
                                                        NULL, NULL, &item);
            }
        }
        if (removeItem) {
            if (status == errSecSuccess) status = SecKeychainItemDelete(item);
        } else if (created) {
            SecKeychainAttribute attribute = {
                kSecLabelItemAttr, (UInt32)strlen(label), (void *)label};
            SecKeychainAttributeList attributes = {1, &attribute};
            status = SecKeychainItemModifyAttributesAndData(item, &attributes, 0, NULL);
            if (status != errSecSuccess) {
                OSStatus rollback = SecKeychainItemDelete(item);
                if (rollback != errSecSuccess) status = rollback;
            }
        } else if (status == errSecSuccess) {
            // Do not replace a retained item whose value this identity cannot
            // read: the mutation would commit a new Token without usable access.
            CFDataRef previous = NULL;
            status = aigwQueryKeychainItem(service, account, path,
                                           (CFTypeRef *)&previous, true);
            if (previous != NULL) CFRelease(previous);
            if (status == errSecSuccess) {
                status = SecKeychainItemModifyAttributesAndData(item, NULL, length, value);
            }
        }
    }
    if (item != NULL) CFRelease(item);
    if (keychain != NULL) CFRelease(keychain);
    OSStatus restore = SecKeychainSetUserInteractionAllowed(wasAllowed);
#pragma clang diagnostic pop
    return status == errSecSuccess ? restore : status;
}

static void aigwClearAndFree(void *value, size_t length) {
    if (value != NULL) {
        (void)memset_s(value, length, 0, length);
        free(value);
    }
}

// aigwKeychainUnlocked reports whether the keychain at path, or the default
// keychain without one, is unlocked. It reads the keychain's status, which
// needs no prompt. An explicit path selects an isolated test keychain.
static OSStatus aigwKeychainUnlocked(const char *path, Boolean *unlocked) {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
    SecKeychainRef keychain = NULL;
    OSStatus status = path != NULL && path[0] != '\0'
                          ? SecKeychainOpen(path, &keychain)
                          : SecKeychainCopyDefault(&keychain);
    if (status == errSecSuccess) {
        SecKeychainStatus keychainStatus = 0;
        status = SecKeychainGetStatus(keychain, &keychainStatus);
        if (status == errSecSuccess) {
            *unlocked = (keychainStatus & kSecUnlockStateStatus) != 0;
        }
    }
    if (keychain != NULL) CFRelease(keychain);
#pragma clang diagnostic pop
    return status;
}
*/
import "C"

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"unsafe"
)

func observeCredentialInKeychain(service, account, path string) (bool, error) {
	serviceName := C.CString(service)
	accountName := C.CString(account)
	keychainPath := C.CString(path)
	defer C.free(unsafe.Pointer(serviceName))
	defer C.free(unsafe.Pointer(accountName))
	defer C.free(unsafe.Pointer(keychainPath))

	status := C.aigwObserveKeychainItem(serviceName, accountName, keychainPath)
	switch status {
	case C.errSecSuccess:
		return true, nil
	case C.errSecItemNotFound:
		return false, nil
	default:
		return false, &KeychainError{Status: int32(status)}
	}
}

// withLockState adds to the failure of a read or a write whether the keychain
// was unlocked. macOS fails authentication alike for an item that this program
// may not use and for every item of a locked keychain, so the lock state is
// read after that status and no other.
func withLockState(err error, path string) error {
	if failure, ok := errors.AsType[*KeychainError](err); ok && failure.Status == authenticationFailed {
		failure.Unlocked = keychainUnlocked(path)
	}
	return err
}

// keychainUnlocked reports whether the selected keychain is unlocked.
func keychainUnlocked(path string) bool {
	keychainPath := C.CString(path)
	defer C.free(unsafe.Pointer(keychainPath))

	var unlocked C.Boolean
	status := C.aigwKeychainUnlocked(keychainPath, &unlocked)
	return status == C.errSecSuccess && unlocked != 0
}

func readCredentialFromKeychain(service, account, path string) ([]byte, error) {
	serviceName := C.CString(service)
	accountName := C.CString(account)
	keychainPath := C.CString(path)
	defer C.free(unsafe.Pointer(serviceName))
	defer C.free(unsafe.Pointer(accountName))
	defer C.free(unsafe.Pointer(keychainPath))

	var data C.CFDataRef
	//nolint:gocritic // cgo maps a generated comparison to this call, which has none.
	status := C.aigwReadKeychainItem(serviceName, accountName, keychainPath, &data)
	if data != 0 {
		defer C.CFRelease(C.CFTypeRef(data))
	}
	if status == C.errSecItemNotFound {
		return nil, ErrNotFound
	}
	if status == C.errSecAuthFailed {
		// A value query can fail authentication even when the exact slot is absent.
		if present, err := observeCredentialInKeychain(service, account, path); err == nil && !present {
			return nil, ErrNotFound
		}
	}
	if status != C.errSecSuccess {
		return nil, withLockState(&KeychainError{Status: int32(status)}, path)
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
	return withLockState(mutateCredentialInKeychain(service, account, path, value, false), path)
}

func deleteCredentialFromKeychain(service, account, path string) error {
	return mutateCredentialInKeychain(service, account, path, nil, true)
}

func keychainItemLabel(account string) string {
	if provider, diagnostic := strings.CutPrefix(account, "diagnostic@"); diagnostic {
		return "AIGW Provider Diagnostic: " + provider
	}
	return "AIGW Account Token: " + account
}

func mutateCredentialInKeychain(service, account, path string, value []byte, remove bool) error {
	serviceName := C.CString(service)
	accountName := C.CString(account)
	labelName := C.CString(keychainItemLabel(account))
	keychainPath := C.CString(path)
	defer C.free(unsafe.Pointer(serviceName))
	defer C.free(unsafe.Pointer(accountName))
	defer C.free(unsafe.Pointer(labelName))
	defer C.free(unsafe.Pointer(keychainPath))

	var stored []byte
	if !remove {
		stored = storedCredential(value)
		defer clear(stored)
	}
	var data unsafe.Pointer
	if len(stored) > 0 {
		data = C.CBytes(stored)
		defer C.aigwClearAndFree(data, C.size_t(len(stored)))
	}
	var deleting C.Boolean
	if remove {
		deleting = 1
	}
	status := C.aigwMutateKeychainItem(
		serviceName, accountName, labelName, keychainPath, data, C.UInt32(len(stored)), deleting,
	)
	if status == C.errSecItemNotFound {
		return ErrNotFound
	}
	if status != C.errSecSuccess {
		return &KeychainError{Status: int32(status)}
	}
	return nil
}
