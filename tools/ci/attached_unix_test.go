//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// An attached command's child stays in this process's group, so that a signal
// to the group reaches it and it can stop what it started.
func TestAttachedCommandKeepsItsChildInThisProcessGroup(t *testing.T) {
	if os.Getenv("AIGW_CI_TEST_PROCESS_GROUP") == "1" {
		fmt.Printf("process group %d\n", syscall.Getpgrp())
		return
	}
	output, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = output.Close() })
	previous := os.Stdout
	os.Stdout = output
	err = runAttached(command{
		Name: os.Args[0],
		Args: []string{"-test.run=^TestAttachedCommandKeepsItsChildInThisProcessGroup$"},
		Env:  []string{"AIGW_CI_TEST_PROCESS_GROUP=1"},
	})
	os.Stdout = previous
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("process group %d\n", syscall.Getpgrp())
	if got := string(readFile(t, output.Name())); !strings.Contains(got, want) {
		t.Fatalf("attached child output = %q, want %q", got, want)
	}
}
