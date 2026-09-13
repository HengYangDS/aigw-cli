package upgrade

import (
	"aigw-cli/internal/platform"
	"aigw-cli/internal/process"
	"aigw-cli/internal/transaction"
	"aigw-cli/internal/upgrade/artifact"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/rogpeppe/go-internal/robustio"
)

// ErrRollbackConfiguration means the retained program cannot read the current configuration.
var ErrRollbackConfiguration = errors.New("retained program cannot read the current configuration")

// installPortableArchive verifies and extracts a portable archive, then
// installs the contained binary using one cross-platform recoverable replacement
// owner. Startup and version verification precede any installation mutation.
func (u Updater) installPortableArchive(ctx context.Context, archivePath, checksumsPath, version string) error {
	binary, err := (artifact.Target{OS: u.GOOS, Arch: u.GOARCH}).ReadProgram(archivePath, checksumsPath, version)
	if err != nil {
		return err
	}
	if err := u.verifyProgram(ctx, binary, version, nil); err != nil {
		return err
	}
	return u.replacePortableBinary(ctx, binary)
}

func (u Updater) verifyProgram(ctx context.Context, binary []byte, version string, config []byte) (result error) {
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
	home := filepath.Join(directory, "home")
	environment := map[string]string{
		"HOME": home, "USERPROFILE": home,
		"XDG_CONFIG_HOME": home, "XDG_DATA_HOME": home,
		"APPDATA": home, "LOCALAPPDATA": home,
	}
	plan := process.Plan{Executable: program, Args: []string{"--version"}, Env: []string{
		"PATH=", "AIGW_SECRET_BACKEND=env", "NO_COLOR=1",
		"TMPDIR=" + directory, "TMP=" + directory, "TEMP=" + directory,
	}}
	if root := os.Getenv("SystemRoot"); runtime.GOOS == "windows" && root != "" {
		plan.Env = append(plan.Env, "SystemRoot="+root)
	}
	for name, value := range environment {
		plan.Env = append(plan.Env, name+"="+value)
	}
	verificationContext, cancel := context.WithTimeout(ctx, releaseRequestTimeout)
	defer cancel()
	output, err := u.captureReleaseCommand(verificationContext, plan)
	if err != nil {
		return fmt.Errorf("candidate program failed startup verification; installed program is unchanged: %w", err)
	}
	reported := strings.TrimSpace(string(output))
	parsed, parseErr := parseVersion(strings.TrimPrefix(reported, "aigw version "))
	if !strings.HasPrefix(reported, "aigw version ") || parseErr != nil || parsed == nil {
		return errors.New("program did not report a valid version; installed program is unchanged")
	}
	if version != "" && reported != "aigw version "+version {
		return fmt.Errorf("candidate program did not report expected version %s; installed program is unchanged", version)
	}
	if config != nil {
		path, err := platform.ConfigPathFor(runtime.GOOS, environment)
		if err != nil {
			return err
		}
		if err := transaction.WriteFileAtomicExactMode(path, config, 0o600); err != nil {
			return fmt.Errorf("stage configuration verification: %w", err)
		}
		plan.Args = []string{"config", "export"}
		output, runErr := u.Runner.RunCapture(verificationContext, plan)
		var manifest struct {
			Version  int `toml:"version"`
			Profiles map[string]struct {
				Model string `toml:"model"`
			} `toml:"profiles"`
		}
		parseErr := toml.Unmarshal(output, &manifest)
		if runErr != nil || parseErr != nil || manifest.Version <= 0 || len(manifest.Profiles) == 0 {
			return errors.Join(ErrRollbackConfiguration, runErr)
		}
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
	return commitProgramReplacement(candidate, u.Executable, RollbackPath(u.Executable), robustio.Rename)
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
func (u Updater) Rollback(ctx context.Context, config []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if strings.TrimSpace(u.Executable) == "" {
		return "", errors.New("AIGW executable path is empty")
	}
	backup := RollbackPath(u.Executable)
	previous, err := os.ReadFile(backup)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("no previous portable AIGW binary is available")
		}
		return "", fmt.Errorf("read previous AIGW executable: %w", err)
	}
	if _, err := os.Stat(u.Executable); err != nil {
		return "", fmt.Errorf("inspect current AIGW executable: %w", err)
	}
	if err := u.verifyProgram(ctx, previous, "", config); err != nil {
		return "", err
	}
	if err := u.replacePortableBinary(ctx, previous); err != nil {
		return "", fmt.Errorf("restore previous AIGW executable: %w", err)
	}
	return "restored the previous program version. If that older program does not support `aigw update --rollback`, download the current portable package and run its installer; it replaces only AIGW and retains one predecessor.", nil
}

// RollbackPath derives the sibling backup path for executable using the
// separator style already present in executable, rather than the host OS's
// native separator. This keeps the result stable across platforms: a
// POSIX-style path (e.g. produced by a Windows binary staged from a
// forward-slash working directory) must not be rewritten with backslashes,
// and vice versa.
func RollbackPath(executable string) string {
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
