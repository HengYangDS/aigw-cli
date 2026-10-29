package secret

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"aigw-cli/internal/store"
)

const entrypointDirectory = "credential"

// executableReadSize keeps one command's reads within bounded memory even when
// a collection empties the pool: a command streams the program and its reader
// up to three times.
const executableReadSize = 64 << 10

// Reuse read buffers, never executable identities; each call still hashes every byte.
var executableReadBuffers = sync.Pool{New: func() any { return new([executableReadSize]byte) }}

// VersionedEntrypointPath binds a private executable path to the exact source
// bytes without creating files or relying on an installer-owned public link.
func VersionedEntrypointPath(dataDir, source, installName string) (path string, err error) {
	program, err := ReadProgram(nil, source)
	if err != nil {
		return "", err
	}
	return program.EntrypointPath(dataDir, installName)
}

// executables reads AIGW executables through one injected file capability.
type executables struct {
	files Files
}

func (e executables) open(name string) (*os.File, error) {
	if e.files == nil {
		return os.Open(name)
	}
	return e.files.Open(name)
}

// observation is one read of an executable: its digest, the opened file's
// identity and, when the caller keeps them, the bytes behind that digest.
type observation struct {
	digest [sha256.Size]byte
	file   os.FileInfo
	data   []byte
}

func (e executables) observe(path string, keep bool) (observed observation, err error) {
	executable, err := e.open(path)
	if err != nil {
		return observed, fmt.Errorf("open AIGW executable for credential identity: %w", err)
	}
	defer func() { err = errors.Join(err, executable.Close()) }()
	info, err := executable.Stat()
	if err != nil {
		return observed, fmt.Errorf("inspect AIGW executable for credential identity: %w", err)
	}
	if !info.Mode().IsRegular() {
		return observed, errors.New("AIGW credential source is not a regular executable")
	}
	if info.Size() == 0 {
		return observed, errors.New("AIGW credential source is empty")
	}
	hash := sha256.New()
	destination := io.Writer(hash)
	var kept bytes.Buffer
	if keep {
		kept.Grow(int(info.Size()))
		destination = io.MultiWriter(hash, &kept)
	}
	buffer, ok := executableReadBuffers.Get().(*[executableReadSize]byte)
	if !ok || buffer == nil {
		return observed, errors.New("AIGW executable read buffer is unavailable")
	}
	defer executableReadBuffers.Put(buffer)
	// Hide File.WriteTo so CopyBuffer preserves the selected read size on every OS.
	if _, err := io.CopyBuffer(destination, struct{ io.Reader }{executable}, buffer[:]); err != nil {
		return observed, fmt.Errorf("hash AIGW executable for credential identity: %w", err)
	}
	return observation{digest: [sha256.Size]byte(hash.Sum(nil)), file: info, data: kept.Bytes()}, nil
}

// ValidateRetainedEntrypoint accepts only an intact predecessor in the current
// AIGW installation's versioned reader namespace. The current reader need not
// exist yet: an update can replace the CLI before the next client sync.
func ValidateRetainedEntrypoint(current, retained string) error {
	if current == retained {
		return errors.New("retained credential reader must differ from the current reader")
	}
	currentNamespace, currentName, currentOK := versionedEntrypointIdentity(current)
	retainedNamespace, retainedName, retainedOK := versionedEntrypointIdentity(retained)
	if !currentOK || !retainedOK {
		return errors.New("credential reader path is not versioned")
	}
	if currentName != retainedName {
		return errors.New("retained credential reader belongs to another installation")
	}
	for _, directory := range privateEntrypointDirectories(current) {
		info, err := os.Lstat(directory)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return errors.New("current credential reader namespace is unavailable")
		}
		if err := validatePrivateDirectory(directory, info); err != nil {
			return err
		}
	}
	currentInfo, currentErr := os.Stat(currentNamespace)
	retainedInfo, retainedErr := os.Stat(retainedNamespace)
	if currentErr != nil || retainedErr != nil || !os.SameFile(currentInfo, retainedInfo) {
		return errors.New("retained credential reader belongs to another installation")
	}
	missing, err := EntrypointNeeded(retained)
	if err != nil || missing {
		return errors.New("retained credential reader is unavailable or invalid")
	}
	return nil
}

// IsVersionedEntrypointPath identifies only the owned path grammar; it does not
// assert that executable bytes exist or remain valid.
func IsVersionedEntrypointPath(path string) bool {
	_, _, ok := versionedEntrypointIdentity(path)
	return ok
}

func versionedEntrypointIdentity(path string) (namespace, name string, ok bool) {
	parent := filepath.Dir(path)
	digest := filepath.Base(parent)
	decoded, err := hex.DecodeString(digest)
	if !filepath.IsAbs(path) || err != nil || len(decoded) != sha256.Size || hex.EncodeToString(decoded) != digest {
		return "", "", false
	}
	namespace = filepath.Dir(parent)
	if filepath.Base(namespace) != entrypointDirectory {
		return "", "", false
	}
	return namespace, filepath.Base(path), true
}

