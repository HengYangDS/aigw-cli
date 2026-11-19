package process

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunnerReportsCreationBeforeTheStartObservation(t *testing.T) {
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	plan := Plan{Executable: program, Args: []string{"-test.run=^$"}, Env: append(os.Environ(), "GORACE=atexit_sleep_ms=0"),
		OnCreated: func() { order = append(order, "created") },
		OnStart: func(*os.Process) error {
			order = append(order, "started")
			return nil
		}}
	if _, err := (Runner{}).RunCapture(t.Context(), plan); err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, " ") != "created started" {
		t.Fatalf("observations = %v, want the creation, then the start", order)
	}
	// A child that cannot be created is never observed.
	order = nil
	plan.Executable = filepath.Join(t.TempDir(), "absent")
	if _, err := (Runner{}).RunCapture(t.Context(), plan); err == nil || len(order) != 0 {
		t.Fatalf("an absent program: error=%v observations=%v", err, order)
	}
}
