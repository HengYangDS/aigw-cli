package codex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
)

// ExecutableIdentity identifies the exact Codex executable used for a probe.
type ExecutableIdentity struct {
	Version string
	SHA256  string
}

func (identity ExecutableIdentity) known() bool {
	return identity.Version != "" && identity.SHA256 != ""
}

func (identity ExecutableIdentity) same(other ExecutableIdentity) bool {
	return identity.known() && identity == other
}

// IdentifyExecutable measures a Codex executable through its public version
// command and the bytes at the configured path.
func IdentifyExecutable(ctx context.Context, runner process.VerificationRunner, executable, codexHome, temporary string) (ExecutableIdentity, error) {
	if strings.TrimSpace(executable) == "" {
		return ExecutableIdentity{}, fmt.Errorf("Codex executable is not configured")
	}
	if runner == nil {
		return ExecutableIdentity{}, fmt.Errorf("Codex capture runner is unavailable")
	}
	digest, err := fileSHA256(executable)
	if err != nil {
		return ExecutableIdentity{}, err
	}
	output, diagnostic, err := runner.RunCaptureStreams(ctx, process.Plan{
		Executable: executable,
		Args:       []string{"--version"},
		Env:        codexEnvironment(codexHome, temporary),
	})
	if err != nil {
		return ExecutableIdentity{}, fmt.Errorf("inspect Codex version: %w", err)
	}
	if process.DiagnosticFailure(diagnostic) {
		return ExecutableIdentity{}, fmt.Errorf("Codex version probe emitted a native-client warning or error; executable identity is unqualified")
	}
	version := strings.TrimSpace(string(output))
	if version == "" {
		return ExecutableIdentity{}, fmt.Errorf("Codex reported no version")
	}
	return ExecutableIdentity{Version: version, SHA256: digest}, nil
}

// VerificationPlan builds the non-persistent real-client request used by
// `aigw verify`. The selected Codex home supplies the projected route and
// authentication; the final message is written to a private temporary file so
// diagnostic output cannot be mistaken for model evidence.
func VerificationPlan(executable, configPath, outputPath string, runtime configuration.Runtime) (process.Plan, error) {
	if strings.TrimSpace(executable) == "" {
		return process.Plan{}, fmt.Errorf("Codex executable is not configured")
	}
	if strings.TrimSpace(configPath) == "" {
		return process.Plan{}, fmt.Errorf("Codex configuration target is not configured")
	}
	if strings.TrimSpace(runtime.Model) == "" {
		return process.Plan{}, fmt.Errorf("Route %q has no Codex model", runtime.RouteID)
	}
	if strings.TrimSpace(outputPath) == "" {
		return process.Plan{}, fmt.Errorf("Codex verification output path is not configured")
	}
	return process.Plan{
		Executable: executable,
		Args: []string{
			"exec",
			"--ephemeral",
			"--ignore-rules",
			"--skip-git-repo-check",
			"--strict-config",
			"--sandbox", "read-only",
			"--color", "never",
			"--cd", filepath.Dir(outputPath),
			"--output-last-message", outputPath,
			"--model", runtime.Model,
			"Reply with exactly: AIGW_OK",
		},
		Env: codexEnvironment(filepath.Dir(configPath), filepath.Dir(outputPath)),
	}, nil
}

func runCodexReadOnly(ctx context.Context, runner process.VerificationRunner, executable, codexHome, temporary string, args ...string) ([]byte, error) {
	output, diagnostic, err := runner.RunCaptureStreams(ctx, process.Plan{
		Executable: executable,
		Args:       args,
		Env:        codexEnvironment(codexHome, temporary),
	})
	if err != nil {
		return nil, fmt.Errorf("run %s %s: %w", filepath.Base(executable), strings.Join(args, " "), err)
	}
	if process.DiagnosticFailure(diagnostic) {
		return nil, fmt.Errorf("Codex catalogue probe emitted a native-client warning or error; catalogue identity is unqualified")
	}
	return output, nil
}

func codexEnvironment(home, temporary string) []string {
	environment := slices.DeleteFunc(os.Environ(), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		return key == "CODEX_HOME" || temporary != "" && slices.Contains([]string{"TMPDIR", "TMP", "TEMP"}, key)
	})
	if home != "" {
		environment = append(environment, "CODEX_HOME="+home)
	}
	if temporary != "" {
		for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
			environment = append(environment, key+"="+temporary)
		}
	}
	return environment
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read Codex executable: %w", err)
	}
	defer func() { _ = file.Close() }()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", fmt.Errorf("hash Codex executable: %w", err)
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
