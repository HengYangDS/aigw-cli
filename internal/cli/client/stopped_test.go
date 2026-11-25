package client

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
)

// The repair command refuses, before anything is read or written, an
// --executable that is not a file that can run: a path that is not there, a
// folder, and a file without an execute bit. Nothing is run.
func TestAnExecutableThatCannotRunIsRefused(t *testing.T) {
	plain := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(plain, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, executable := range []string{filepath.Join(t.TempDir(), "gone"), t.TempDir(), plain} {
		for _, args := range [][]string{{"enable", claude.ID, "--executable", executable}, {"edit", claude.ID, "--executable", executable}} {
			runtime, _, secretStore, _ := adapterRuntime(t, adapterConfig())
			if err := secretStore.Set("gateway", "token"); err != nil {
				t.Fatal(err)
			}
			before, err := runtime.Config.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			if err := executeAdapter(t, runtime, args...); err == nil || !strings.Contains(err.Error(), "is not a file that can run") {
				t.Fatalf("%v = %v, want the executable refused", args, err)
			}
			if after, err := runtime.Config.CaptureSnapshot(); err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("%v changed the configuration: %v", args, err)
			}
		}
	}
}
