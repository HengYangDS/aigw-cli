//go:build !windows

package hermes

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func writeGatewayState(t *testing.T, home, data string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(home, "gateway_state.json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGatewayAbsentWaitsForTheRecordedGatewayProcess(t *testing.T) {
	home := t.TempDir()
	if absent, err := GatewayAbsent(home); err != nil || !absent {
		t.Fatalf("home without a gateway: absent=%t err=%v", absent, err)
	}
	writeGatewayState(t, home, `{"pid": `+strconv.Itoa(os.Getpid())+`, "kind": "gateway"}`)
	if absent, err := GatewayAbsent(home); err != nil || absent {
		t.Fatalf("free lock with a running recorded gateway: absent=%t err=%v", absent, err)
	}
	exited := exec.Command(os.Args[0], "-test.run=^$")
	if err := exited.Run(); err != nil {
		t.Fatal(err)
	}
	writeGatewayState(t, home, `{"pid": `+strconv.Itoa(exited.Process.Pid)+`, "gateway_state": "stopped"}`)
	if absent, err := GatewayAbsent(home); err != nil || !absent {
		t.Fatalf("recorded gateway that exited: absent=%t err=%v", absent, err)
	}
	release := holdGatewayLock(t, home)
	if absent, err := GatewayAbsent(home); err != nil || absent {
		t.Fatalf("held lock after the recorded gateway exited: absent=%t err=%v", absent, err)
	}
	release()
	writeGatewayState(t, home, `{"pid":`)
	if absent, err := GatewayAbsent(home); err == nil || absent {
		t.Fatalf("unreadable gateway state: absent=%t err=%v", absent, err)
	}
}

// A gateway that a chat launched writes its standard-output log before it
// takes its lock or records its process. Until its lifecycle record says it
// exited, that home still has a gateway on the way, and removing the home
// would let the gateway re-create it.
func TestGatewayAbsentWaitsForALaunchedGatewayStillStarting(t *testing.T) {
	home := t.TempDir()
	logs := filepath.Join(home, "logs")
	if err := os.Mkdir(logs, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logs, "gateway-stdio.log"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if absent, err := GatewayAbsent(home); err != nil || absent {
		t.Fatalf("launched gateway without a lock or record: absent=%t err=%v", absent, err)
	}
	state := filepath.Join(home, "state")
	if err := os.Mkdir(state, 0o700); err != nil {
		t.Fatal(err)
	}
	lifecycle := filepath.Join(state, "gateway.lifecycle.json")
	running := `{"phase": "running", "pid": ` + strconv.Itoa(os.Getpid()) + `}`
	if err := os.WriteFile(lifecycle, []byte(running), 0o600); err != nil {
		t.Fatal(err)
	}
	if absent, err := GatewayAbsent(home); err != nil || absent {
		t.Fatalf("launched gateway recorded as running: absent=%t err=%v", absent, err)
	}
	// Hermes reads a running record whose process is gone as an unclean exit.
	died := exec.Command(os.Args[0], "-test.run=^$")
	if err := died.Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lifecycle, []byte(`{"phase": "running", "pid": `+strconv.Itoa(died.Process.Pid)+`}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if absent, err := GatewayAbsent(home); err != nil || !absent {
		t.Fatalf("launched gateway that died while running: absent=%t err=%v", absent, err)
	}
	if err := os.WriteFile(lifecycle, []byte(`{"phase":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if absent, err := GatewayAbsent(home); err == nil || absent {
		t.Fatalf("unreadable gateway lifecycle: absent=%t err=%v", absent, err)
	}
	if err := os.WriteFile(lifecycle, []byte(`{"phase": "exited", "pid": 1, "exit_reason": "graceful_shutdown"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if absent, err := GatewayAbsent(home); err != nil || !absent {
		t.Fatalf("launched gateway recorded as exited: absent=%t err=%v", absent, err)
	}
}
