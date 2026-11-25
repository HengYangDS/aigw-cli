// Package processtest writes the programs that tests start, safely while the
// other tests of the same process start processes.
package processtest

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
)

// Failing is a program that runs and exits with status 1: an executable that
// a test names and never needs to answer.
const Failing = "#!/bin/sh\nexit 1\n"

// Write writes program at path with an execute bit. The tests run in
// parallel, so another test may start a process while this one writes: on
// Unix the child holds every descriptor of this process until it runs its own
// program, and Linux refuses to start a file that is open for writing
// (ETXTBSY, https://go.dev/issue/22315). The file is written under
// [syscall.ForkLock], which every fork of this process holds for writing.
func Write(t testing.TB, path, program string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	if err := os.WriteFile(path, []byte(program), 0o755); err != nil {
		t.Fatal(err)
	}
}

// folders holds the folder of each running test's programs.
var folders sync.Map

// Program returns a runnable file, Failing, that stands for the executable at
// path, in a folder of t's own named after path's folder: one file for each
// test and path, so that a test that names a path twice names one file.
func Program(t testing.TB, path string) string {
	t.Helper()
	folder, known := "", false
	if stored, loaded := folders.Load(t); loaded {
		folder, known = stored.(string)
	}
	if !known {
		folder = t.TempDir()
		folders.Store(t, folder)
		t.Cleanup(func() { folders.Delete(t) })
	}
	full := filepath.Join(folder, strings.ReplaceAll(strings.Trim(filepath.Dir(path), "/"), "/", "_"), filepath.Base(path))
	if _, err := os.Stat(full); err != nil {
		Write(t, full, Failing)
	}
	return full
}
