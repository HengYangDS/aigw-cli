package synchronization

import (
	"os"
	"path/filepath"
	"testing"
)

// clientProgram is a runnable file that stands for a client's configured
// program, which a synchronization requires: it stops a client whose
// configured program is missing.
var clientProgram string

func TestMain(m *testing.M) {
	directory, err := os.MkdirTemp("", "aigw-synchronization-programs-")
	if err != nil {
		panic(err)
	}
	clientProgram = filepath.Join(directory, "client")
	if err := os.WriteFile(clientProgram, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(directory)
	os.Exit(code)
}
