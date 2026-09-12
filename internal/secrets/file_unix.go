//go:build !windows

package secrets

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

func validateOwnedFile(info os.FileInfo) error {
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return errors.New("Token path must be an owner-only regular file")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil || stat.Nlink != 1 {
		return errors.New("Token file ownership is ambiguous")
	}
	return validateOwner(info)
}

func validateOwner(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil || int(stat.Uid) != os.Geteuid() {
		return errors.New("Token storage is not owned by the current user")
	}
	return nil
}

func validateSecureRoot(info os.FileInfo) error {
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		return errors.New("Token directory must be an owner-only directory")
	}
	return validateOwner(info)
}

func syncCredentialDirectory(directory syncer) error { return directory.Sync() }
