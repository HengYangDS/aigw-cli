//go:build windows

package native

import (
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	keyring "github.com/zalando/go-keyring"
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

func observeCredential(service, account string) (bool, error) {
	target := service + ":" + account
	found := false
	err := enumerateCredentials(target, func(name string) {
		found = found || name == target
	})
	return found, err
}

// listCredentials names a service's accounts from their target names.
func listCredentials(service string) ([]string, error) {
	prefix := service + ":"
	accounts := []string{}
	err := enumerateCredentials(prefix+"*", func(name string) {
		if account, ok := strings.CutPrefix(name, prefix); ok {
			accounts = append(accounts, account)
		}
	})
	return accounts, err
}

// enumerateCredentials visits the target name of each credential that filter
// selects. The enumeration hands this process every credential's blob; each
// one is zeroed before CredFree and none is read, so only names leave here.
func enumerateCredentials(pattern string, visit func(name string)) error {
	filter, err := windows.UTF16PtrFromString(pattern)
	if err != nil {
		return fmt.Errorf("encode Windows credential target: %w", err)
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
			return nil
		}
		return fmt.Errorf("enumerate Windows credential metadata: %w", callErr)
	}
	defer func() { _, _, _ = credentialFree.Call(uintptr(credentialsPointer)) }()

	// #nosec G103 -- Count and array share the Win32 allocation and remain live until CredFree.
	credentials := unsafe.Slice((**windowsCredential)(credentialsPointer), count)
	for _, credential := range credentials {
		if credential == nil {
			continue
		}
		if credential.CredentialBlob != nil && credential.CredentialBlobSize > 0 {
			// #nosec G103 -- The size and pointer come from the same Win32 allocation, which stays live until CredFree.
			clear(unsafe.Slice(credential.CredentialBlob, credential.CredentialBlobSize))
		}
		visit(windows.UTF16PtrToString(credential.TargetName))
	}
	return nil
}

func nativeEnvironment(getenv func(string) string) []string {
	return retainedEnvironment(getenv, "USERPROFILE", "APPDATA", "LOCALAPPDATA", "SystemRoot", "HOMEDRIVE", "HOMEPATH")
}

// Credential Manager authorizes the user rather than the reading executable, so
// every program role reads and writes the same item.
func writeCredential(service, account string, value []byte, _ Role) error {
	return keyring.Set(service, account, string(value))
}

// restoreCredential and reclaimCredential have no Windows meaning: a rollback
// needs no conversion where the user, not the executable, is authorized.
func restoreCredential(string, string) error { return ErrUnavailable }

func reclaimCredential(string, string) error { return ErrUnavailable }
