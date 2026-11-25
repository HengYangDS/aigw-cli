package sync

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakePrograms is the private folder in which fakeProgram creates the runnable
// files that stand for clients' executables. A synchronization stops a
// client whose configured executable is missing, so a test whose client is
// written names an executable that runs.
var fakePrograms = sync.OnceValue(func() string {
	root, err := os.MkdirTemp("", "aigw-client-programs-")
	if err != nil {
		panic(err)
	}
	return root
})

// fakeProgram returns a runnable file that stands for the executable at path, in
// a folder of fakePrograms named after path's folder, so that two paths of a
// test name two files with the same base names.
func fakeProgram(path string) string {
	folder := filepath.Join(fakePrograms(), strings.ReplaceAll(strings.Trim(filepath.Dir(path), "/"), "/", "_"))
	full := filepath.Join(folder, filepath.Base(path))
	if _, err := os.Stat(full); err == nil {
		return full
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(full, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		panic(err)
	}
	return full
}

func TestMain(m *testing.M) {
	code := m.Run()
	_ = os.RemoveAll(fakePrograms())
	os.Exit(code)
}