func validateVersionedDigest(path string, sum [sha256.Size]byte) error {
	parent := filepath.Dir(path)
	if filepath.Base(filepath.Dir(parent)) == entrypointDirectory && filepath.Base(parent) != hex.EncodeToString(sum[:]) {
		return errors.New("credential entrypoint bytes do not match their versioned path")
	}
	return nil
}

func privateEntrypointDirectories(path string) []string {
	parent := filepath.Dir(path)
	dataRoot := filepath.Dir(parent)
	if IsVersionedEntrypointPath(path) {
		return []string{filepath.Dir(dataRoot), dataRoot, parent}
	}
	return []string{dataRoot, parent}
}

// EntrypointNeeded reports whether the AIGW-owned credential executable is
// absent. An existing non-executable or redirected path is never adopted.
func EntrypointNeeded(path string) (bool, error) {
	return executables{}.needed(path)
}

func (e executables) needed(path string) (bool, error) {
	_, absent, err := e.verify(path)
	return absent, err
}

// verify reads the reader at path once and checks its bytes against its
// receipt and versioned path. An absent reader is not an error.
func (e executables) verify(path string) (Entrypoint, bool, error) {
	if _, absent, err := inspectEntrypoint(path); err != nil || absent {
		return Entrypoint{}, absent, err
	}
	observed, err := e.observe(path, false)
	if err != nil {
		return Entrypoint{}, false, fmt.Errorf("read credential entrypoint: %w", err)
	}
	identity, err := os.ReadFile(path + ".sha256")
	if err != nil {
		return Entrypoint{}, false, fmt.Errorf("read credential entrypoint receipt: %w", err)
	}
	if err := validateVersionedDigest(path, observed.digest); err != nil {
		return Entrypoint{}, false, err
	}
	if string(identity) != hex.EncodeToString(observed.digest[:])+"\n" {
		return Entrypoint{}, false, fmt.Errorf("credential entrypoint differs from its recorded bytes")
	}
	return Entrypoint{executables: e, path: path, digest: observed.digest, file: observed.file}, false, nil
}

// inspectEntrypoint validates the private namespace and the reader pair's
// types and permissions without reading either file. A missing namespace
// directory or a missing pair means that the reader is absent.
func inspectEntrypoint(path string) (os.FileInfo, bool, error) {
	if !filepath.IsAbs(path) {
		return nil, false, fmt.Errorf("credential entrypoint must be an absolute path")
	}
	for _, directory := range privateEntrypointDirectories(path) {
		info, err := os.Lstat(directory)
		if errors.Is(err, os.ErrNotExist) {
			return nil, true, nil
		}
		if err != nil {
			return nil, false, fmt.Errorf("inspect credential directory: %w", err)
		}
		if err := validatePrivateDirectory(directory, info); err != nil {
			return nil, false, err
		}
	}
	binary, binaryErr := os.Lstat(path)
	receipt, receiptErr := os.Lstat(path + ".sha256")
	if errors.Is(binaryErr, os.ErrNotExist) && errors.Is(receiptErr, os.ErrNotExist) {
		return nil, true, nil
	}
	if binaryErr != nil || receiptErr != nil {
		return nil, false, fmt.Errorf("credential entrypoint and receipt must exist together: %w", errors.Join(binaryErr, receiptErr))
	}
	if !binary.Mode().IsRegular() {
		return nil, false, fmt.Errorf("credential entrypoint is not a regular executable")
	}
	if !receipt.Mode().IsRegular() {
		return nil, false, fmt.Errorf("credential entrypoint receipt is not a regular file")
	}
	if err := validatePrivateFile(path, binary, 0o700); err != nil {
		return nil, false, err
	}
	if err := validatePrivateFile(path+".sha256", receipt, 0o600); err != nil {
		return nil, false, err
	}
	return binary, false, nil
}

// EnsureEntrypoint retains one executable outside a package manager's CLI
// link. It never replaces an existing entrypoint during an ordinary sync.
// The returned compensation removes only bytes created by this operation.
func EnsureEntrypoint(source, target string) (func() error, error) {
	return ensureEntrypoint(source, target, store.WriteFileAtomicExactModeIfUnchanged)
}

func ensureEntrypoint(
	source, target string,
	write func(string, store.FileSnapshot, []byte, os.FileMode) (store.FileSnapshot, error),
) (func() error, error) {
	program, err := ReadProgram(nil, source)
	if err != nil {
		return nil, err
	}
	_, undo, err := program.ensure(target, write)
	return undo, err
}

