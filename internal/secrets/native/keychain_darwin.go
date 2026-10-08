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
        if (removeItem) {
            if (status == errSecSuccess) status = SecKeychainItemDelete(item);
        } else if (status == errSecItemNotFound) {
            status = SecKeychainAddGenericPassword(keychain, serviceLength, service,
                                                   accountLength, account,
                                                   length, value, &item);
            if (status == errSecSuccess) {
                SecKeychainAttribute attribute = {
                    kSecLabelItemAttr, (UInt32)strlen(label), (void *)label};
                SecKeychainAttributeList attributes = {1, &attribute};
                status = SecKeychainItemModifyAttributesAndData(item, &attributes, 0, NULL);
                if (status != errSecSuccess) {
                    OSStatus rollback = SecKeychainItemDelete(item);
                    if (rollback != errSecSuccess) status = rollback;
                }
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
*/
import "C"

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
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
		return false, fmt.Errorf("noninteractive Keychain metadata query failed (%d): %w", int(status), ErrUnavailable)
	}
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
		return nil, fmt.Errorf("noninteractive Keychain read failed (%d): %w", int(status), ErrUnavailable)
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

func writeCredentialToKeychain(service, account, path string, value []byte) error {
	if len(value) == 0 || len(value) > maxStoredValue {
		return ErrUnavailable
	}
	return mutateCredentialInKeychain(service, account, path, value, false)
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
		const prefix = "go-keyring-base64:"
		stored = make([]byte, len(prefix)+base64.StdEncoding.EncodedLen(len(value)))
		copy(stored, prefix)
		base64.StdEncoding.Encode(stored[len(prefix):], value)
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
		return fmt.Errorf("noninteractive Keychain mutation failed (%d): %w", int(status), ErrUnavailable)
	}
	return nil
}
