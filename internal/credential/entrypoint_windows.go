//go:build windows

package credential

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func validatePrivateDirectory(path string, info os.FileInfo) error {
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("credential directory is not a real directory")
	}
	return validateOwnedWindowsACL(path)
}

func validatePrivateFile(path string, info os.FileInfo, _ os.FileMode) error {
	if !info.Mode().IsRegular() {
		return errors.New("credential entrypoint file is not regular")
	}
	return validateOwnedWindowsACL(path)
}

func validateOwnedWindowsACL(path string) error {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("credential path encoding: %w", err)
	}
	attributes, err := windows.GetFileAttributes(name)
	if err != nil {
		return fmt.Errorf("inspect credential path attributes: %w", err)
	}
	if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return errors.New("credential path is a reparse point")
	}
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("inspect credential path ACL: %w", err)
	}
	if descriptor == nil {
		return errors.New("credential path has no security descriptor")
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		return fmt.Errorf("inspect credential path owner: %w", err)
	}
	if owner == nil {
		return errors.New("credential path has no owner")
	}
	current, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("inspect current Windows user: %w", err)
	}
	if !owner.Equals(current.User.Sid) {
		return errors.New("credential path is not owned by the current Windows user")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return fmt.Errorf("credential path has no restrictive ACL: %w", err)
	}
	if dacl == nil {
		return errors.New("credential path has no restrictive ACL")
	}
	const foreignWrite = windows.GENERIC_ALL | windows.GENERIC_WRITE |
		windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA |
		windows.FILE_WRITE_EA | windows.FILE_WRITE_ATTRIBUTES |
		windows.DELETE | windows.WRITE_DAC | windows.WRITE_OWNER | 0x40 // FILE_DELETE_CHILD.
	for i := range dacl.AceCount {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, uint32(i), &ace); err != nil {
			return fmt.Errorf("inspect credential ACL entry: %w", err)
		}
		if ace == nil {
			return errors.New("credential ACL contains an invalid entry")
		}
		if ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 || ace.Header.AceType == windows.ACCESS_DENIED_ACE_TYPE {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return errors.New("credential ACL contains an unsupported grant")
		}
		// #nosec G103 -- GetAce returns an OS-owned allow ACE with its SID inline at SidStart.
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if !sid.IsValid() {
			return errors.New("credential ACL contains an invalid principal")
		}
		if sid.Equals(current.User.Sid) || sid.IsWellKnown(windows.WinLocalSystemSid) || sid.IsWellKnown(windows.WinBuiltinAdministratorsSid) {
			continue
		}
		if ace.Mask&foreignWrite != 0 {
			return errors.New("credential path is writable by another Windows principal")
		}
	}
	return nil
}
