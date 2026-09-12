package upgrade

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ProgramFile identifies observed program bytes, not their release trust or readiness.
type ProgramFile struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
}

// Installation describes the running portable program and its retained predecessor.
type Installation struct {
	SchemaVersion int          `json:"schema_version"`
	Version       string       `json:"version"`
	CommandPath   string       `json:"command_path"`
	Payload       ProgramFile  `json:"payload"`
	Rollback      *ProgramFile `json:"rollback"`
}

// InspectInstallation observes existing program files without executing or changing them.
func InspectInstallation(executable, version string) (Installation, error) {
	if strings.TrimSpace(executable) == "" {
		return Installation{}, errors.New("portable command path is unavailable")
	}
	command, err := filepath.Abs(executable)
	if err != nil {
		return Installation{}, fmt.Errorf("resolve portable command path: %w", err)
	}
	payload, err := inspectProgramFile(command)
	if err != nil {
		return Installation{}, fmt.Errorf("observe current portable program: %w", err)
	}
	result := Installation{SchemaVersion: 1, Version: version, CommandPath: command, Payload: payload}
	previous := RollbackPath(command)
	if _, err := os.Lstat(previous); errors.Is(err, os.ErrNotExist) {
		return result, nil
	} else if err != nil {
		return Installation{}, fmt.Errorf("observe retained portable program: %w", err)
	}
	rollback, err := inspectProgramFile(previous)
	if err != nil {
		return Installation{}, fmt.Errorf("observe retained portable program: %w", err)
	}
	result.Rollback = &rollback
	return result, nil
}

func inspectProgramFile(path string) (result ProgramFile, resultErr error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return result, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() {
		return result, fmt.Errorf("program path is not a regular file: %s", path)
	}
	file, err := os.Open(resolved)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	digest := sha256.New()
	size, err := io.Copy(digest, file)
	if err != nil {
		return result, err
	}
	return ProgramFile{Path: resolved, SHA256: fmt.Sprintf("%x", digest.Sum(nil)), SizeBytes: size}, nil
}
