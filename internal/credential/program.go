package credential

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"aigw-cli/internal/transaction"
)

// Files opens AIGW executables whose content identity AIGW verifies. A nil
// Files opens host files.
type Files interface {
	Open(name string) (*os.File, error)
}

// Program is the running AIGW program as one content read observed it.
type Program struct {
	executables executables
	path        string
	digest      [sha256.Size]byte
}

// ReadProgram streams the running AIGW program once for its content identity.
// It keeps no program bytes; installing a reader reads the program again and
// accepts only bytes with this identity.
func ReadProgram(files Files, path string) (Program, error) {
	if !filepath.IsAbs(path) {
		return Program{}, errors.New("AIGW program must be an absolute path")
	}
	reader := executables{files: files}
	observed, err := reader.observe(path, false)
	if err != nil {
		return Program{}, err
	}
	return Program{executables: reader, path: path, digest: observed.digest}, nil
}

// EntrypointPath names the private reader for this program's exact bytes
// without creating files or relying on an installer-owned public link.
func (p Program) EntrypointPath(dataDir, installName string) (string, error) {
	if !filepath.IsAbs(dataDir) {
		return "", errors.New("credential data directory must be an absolute path")
	}
	if installName == "" || installName == "." || installName == ".." || filepath.Base(installName) != installName {
		return "", errors.New("credential executable name must be one file name")
	}
	return filepath.Join(dataDir, entrypointDirectory, hex.EncodeToString(p.digest[:]), installName), nil
}

// Entrypoint is a credential reader whose bytes one read verified.
type Entrypoint struct {
	executables executables
	path        string
	digest      [sha256.Size]byte
	file        os.FileInfo
}

// EntrypointNeededFrom reports, like EntrypointNeeded, whether the reader at
// path is absent, reading its bytes through files.
func EntrypointNeededFrom(files Files, path string) (bool, error) {
	return executables{files: files}.needed(path)
}

// EnsureEntrypoint verifies this program's reader at target or installs it,
// and never replaces an existing reader. An installation copies only program
// bytes whose digest still equals ReadProgram's. The returned compensation
// removes only bytes created by this operation.
func (p Program) EnsureEntrypoint(target string) (Entrypoint, func() error, error) {
	return p.ensure(target, transaction.WriteFileAtomicExactModeIfUnchanged)
}

func (p Program) ensure(
	target string,
	write func(string, transaction.FileSnapshot, []byte, os.FileMode) (transaction.FileSnapshot, error),
) (Entrypoint, func() error, error) {
	reader, absent, err := p.executables.verify(target)
	if err != nil {
		return Entrypoint{}, nil, err
	}
	if absent {
		if err := validateVersionedDigest(target, p.digest); err != nil {
			return Entrypoint{}, nil, err
		}
		copied, err := p.executables.observe(p.path, true)
		if err != nil {
			return Entrypoint{}, nil, fmt.Errorf("read AIGW credential source: %w", err)
		}
		if copied.digest != p.digest {
			return Entrypoint{}, nil, errors.New("AIGW program changed after verification; refusing to install unverified bytes")
		}
		return p.executables.install(copied.data, p.digest, target, write)
	}
	if reader.digest != p.digest {
		return Entrypoint{}, nil, errors.New("credential entrypoint does not match the current AIGW executable; refusing to replace active bytes")
	}
	return reader, func() error { return nil }, nil
}

// Confirm refuses a verified reader that disappeared or whose bytes or receipt
// no longer match what was verified, even after a same-size rewrite with a
// restored timestamp. It streams the reader once more with bounded memory.
func (e Entrypoint) Confirm() error {
	binary, absent, err := inspectEntrypoint(e.path)
	if err == nil && !absent {
		err = e.matches(binary)
	}
	if err != nil {
		return fmt.Errorf("inspect credential entrypoint: %w", err)
	}
	if absent {
		return errors.New("credential entrypoint disappeared after client projection")
	}
	return nil
}

func (e Entrypoint) matches(binary os.FileInfo) error {
	if !os.SameFile(e.file, binary) {
		return errors.New("credential entrypoint changed after verification")
	}
	receipt, err := os.ReadFile(e.path + ".sha256")
	if err != nil || string(receipt) != hex.EncodeToString(e.digest[:])+"\n" {
		return errors.Join(errors.New("credential entrypoint receipt changed after verification"), err)
	}
	observed, err := e.executables.observe(e.path, false)
	if err != nil {
		return err
	}
	if observed.digest != e.digest {
		return errors.New("credential entrypoint bytes changed after verification")
	}
	return nil
}
