// Package transaction owns guarded, byte-exact filesystem mutation and
// compensation primitives used by higher-level domains.
package transaction

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FileSnapshot is the exact file state observed during transaction
// preparation. It deliberately captures only data, existence, digest, and
// POSIX mode: ownership, ACLs, and extended attributes need a separately
// designed cross-platform contract.
type FileSnapshot struct {
	Exists bool
	Data   []byte
	SHA256 string
	Mode   os.FileMode
}

type snapshotFile interface {
	io.Reader
	Stat() (os.FileInfo, error)
	Close() error
}

type temporaryFile interface {
	Name() string
	Write([]byte) (int, error)
	Chmod(os.FileMode) error
	Sync() error
	Close() error
}

// CaptureFileSnapshot records a file's byte-exact current state. A missing
// file is a valid snapshot because sidecars are optional.
func CaptureFileSnapshot(path string) (FileSnapshot, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return FileSnapshot{}, nil
	}
	if err != nil {
		return FileSnapshot{}, fmt.Errorf("read %s: %w", path, err)
	}
	return captureOpenedFile(path, file)
}

func captureOpenedFile(path string, file snapshotFile) (FileSnapshot, error) {
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(file)
	if err != nil {
		return FileSnapshot{}, fmt.Errorf("read %s: %w", path, err)
	}
	info, err := file.Stat()
	if err != nil {
		return FileSnapshot{}, fmt.Errorf("inspect %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return FileSnapshot{
		Exists: true,
		Data:   append([]byte(nil), data...),
		SHA256: hex.EncodeToString(sum[:]),
		Mode:   info.Mode().Perm(),
	}, nil
}

// WriteFileAtomicIfUnchanged makes a best-effort guarded write: it refuses to
// replace a file whose current snapshot differs from the prepared preimage.
// This is not a cross-process compare-and-swap; an uncooperative writer can
// still race the subsequent atomic rename.
func WriteFileAtomicIfUnchanged(path string, expected FileSnapshot, data []byte, defaultMode os.FileMode) (FileSnapshot, error) {
	return writeFileAtomicIfUnchanged(path, expected, data, defaultMode, WriteFileAtomic)
}

// WriteFileAtomicExactModeIfUnchanged is the same guarded write with the mode
// taken as given rather than inherited from the file already there. An artifact
// whose permissions are part of its contract uses this, so a mode that drifted
// wider is corrected instead of carried forward.
func WriteFileAtomicExactModeIfUnchanged(path string, expected FileSnapshot, data []byte, mode os.FileMode) (FileSnapshot, error) {
	return writeFileAtomicIfUnchanged(path, expected, data, mode, WriteFileAtomicExactMode)
}

func writeFileAtomicIfUnchanged(
	path string,
	expected FileSnapshot,
	data []byte,
	defaultMode os.FileMode,
	write func(string, []byte, os.FileMode) error,
) (FileSnapshot, error) {
	current, err := CaptureFileSnapshot(path)
	if err != nil {
		return FileSnapshot{}, err
	}
	if !sameFileSnapshot(current, expected) {
		return FileSnapshot{}, fmt.Errorf("preimage changed for %s; refusing to overwrite newer state", path)
	}
	if err := write(path, data, defaultMode); err != nil {
		return FileSnapshot{}, err
	}
	return CaptureFileSnapshot(path)
}

// RemoveFileIfUnchanged removes a prepared file only when it has not changed
// since preparation. It is used for sidecars that were absent before a
// projection and must be absent again after a compensated rollback.
func RemoveFileIfUnchanged(path string, expected FileSnapshot) (FileSnapshot, error) {
	return removeFileIfUnchanged(path, expected, os.Remove)
}

func removeFileIfUnchanged(path string, expected FileSnapshot, remove func(string) error) (FileSnapshot, error) {
	current, err := CaptureFileSnapshot(path)
	if err != nil {
		return FileSnapshot{}, err
	}
	if !sameFileSnapshot(current, expected) {
		return FileSnapshot{}, fmt.Errorf("preimage changed for %s; refusing to overwrite newer state", path)
	}
	if !current.Exists {
		return FileSnapshot{}, nil
	}
	if err := remove(path); err != nil {
		return FileSnapshot{}, fmt.Errorf("remove %s: %w", path, err)
	}
	return FileSnapshot{}, nil
}

// RestoreFileAtomicIfPostimage restores a prepared preimage only while the
// file still equals the transaction's own postimage. It never overwrites a
// newer external edit.
func RestoreFileAtomicIfPostimage(path string, preimage, postimage FileSnapshot) error {
	return restoreFileAtomicIfPostimage(path, preimage, postimage, os.Remove)
}

func restoreFileAtomicIfPostimage(path string, preimage, postimage FileSnapshot, remove func(string) error) error {
	current, err := CaptureFileSnapshot(path)
	if err != nil {
		return err
	}
	if !sameFileSnapshot(current, postimage) {
		return fmt.Errorf("postimage changed for %s; refusing to overwrite newer state", path)
	}
	if preimage.Exists {
		return WriteFileAtomicExactMode(path, preimage.Data, preimage.Mode)
	}
	if !current.Exists {
		return nil
	}
	if err := remove(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func sameFileSnapshot(left, right FileSnapshot) bool {
	return left.Exists == right.Exists &&
		left.SHA256 == right.SHA256 &&
		left.Mode == right.Mode &&
		bytes.Equal(left.Data, right.Data)
}

func WriteFileAtomic(path string, data []byte, defaultMode os.FileMode) error {
	return writeFileAtomic(path, data, defaultMode, true)
}

// WriteFileAtomicExactMode replaces a file without inheriting the current
// target mode. It is used where the mode is part of what is being written:
// byte-and-mode-exact transaction rollback, and artifacts whose permissions are
// a contract rather than a user preference.
func WriteFileAtomicExactMode(path string, data []byte, mode os.FileMode) error {
	return writeFileAtomic(path, data, mode, false)
}

func writeFileAtomic(path string, data []byte, defaultMode os.FileMode, preserveExistingMode bool) error {
	mode := defaultMode
	if preserveExistingMode {
		if info, err := os.Stat(path); err == nil {
			mode = info.Mode().Perm()
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect %s: %w", path, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".aigw-write-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()
	return commitTemporaryFile(tmp, path, data, mode)
}

func commitTemporaryFile(tmp temporaryFile, path string, data []byte, mode os.FileMode) error {
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		return fmt.Errorf("set temporary mode: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
