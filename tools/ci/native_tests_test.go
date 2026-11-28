package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/tools/release/construction"
)

// The native test attempt runs once, in native acceptance's isolated
// environment, with the reports its caller names, and leaves no workspace.
func TestNativeTestsRunTheHostAttemptInItsIsolatedEnvironment(t *testing.T) {
	junit, lcov := filepath.Join("reports", "go-junit.xml"), filepath.Join("reports", "go.lcov")
	var calls []command
	var workspace string
	runner := func(call command) error {
		calls = append(calls, call)
		for _, entry := range call.Env {
			if value, found := strings.CutPrefix(entry, "TMPDIR="); found {
				workspace = value
			}
		}
		if info, err := os.Stat(workspace); err != nil || !info.IsDir() {
			t.Errorf("the attempt's workspace %q is missing while it runs: %v", workspace, err)
		}
		return nil
	}
	if err := run([]string{"native-tests", "--junit-output", junit, "--lcov-output", lcov}, &bytes.Buffer{}, runner); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("native-tests ran %d commands, want one attempt: %#v", len(calls), calls)
	}
	want := nativeTestCommand(runtime.GOOS, junit, lcov)
	if calls[0].Name != want.Name || !slices.Equal(calls[0].Args, want.Args) {
		t.Fatalf("attempt = %#v, want %#v", calls[0], want)
	}
	if !slices.Equal(calls[0].Env, construction.NativeTestEnvironment(workspace, "", "")) {
		t.Fatalf("attempt environment = %q, want native acceptance's isolated environment", calls[0].Env)
	}
	if _, err := os.Stat(workspace); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the attempt's workspace %q survives it: %v", workspace, err)
	}
}

// The caller's report paths follow the coverage tool's own flags; without
// them the JUnit report stays with the coverage evidence and no LCOV is written.
func TestNativeTestCommandWritesTheNamedReports(t *testing.T) {
	for _, platform := range []string{"darwin", "linux"} {
		named := nativeTestCommand(platform, "junit.xml", "go.lcov")
		if !followedBy(named.Args, "--junit-output", "junit.xml") || !followedBy(named.Args, "--lcov-output", "go.lcov") ||
			!slices.Contains(named.Args, "--tags=native_resource_acceptance") || !slices.Contains(named.Args, "--race") {
			t.Errorf("%s attempt does not write the named reports of a native race attempt: %q", platform, named.Args)
		}
		defaulted := nativeTestCommand(platform, "", "")
		if slices.Contains(defaulted.Args, "--lcov-output") ||
			!followedBy(defaulted.Args, "--junit-output", filepath.Join("build", "verification", "coverage", "junit.xml")) {
			t.Errorf("%s default attempt = %q", platform, defaulted.Args)
		}
	}
}

func TestNativeTestsRefuseOtherArgumentsAndReportFailures(t *testing.T) {
	refuse := func(call command) error {
		t.Errorf("refused invocation ran %#v", call)
		return nil
	}
	for _, args := range [][]string{{"extra"}, {"--platform", "linux"}, {"--junit-output"}} {
		err := run(append([]string{"native-tests"}, args...), &bytes.Buffer{}, refuse)
		if err == nil || err.Error() != nativeTestsUsage {
			t.Errorf("native-tests %q = %v, want its usage", args, err)
		}
	}
	failure := errors.New("attempt failed")
	var workspace string
	err := run([]string{"native-tests"}, &bytes.Buffer{}, func(call command) error {
		for _, entry := range call.Env {
			if value, found := strings.CutPrefix(entry, "TMPDIR="); found {
				workspace = value
			}
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("a failed attempt reported %v", err)
	}
	if _, statErr := os.Stat(workspace); workspace == "" || !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("a failed attempt left its workspace %q: %v", workspace, statErr)
	}
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if err := run([]string{"native-tests"}, &bytes.Buffer{}, refuse); err == nil {
		t.Fatal("native-tests ran without a workspace")
	}
}

func followedBy(arguments []string, name, value string) bool {
	at := slices.Index(arguments, name)
	return at >= 0 && at+1 < len(arguments) && arguments[at+1] == value
}
