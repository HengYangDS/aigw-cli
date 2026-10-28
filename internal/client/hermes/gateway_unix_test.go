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
