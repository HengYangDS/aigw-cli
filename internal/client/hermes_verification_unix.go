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

func (hermesAdapter) Verify(ctx context.Context, deps Dependencies, cfg configuration.Config, selected configuration.Runtime, _ string) (_ Verification, result error) {
	if deps.Runner == nil {
		return Verification{}, errors.New("hermes verification requires a process runner")
	}
	configured := cfg.Clients[configuration.ClientHermes]
	if !configured.Enabled {
		return Verification{}, errors.New("hermes adapter is disabled; run aigw sync")
	}
	if len(configured.Targets) != 1 {
		return Verification{}, errors.New("hermes verification requires one native configuration home; run aigw sync")
	}
	home, cleanup, err := hermesconfig.CreateVerificationHome(configured.Targets[0])
	if err != nil {
		return Verification{}, err
	}
	defer func() { result = errors.Join(result, cleanup()) }()
	verificationConfig := filepath.Join(home, "config.yaml")
	policy := "security:\n  allow_lazy_installs: false\nupdates:\n  check: false\ncli:\n  expose_on_path: false\nplugins:\n  enabled: []\nmemory:\n  enabled: false\n"
	if err := os.WriteFile(verificationConfig, []byte(policy), 0o600); err != nil {
		return Verification{}, fmt.Errorf("prepare isolated Hermes verification policy: %w", err)
	}
	desired, err := hermesDesired(deps, cfg, selected)
	if err != nil {
		return Verification{}, err
	}
	plan, err := hermesconfig.Prepare(verificationConfig, &desired)
	if err != nil {
		return Verification{}, err
	}
	if _, err := plan.Apply(); err != nil {
		return Verification{}, err
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
	version, err := deps.Runner.RunCapture(probeCtx, probe)
	if err != nil {
		return Verification{}, errors.New("hermes executable identity could not be observed")
	}
	probe.Args = []string{"chat", "--quiet", "--query-file", "-", "--oneshot", "--max-turns", "1", "--run-budget", "45", "--ignore-rules", "--source", "tool"}
	probe.Stdin = "Reply with exactly AIGW_OK."
	response, err := deps.Runner.RunCapture(probeCtx, probe)
	if err != nil {
		return Verification{}, errors.Join(errors.New("hermes inference failed; external credential diagnostics suppressed"), probeCtx.Err())
	}
	if !strings.Contains(string(response), "AIGW_OK") {
		return Verification{}, errors.New("hermes model response did not return the expected AIGW_OK verification marker")
	}
	executable, err := os.ReadFile(configured.Executable)
	if err != nil {
		return Verification{}, err
	}
	return Verification{Version: strings.TrimSpace(string(version)), SHA256: fmt.Sprintf("%x", sha256.Sum256(executable))}, nil
}
