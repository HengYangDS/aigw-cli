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
	var credentialsPointer uintptr
	result, _, callErr := syscall.SyscallN(
		credentialEnumerate.Addr(),
		uintptr(unsafe.Pointer(filter)),
		0,
		uintptr(unsafe.Pointer(&count)),
		uintptr(unsafe.Pointer(&credentialsPointer)),
	)
	runtime.KeepAlive(filter)
	if result == 0 {
		if callErr == windows.ERROR_NOT_FOUND {
			return false, nil
		}
		return false, fmt.Errorf("enumerate Windows credential metadata: %w", callErr)
	}
	defer credentialFree.Call(credentialsPointer)

	credentials := unsafe.Slice((**windowsCredential)(unsafe.Pointer(credentialsPointer)), count)
	for _, credential := range credentials {
		if credential != nil && windows.UTF16PtrToString(credential.TargetName) == target {
			return true, nil
		}
	}
	return false, nil
}
