//go:build !windows

package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
)

// The go test child stays in this process's group, so that a signal to the
// group reaches it and it can stop what it started.
func TestSystemRunnerKeepsItsChildInThisProcessGroup(t *testing.T) {
	if os.Getenv("AIGW_COVERAGE_TEST_PROCESS_GROUP") == "1" {
		fmt.Printf("process group %d\n", syscall.Getpgrp())
		return
	}
	t.Setenv("AIGW_COVERAGE_TEST_PROCESS_GROUP", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := (systemRunner{}).Run(executable, []string{"-test.run=^TestSystemRunnerKeepsItsChildInThisProcessGroup$"}, &stdout, &stderr); err != nil {
		t.Fatalf("%v: %s", err, &stderr)
	}
	want := fmt.Sprintf("process group %d\n", syscall.Getpgrp())
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("child output = %q, want %q", &stdout, want)
	}
}
