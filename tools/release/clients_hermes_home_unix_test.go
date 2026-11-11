//go:build client_acceptance && !windows

package main

import (
	"context"
	"path/filepath"
	"time"

	"aigw-cli/internal/adapter/hermes"
	clientverification "aigw-cli/internal/adapter/verification"
	"aigw-cli/internal/platform/process"
)

// isolateHermesHome gives the journey a Hermes home of its own, beside the
// supplied installation, whose gateway it stops when the journey ends.
func (j *journeyFixture) isolateHermesHome(executable string) error {
	installation, err := requiredClientInput("AIGW_ACCEPTANCE_HERMES_HOME", true)
	if err != nil {
		return err
	}
	profile, cleanup, err := hermes.CreateVerificationHome(filepath.Join(installation, "config.yaml"))
	if err != nil {
		return err
	}
	t := j.testing
	t.Cleanup(func() {
		j.stopHermesGateway(executable, profile)
		if err := cleanup(); err != nil {
			t.Errorf("remove native Hermes acceptance profile: %v", err)
		}
	})
	j.setEnvironment("HERMES_HOME", profile)
	j.setEnvironment("HERMES_GATEWAY_LOCK_DIR", hermes.GatewayLockDirectory(profile))
	// As in AIGW's own Hermes verification, the journey uses the supplied
	// installation's dependencies and never installs one on demand.
	j.setEnvironment("HERMES_DISABLE_LAZY_INSTALLS", "1")
	return nil
}

// stopHermesGateway stops the gateway that the journey's chats started in its
// Hermes home through the native command, which refuses an active turn, and
// waits until the gateway releases the home's lock.
func (j *journeyFixture) stopHermesGateway(executable, home string) {
	if hermes.AwaitGatewayAbsence(home, 0, 0) {
		return
	}
	environment := environmentWith(j.environment, map[string]string{
		"HERMES_HOME": home, "HERMES_GATEWAY_LOCK_DIR": hermes.GatewayLockDirectory(home),
	})
	ctx, cancel := context.WithTimeout(context.Background(), clientverification.ProtocolTimeout)
	defer cancel()
	_, _, _ = (process.Runner{}).RunCaptureStreams(ctx, process.Plan{Executable: executable, Directory: home, Env: environment, Args: []string{"gateway", "stop"}})
	hermes.AwaitGatewayAbsence(home, 15*time.Second, 250*time.Millisecond)
}
