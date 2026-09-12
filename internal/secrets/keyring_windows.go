//go:build windows

package secrets

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsCredential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         unsafe.Pointer
	TargetAlias        *uint16
	UserName           *uint16
}

var (
	advapi32            = windows.NewLazySystemDLL("advapi32.dll")
	credentialEnumerate = advapi32.NewProc("CredEnumerateW")
	credentialFree      = advapi32.NewProc("CredFree")
)

func observeKeyringItem(service, slot string) (bool, error) {
	target := service + ":" + slot
	filter, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return false, fmt.Errorf("encode Windows credential target: %w", err)
	}

	var count uint32
	var credentialsPointer unsafe.Pointer
	result, _, callErr := syscall.SyscallN(
		credentialEnumerate.Addr(),
		uintptr(unsafe.Pointer(filter)), // #nosec G103 -- NUL-terminated UTF-16 input remains live through this synchronous Win32 call.
		0,
		uintptr(unsafe.Pointer(&count)), // #nosec G103 -- Win32 writes its DWORD count to this correctly sized output.
		uintptr(unsafe.Pointer(&credentialsPointer)), // #nosec G103 -- Win32 returns one allocation, released by CredFree below.
	)
	runtime.KeepAlive(filter)
	if result == 0 {
		if callErr == windows.ERROR_NOT_FOUND {
			return false, nil
		}
		return false, fmt.Errorf("enumerate Windows credential metadata: %w", callErr)
	}
	// CredFree returns VOID; LazyProc's return values do not represent errors.
	defer func() { _, _, _ = credentialFree.Call(uintptr(credentialsPointer)) }()

	// #nosec G103 -- Count and array come from the same Win32 allocation; inspect names only before CredFree.
	credentials := unsafe.Slice((**windowsCredential)(credentialsPointer), count)
	for _, credential := range credentials {
		if credential != nil && windows.UTF16PtrToString(credential.TargetName) == target {
			return true, nil
		}
	}
	return false, nil
}
