//go:build darwin && cgo

package native

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Security -framework LocalAuthentication -framework Foundation
#include <Security/Security.h>
#include <LocalAuthentication/LocalAuthentication.h>
#include <stdlib.h>

static OSStatus aigwQueryKeychainItem(const char *service, const char *account,
                                     const char *path, CFDataRef *result) {
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
        CFDictionarySetValue(query, kSecReturnData, kCFBooleanTrue);
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
            status = SecItemCopyMatching(query, (CFTypeRef *)result);
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
    status = aigwQueryKeychainItem(service, account, path, result);
    OSStatus restore = SecKeychainSetUserInteractionAllowed(wasAllowed);
#pragma clang diagnostic pop
    return status == errSecSuccess ? restore : status;
}
*/
import "C"

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"unsafe"
)

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
	if status != C.errSecSuccess || data == 0 {
		return nil, fmt.Errorf("noninteractive Keychain read failed (%d): %w", int(status), ErrUnavailable)
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
