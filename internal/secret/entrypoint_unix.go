//go:build !windows

package secret

import (
	"errors"
	"os"
	"syscall"
)

func validatePrivateDirectory(_ string, info os.FileInfo) error {
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		return errors.New("credential directory is not owner-only")
	}
	return validateOwner(info, "credential entrypoint path")
}

func validatePrivateFile(_ string, info os.FileInfo, mode os.FileMode) error {
	if !info.Mode().IsRegular() || info.Mode().Perm() != mode {
		return errors.New("credential entrypoint file is not owner-only")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil || stat.Nlink != 1 {
		return errors.New("credential entrypoint file ownership is ambiguous")
	}
	return validateOwner(info, "credential entrypoint path")
}
