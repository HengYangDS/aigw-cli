//go:build !windows

package client

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"aigw-cli/internal/process"
)

// gatewayLockHolder stands in for the detached gateway that a Hermes chat
// starts: it holds the per-home gateway lock until released.
type gatewayLockHolder struct {
	mu       sync.Mutex
	lock     *os.File
	released time.Time
}

func (holder *gatewayLockHolder) take(t *testing.T, home string) {
	t.Helper()
	lock, err := os.OpenFile(filepath.Join(home, "gateway.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	holder.mu.Lock()
	holder.lock = lock
	holder.mu.Unlock()
	t.Cleanup(holder.release)
}

func (holder *gatewayLockHolder) release() {
	holder.mu.Lock()
	defer holder.mu.Unlock()
	if holder.lock != nil {
		_ = holder.lock.Close()
		holder.lock = nil
		holder.released = time.Now()
	}
}

func shortenHermesGatewayWaits(t *testing.T, idle, stop time.Duration) {
	t.Helper()
	previousIdle, previousStop, previousPoll := hermesGatewayIdleWait, hermesGatewayStopWait, hermesGatewayPoll
	hermesGatewayIdleWait, hermesGatewayStopWait, hermesGatewayPoll = idle, stop, 20*time.Millisecond
	t.Cleanup(func() {
		hermesGatewayIdleWait, hermesGatewayStopWait, hermesGatewayPoll = previousIdle, previousStop, previousPoll
	})
}

func TestHermesVerificationDeletesItsHomeOnlyAfterItsGatewayExits(t *testing.T) {
	cfg, selected, deps := hermesVerificationFixture(t)
	shortenHermesGatewayWaits(t, 5*time.Second, time.Second)
	holder := &gatewayLockHolder{}
	runner := &captureAdapterRunner{outputs: [][]byte{[]byte("Hermes Agent v1\n"), []byte("AIGW_OK\n")}}
	runner.observe = func(plan process.Plan) {
		if plan.Args[0] == "chat" {
			holder.take(t, plan.Directory)
			time.AfterFunc(300*time.Millisecond, holder.release)
		}
	}
	deps.Runner = runner
	if _, err := (hermesAdapter{}).Verify(t.Context(), deps, cfg, selected, ""); err != nil {
		t.Fatal(err)
	}
	finished := time.Now()
	home := runner.plans[1].Directory
	if _, err := os.Lstat(home); !os.IsNotExist(err) {
		t.Fatalf("verification home survived after its gateway exited: %v", err)
	}
	holder.mu.Lock()
	released := holder.released
	holder.mu.Unlock()
	if released.IsZero() || released.After(finished) {
		t.Fatal("verification returned before its gateway released the home")
	}
	if len(runner.plans) != 2 {
		t.Fatalf("an idle gateway was stopped natively: %#v", runner.plans[2:])
	}
}

func TestHermesVerificationWaitsForItsGatewayProcessAfterTheLock(t *testing.T) {
	cfg, selected, deps := hermesVerificationFixture(t)
	shortenHermesGatewayWaits(t, 5*time.Second, time.Second)
	holder := &gatewayLockHolder{}
	// The gateway's exit tail: its process outlives the lock it already released.
	tail := exec.Command("sleep", "1")
	exited := make(chan time.Time, 1)
	runner := &captureAdapterRunner{outputs: [][]byte{[]byte("Hermes Agent v1\n"), []byte("AIGW_OK\n")}}
	runner.observe = func(plan process.Plan) {
		if plan.Args[0] != "chat" {
			return
		}
		holder.take(t, plan.Directory)
		if err := tail.Start(); err != nil {
			t.Fatal(err)
		}
		go func() {
			_ = tail.Wait()
			exited <- time.Now()
		}()
		state := fmt.Sprintf(`{"pid": %d, "kind": "gateway", "gateway_state": "running"}`, tail.Process.Pid)
		if err := os.WriteFile(filepath.Join(plan.Directory, "gateway_state.json"), []byte(state), 0o600); err != nil {
			t.Fatal(err)
		}
		time.AfterFunc(100*time.Millisecond, holder.release)
	}
	deps.Runner = runner
	if _, err := (hermesAdapter{}).Verify(t.Context(), deps, cfg, selected, ""); err != nil {
		t.Fatal(err)
	}
	finished := time.Now()
	if processExit := <-exited; processExit.After(finished) {
		t.Fatal("verification deleted its home while the gateway process still ran")
	}
	if _, err := os.Lstat(runner.plans[1].Directory); !os.IsNotExist(err) {
		t.Fatalf("verification home survived after its gateway process exited: %v", err)
	}
}

func TestHermesVerificationStopsItsGatewayOnceAfterTheIdleBound(t *testing.T) {
	cfg, selected, deps := hermesVerificationFixture(t)
	shortenHermesGatewayWaits(t, 200*time.Millisecond, 5*time.Second)
	holder := &gatewayLockHolder{}
	runner := &captureAdapterRunner{outputs: [][]byte{[]byte("Hermes Agent v1\n"), []byte("AIGW_OK\n")}}
	runner.observe = func(plan process.Plan) {
		switch {
		case plan.Args[0] == "chat":
			holder.take(t, plan.Directory)
		case slices.Equal(plan.Args, []string{"gateway", "stop"}):
			holder.release()
		}
	}
	deps.Runner = runner
	if _, err := (hermesAdapter{}).Verify(t.Context(), deps, cfg, selected, ""); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 3 {
		t.Fatalf("verification plans = %#v, want version, chat and one native stop", runner.plans)
	}
	home, stop := runner.plans[1].Directory, runner.plans[2]
	if stop.Stdin != "" || stop.Executable != runner.plans[1].Executable || stop.Directory != home ||
		!slices.Contains(stop.Env, "HERMES_HOME="+home) || !slices.Equal(stop.Env, runner.plans[1].Env) {
		t.Fatalf("native gateway stop = %#v, want closed input in the verification environment", stop)
	}
	if !runner.deadlines[2] {
		t.Fatal("native gateway stop ran without a deadline")
	}
	if _, err := os.Lstat(home); !os.IsNotExist(err) {
		t.Fatalf("verification home survived after its gateway stopped: %v", err)
	}
}

func TestHermesVerificationRetainsAHomeItsGatewayStillHolds(t *testing.T) {
	cfg, selected, deps := hermesVerificationFixture(t)
	shortenHermesGatewayWaits(t, 100*time.Millisecond, 100*time.Millisecond)
	holder := &gatewayLockHolder{}
	runner := &captureAdapterRunner{outputs: [][]byte{[]byte("Hermes Agent v1\n"), []byte("AIGW_OK\n")}}
	runner.observe = func(plan process.Plan) {
		if plan.Args[0] == "chat" {
			holder.take(t, plan.Directory)
		}
	}
	deps.Runner = runner
	_, err := (hermesAdapter{}).Verify(t.Context(), deps, cfg, selected, "")
	home := runner.plans[1].Directory
	t.Cleanup(func() {
		holder.release()
		_ = os.RemoveAll(home)
	})
	if err == nil || !strings.Contains(err.Error(), home) {
		t.Fatalf("verification under a live gateway = %v, want an error naming the retained home", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, "config.yaml")); statErr != nil {
		t.Fatalf("verification deleted state under a live gateway: %v", statErr)
	}
}
