//go:build !windows

package performance

import (
	"context"
	"path/filepath"
	"testing"
)

func TestUnixExecutionObservationPreservesNativeTiming(t *testing.T) {
	rows, err := ObserveCurrent()
	if err != nil || rows != nil {
		t.Fatalf("Unix controller acquired Windows-only observation: rows=%v error=%v", rows, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	// Unix measurements must not execute a second workload or require a debugger.
	// The nonexistent command distinguishes observation from native timing.
	command := filepath.Join(t.TempDir(), "not-an-extra-unix-workload")
	rows, controllers, err := ObserveCommand(ctx, Command{Tool: command}, Workload{Command: []string{command}})
	if err != nil || rows != nil || controllers != nil {
		t.Fatalf("Unix observation changed the timed workload envelope: rows=%v controllers=%v error=%v", rows, controllers, err)
	}
}