// install writes exactly the given source bytes, whose digest the caller
// computed from those bytes, as the reader and its receipt, and verifies the
// written reader once.
func (e executables) install(
	data []byte, sum [sha256.Size]byte, target string,
	write func(string, store.FileSnapshot, []byte, os.FileMode) (store.FileSnapshot, error),
) (Entrypoint, func() error, error) {
	if err := validateVersionedDigest(target, sum); err != nil {
		return Entrypoint{}, nil, err
	}
	identity := hex.EncodeToString(sum[:]) + "\n"
	parent := filepath.Dir(target)
	cleanupDirectories, err := createCredentialDirectoryChain(parent)
	if err != nil {
		return Entrypoint{}, nil, err
	}
	for _, directory := range privateEntrypointDirectories(target) {
		info, err := os.Lstat(directory)
		if err != nil {
			return Entrypoint{}, nil, errors.Join(fmt.Errorf("inspect credential directory: %w", err), cleanupDirectories())
		}
		if err := validatePrivateDirectory(directory, info); err != nil {
			return Entrypoint{}, nil, errors.Join(err, cleanupDirectories())
		}
	}
	post, err := write(target, store.FileSnapshot{}, data, 0o700)
	if err != nil {
		return Entrypoint{}, nil, errors.Join(err, cleanupDirectories())
	}
	receiptPath := target + ".sha256"
	receiptPost, err := write(receiptPath, store.FileSnapshot{}, []byte(identity), 0o600)
	if err != nil {
		_, cleanupErr := store.RemoveFileIfUnchanged(target, post)
		return Entrypoint{}, nil, errors.Join(err, cleanupErr, cleanupDirectories())
	}
	undo := func() error {
		if err := removeEntrypointPair(target, post, receiptPost); err != nil {
			return err
		}
		return cleanupDirectories()
	}
	reader, absent, err := e.verify(target)
	if err != nil || absent {
		if absent {
			err = errors.New("credential entrypoint disappeared during installation")
		}
		return Entrypoint{}, nil, errors.Join(err, undo())
	}
	return reader, undo, nil
}

func createCredentialDirectoryChain(parent string) (func() error, error) {
	var missing []string
	for path := parent; ; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err == nil {
			if !info.IsDir() {
				return nil, errors.New("credential directory path contains a non-directory")
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("inspect credential directory chain: %w", err)
		}
		missing = append(missing, path)
	}
	type createdDirectory struct {
		path string
		info os.FileInfo
	}
	created := make([]createdDirectory, 0, len(missing))
	cleanup := func() error {
		for _, directory := range slices.Backward(created) {
			current, err := os.Lstat(directory.path)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return fmt.Errorf("inspect created credential directory: %w", err)
			}
			if !os.SameFile(current, directory.info) {
				return nil
			}
			if err := os.Remove(directory.path); err != nil && !errors.Is(err, os.ErrNotExist) {
				entries, readErr := os.ReadDir(directory.path)
				if readErr == nil && len(entries) > 0 {
					return nil // New content is not owned by this entrypoint.
				}
				return errors.Join(fmt.Errorf("remove created credential directory: %w", err), readErr)
			}
		}
		return nil
	}
	for _, path := range slices.Backward(missing) {
		if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, errors.Join(fmt.Errorf("create credential directory: %w", err), cleanup())
		} else if errors.Is(err, os.ErrExist) {
			info, statErr := os.Lstat(path)
			if statErr != nil || !info.IsDir() {
				return nil, errors.Join(errors.New("credential directory path changed during creation"), statErr, cleanup())
			}
			continue
		}
		// Stat the opened directory: Windows defers the identity in Lstat until
		// SameFile, when the path may already name a replacement.
		directory, err := os.Open(path)
		if err != nil {
			return nil, errors.Join(errors.New("created credential directory changed during creation"), err, cleanup())
		}
		info, statErr := directory.Stat()
		closeErr := directory.Close()
		if statErr != nil || closeErr != nil || !info.IsDir() {
			return nil, errors.Join(errors.New("created credential directory changed during creation"), statErr, closeErr, cleanup())
		}
		created = append(created, createdDirectory{path: path, info: info})
	}
	return cleanup, nil
}

func removeEntrypointPair(target string, binary, receipt store.FileSnapshot) error {
	current, err := store.CaptureFileSnapshot(target)
	if err != nil {
		return err
	}
	if !current.Equal(binary) {
		return errors.New("credential executable changed; refusing to remove its identity record")
	}
	receiptPath := target + ".sha256"
	if _, err := store.RemoveFileIfUnchanged(receiptPath, receipt); err != nil {
		return err
	}
	if _, err := store.RemoveFileIfUnchanged(target, binary); err != nil {
		_, restoreErr := store.WriteFileAtomicExactModeIfUnchanged(receiptPath, store.FileSnapshot{}, receipt.Data, receipt.Mode)
		return errors.Join(err, restoreErr)
	}
	return nil
}
