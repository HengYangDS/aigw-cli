//go:build !windows

package secret

import (
	"errors"
	"os"
	"syscall"
)

func encodeCredential(value []byte) ([]byte, error) {
	return append([]byte(nil), value...), nil
}

func decodeCredential(value []byte) ([]byte, error) {
	return append([]byte(nil), value...), nil
}

// tokenFileRequirement and tokenDirectoryRequirement say what AIGW accepts of
// a Token file and of the directory that holds it.
const (
	tokenFileRequirement      = "an owner-only regular file (mode 600)"
	tokenDirectoryRequirement = "an owner-only directory (mode 700)"
)

func validateOwnedFile(info os.FileInfo) error {
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return unsafeTokenFileError{name: info.Name(), message: "Token path must be an owner-only regular file"}
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil || stat.Nlink != 1 {
		return errors.New("Token file ownership is ambiguous")
	}
	return validateOwner(info, "Token storage")
}

// validateOwner refuses a path that the current user does not own; subject
// names the path in the error.
func validateOwner(info os.FileInfo, subject string) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil || int(stat.Uid) != os.Geteuid() {
		return errors.New(subject + " is not owned by the current user")
	}
	return nil
}

func validateSecureRoot(info os.FileInfo) error {
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		return unsafeTokenDirectoryError{message: "Token directory must be an owner-only directory"}
	}
	return validateOwner(info, "Token storage")
}

func syncCredentialDirectory(directory syncer) error { return directory.Sync() }
