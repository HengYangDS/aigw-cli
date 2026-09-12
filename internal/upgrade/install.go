package upgrade

import (
	"aigw-cli/internal/process"
	"aigw-cli/internal/transaction"
	"aigw-cli/internal/upgrade/artifact"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rogpeppe/go-internal/robustio"
)

// installPortableArchive verifies and extracts a portable archive, then
// installs the contained binary using one cross-platform recoverable replacement
// owner. Startup and version verification precede any installation mutation.
func (u Updater) installPortableArchive(ctx context.Context, archivePath, checksumsPath, version string) error {
	binary, err := (artifact.Target{OS: u.GOOS, Arch: u.GOARCH}).ReadProgram(archivePath, checksumsPath, version)
	if err != nil {
		return err
	}
	if err := u.verifyCandidateProgram(ctx, binary, version); err != nil {
		return err
	}
	return u.replacePortableBinary(ctx, binary)
}

func (u Updater) verifyCandidateProgram(ctx context.Context, binary []byte, version string) (result error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	directory, err := os.MkdirTemp(filepath.Dir(u.Executable), ".aigw-verify-")
	if err != nil {
		return fmt.Errorf("prepare candidate verification: %w", err)
	}
	defer func() { result = errors.Join(result, robustio.RemoveAll(directory)) }()
	program := filepath.Join(directory, filepath.Base(u.Executable))
	if err := os.WriteFile(program, binary, 0o700); err != nil {
		return fmt.Errorf("stage candidate verification: %w", err)
	}
	if u.Runner == nil {
		u.Runner = process.Runner{}
	}
	verificationContext, cancel := context.WithTimeout(ctx, releaseRequestTimeout)
	defer cancel()
	output, err := u.captureReleaseCommand(verificationContext, process.Plan{Executable: program, Args: []string{"--version"}})
	if err != nil {
		return fmt.Errorf("candidate program failed startup verification; installed program is unchanged: %w", err)
	}
	if strings.TrimSpace(string(output)) != "aigw version "+version {
		return fmt.Errorf("candidate program did not report expected version %s; installed program is unchanged", version)
	}
	return ctx.Err()
}

func (u Updater) replacePortableBinary(ctx context.Context, binary []byte) (result error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(u.Executable) == "" {
		return errors.New("AIGW executable path is empty")
	}
	information, err := os.Stat(u.Executable)
	if err != nil {
		return fmt.Errorf("inspect current AIGW executable: %w", err)
	}
	directory, err := os.MkdirTemp(filepath.Dir(u.Executable), ".aigw-replace-")
	if err != nil {
		return fmt.Errorf("prepare AIGW replacement: %w", err)
	}
	defer func() { result = errors.Join(result, robustio.RemoveAll(directory)) }()
	candidate := filepath.Join(directory, filepath.Base(u.Executable))
	if err := transaction.WriteFileAtomicExactMode(candidate, binary, information.Mode().Perm()); err != nil {
		return fmt.Errorf("stage AIGW replacement: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return commitProgramReplacement(candidate, u.Executable, rollbackPath(u.Executable), robustio.Rename)
}

func commitProgramReplacement(candidate, current, previous string, rename func(string, string) error) error {
	if err := rename(current, previous); err != nil {
		return fmt.Errorf("replace AIGW executable: %w", err)
	}
	if err := rename(candidate, current); err != nil {
		if rollbackErr := rename(previous, current); rollbackErr != nil {
			return fmt.Errorf("replace AIGW executable: %w; restore current executable: %w", err, rollbackErr)
		}
		return fmt.Errorf("replace AIGW executable: %w", err)
	}
	return nil
}

// Rollback restores the immediately preceding portable AIGW executable without
// accessing the network. It swaps the current and previous binaries so the
// action itself remains reversible and never creates an unbounded chain.
func (u Updater) Rollback(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if strings.TrimSpace(u.Executable) == "" {
		return "", errors.New("AIGW executable path is empty")
	}
	backup := rollbackPath(u.Executable)
	previous, err := os.ReadFile(backup)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("no previous portable AIGW binary is available")
		}
		return "", fmt.Errorf("read previous AIGW executable: %w", err)
	}
	if err := u.replacePortableBinary(ctx, previous); err != nil {
		return "", fmt.Errorf("restore previous AIGW executable: %w", err)
	}
	return "restored the previous program version. If that older program does not support `aigw update --rollback`, download the current portable package and run its installer; it replaces only AIGW and retains one predecessor.", nil
}

// rollbackPath derives the sibling backup path for executable using the
// separator style already present in executable, rather than the host OS's
// native separator. This keeps the result stable across platforms: a
// POSIX-style path (e.g. produced by a Windows binary staged from a
// forward-slash working directory) must not be rewritten with backslashes,
// and vice versa.
func rollbackPath(executable string) string {
	suffix := ".aigw.previous"
	if strings.EqualFold(filepath.Ext(executable), ".exe") {
		suffix += ".exe"
	}
	if strings.Contains(executable, `\`) && !strings.Contains(executable, "/") {
		index := strings.LastIndex(executable, `\`)
		return executable[:index+1] + suffix
	}
	if index := strings.LastIndex(executable, "/"); index >= 0 {
		return executable[:index+1] + suffix
	}
	return suffix
}
