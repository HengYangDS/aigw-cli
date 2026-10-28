//go:build !windows

package hermes

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aigw-cli/internal/client"
	clientverification "aigw-cli/internal/client/verification"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
)

// Bounds for the gateway that the verification chat auto-starts: it exits on
// its own once idle, a native stop is the one fallback, and the home's remover
// keeps the home while the gateway still holds it.
var (
	hermesGatewayIdleWait = 15 * time.Second
	hermesGatewayStopWait = 15 * time.Second
	hermesGatewayPoll     = 250 * time.Millisecond
)

// verifyIsolated runs the bounded probes from a disposable home that borrows the
// configured installation's committed dependencies.
func verifyIsolated(ctx context.Context, deps client.Dependencies, configured configuration.ClientBinding, data []byte) (_ client.Verification, result error) {
	home, cleanup, err := CreateVerificationHome(configured.Targets[0])
	if err != nil {
		return client.Verification{}, err
	}
	environment := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "HERMES_HOME=") && !strings.HasPrefix(entry, "HERMES_DISABLE_LAZY_INSTALLS=") && !strings.HasPrefix(entry, "HERMES_GATEWAY_LOCK_DIR=") {
			environment = append(environment, entry)
		}
	}
	environment = append(environment, "HERMES_HOME="+home, "HERMES_DISABLE_LAZY_INSTALLS=1", "HERMES_GATEWAY_LOCK_DIR="+GatewayLockDirectory(home))
	defer func() {
		releaseVerificationGateway(ctx, deps.Runner, process.Plan{Executable: configured.Executable, Directory: home, Env: environment, Args: []string{"gateway", "stop"}})
		result = errors.Join(result, cleanup())
	}()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), data, 0o600); err != nil {
		return client.Verification{}, fmt.Errorf("prepare isolated Hermes verification policy: %w", err)
	}
	probeCtx, cancel := context.WithTimeout(ctx, clientverification.ProtocolTimeout)
	defer cancel()
	probe := process.Plan{Executable: configured.Executable, Directory: home, Env: environment, Args: []string{"--version"}}
	version, diagnostic, err := deps.Runner.RunCaptureStreams(probeCtx, probe)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return client.Verification{}, errors.New("hermes version probe timed out")
		case errors.Is(err, context.Canceled):
			return client.Verification{}, errors.New("hermes version probe interrupted")
		default:
			return client.Verification{}, errors.New("hermes version probe failed")
		}
	}
	if process.DiagnosticFailure(diagnostic) {
		return client.Verification{}, errors.New("hermes emitted a native-client warning or error; verification is incomplete; diagnostics suppressed")
	}
	probe.Args = []string{"chat", "--quiet", "--query-file", "-", "--oneshot", "--max-turns", "1", "--run-budget", "45", "--ignore-rules", "--source", "tool"}
	probe.Stdin = "Reply with exactly AIGW_OK."
	response, diagnostic, err := deps.Runner.RunCaptureStreams(probeCtx, probe)
	if err != nil {
		return client.Verification{}, errors.Join(errors.New("hermes inference failed; external credential diagnostics suppressed"), probeCtx.Err())
	}
	if process.DiagnosticFailure(diagnostic) {
		return client.Verification{}, errors.New("hermes emitted a native-client warning or error; verification is incomplete; diagnostics suppressed")
	}
	if !strings.Contains(string(response), "AIGW_OK") {
		return client.Verification{}, errors.New("hermes model response did not return the expected AIGW_OK verification marker")
	}
	executable, err := os.ReadFile(configured.Executable)
	if err != nil {
		return client.Verification{}, err
	}
	return client.Verification{Version: strings.TrimSpace(string(version)), SHA256: fmt.Sprintf("%x", sha256.Sum256(executable)), Scope: "Native text response; tools and continuation are not probed"}, nil
}

// releaseVerificationGateway waits for the verification gateway to exit on
// idle and, when it outlives that bound, asks it once to stop through the
// native command, which refuses an active turn. It never signals the gateway;
// the home's remover decides deletion from the gateway lock and the recorded
// gateway process it leaves. The
// stop outlives a canceled verification, so an interrupted run still releases
// its home within the bounds.
func releaseVerificationGateway(ctx context.Context, runner process.VerificationRunner, stop process.Plan) {
	if AwaitGatewayAbsence(stop.Directory, hermesGatewayIdleWait, hermesGatewayPoll) {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), clientverification.ProtocolTimeout)
	defer cancel()
	_, _, _ = runner.RunCaptureStreams(ctx, stop)
	AwaitGatewayAbsence(stop.Directory, hermesGatewayStopWait, hermesGatewayPoll)
}
