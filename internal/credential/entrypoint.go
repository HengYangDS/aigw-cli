package credential

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"aigw-cli/internal/transaction"
)

const entrypointDirectory = "credential"

// VersionedEntrypointPath binds a private executable path to the exact source
// bytes without creating files or relying on an installer-owned public link.
func VersionedEntrypointPath(dataDir, source, installName string) (path string, err error) {
	if !filepath.IsAbs(dataDir) || !filepath.IsAbs(source) {
		return "", errors.New("credential data directory and source must be absolute paths")
	}
	if installName == "" || installName == "." || installName == ".." || filepath.Base(installName) != installName {
		return "", errors.New("credential executable name must be one file name")
	}
	executable, err := os.Open(source)
	if err != nil {
		return "", fmt.Errorf("open AIGW executable for credential identity: %w", err)
	}
	defer func() { err = errors.Join(err, executable.Close()) }()
	info, err := executable.Stat()
	if err != nil {
		return "", fmt.Errorf("inspect AIGW executable for credential identity: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("AIGW credential source is not a regular executable")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, executable); err != nil {
		return "", fmt.Errorf("hash AIGW executable for credential identity: %w", err)
	}
	return filepath.Join(dataDir, entrypointDirectory, hex.EncodeToString(hash.Sum(nil)), installName), nil
}

func validateVersionedDigest(path string, sum [sha256.Size]byte) error {
	parent := filepath.Dir(path)
	if filepath.Base(filepath.Dir(parent)) == entrypointDirectory && filepath.Base(parent) != hex.EncodeToString(sum[:]) {
		return errors.New("credential entrypoint bytes do not match their versioned path")
	}
	return nil
}

// EntrypointNeeded reports whether the AIGW-owned credential executable is
// absent. An existing non-executable or redirected path is never adopted.
func EntrypointNeeded(path string) (bool, error) {
	if !filepath.IsAbs(path) {
		return false, fmt.Errorf("credential entrypoint must be an absolute path")
	}
	parent := filepath.Dir(path)
	dataRoot := filepath.Dir(parent)
	dataInfo, dataErr := os.Lstat(dataRoot)
	if errors.Is(dataErr, os.ErrNotExist) {
		return true, nil
	}
	if dataErr != nil {
		return false, fmt.Errorf("inspect credential data directory: %w", dataErr)
	}
	if err := validatePrivateDirectory(dataRoot, dataInfo); err != nil {
		return false, err
	}
	info, err := os.Lstat(parent)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect credential directory: %w", err)
	}
	if err := validatePrivateDirectory(parent, info); err != nil {
		return false, err
	}
	binary, binaryErr := os.Lstat(path)
	receipt, receiptErr := os.Lstat(path + ".sha256")
	if errors.Is(binaryErr, os.ErrNotExist) && errors.Is(receiptErr, os.ErrNotExist) {
		return true, nil
	}
	if binaryErr != nil || receiptErr != nil {
		return false, fmt.Errorf("credential entrypoint and receipt must exist together: %w", errors.Join(binaryErr, receiptErr))
	}
	if !binary.Mode().IsRegular() {
		return false, fmt.Errorf("credential entrypoint is not a regular executable")
	}
	if !receipt.Mode().IsRegular() {
		return false, fmt.Errorf("credential entrypoint receipt is not a regular file")
	}
	if err := validatePrivateFile(path, binary, 0o700); err != nil {
		return false, err
	}
	if err := validatePrivateFile(path+".sha256", receipt, 0o600); err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read credential entrypoint: %w", err)
	}
	identity, err := os.ReadFile(path + ".sha256")
	if err != nil {
		return false, fmt.Errorf("read credential entrypoint receipt: %w", err)
	}
	sum := sha256.Sum256(data)
	if err := validateVersionedDigest(path, sum); err != nil {
		return false, err
	}
	if string(identity) != hex.EncodeToString(sum[:])+"\n" {
		return false, fmt.Errorf("credential entrypoint differs from its recorded bytes")
	}
	return false, nil
}

// EnsureEntrypoint retains one executable outside a package manager's CLI
// link. It never replaces an existing entrypoint during an ordinary sync.
// The returned compensation removes only bytes created by this operation.
func EnsureEntrypoint(source, target string) (func() error, error) {
	return ensureEntrypoint(source, target, transaction.WriteFileAtomicExactModeIfUnchanged)
}

