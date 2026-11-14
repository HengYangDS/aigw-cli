//go:build !windows

package hermes

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"aigw-cli/internal/adapter"
	clientverification "aigw-cli/internal/adapter/verification"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
)

// Bounds for the gateway that the verification chat auto-starts: it exits on
// its own once idle, a native stop is the one fallback, and the home's remover
// keeps the home while the gateway still holds it.
var (
	hermesGatewayIdleWait = 15 * time.Second
	hermesGatewayStopWait = 15 * time.Second
	hermesGatewayPoll     = 250 * time.Millisecond
)

// verificationEnvironment is env as Hermes' verification runs with it: the
// disposable home and its gateway lock, and none of the variables through
// which a parent Hermes session selects its own provider, model, profile and
// session, nor the ones that its launch flags --safe-mode,
// --ignore-user-config, --ignore-rules, --yolo and --accept-hooks export to
// its children, since the check verifies the configuration that AIGW wrote.
// PATH, HOME, proxies, locale and Hermes' other settings stay.
func verificationEnvironment(env []string, home, lockDirectory string) []string {
	environment := []string{}
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if !parentSessionVariable(key) {
			environment = append(environment, entry)
		}
	}
	return append(environment, "HERMES_HOME="+home, "HERMES_DISABLE_LAZY_INSTALLS=1", "HERMES_GATEWAY_LOCK_DIR="+lockDirectory)
}

func parentSessionVariable(key string) bool {
	return strings.HasPrefix(key, "HERMES_SESSION_") || slices.Contains([]string{
		"HERMES_HOME", "HERMES_DISABLE_LAZY_INSTALLS", "HERMES_GATEWAY_LOCK_DIR", "HERMES_GATEWAY_SESSION", "HERMES_INTERACTIVE", "HERMES_EXEC_ASK",
		"HERMES_CRON_SESSION", "HERMES_INFERENCE_PROVIDER", "HERMES_INFERENCE_MODEL", "HERMES_MODEL", "HERMES_PROFILE",
		"HERMES_SAFE_MODE", "HERMES_IGNORE_USER_CONFIG", "HERMES_IGNORE_RULES", "HERMES_YOLO_MODE", "HERMES_ACCEPT_HOOKS",
	}, key)
}

// verifyIsolated runs the bounded probes from a disposable home that borrows the
// configured installation's committed dependencies.
func verifyIsolated(ctx context.Context, deps adapter.Dependencies, configured domain.ClientBinding, data []byte) (_ adapter.Verification, result error) {
	home, cleanup, err := CreateVerificationHome(configured.Targets[0])
	if err != nil {
		return adapter.Verification{}, err
	}
	environment := verificationEnvironment(os.Environ(), home, GatewayLockDirectory(home))
	defer func() {
		releaseVerificationGateway(ctx, deps.Runner, process.Plan{Executable: configured.Executable, Directory: home, Env: environment, Args: []string{"gateway", "stop"}})
		result = errors.Join(result, cleanup())
	}()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), data, 0o600); err != nil {
		return adapter.Verification{}, fmt.Errorf("prepare isolated Hermes verification policy: %w", err)
	}
	probeCtx, cancel := context.WithTimeout(ctx, clientverification.ProtocolTimeout)
	defer cancel()
	probe := process.Plan{Executable: configured.Executable, Directory: home, Env: environment, Args: []string{"--version"}}
	version, diagnostic, err := deps.Runner.RunCaptureStreams(probeCtx, probe)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return adapter.Verification{}, errors.New("hermes version probe timed out")
		case errors.Is(err, context.Canceled):
			return adapter.Verification{}, errors.New("hermes version probe interrupted")
		default:
			return adapter.Verification{}, errors.New("hermes version probe failed")
		}
	}
	if process.DiagnosticFailure(diagnostic) {
		return adapter.Verification{}, clientverification.Diagnostic("Hermes", diagnostic)
	}
	probe.Args = []string{"chat", "--quiet", "--query-file", "-", "--oneshot", "--max-turns", "1", "--run-budget", "45", "--ignore-rules", "--source", "tool"}
	probe.Stdin = "Reply with exactly AIGW_OK."
	response, diagnostic, err := deps.Runner.RunCaptureStreams(probeCtx, probe)
	if err != nil {
		return adapter.Verification{}, errors.Join(errors.New("hermes inference failed; external credential diagnostics suppressed"), probeCtx.Err())
	}
	if process.DiagnosticFailure(diagnostic) {
		return adapter.Verification{}, clientverification.Diagnostic("Hermes", diagnostic)
	}
	if !strings.Contains(string(response), "AIGW_OK") {
		return adapter.Verification{}, errors.New("hermes model response did not return the expected AIGW_OK verification marker")
	}
	executable, err := os.ReadFile(configured.Executable)
	if err != nil {
		return adapter.Verification{}, err
	}
	return adapter.Verification{Version: strings.TrimSpace(string(version)), SHA256: fmt.Sprintf("%x", sha256.Sum256(executable)), Scope: "Native text response; tools and continuation are not probed"}, nil
}

// releaseVerificationGateway waits for the verification gateway to exit on
// idle and, when it outlives that bound, asks it once to stop through the
// native command, which refuses an active turn. It never signals the gateway;
// the home's remover decides deletion from GatewayAbsent's records. The stop
// outlives a canceled verification, so an interrupted run still releases its
// home within the bounds.
func releaseVerificationGateway(ctx context.Context, runner process.VerificationRunner, stop process.Plan) {
	if AwaitGatewayAbsence(stop.Directory, hermesGatewayIdleWait, hermesGatewayPoll) {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), clientverification.ProtocolTimeout)
	defer cancel()
	_, _, _ = runner.RunCaptureStreams(ctx, stop)
	AwaitGatewayAbsence(stop.Directory, hermesGatewayStopWait, hermesGatewayPoll)
}
