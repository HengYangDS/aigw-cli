//go:build darwin && cgo

package native

/*
#include <Security/Security.h>
#include <LocalAuthentication/LocalAuthentication.h>
#include <stdlib.h>
#include <string.h>

// aigwJoinKeychainAccounts copies each item's account name, one per line. A
// name too long to be a provider ID is skipped.
static char *aigwJoinKeychainAccounts(CFArrayRef items) {
    CFIndex count = CFArrayGetCount(items);
    char *joined = malloc((size_t)count * 129 + 1);
    if (joined == NULL) return NULL;
    size_t used = 0;
    for (CFIndex index = 0; index < count; index++) {
        CFTypeRef item = CFArrayGetValueAtIndex(items, index);
        if (item == NULL || CFGetTypeID(item) != CFDictionaryGetTypeID()) continue;
        CFTypeRef account = CFDictionaryGetValue((CFDictionaryRef)item, kSecAttrAccount);
        if (account == NULL || CFGetTypeID(account) != CFStringGetTypeID()) continue;
        char name[129];
        if (!CFStringGetCString((CFStringRef)account, name, sizeof name, kCFStringEncodingUTF8)) continue;
        size_t length = strlen(name);
        memcpy(joined + used, name, length);
        used += length;
        joined[used++] = '\n';
    }
    joined[used] = '\0';
    return joined;
}

// aigwCopyKeychainAttributes runs the attribute-only query of every item of a
// service. It asks for no item's data and forbids any authentication UI.
static OSStatus aigwCopyKeychainAttributes(CFStringRef service, const char *path, CFTypeRef *results) {
    CFMutableDictionaryRef query = CFDictionaryCreateMutable(
        NULL, 0, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
    if (query == NULL) return errSecAllocate;
    CFDictionarySetValue(query, kSecClass, kSecClassGenericPassword);
    CFDictionarySetValue(query, kSecAttrService, service);
    CFDictionarySetValue(query, kSecReturnAttributes, kCFBooleanTrue);
    CFDictionarySetValue(query, kSecMatchLimit, kSecMatchLimitAll);
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
    if (status == errSecSuccess) status = SecItemCopyMatching(query, results);
    if (searchList != NULL) CFRelease(searchList);
    if (keychain != NULL) CFRelease(keychain);
    [context release];
    CFRelease(query);
    return status;
}

// aigwListKeychainAccounts lists a service's accounts with user interaction
// off for the single-operation worker, as the item observation does.
static OSStatus aigwListKeychainAccounts(const char *service, const char *path, char **joined) {
    *joined = NULL;
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
    Boolean wasAllowed = false;
    OSStatus status = SecKeychainGetUserInteractionAllowed(&wasAllowed);
    if (status != errSecSuccess) return status;
    status = SecKeychainSetUserInteractionAllowed(false);
    if (status != errSecSuccess) return status;
    @autoreleasepool {
        CFStringRef serviceName = CFStringCreateWithCString(NULL, service, kCFStringEncodingUTF8);
        if (serviceName == NULL) {
            status = errSecParam;
        } else {
            CFTypeRef results = NULL;
            status = aigwCopyKeychainAttributes(serviceName, path, &results);
            if (status == errSecSuccess) {
                if (results != NULL && CFGetTypeID(results) == CFArrayGetTypeID()) {
                    *joined = aigwJoinKeychainAccounts((CFArrayRef)results);
                }
                if (*joined == NULL) status = errSecAllocate;
            }
            if (results != NULL) CFRelease(results);
            CFRelease(serviceName);
        }
    }
    OSStatus restore = SecKeychainSetUserInteractionAllowed(wasAllowed);
#pragma clang diagnostic pop
    return status == errSecSuccess ? restore : status;
}
*/
import "C"

import (
	"strings"
	"unsafe"
)

// listAccountsInKeychain names the accounts of a service's items from their
// attributes alone. A keychain that would need a person fails with its status.
func listAccountsInKeychain(service, path string) ([]string, error) {
	serviceName := C.CString(service)
	keychainPath := C.CString(path)
	defer C.free(unsafe.Pointer(serviceName))
	defer C.free(unsafe.Pointer(keychainPath))

	var joined *C.char
	//nolint:gocritic // cgo maps a generated comparison to this call, which has none.
	status := C.aigwListKeychainAccounts(serviceName, keychainPath, &joined)
	if joined != nil {
		defer C.free(unsafe.Pointer(joined))
	}
	switch status {
	case C.errSecSuccess:
		return strings.Split(C.GoString(joined), "\n"), nil
	case C.errSecItemNotFound:
		return nil, nil
	default:
		return nil, &KeychainError{Status: int32(status)}
	}
}