func ensureEntrypoint(
	source, target string,
	write func(string, transaction.FileSnapshot, []byte, os.FileMode) (transaction.FileSnapshot, error),
) (func() error, error) {
	needed, err := EntrypointNeeded(target)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, fmt.Errorf("read AIGW credential source: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("AIGW credential source is empty")
	}
	sum := sha256.Sum256(data)
	if err := validateVersionedDigest(target, sum); err != nil {
		return nil, err
	}
	identity := hex.EncodeToString(sum[:]) + "\n"
	if !needed {
		recorded, err := os.ReadFile(target + ".sha256")
		if err != nil {
			return nil, fmt.Errorf("read credential entrypoint receipt: %w", err)
		}
		if string(recorded) != identity {
			return nil, errors.New("credential entrypoint does not match the current AIGW executable; refusing to replace active bytes")
		}
		return func() error { return nil }, nil
	}
	parent := filepath.Dir(target)
	info, err := os.Lstat(parent)
	createdParent := errors.Is(err, os.ErrNotExist)
	if err != nil && !createdParent {
		return nil, fmt.Errorf("inspect credential directory: %w", err)
	}
	if createdParent {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			return nil, fmt.Errorf("create credential directory: %w", err)
		}
		info, err = os.Lstat(parent)
		if err != nil {
			return nil, fmt.Errorf("inspect created credential directory: %w", err)
		}
	}
	dataRoot := filepath.Dir(parent)
	dataInfo, err := os.Lstat(dataRoot)
	if err != nil {
		return nil, fmt.Errorf("inspect credential data directory: %w", err)
	}
	if err := validatePrivateDirectory(dataRoot, dataInfo); err != nil {
		return nil, errors.Join(err, removeCreatedDirectory(parent, createdParent))
	}
	if err := validatePrivateDirectory(parent, info); err != nil {
		return nil, errors.Join(err, removeCreatedDirectory(parent, createdParent))
	}
	post, err := write(target, transaction.FileSnapshot{}, data, 0o700)
	if err != nil {
		return nil, errors.Join(err, removeCreatedDirectory(parent, createdParent))
	}
	receiptPath := target + ".sha256"
	receiptPost, err := write(receiptPath, transaction.FileSnapshot{}, []byte(identity), 0o600)
	if err != nil {
		_, cleanupErr := transaction.RemoveFileIfUnchanged(target, post)
		return nil, errors.Join(err, cleanupErr, removeCreatedDirectory(parent, createdParent))
	}
	undo := func() error {
		if err := removeEntrypointPair(target, post, receiptPost); err != nil {
			return err
		}
		return removeCreatedDirectory(parent, createdParent)
	}
	if needed, err := EntrypointNeeded(target); err != nil || needed {
		if needed {
			err = errors.New("credential entrypoint disappeared during installation")
		}
		return nil, errors.Join(err, undo())
	}
	return undo, nil
}

func removeCreatedDirectory(path string, created bool) error {
	if !created {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove credential directory: %w", err)
	}
	return nil
}

func removeEntrypointPair(target string, binary, receipt transaction.FileSnapshot) error {
	current, err := transaction.CaptureFileSnapshot(target)
	if err != nil {
		return err
	}
	if !current.Equal(binary) {
		return errors.New("credential executable changed; refusing to remove its identity record")
	}
	receiptPath := target + ".sha256"
	if _, err := transaction.RemoveFileIfUnchanged(receiptPath, receipt); err != nil {
		return err
	}
	if _, err := transaction.RemoveFileIfUnchanged(target, binary); err != nil {
		_, restoreErr := transaction.WriteFileAtomicExactModeIfUnchanged(receiptPath, transaction.FileSnapshot{}, receipt.Data, receipt.Mode)
		return errors.Join(err, restoreErr)
	}
	return nil
}

// RemoveEntrypoint withdraws only an intact AIGW-owned executable and receipt.
// A changed file is preserved for explicit recovery rather than overwritten.
func RemoveEntrypoint(target string) error {
	if target == "" {
		return nil
	}
	needed, err := EntrypointNeeded(target)
	if err != nil || needed {
		return err
	}
	binary, err := transaction.CaptureFileSnapshot(target)
	if err != nil {
		return err
	}
	receiptPath := target + ".sha256"
	receipt, err := transaction.CaptureFileSnapshot(receiptPath)
	if err != nil {
		return err
	}
	if err := removeEntrypointPair(target, binary, receipt); err != nil {
		return err
	}
	parent := filepath.Dir(target)
	if err := os.Remove(parent); err != nil && !errors.Is(err, os.ErrNotExist) {
		entries, readErr := os.ReadDir(parent)
		if readErr == nil && len(entries) > 0 {
			return nil // The directory contains content this entrypoint does not own.
		}
		return errors.Join(fmt.Errorf("remove credential directory: %w", err), readErr)
	}
	return nil
}
