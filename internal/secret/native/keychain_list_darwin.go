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
static OSStatus aigwCopyKeychainAttributes(CFStringRef service, CFArrayRef keychains, CFTypeRef *results) {
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
    if (keychains != NULL) CFDictionarySetValue(query, kSecMatchSearchList, keychains);
    OSStatus status = SecItemCopyMatching(query, results);
    [context release];
    CFRelease(query);
    return status;
}

// aigwListKeychainAccounts lists a service's accounts with user interaction
// off for the single-operation worker, as the item observation does.
static OSStatus aigwListKeychainAccounts(const char *service, CFArrayRef keychains, char **joined) {
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
            status = aigwCopyKeychainAttributes(serviceName, keychains, &results);
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
	keychains, err := openKeychains(path)
	if err != nil {
		return nil, err
	}
	defer releaseKeychains(keychains)
	serviceName := C.CString(service)
	defer C.free(unsafe.Pointer(serviceName))

	var joined *C.char
	//nolint:gocritic // cgo maps a generated comparison to this call, which has none.
	status := C.aigwListKeychainAccounts(serviceName, keychains, &joined)
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
