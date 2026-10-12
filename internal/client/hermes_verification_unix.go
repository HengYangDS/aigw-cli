//go:build !windows

package client

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	clientverification "aigw-cli/internal/client/verification"
	"aigw-cli/internal/configuration"
	hermesconfig "aigw-cli/internal/hermes/configuration"
	"aigw-cli/internal/process"
)

// verifyIsolated runs the bounded probes from a disposable home that borrows the
// configured installation's committed dependencies.
func verifyIsolated(ctx context.Context, deps Dependencies, configured configuration.ClientBinding, data []byte) (_ Verification, result error) {
	home, cleanup, err := hermesconfig.CreateVerificationHome(configured.Targets[0])
	if err != nil {
		return Verification{}, err
	}
	defer func() { result = errors.Join(result, cleanup()) }()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), data, 0o600); err != nil {
		return Verification{}, fmt.Errorf("prepare isolated Hermes verification policy: %w", err)
	}
	environment := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "HERMES_HOME=") && !strings.HasPrefix(entry, "HERMES_DISABLE_LAZY_INSTALLS=") {
			environment = append(environment, entry)
		}
	}
	environment = append(environment, "HERMES_HOME="+home, "HERMES_DISABLE_LAZY_INSTALLS=1")
	probeCtx, cancel := context.WithTimeout(ctx, clientverification.ProtocolTimeout)
	defer cancel()
	probe := process.Plan{Executable: configured.Executable, Directory: home, Env: environment, Args: []string{"--version"}}
	version, diagnostic, err := deps.Runner.RunCaptureStreams(probeCtx, probe)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return Verification{}, errors.New("hermes version probe timed out")
		case errors.Is(err, context.Canceled):
			return Verification{}, errors.New("hermes version probe interrupted")
		default:
			return Verification{}, errors.New("hermes version probe failed")
		}
	}
	if process.DiagnosticFailure(diagnostic) {
		return Verification{}, errors.New("hermes emitted a native-client warning or error; verification is incomplete; diagnostics suppressed")
	}
	probe.Args = []string{"chat", "--quiet", "--query-file", "-", "--oneshot", "--max-turns", "1", "--run-budget", "45", "--ignore-rules", "--source", "tool"}
	probe.Stdin = "Reply with exactly AIGW_OK."
	response, diagnostic, err := deps.Runner.RunCaptureStreams(probeCtx, probe)
	if err != nil {
		return Verification{}, errors.Join(errors.New("hermes inference failed; external credential diagnostics suppressed"), probeCtx.Err())
	}
	if process.DiagnosticFailure(diagnostic) {
		return Verification{}, errors.New("hermes emitted a native-client warning or error; verification is incomplete; diagnostics suppressed")
	}
	if !strings.Contains(string(response), "AIGW_OK") {
		return Verification{}, errors.New("hermes model response did not return the expected AIGW_OK verification marker")
	}
	executable, err := os.ReadFile(configured.Executable)
	if err != nil {
		return Verification{}, err
	}
	return Verification{Version: strings.TrimSpace(string(version)), SHA256: fmt.Sprintf("%x", sha256.Sum256(executable)), Scope: "Native text response; tools and continuation are not probed"}, nil
}
